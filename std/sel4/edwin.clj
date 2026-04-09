;; Edwin on seL4 — MIT Scheme Editor as Capability-Secured Protection Domain
;;
;; Edwin is an Emacs v18 clone written entirely in MIT/GNU Scheme.
;; On seL4, Edwin becomes the natural editor: Scheme for extension,
;; small enough to audit, capability-gated buffer access.
;;
;; Architecture:
;;
;;   ┌────────────────────────────────────────────────────────┐
;;   │  Edwin (MIT Scheme runtime)                            │
;;   │  ┌──────────┐ ┌──────────┐ ┌────────────────────────┐ │
;;   │  │ Buffers  │ │ Keymaps  │ │ Scheme REPL            │ │
;;   │  │ (shared  │ │ (cap-    │ │ (eval = capability     │ │
;;   │  │  memory) │ │  gated)  │ │  invocation)           │ │
;;   │  └──────────┘ └──────────┘ └────────────────────────┘ │
;;   │     Protection Domain: edwin-pd                        │
;;   └──────────────┬──────────────────────────┬──────────────┘
;;                  │ seL4 IPC                  │ seL4 IPC
;;                  ↓                           ↓
;;   ┌──────────────────────┐    ┌──────────────────────────┐
;;   │ Window Manager       │    │ File Server              │
;;   │ (sel4.window)        │    │ (capability-gated FS)    │
;;   │ XMonad tiling        │    │ read/write per-cap       │
;;   │ Protection Domain:   │    │ Protection Domain:       │
;;   │   wm-pd              │    │   fs-pd                  │
;;   └──────────────────────┘    └──────────────────────────┘
;;
;; Key insight: Edwin's eval IS capability invocation.
;; (find-file "/path") requires a read capability for that path.
;; (save-buffer) requires a write capability.
;; (shell-command) requires an exec capability.
;; The seL4 capability model makes this EXPLICIT rather than implicit.

(ns sel4.edwin
  "Edwin/MIT-Scheme integration for seL4 protection domains."
  (:require [sel4.common :as common]
            [sel4.ipc :as ipc]
            [sel4.capability :as cap]
            [sel4.balance :as bal]))

;; ============================================================================
;; Edwin Protection Domain — the MIT Scheme runtime as seL4 PD
;; ============================================================================

(defrecord EdwinPD
  [name           ;; "edwin-pd"
   thread         ;; seL4 TCB for the Scheme runtime
   cspace         ;; CSpace with Edwin's capabilities
   vspace         ;; VSpace (address space) for MIT Scheme heap
   endpoints      ;; IPC endpoints Edwin can talk to
   buffers        ;; Shared memory regions for buffer contents
   capabilities   ;; What Edwin is allowed to do
   ])

(defn make-edwin-pd
  "Create an Edwin protection domain.
   Allocates: TCB, CSpace, VSpace, IPC buffer, fault handler.
   Edwin starts with minimal capabilities — must be granted more."
  [untyped priority]
  (let [env (common/make-vm-environment "edwin-pd" priority untyped)]
    (map->EdwinPD
      {:name "edwin-pd"
       :thread (:thread env)
       :cspace (:cnode env)
       :vspace (:vspace-root env)
       :endpoints (atom {})
       :buffers (atom {})
       :capabilities (atom #{})})))

;; ============================================================================
;; Buffer Management — Edwin buffers as seL4 shared memory
;; ============================================================================

(defrecord EdwinBuffer
  [name           ;; Buffer name (e.g., "*scheme*", "foo.scm")
   region         ;; SharedRegion from common.clj
   read-cap       ;; Capability: can read buffer contents
   write-cap      ;; Capability: can modify buffer contents
   mode           ;; :scheme :text :read-only :special
   modified?      ;; Has unsaved changes
   ])

(defn make-buffer
  "Create an Edwin buffer backed by seL4 shared memory.
   Returns buffer with read+write capabilities."
  [name size-bytes mode]
  (let [region (common/make-shared-region (str "edwin:" name) size-bytes)
        read-cap (cap/make-capability (str "buf-read:" name)
                   :rights 0x01)   ;; Read only
        write-cap (cap/make-capability (str "buf-write:" name)
                    :rights 0x03)] ;; Read + Write
    (->EdwinBuffer name region read-cap write-cap mode false)))

(defn grant-buffer-access
  "Grant another protection domain access to an Edwin buffer.
   Attenuates: can grant read-only or read-write."
  [buffer target-endpoint access-level]
  (case access-level
    :read-only
    (do (common/map-region (:region buffer) target-endpoint 0)
        (cap/delegate (:read-cap buffer) target-endpoint))

    :read-write
    (do (common/map-region (:region buffer) target-endpoint 0)
        (cap/delegate (:write-cap buffer) target-endpoint))))

(defn revoke-buffer-access
  "Revoke a protection domain's access to a buffer."
  [buffer target-endpoint]
  (cap/revoke-capability (:read-cap buffer))
  (cap/revoke-capability (:write-cap buffer)))

;; ============================================================================
;; Edwin <-> Window Manager IPC
;; ============================================================================

(def wm-endpoint "sel4:xmonad-wm")

(defn edwin-request-frame
  "Edwin requests a display frame from the window manager.
   Returns frame dimensions and framebuffer capability."
  [edwin-pd]
  (ipc/send-recv wm-endpoint
    (ipc/->Message wm-endpoint :query-layout 0
                   ["edwin-pd"]
                   0 [])))

(defn edwin-set-frame-title
  "Set the window title (modeline equivalent)."
  [title]
  (ipc/send-message wm-endpoint
    (ipc/->Message wm-endpoint :move-window 0
                   ["edwin-pd" :set-title title]
                   0 [])))

(defn edwin-split-frame
  "Split Edwin frame (like C-x 2 / C-x 3).
   Requests window manager to tile Edwin's region."
  [direction]
  (ipc/send-message wm-endpoint
    (ipc/->Message wm-endpoint :move-window 0
                   ["edwin-pd" :split direction]
                   0 [])))

;; ============================================================================
;; Edwin <-> File Server IPC
;; ============================================================================

(def fs-endpoint "sel4:file-server")

(defn edwin-find-file
  "find-file equivalent: request file from file server.
   Requires: read capability for the file path.
   File server checks capability before serving content."
  [path]
  (let [read-cap (cap/make-capability (str "file-read:" path)
                   :endpoint fs-endpoint
                   :rights 0x01)]
    (ipc/send-recv fs-endpoint
      (ipc/->Message fs-endpoint :query-layout 0  ;; overloaded: file-read
                     [:read-file path]
                     0 [read-cap]))))

(defn edwin-save-buffer
  "save-buffer equivalent: write buffer contents to file server.
   Requires: write capability for the file path."
  [buffer path]
  (let [write-cap (cap/make-capability (str "file-write:" path)
                    :endpoint fs-endpoint
                    :rights 0x03)]
    (ipc/send-recv fs-endpoint
      (ipc/->Message fs-endpoint :move-window 0  ;; overloaded: file-write
                     [:write-file path (:name buffer)]
                     0 [write-cap]))))

;; ============================================================================
;; Capability-Gated Eval — Scheme evaluation as capability invocation
;; ============================================================================

(def eval-capabilities
  "What each Edwin operation requires in terms of seL4 capabilities."
  {;; File operations
   :find-file        {:needs [:file-read-cap]
                      :trit 0
                      :doc "Read file into buffer. Needs read cap for path."}
   :save-buffer      {:needs [:file-write-cap]
                      :trit 0
                      :doc "Write buffer to file. Needs write cap for path."}
   :write-file       {:needs [:file-write-cap]
                      :trit 0
                      :doc "Write buffer to named file. Needs write cap."}

   ;; Process operations
   :shell-command    {:needs [:exec-cap :process-creator-cap]
                      :trit 1
                      :doc "Run shell command. Needs exec cap + process creation."}
   :compile          {:needs [:exec-cap :file-read-cap]
                      :trit 1
                      :doc "Compile buffer. Needs exec + read caps."}
   :eval-expression  {:needs [:scheme-eval-cap]
                      :trit 0
                      :doc "Evaluate Scheme expression. Sandboxed by default."}

   ;; Network operations
   :url-retrieve     {:needs [:network-cap]
                      :trit 0
                      :doc "Fetch URL. Needs network capability."}
   :irc-connect      {:needs [:network-cap :endpoint-cap]
                      :trit 1
                      :doc "Connect to IRC. Needs network + endpoint."}

   ;; Buffer operations (always granted to Edwin PD)
   :switch-buffer    {:needs []
                      :trit 0
                      :doc "Switch to another buffer. No extra cap needed."}
   :kill-buffer      {:needs []
                      :trit -1
                      :doc "Kill buffer. Revokes shared memory mappings."}

   ;; Window operations
   :split-window     {:needs [:wm-cap]
                      :trit 0
                      :doc "Split window. Needs window manager capability."}
   :delete-window    {:needs [:wm-cap]
                      :trit -1
                      :doc "Delete window. Needs window manager capability."}})

(defn check-eval-capability
  "Before Edwin evaluates a Scheme form, check if the required
   capabilities are present. This is the seL4 equivalent of
   Emacs's 'command-execute with permission checking.

   Returns {:allowed true} or {:allowed false :missing [caps]}."
  [edwin-pd operation]
  (let [required (get-in eval-capabilities [operation :needs] [])
        held @(:capabilities edwin-pd)
        missing (remove #(contains? held %) required)]
    (if (empty? missing)
      {:allowed true :operation operation}
      {:allowed false :operation operation :missing (vec missing)})))

;; ============================================================================
;; Edwin Scheme Mode — seL4-aware extensions
;; ============================================================================

(def edwin-scheme-extensions
  "Scheme procedures available in Edwin on seL4.
   These extend standard Edwin with seL4 capability awareness."
  {:sel4-caps
   {:doc "List capabilities held by Edwin."
    :scheme "(sel4-caps) => ((file-read \"/home\") (wm) (scheme-eval))"
    :trit -1}

   :sel4-grant
   {:doc "Grant a capability to another protection domain."
    :scheme "(sel4-grant 'file-read \"/home\" \"compiler-pd\")"
    :trit -1}

   :sel4-revoke
   {:doc "Revoke a previously granted capability."
    :scheme "(sel4-revoke 'file-read \"/home\" \"compiler-pd\")"
    :trit -1}

   :sel4-ipc-send
   {:doc "Send an IPC message to an endpoint."
    :scheme "(sel4-ipc-send \"sel4:compiler\" '(compile \"foo.scm\"))"
    :trit 0}

   :sel4-balance
   {:doc "Check GF(3) balance of recent operations."
    :scheme "(sel4-balance) => {:sum 0 :balanced? true}"
    :trit -1}

   :sel4-fault-handler
   {:doc "Register a Scheme procedure as fault handler."
    :scheme "(sel4-fault-handler 'vm-fault (lambda (addr ip) ...))"
    :trit -1}

   :sel4-notification-wait
   {:doc "Wait on a notification object (blocks Edwin thread)."
    :scheme "(sel4-notification-wait 'my-ntfn) => badge"
    :trit 0}

   :sel4-shared-read
   {:doc "Read from shared memory region."
    :scheme "(sel4-shared-read \"compiler-output\" 0 1024)"
    :trit 0}

   :sel4-shared-write
   {:doc "Write to shared memory region."
    :scheme "(sel4-shared-write \"input-buffer\" 0 data)"
    :trit 0}})

;; ============================================================================
;; SICP on seL4 — pedagogical bridge
;; ============================================================================

(def sicp-capability-mapping
  "SICP concepts mapped to seL4 primitives.
   Edwin is the natural teaching environment for this."
  {"Environment model"     "CSpace (capability space) — each environment frame
                            is a CNode with slots for bindings"
   "define"                "seL4_Untyped_Retype — creating a new object"
   "set!"                  "Requires write capability to the environment frame"
   "lambda"                "Creates a new protection domain (lightweight thread)"
   "apply"                 "seL4_Call — synchronous IPC to the closure's PD"
   "continuation"          "seL4_ReplyRecv — the reply capability IS the continuation"
   "streams (lazy)"        "seL4 notification — signal when next element available"
   "metacircular eval"     "CAmkES assembly — the evaluator IS described as data
                            (sel4-provider.edn)"
   "register machine"      "TCB (Thread Control Block) — registers, PC, SP"
   "garbage collection"    "seL4_Untyped_Retype reclaims revoked capabilities"})

;; ============================================================================
;; Bootstrap sequence
;; ============================================================================

(defn bootstrap-edwin
  "Boot Edwin on seL4. Sequence:
   1. Create protection domain (TCB + CSpace + VSpace)
   2. Load MIT Scheme runtime into VSpace
   3. Create *scheme* buffer (shared memory)
   4. Connect to window manager (IPC endpoint)
   5. Connect to file server (IPC endpoint)
   6. Grant minimal capabilities
   7. Start REPL"
  [untyped priority]
  (let [;; 1. Create protection domain
        edwin (make-edwin-pd untyped priority)

        ;; 2. MIT Scheme runtime loaded (simulated)
        _ (println "Loading MIT Scheme 12.1 runtime into edwin-pd VSpace...")

        ;; 3. Create initial buffer
        scheme-buf (make-buffer "*scheme*" (* 64 1024) :scheme)
        _ (swap! (:buffers edwin) assoc "*scheme*" scheme-buf)

        ;; 4. Connect to window manager
        _ (swap! (:endpoints edwin) assoc :wm wm-endpoint)

        ;; 5. Connect to file server
        _ (swap! (:endpoints edwin) assoc :fs fs-endpoint)

        ;; 6. Grant minimal capabilities
        _ (swap! (:capabilities edwin) conj
                 :scheme-eval-cap  ;; Can evaluate Scheme
                 :wm-cap           ;; Can talk to window manager
                 :file-read-cap)   ;; Can read files (not write!)

        ;; 7. Edwin is ready
        _ (println "Edwin ready. Capabilities: " @(:capabilities edwin))
        _ (println "Buffers: " (keys @(:buffers edwin)))
        _ (println "Endpoints: " (keys @(:endpoints edwin)))]

    edwin))

(defn edwin-status
  "Report Edwin protection domain status."
  [edwin]
  {:name (:name edwin)
   :buffers (count @(:buffers edwin))
   :capabilities (count @(:capabilities edwin))
   :endpoints (keys @(:endpoints edwin))
   :trit bal/TRIT_ZERO})
