;; vz/bridge.cljs — Squint/SCI bridge for boxxy provider
;;
;; This file is the metacircular evaluator's self-description:
;; it reads provider.edn and becomes the provider.
;;
;; Two hops from identical:
;;   1. provider.edn (data, normal form)
;;   2. bridge.cljs  (evaluator that interprets the data)
;;   3. Go vm.go      (native impl behind ExternalValue)
;;
;; In the browser: hop 3 becomes a WebSocket/HTTP proxy to
;; a local boxxy process, making the shape identical.
;;
;; Borkdude pattern: SCI interprets ClojureScript,
;; boxxy's Go Lisp interprets .joke files,
;; both read the same EDN shape — metacircular.

(ns vz.bridge
  "Bridge between squint/SCI and boxxy's Go Lisp evaluator.

   On local machine: spawns boxxy process, pipes Lisp over stdin.
   In browser: connects to boxxy WebSocket proxy on localhost.
   In both cases: same EDN shape, same colors, same eval."
  (:require [clojure.edn :as edn]))

;; ============================================================================
;; COLORS (from gay MCP seed=42)
;; ============================================================================

(def colors
  "Rainbow parens for vz S-expressions — Ising spin assignment."
  {:root     "#0BC68E"   ; depth-0, spin +1
   :config   "#91BE25"   ; depth-1, spin -1
   :instance "#1533EA"   ; depth-2, spin +1
   :snapshot "#D822A5"   ; depth-3, spin -1
   :exec     "#B09A11"   ; depth-4, spin +1
   :branch   "#E2799D"   ; depth-5, spin -1
   :volume   "#A4DE31"}) ; depth-6, spin +1

;; ============================================================================
;; PROVIDER EDN (normal form)
;; ============================================================================

(defn load-provider
  "Load provider spec from EDN. This is the normal form —
   identical across Go, squint, SCI, babashka, joker."
  [edn-str]
  (edn/read-string edn-str))

;; ============================================================================
;; TRANSPORT LAYER
;; ============================================================================

(defprotocol IBoxxyTransport
  "Transport between this evaluator and the Go boxxy process."
  (send-expr [this expr] "Send a Lisp expression, return result.")
  (connected? [this] "Is the transport alive?")
  (close! [this] "Close the transport."))

;; Local process transport (Node.js / Bun)
(defn- make-process-transport
  "Spawn boxxy repl as a child process, pipe S-expressions."
  [boxxy-path]
  (let [proc   (js/require "child_process")
        child  (.spawn proc boxxy-path #js ["repl"]
                       #js {:stdio #js ["pipe" "pipe" "pipe"]})
        stdout (.-stdout child)
        stdin  (.-stdin child)]
    (reify IBoxxyTransport
      (send-expr [_ expr]
        (js/Promise.
         (fn [resolve reject]
           (let [buf (atom "")]
             (.once stdout "data"
                    (fn [data]
                      (swap! buf str (.toString data "utf-8"))
                      (resolve @buf)))
             (.write stdin (str (pr-str expr) "\n"))))))
      (connected? [_]
        (nil? (.-exitCode child)))
      (close! [_]
        (.kill child "SIGTERM")))))

;; WebSocket transport (browser)
(defn- make-ws-transport
  "Connect to boxxy WebSocket proxy on localhost.
   The proxy translates JSON↔Lisp and forwards to boxxy repl."
  [ws-url]
  (let [ws  (js/WebSocket. ws-url)
        res (atom {})]
    (set! (.-onmessage ws)
          (fn [e]
            (let [data (js/JSON.parse (.-data e))
                  id   (.-id data)
                  cb   (get @res id)]
              (when cb
                (swap! res dissoc id)
                (cb (.-result data))))))
    (reify IBoxxyTransport
      (send-expr [_ expr]
        (js/Promise.
         (fn [resolve _reject]
           (let [id (str (random-uuid))]
             (swap! res assoc id resolve)
             (.send ws (js/JSON.stringify
                        #js {:id id :expr (pr-str expr)}))))))
      (connected? [_]
        (= 1 (.-readyState ws)))
      (close! [_]
        (.close ws)))))

;; ============================================================================
;; BOXXY PROVIDER (implements ComputeProvider shape from EDN)
;; ============================================================================

(defn create-provider
  "Create a boxxy provider from EDN spec + transport.

   This is the metacircular step: the EDN describes what vz/ functions
   exist, and the provider calls them through the transport.

   Options:
     :transport  - :process or :websocket
     :boxxy-path - path to boxxy binary (for :process)
     :ws-url     - WebSocket URL (for :websocket)
     :spec       - provider EDN map (or nil to load from file)"
  [{:keys [transport boxxy-path ws-url spec]}]
  (let [tp   (case transport
               :process   (make-process-transport
                           (or boxxy-path
                               (str js/process.env.HOME "/.local/bin/boxxy")))
               :websocket (make-ws-transport
                           (or ws-url "ws://localhost:7888/vz")))
        spec (or spec (load-provider (js/require "fs").readFileSync
                                     (str js/process.env.HOME "/i/boxxy/std/provider.edn")
                                     "utf-8"))]
    {:name         (:provider/name spec)
     :color        (:provider/color spec)
     :trit         (:provider/trit spec)
     :capabilities (:capabilities spec)
     :sizes        (:sizes spec)
     :transport    tp

     ;; ── vz/ function caller ───────────────────────────────────
     :call-vz
     (fn [fn-name & args]
       (send-expr tp (cons (symbol (name fn-name)) args)))

     ;; ── ComputeProvider methods ───────────────────────────────
     :initialize
     (fn []
       (send-expr tp '(do (println "[boxxy] initialized from squint bridge"))))

     :start-instance
     (fn [{:keys [machine-size kernel-path initrd-path disk-size-gb]
           :or   {machine-size :medium disk-size-gb 16}}]
       (let [resources (get-in spec [:sizes machine-size])
             cpus      (:cpus resources)
             mem-gb    (:memory-gb resources)]
         (send-expr tp
           `(do
              (def disk-path (str ~(str js/process.env.HOME) "/.boxxy/disks/"
                                  ~(str (random-uuid)) ".img"))
              (vz/create-disk-image disk-path ~disk-size-gb)
              ~(if kernel-path
                 `(def boot (vz/new-linux-boot-loader ~kernel-path
                              ~(or initrd-path "") "console=hvc0"))
                 `(do
                    (def store (vz/new-efi-variable-store
                                (str disk-path ".nvram") true))
                    (def boot (vz/new-efi-boot-loader store))))
              (def platform (vz/new-generic-platform))
              (def config (vz/new-vm-config ~cpus ~mem-gb boot platform))
              (def disk-att (vz/new-disk-attachment disk-path false))
              (def disk (vz/new-virtio-block-device disk-att))
              (vz/add-storage-devices config [disk])
              (def nat (vz/new-nat-network))
              (def net (vz/new-virtio-network nat))
              (vz/add-network-devices config [net])
              (vz/validate-config config)
              (def vm (vz/new-vm config))
              (vz/start-vm! vm)
              {:status "running" :cpus ~cpus :memory-gb ~mem-gb}))))

     :stop-instance
     (fn [_id]
       (send-expr tp '(vz/stop-vm! vm)))

     :pause-instance
     (fn [_id]
       (send-expr tp '(vz/pause-vm! vm)))

     :resume-instance
     (fn [_id]
       (send-expr tp '(vz/resume-vm! vm)))

     :vm-state
     (fn []
       (send-expr tp '(vz/vm-state vm)))

     :shutdown
     (fn []
       (send-expr tp '(do (vz/stop-vm! vm) (println "[boxxy] shutdown")))
       (close! tp))}))

;; ============================================================================
;; CONVENIENCE — browser one-liner
;; ============================================================================

(defn browser-provider
  "Create a boxxy provider for browser use.
   Connects to local boxxy WebSocket proxy."
  ([] (browser-provider {}))
  ([opts]
   (create-provider (merge {:transport :websocket} opts))))

(defn local-provider
  "Create a boxxy provider for local CLI use.
   Spawns boxxy repl as a child process."
  ([] (local-provider {}))
  ([opts]
   (create-provider (merge {:transport :process} opts))))

;; ============================================================================
;; EDN ROUNDTRIP (the "identical" property)
;; ============================================================================

(defn provider->edn
  "Serialize provider state back to EDN.
   provider.edn → bridge.cljs → provider→edn = identical.
   This is the normal form property."
  [provider]
  (pr-str
   {:provider/name  (:name provider)
    :provider/color (:color provider)
    :provider/trit  (:trit provider)
    :capabilities   (:capabilities provider)
    :sizes          (:sizes provider)}))
