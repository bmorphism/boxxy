;; seL4 Bridge — TCP relay between Emacs (:9998) and Nashator (:9999)
;; Wire protocol: 4-byte big-endian length prefix + JSON-RPC 2.0
;;
;; Architecture:
;;   Emacs (sel4-mode.el) <--> bridge (:9998) <--> Nashator (:9999)
;;
;; Methods:
;;   sel4.ipc.{send,recv,call}
;;   sel4.cap.{query,grant,revoke}
;;   sel4.balance.{report,verify}
;;   sel4.fault.{log,register}
;;   sel4.monitor.{subscribe,unsubscribe}

(ns sel4.bridge
  "TCP relay for seL4 JSON-RPC 2.0 over 4-byte BE framed wire protocol.
   Bridges Emacs clients on :9998 to Nashator on :9999."
  (:require [sel4.balance :as bal]
            [sel4.ipc :as ipc]
            [sel4.capability :as cap])
  (:import [java.net ServerSocket Socket InetAddress]
           [java.io InputStream OutputStream DataInputStream DataOutputStream
                    BufferedInputStream BufferedOutputStream]
           [java.nio ByteBuffer]))

;; ============================================================================
;; Wire protocol — 4-byte BE length prefix + JSON-RPC 2.0
;; Matches zig-syrup/src/message_frame.zig exactly:
;;   [u32 big-endian length][JSON payload bytes]
;;   MAX_MESSAGE_SIZE = 4MB
;; ============================================================================

(def MAX_MESSAGE_SIZE (* 4 1024 1024))

(defn encode-frame
  "Encode a JSON string into a 4-byte BE length-prefixed frame."
  [^String json-str]
  (let [payload (.getBytes json-str "UTF-8")
        len (alength payload)
        _ (when (> len MAX_MESSAGE_SIZE)
            (throw (ex-info "Message too large" {:len len :max MAX_MESSAGE_SIZE})))
        buf (byte-array (+ 4 len))]
    ;; 4-byte big-endian length prefix
    (aset-byte buf 0 (unchecked-byte (bit-shift-right len 24)))
    (aset-byte buf 1 (unchecked-byte (bit-shift-right len 16)))
    (aset-byte buf 2 (unchecked-byte (bit-shift-right len 8)))
    (aset-byte buf 3 (unchecked-byte len))
    (System/arraycopy payload 0 buf 4 len)
    buf))

(defn decode-frame
  "Read one 4-byte BE length-prefixed frame from an InputStream.
   Returns the JSON payload string, or nil on EOF."
  [^DataInputStream dis]
  (try
    (let [b0 (.read dis)]
      (when (neg? b0) (throw (java.io.EOFException.)))
      (let [b1 (.read dis)
            b2 (.read dis)
            b3 (.read dis)
            _ (when (or (neg? b1) (neg? b2) (neg? b3))
                (throw (java.io.EOFException.)))
            len (bit-or (bit-shift-left (bit-and b0 0xFF) 24)
                        (bit-shift-left (bit-and b1 0xFF) 16)
                        (bit-shift-left (bit-and b2 0xFF) 8)
                        (bit-and b3 0xFF))]
        (when (> len MAX_MESSAGE_SIZE)
          (throw (ex-info "Frame too large" {:len len})))
        (let [payload (byte-array len)]
          (.readFully dis payload)
          (String. payload "UTF-8"))))
    (catch java.io.EOFException _ nil)))

(defn send-frame!
  "Send a JSON-RPC frame over a DataOutputStream."
  [^DataOutputStream dos ^String json-str]
  (let [frame (encode-frame json-str)]
    (.write dos frame)
    (.flush dos)))

;; ============================================================================
;; JSON-RPC 2.0 helpers
;; ============================================================================

(defn json-rpc-response [id result]
  (str "{\"jsonrpc\":\"2.0\",\"id\":" id
       ",\"result\":" (pr-str result) "}"))

(defn json-rpc-error [id code message]
  (str "{\"jsonrpc\":\"2.0\",\"id\":" id
       ",\"error\":{\"code\":" code
       ",\"message\":" (pr-str message) "}}"))

(defn json-rpc-notification [method params]
  (str "{\"jsonrpc\":\"2.0\",\"method\":" (pr-str method)
       ",\"params\":" (pr-str params) "}"))

(defn parse-json
  "Minimal JSON parse — uses clojure.data.json if available, else read-string."
  [^String s]
  (try
    (clojure.data.json/read-str s :key-fn keyword)
    (catch Exception _
      ;; Fallback: basic Clojure reader (for babashka compatibility)
      (read-string (str "{" (-> s
                                (.replace "{" "")
                                (.replace "}" "")
                                (.replace "\"jsonrpc\"" ":jsonrpc")
                                (.replace "\"method\"" ":method")
                                (.replace "\"params\"" ":params")
                                (.replace "\"id\"" ":id")
                                (.replace "\"result\"" ":result")) "}")))))

(defn to-json
  "Minimal JSON encode."
  [m]
  (try
    (clojure.data.json/write-str m)
    (catch Exception _
      (pr-str m))))

;; ============================================================================
;; DuckDB IPC monitor — log all messages for the IPC monitor buffer
;; ============================================================================

(def ipc-log (atom []))
(def ipc-log-max 1000)

(defn record-ipc!
  "Record an IPC message to the DuckDB IPC monitor log.
   In production this writes to boxxy/.topos/sel4_ipc.duckdb;
   for now we keep an in-memory ring buffer."
  [direction method params result trit]
  (let [entry {:timestamp (System/currentTimeMillis)
               :time (let [fmt (java.text.SimpleDateFormat. "HH:mm:ss.SSS")]
                       (.format fmt (java.util.Date.)))
               :direction direction
               :method method
               :params params
               :result result
               :trit trit}]
    (swap! ipc-log (fn [log]
                     (let [log' (conj log entry)]
                       (if (> (count log') ipc-log-max)
                         (vec (drop (- (count log') ipc-log-max) log'))
                         log'))))))

;; ============================================================================
;; GF(3) balance checking
;; ============================================================================

(def method-trits
  {"sel4.ipc.send"              bal/TRIT_ZERO
   "sel4.ipc.recv"              bal/TRIT_ZERO
   "sel4.ipc.call"              bal/TRIT_ZERO
   "sel4.cap.query"             bal/TRIT_MINUS
   "sel4.cap.grant"             bal/TRIT_PLUS
   "sel4.cap.revoke"            bal/TRIT_MINUS
   "sel4.balance.report"        bal/TRIT_ZERO
   "sel4.balance.verify"        bal/TRIT_MINUS
   "sel4.fault.log"             bal/TRIT_ZERO
   "sel4.fault.register"        bal/TRIT_MINUS
   "sel4.monitor.subscribe"     bal/TRIT_ZERO
   "sel4.monitor.unsubscribe"   bal/TRIT_ZERO})

(def session-trit-sum (atom 0))

(defn gf3-check
  "Check GF(3) balance for a method call. Returns {:trit t :sum s :balanced? b}."
  [method]
  (let [trit (get method-trits method bal/TRIT_ZERO)
        new-sum (swap! session-trit-sum + trit)]
    {:trit trit
     :sum new-sum
     :balanced? (zero? (mod (+ new-sum 3000) 3))}))

;; ============================================================================
;; Monitor subscriptions — multiple concurrent Emacs connections
;; ============================================================================

(def monitor-subscribers (atom {}))
;; {client-id -> {:dos DataOutputStream, :subscribed? true, :filters #{...}}}

(defn notify-subscribers!
  "Push IPC event to all subscribed Emacs clients."
  [event]
  (doseq [[client-id sub] @monitor-subscribers]
    (when (:subscribed? sub)
      (try
        (send-frame! (:dos sub)
                     (json-rpc-notification "sel4.monitor.event"
                                            (to-json event)))
        (catch Exception e
          ;; Client disconnected, remove subscription
          (swap! monitor-subscribers dissoc client-id))))))

;; ============================================================================
;; Nashator relay — forward messages to/from :9999
;; ============================================================================

(def nashator-host "127.0.0.1")
(def nashator-port 9999)

(defn nashator-forward!
  "Forward a JSON-RPC request to Nashator on :9999 and return the response.
   Uses the same 4-byte BE + JSON-RPC 2.0 wire protocol."
  [json-request]
  (try
    (with-open [sock (Socket. ^String nashator-host ^int nashator-port)
                dos (DataOutputStream. (BufferedOutputStream. (.getOutputStream sock)))
                dis (DataInputStream. (BufferedInputStream. (.getInputStream sock)))]
      (.setSoTimeout sock 5000)
      (send-frame! dos json-request)
      (decode-frame dis))
    (catch Exception e
      (to-json {:error {:code -32603
                        :message (str "Nashator unreachable: " (.getMessage e))}}))))

;; ============================================================================
;; Method handlers
;; ============================================================================

(defn handle-ipc-send [id params]
  (let [endpoint (get params :endpoint "default")
        args (get params :args [])
        badge (get params :badge 0)
        gf3 (gf3-check "sel4.ipc.send")
        result {:status "sent"
                :endpoint endpoint
                :badge badge
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "send" "sel4.ipc.send" params result (:trit gf3))
    (notify-subscribers! {:type "ipc" :direction "send" :method "sel4.ipc.send"
                          :endpoint endpoint :trit (:trit gf3)})
    (json-rpc-response id (to-json result))))

(defn handle-ipc-recv [id params]
  (let [endpoint (get params :endpoint "default")
        timeout (get params :timeout 5000)
        gf3 (gf3-check "sel4.ipc.recv")
        result {:status "received"
                :endpoint endpoint
                :timeout timeout
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "recv" "sel4.ipc.recv" params result (:trit gf3))
    (notify-subscribers! {:type "ipc" :direction "recv" :method "sel4.ipc.recv"
                          :endpoint endpoint :trit (:trit gf3)})
    (json-rpc-response id (to-json result))))

(defn handle-ipc-call [id params]
  (let [endpoint (get params :endpoint "default")
        method-name (get params :method "unknown")
        args (get params :args [])
        gf3 (gf3-check "sel4.ipc.call")
        ;; Forward to Nashator if the call targets a game-theory method
        nashator-result (when (.startsWith ^String method-name "nashator.")
                          (nashator-forward! (to-json {:jsonrpc "2.0"
                                                       :method method-name
                                                       :params params
                                                       :id id})))
        result {:status "called"
                :endpoint endpoint
                :method method-name
                :nashator_response nashator-result
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "call" "sel4.ipc.call" params result (:trit gf3))
    (notify-subscribers! {:type "ipc" :direction "call" :method "sel4.ipc.call"
                          :endpoint endpoint :trit (:trit gf3)})
    (json-rpc-response id (to-json result))))

(defn handle-cap-query [id params]
  (let [cap-id (get params :id "*")
        gf3 (gf3-check "sel4.cap.query")
        ;; Query known capabilities
        result {:capabilities []
                :query cap-id
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "query" "sel4.cap.query" params result (:trit gf3))
    (notify-subscribers! {:type "cap" :direction "query" :method "sel4.cap.query"
                          :trit (:trit gf3)})
    (json-rpc-response id (to-json result))))

(defn handle-cap-grant [id params]
  (let [cap-id (get params :id)
        target (get params :target)
        rights (get params :rights 0x07)
        gf3 (gf3-check "sel4.cap.grant")
        result {:status "granted"
                :capability cap-id
                :target target
                :rights rights
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "grant" "sel4.cap.grant" params result (:trit gf3))
    (notify-subscribers! {:type "cap" :direction "grant" :method "sel4.cap.grant"
                          :capability cap-id :trit (:trit gf3)})
    (json-rpc-response id (to-json result))))

(defn handle-cap-revoke [id params]
  (let [cap-id (get params :id)
        gf3 (gf3-check "sel4.cap.revoke")
        result {:status "revoked"
                :capability cap-id
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "revoke" "sel4.cap.revoke" params result (:trit gf3))
    (notify-subscribers! {:type "cap" :direction "revoke" :method "sel4.cap.revoke"
                          :capability cap-id :trit (:trit gf3)})
    (json-rpc-response id (to-json result))))

(defn handle-balance-report [id params]
  (let [gf3 (gf3-check "sel4.balance.report")
        log-snapshot @ipc-log
        trit-counts (reduce (fn [acc entry]
                              (update acc (:trit entry) (fnil inc 0)))
                            {} log-snapshot)
        result {:session_trit_sum @session-trit-sum
                :balanced (zero? (mod (+ @session-trit-sum 3000) 3))
                :message_count (count log-snapshot)
                :trit_distribution {:-1 (get trit-counts -1 0)
                                    :0  (get trit-counts 0 0)
                                    :1  (get trit-counts 1 0)}
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "report" "sel4.balance.report" params result (:trit gf3))
    (json-rpc-response id (to-json result))))

(defn handle-balance-verify [id params]
  (let [trits (get params :trits [])
        gf3 (gf3-check "sel4.balance.verify")
        trit-sum (reduce + 0 trits)
        balanced? (zero? (mod (+ trit-sum 3000) 3))
        ;; Also forward to Nashator for cross-verification
        nashator-result (nashator-forward!
                          (to-json {:jsonrpc "2.0"
                                    :method "nashator_gf3_check"
                                    :params {:trits trits}
                                    :id id}))
        result {:trits trits
                :sum trit-sum
                :balanced balanced?
                :nashator_verified nashator-result
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "verify" "sel4.balance.verify" params result (:trit gf3))
    (json-rpc-response id (to-json result))))

(defn handle-fault-log [id params]
  (let [gf3 (gf3-check "sel4.fault.log")
        ;; Return recent fault entries from the IPC log
        faults (filter #(= (:direction %) "fault") @ipc-log)
        result {:faults (take 50 faults)
                :count (count faults)
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "log" "sel4.fault.log" params result (:trit gf3))
    (json-rpc-response id (to-json result))))

(defn handle-fault-register [id params]
  (let [fault-type (get params :type "vm-fault")
        handler-ep (get params :handler_endpoint)
        gf3 (gf3-check "sel4.fault.register")
        result {:status "registered"
                :fault_type fault-type
                :handler handler-ep
                :trit (:trit gf3)
                :gf3_balanced (:balanced? gf3)}]
    (record-ipc! "register" "sel4.fault.register" params result (:trit gf3))
    (json-rpc-response id (to-json result))))

(defn handle-monitor-subscribe [id params client-id dos]
  (let [filters (get params :filters #{})
        gf3 (gf3-check "sel4.monitor.subscribe")]
    (swap! monitor-subscribers assoc client-id
           {:dos dos :subscribed? true :filters (set filters)})
    (let [result {:status "subscribed"
                  :client_id client-id
                  :filters filters
                  :trit (:trit gf3)
                  :gf3_balanced (:balanced? gf3)}]
      (record-ipc! "subscribe" "sel4.monitor.subscribe" params result (:trit gf3))
      (json-rpc-response id (to-json result)))))

(defn handle-monitor-unsubscribe [id params client-id]
  (let [gf3 (gf3-check "sel4.monitor.unsubscribe")]
    (swap! monitor-subscribers dissoc client-id)
    (let [result {:status "unsubscribed"
                  :client_id client-id
                  :trit (:trit gf3)
                  :gf3_balanced (:balanced? gf3)}]
      (record-ipc! "unsubscribe" "sel4.monitor.unsubscribe" params result (:trit gf3))
      (json-rpc-response id (to-json result)))))

;; ============================================================================
;; Request dispatcher
;; ============================================================================

(defn dispatch-request
  "Route a JSON-RPC request to the appropriate handler."
  [request client-id dos]
  (let [method (:method request)
        params (or (:params request) {})
        id (:id request)]
    (case method
      "sel4.ipc.send"              (handle-ipc-send id params)
      "sel4.ipc.recv"              (handle-ipc-recv id params)
      "sel4.ipc.call"              (handle-ipc-call id params)
      "sel4.cap.query"             (handle-cap-query id params)
      "sel4.cap.grant"             (handle-cap-grant id params)
      "sel4.cap.revoke"            (handle-cap-revoke id params)
      "sel4.balance.report"        (handle-balance-report id params)
      "sel4.balance.verify"        (handle-balance-verify id params)
      "sel4.fault.log"             (handle-fault-log id params)
      "sel4.fault.register"        (handle-fault-register id params)
      "sel4.monitor.subscribe"     (handle-monitor-subscribe id params client-id dos)
      "sel4.monitor.unsubscribe"   (handle-monitor-unsubscribe id params client-id)
      ;; Unknown method — try forwarding to Nashator
      (let [nashator-result (nashator-forward! (to-json request))]
        (if nashator-result
          nashator-result
          (json-rpc-error id -32601 (str "Method not found: " method)))))))

;; ============================================================================
;; TCP server — one thread per client, supports multiple Emacs connections
;; ============================================================================

(def bridge-port 9998)
(def server-socket (atom nil))
(def client-counter (atom 0))
(def running? (atom false))

(defn handle-client
  "Handle a single Emacs client connection."
  [^Socket client-socket]
  (let [client-id (str "emacs-" (swap! client-counter inc))
        addr (.getRemoteSocketAddress client-socket)]
    (println (str "[sel4-bridge] Client connected: " client-id " from " addr))
    (try
      (with-open [dis (DataInputStream. (BufferedInputStream. (.getInputStream client-socket)))
                  dos (DataOutputStream. (BufferedOutputStream. (.getOutputStream client-socket)))]
        (loop []
          (when @running?
            (when-let [json-str (decode-frame dis)]
              (let [request (try
                              (clojure.data.json/read-str json-str :key-fn keyword)
                              (catch Exception _
                                ;; Fallback parse
                                {:method "unknown" :id 0 :params {}}))
                    response (dispatch-request request client-id dos)]
                (send-frame! dos response)
                (recur))))))
      (catch Exception e
        (println (str "[sel4-bridge] Client " client-id " error: " (.getMessage e))))
      (finally
        (swap! monitor-subscribers dissoc client-id)
        (println (str "[sel4-bridge] Client disconnected: " client-id))
        (.close client-socket)))))

(defn start-bridge!
  "Start the seL4 bridge TCP server on :9998.
   Accepts multiple concurrent Emacs connections."
  ([] (start-bridge! bridge-port))
  ([port]
   (when @running?
     (println "[sel4-bridge] Already running, stopping first...")
     (stop-bridge!))
   (reset! running? true)
   (reset! session-trit-sum 0)
   (reset! ipc-log [])
   (reset! monitor-subscribers {})
   (let [ss (ServerSocket. port 50 (InetAddress/getByName "127.0.0.1"))]
     (reset! server-socket ss)
     (println (str "[sel4-bridge] Listening on 127.0.0.1:" port))
     (println "[sel4-bridge] Wire protocol: 4-byte BE + JSON-RPC 2.0")
     (println "[sel4-bridge] Nashator relay: 127.0.0.1:9999")
     ;; Accept loop in a daemon thread
     (future
       (try
         (while @running?
           (let [client (.accept ss)]
             (.setSoTimeout client 0)
             ;; Each client in its own thread
             (future (handle-client client))))
         (catch Exception e
           (when @running?
             (println (str "[sel4-bridge] Accept error: " (.getMessage e)))))))
     :started)))

(defn stop-bridge!
  "Stop the seL4 bridge TCP server."
  []
  (reset! running? false)
  (when-let [ss @server-socket]
    (try (.close ss) (catch Exception _))
    (reset! server-socket nil))
  ;; Disconnect all monitor subscribers
  (reset! monitor-subscribers {})
  (println "[sel4-bridge] Stopped.")
  :stopped)

;; ============================================================================
;; REPL convenience
;; ============================================================================

(defn status
  "Print bridge status."
  []
  {:running @running?
   :port bridge-port
   :clients (count @monitor-subscribers)
   :ipc_messages (count @ipc-log)
   :session_trit_sum @session-trit-sum
   :balanced (zero? (mod (+ @session-trit-sum 3000) 3))})

(comment
  ;; Start the bridge
  (start-bridge!)

  ;; Check status
  (status)

  ;; Stop the bridge
  (stop-bridge!)

  ;; View IPC log
  (take 10 @ipc-log)
  )
