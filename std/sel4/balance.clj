;; GF(3) Conservation for seL4 IPC Operations
;; Ensure all message passing preserves trit balance

(ns sel4.balance
  "GF(3) balance verification for seL4 system"
  (:require [sel4.ipc :as ipc]))

;; GF(3) Trit values
(def TRIT_MINUS -1)  ;; Validation/verification operations
(def TRIT_ZERO 0)    ;; Coordination/message operations
(def TRIT_PLUS 1)    ;; Generation/creation operations

;; Skill-to-trit mapping (from Gay.jl)
(def skill-trits
  {
   ;; Verification skills (MINUS)
   "sel4:capability-verifier" TRIT_MINUS
   "sel4:security-monitor" TRIT_MINUS
   "sel4:audit-service" TRIT_MINUS

   ;; Coordination skills (ZERO)
   "sel4:xmonad-wm" TRIT_ZERO
   "sel4:ipc-router" TRIT_ZERO
   "sel4:message-broker" TRIT_ZERO

   ;; Generation/creation skills (PLUS)
   "sel4:process-creator" TRIT_PLUS
   "sel4:window-factory" TRIT_PLUS
   "sel4:endpoint-allocator" TRIT_PLUS
   })

;; Operation trits
(def operation-trits
  {
   ;; Verification (MINUS)
   :verify-capability TRIT_MINUS
   :audit-message TRIT_MINUS
   :validate-rights TRIT_MINUS

   ;; Coordination (ZERO)
   :query-layout TRIT_ZERO
   :send-message TRIT_ZERO
   :recv-message TRIT_ZERO

   ;; Generation (PLUS)
   :create-endpoint TRIT_PLUS
   :allocate-capability TRIT_PLUS
   :spawn-process TRIT_PLUS
   })

;; Get trit for skill
(defn skill-trit [skill-name]
  "Get GF(3) trit for skill"
  (get skill-trits skill-name TRIT_ZERO))

;; Get trit for operation
(defn operation-trit [operation]
  "Get GF(3) trit for operation"
  (get operation-trits operation TRIT_ZERO))

;; Analyze message trits
(defn analyze-message [msg]
  "Extract GF(3) trits from message"
  (let [{:keys [endpoint operation label args]} msg
        sender-trit (skill-trit endpoint)
        op-trit (operation-trit operation)]

    {
     :message msg
     :sender-endpoint endpoint
     :sender-trit sender-trit
     :operation operation
     :operation-trit op-trit
     :total-trit (mod (+ sender-trit op-trit) 3)
     }))

;; Verify GF(3) invariant for single message
(defn verify-message-gf3 [msg]
  "Check if message preserves GF(3) - requires balanced sender/receiver"
  (let [analysis (analyze-message msg)]
    {
     :valid (zero? (mod (+ (:sender-trit analysis)
                           (:operation-trit analysis))
                        3))
     :analysis analysis
     }))

;; Build message round-trip
(defrecord MessageRoundTrip
  [sender          ;; Sending endpoint
   operation       ;; Operation performed
   receiver        ;; Receiving endpoint
   response        ;; Response operation
   timestamp       ;; When message was sent
   ])

;; Analyze round-trip for GF(3) balance
(defn verify-round-trip [round-trip]
  "Verify GF(3) conservation in send-recv cycle"
  (let [{:keys [sender operation receiver response]} round-trip
        sender-trit (skill-trit sender)
        op-trit (operation-trit operation)
        receiver-trit (skill-trit receiver)
        response-trit (operation-trit response)

        ;; GF(3) conservation: sender + op + receiver + response ≡ 0 (mod 3)
        total (+ sender-trit op-trit receiver-trit response-trit)
        balanced? (zero? (mod total 3))]

    {
     :balanced? balanced?
     :total-trit (mod total 3)
     :breakdown {
       :sender-trit sender-trit
       :operation-trit op-trit
       :receiver-trit receiver-trit
       :response-trit response-trit
     }
     :message (str
       "Sender(" sender-trit ") + Op(" op-trit ") + Receiver(" receiver-trit
       ") + Response(" response-trit ") = " (mod total 3) " (mod 3)")
     }))

;; Verify sequence of messages
(defn verify-message-sequence [messages]
  "Verify GF(3) conservation across multiple messages"
  (let [analyses (map analyze-message messages)
        total-trit (apply + (map :total-trit analyses))
        balanced? (zero? (mod total-trit 3))]

    {
     :count (count messages)
     :balanced? balanced?
     :total-trit (mod total-trit 3)
     :analyses analyses
     :summary (str (count messages) " messages, total trit = "
                   (mod total-trit 3) " (mod 3)")
     }))

;; Record and verify IPC session
(def session-messages (atom []))

(defn record-message [msg]
  "Record message for batch verification"
  (swap! session-messages conj msg))

(defn get-session-messages []
  "Get all recorded messages in current session"
  @session-messages)

(defn verify-session-gf3 []
  "Verify entire session preserves GF(3)"
  (let [messages @session-messages]
    (verify-message-sequence messages)))

(defn reset-session []
  "Clear session message record"
  (reset! session-messages []))

;; Test vectors
(defn test-balanced-message []
  "Test message that should be balanced"
  (let [msg (ipc/->Message
             "sel4:window-factory"    ;; PLUS endpoint
             :send-message            ;; ZERO operation
             0 [] 0 [])
        result (verify-message-gf3 msg)]

    {:test :balanced-message
     :expected true
     :actual (:valid result)
     :passed (= (:valid result) true)}))

(defn test-unbalanced-message []
  "Test message that violates GF(3)"
  (let [msg (ipc/->Message
             "sel4:process-creator"   ;; PLUS endpoint
             :create-endpoint         ;; PLUS operation
             0 [] 0 [])
        result (verify-message-gf3 msg)]

    {:test :unbalanced-message
     :expected false
     :actual (:valid result)
     :passed (= (:valid result) false)}))

;; Report GF(3) violations
(def gf3-violations (atom []))

(defn record-violation [msg reason]
  "Record a GF(3) violation"
  (swap! gf3-violations conj
         {
          :timestamp (System/currentTimeMillis)
          :message msg
          :reason reason
          }))

(defn get-violations []
  "Get all recorded violations"
  @gf3-violations)

(defn clear-violations []
  "Clear violation record"
  (reset! gf3-violations []))

;; Enforce GF(3) at runtime (optional)
(def enforce-gf3 (atom false))

(defn enable-gf3-enforcement []
  "Enable strict GF(3) checking"
  (reset! enforce-gf3 true))

(defn disable-gf3-enforcement []
  "Disable strict GF(3) checking"
  (reset! enforce-gf3 false))

(defn check-or-warn [msg]
  "Either enforce or warn about GF(3) violations"
  (let [result (verify-message-gf3 msg)]
    (when-not (:valid result)
      (if @enforce-gf3
        (do
          (println "ERROR: GF(3) violation detected!")
          (record-violation msg "GF(3) constraint violated")
          (throw (Exception. "GF(3) enforcement failure")))
        (do
          (println "WARNING: GF(3) violation detected (non-fatal)")
          (record-violation msg "GF(3) constraint warning"))))))

;; Visualization/reporting
(defn print-gf3-report []
  "Print human-readable GF(3) report"
  (let [session (verify-session-gf3)
        violations @gf3-violations]

    (println "\n=== GF(3) Balance Report ===")
    (println (str "Messages analyzed: " (:count session)))
    (println (str "Total trit sum: " (:total-trit session) " (mod 3)"))
    (println (str "Balanced: " (:balanced? session)))
    (println (str "Summary: " (:summary session)))

    (when-not (empty? violations)
      (println "\nViolations detected:")
      (doseq [v violations]
        (println (str "  - " (:reason v) " at " (:timestamp v)))))))

;; GF(3) triads (well-known balanced combinations)
(def canonical-triads
  [
   ;; Attacker (-1), Arbiter (0), Defender (+1)
   {
    :name "Adversarial"
    :players ["sel4:security-monitor" "sel4:ipc-router" "sel4:process-creator"]
    :trits [TRIT_MINUS TRIT_ZERO TRIT_PLUS]
    }

   ;; Query (0), Allocate (+1), Verify (-1)
   {
    :name "Resource Lifecycle"
    :operations [:send-message :create-endpoint :verify-capability]
    :trits [TRIT_ZERO TRIT_PLUS TRIT_MINUS]
    }

   ;; Reveal (-1), Blur (0), Encrypt (+1)
   {
    :name "Cryptographic"
    :operations [:audit-message :query-layout :allocate-capability]
    :trits [TRIT_MINUS TRIT_ZERO TRIT_PLUS]
    }
   ])

(defn verify-triad [endpoints-or-ops]
  "Verify a triad (3 items) conserves GF(3)"
  (let [trits (map #(if (keyword? %)
                      (operation-trit %)
                      (skill-trit %))
                   endpoints-or-ops)
        total (apply + trits)]

    {
     :items endpoints-or-ops
     :trits trits
     :total-trit (mod total 3)
     :balanced? (zero? (mod total 3))
     }))
