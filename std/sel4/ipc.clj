;; seL4 Inter-Process Communication (IPC) Namespace
;; Clojure bindings for seL4 message passing

(ns sel4.ipc
  "Inter-process communication with seL4 endpoints"
  (:require [sel4.protocol :as proto]
            [sel4.types :as types]))

;; IPC Message Structure
(defrecord Message
  [endpoint      ;; Target IPC endpoint (string)
   operation     ;; Operation code (keyword)
   label         ;; Message label (numeric)
   args          ;; Arguments (vector)
   tag           ;; Message tag
   capabilities  ;; Capability references (vector)
   ])

;; Standard seL4 Operations
(def operations
  {
   ;; Window Manager Operations
   :query-layout      0x01
   :move-window       0x02
   :resize-window     0x03
   :focus-window      0x04
   :close-window      0x05
   :list-windows      0x06

   ;; Input Device Operations
   :inject-key        0x10
   :inject-mouse      0x11
   :set-mouse-pos     0x12
   :query-input-state 0x13

   ;; Display Driver Operations
   :framebuffer-read  0x20
   :framebuffer-write 0x21
   :query-resolution  0x22
   :set-resolution    0x23

   ;; Capability Operations
   :bind-capability   0x30
   :revoke-capability 0x31
   :query-capability  0x32
   :grant-capability  0x33
   })

;; Message Serialization
(defn serialize-message [msg]
  "Serialize Clojure message to seL4 wire format"
  (let [{:keys [endpoint operation label args tag capabilities]} msg
        op-code (get operations operation)
        arg-vector (into [] args)]
    {
     :endpoint endpoint
     :operation op-code
     :label label
     :args arg-vector
     :tag tag
     :caps capabilities
     }))

(defn deserialize-message [wire-msg]
  "Deserialize seL4 wire format to Clojure message"
  (let [{:keys [endpoint operation label args tag caps]} wire-msg
        op-key (first (filter #(= (% 1) operation) operations))
        op-name (key op-key)]
    (->Message endpoint op-name label (vec args) tag (vec caps))))

;; IPC Endpoint Management
(defn endpoint? [ep]
  "Check if valid endpoint reference"
  (and (string? ep) (.startsWith ep "sel4:")))

(defn create-endpoint [name]
  "Create new IPC endpoint"
  (when-not (endpoint? name)
    (throw (Exception. "Invalid endpoint name")))
  {:name name
   :active true
   :message-queue []
   :subscribers []})

(defn send-message [endpoint msg]
  "Send message to seL4 endpoint (non-blocking)"
  (when-not (endpoint? endpoint)
    (throw (Exception. "Invalid endpoint")))

  (let [serialized (serialize-message
                     (assoc msg :endpoint endpoint))]
    ;; In actual implementation, this would call native seL4_Send
    {:status :sent
     :endpoint endpoint
     :message serialized
     :timestamp (System/currentTimeMillis)}))

(defn recv-message [endpoint & {:keys [timeout] :or {timeout 5000}}]
  "Receive message from seL4 endpoint (blocking with timeout)"
  (when-not (endpoint? endpoint)
    (throw (Exception. "Invalid endpoint")))

  ;; In actual implementation, this would call native seL4_Recv
  {:status :received
   :endpoint endpoint
   :message {}
   :timestamp (System/currentTimeMillis)})

(defn send-recv [endpoint msg & {:keys [timeout] :or {timeout 5000}}]
  "Send message and wait for synchronous response"
  (when-not (endpoint? endpoint)
    (throw (Exception. "Invalid endpoint")))

  (let [sent (send-message endpoint msg)]
    (if (= (:status sent) :sent)
      (recv-message endpoint :timeout timeout)
      {:error "Failed to send message"})))

;; High-level message builders
(defn query-layout [endpoint]
  "Query window layout from window manager"
  (send-recv endpoint
    (->Message endpoint :query-layout 0 [] 0 [])))

(defn move-window [endpoint wm-id x y width height]
  "Move/resize window"
  (send-recv endpoint
    (->Message endpoint :move-window 1
               [wm-id x y width height]
               0 [])))

(defn focus-window [endpoint wm-id]
  "Focus specific window"
  (send-recv endpoint
    (->Message endpoint :focus-window 0
               [wm-id]
               0 [])))

(defn inject-key [endpoint keycode]
  "Inject keyboard event"
  (send-message endpoint
    (->Message endpoint :inject-key 0
               [keycode]
               0 [])))

(defn inject-mouse [endpoint x y buttons]
  "Inject mouse event"
  (send-message endpoint
    (->Message endpoint :inject-mouse 0
               [x y buttons]
               0 [])))

(defn framebuffer-read [endpoint]
  "Read framebuffer from display driver"
  (send-recv endpoint
    (->Message endpoint :framebuffer-read 0 [] 0 [])))

;; Capability-aware messaging
(defn bind-capability-to-message [msg capability-token]
  "Add capability reference to message"
  (update msg :capabilities conj capability-token))

(defn grant-capability [endpoint grant-to capability]
  "Grant seL4 capability to another endpoint"
  (send-message endpoint
    (->Message endpoint :grant-capability 0
               [grant-to]
               0 [capability])))

;; Error handling
(defn verify-response [response]
  "Verify IPC response is valid"
  (if (contains? response :error)
    {:valid false :error (:error response)}
    {:valid true}))

;; Statistics and monitoring
(def stats (atom {:sent-count 0 :recv-count 0 :errors 0}))

(defn record-message [msg-type]
  "Record message statistics"
  (case msg-type
    :sent (swap! stats update :sent-count inc)
    :recv (swap! stats update :recv-count inc)
    :error (swap! stats update :errors inc)))

(defn get-statistics []
  "Get IPC statistics"
  @stats)

(defn reset-statistics []
  "Reset IPC statistics"
  (reset! stats {:sent-count 0 :recv-count 0 :errors 0}))
