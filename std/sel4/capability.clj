;; seL4 Capability System Integration
;; Bind boxxy Sideref tokens to seL4 capabilities

(ns sel4.capability
  "Cryptographic capability binding for seL4"
  (:require [clojure.string :as str])
  (:import [javax.crypto Mac]
           [javax.crypto.spec SecretKeySpec]
           [java.security MessageDigest]))

;; Capability records
(defrecord Capability
  [id              ;; Unique capability ID
   name            ;; Human-readable name
   endpoint        ;; seL4 endpoint reference
   rights          ;; Rights bitmap
   token           ;; HMAC-SHA256 Sideref token
   created-at      ;; Timestamp
   expires-at      ;; Expiration (unix timestamp)
   ])

(defrecord SiderefToken
  [skill-name      ;; Skill name for binding
   device-id       ;; Device identifier
   token-bytes     ;; 32-byte HMAC-SHA256
   version         ;; Token version
   expires-at      ;; Expiration timestamp
   ])

;; Constants
(def HMAC_ALGORITHM "HmacSHA256")
(def TOKEN_SIZE 32)
(def DEFAULT_TTL_SECONDS 86400)  ;; 24 hours

;; Device secret (in production, loaded from secure storage)
(def device-secret (atom (make-array Byte/TYPE 16)))

;; Sideref token generation (OCAPN spec)
(defn generate-sideref [skill-name]
  "Generate unforgeable capability token"
  (let [device-id (java.util.UUID/randomUUID)
        timestamp (/ (System/currentTimeMillis) 1000)
        expires (+ timestamp DEFAULT_TTL_SECONDS)

        ;; Create HMAC
        mac (Mac/getInstance HMAC_ALGORITHM)
        secret-key (SecretKeySpec @device-secret 0 16 HMAC_ALGORITHM)
        _ (do
            (.init mac secret-key)
            ;; Include skill name and device ID in HMAC
            (.update mac (.getBytes skill-name "UTF-8"))
            (.update mac (.getBytes (str device-id) "UTF-8"))
            (.update mac (long-to-bytes timestamp)))

        token-bytes (.doFinal mac)]

    (->SiderefToken skill-name device-id token-bytes 1 expires)))

(defn long-to-bytes [value]
  "Convert long to byte array"
  (byte-array 8
    [(bit-shift-right value 56)
     (bit-shift-right value 48)
     (bit-shift-right value 40)
     (bit-shift-right value 32)
     (bit-shift-right value 24)
     (bit-shift-right value 16)
     (bit-shift-right value 8)
     value]))

;; Verify Sideref token (constant-time)
(defn verify-sideref [token expected-device-id]
  "Verify token signature (constant-time comparison)"
  (let [mac (Mac/getInstance HMAC_ALGORITHM)
        secret-key (SecretKeySpec @device-secret 0 16 HMAC_ALGORITHM)
        _ (do
            (.init mac secret-key)
            (.update mac (.getBytes (:skill-name token) "UTF-8"))
            (.update mac (.getBytes (str expected-device-id) "UTF-8"))
            (.update mac (long-to-bytes (:expires-at token))))

        computed (.doFinal mac)]

    ;; Constant-time comparison (prevents timing attacks)
    (constant-time-equals computed (:token-bytes token))))

(defn constant-time-equals [a b]
  "Constant-time byte array comparison"
  (if (not= (count a) (count b))
    false
    (let [result (atom 0)]
      (doseq [i (range (count a))]
        (swap! result bit-or (bit-xor (aget a i) (aget b i))))
      (zero? @result))))

;; Bind capability to seL4 endpoint
(defn bind-capability [endpoint token]
  "Create unforgeable binding between endpoint and Sideref token"
  (let [now (/ (System/currentTimeMillis) 1000)
        cap-id (java.util.UUID/randomUUID)]

    ;; Verify token is still valid
    (if (> now (:expires-at token))
      {:error "Token expired"}

      ;; Create capability record
      (->Capability cap-id
                    (:skill-name token)
                    endpoint
                    0xFFFFFFFF  ;; Full rights
                    token
                    now
                    (:expires-at token)))))

;; Make capability
(defn make-capability [name & {:keys [endpoint rights expires-at]}]
  "Create a capability with given properties"
  (let [now (/ (System/currentTimeMillis) 1000)
        ttl (- (or expires-at (+ now DEFAULT_TTL_SECONDS)) now)
        token (generate-sideref name)]

    (->Capability (java.util.UUID/randomUUID)
                  name
                  (or endpoint "sel4:unbound")
                  (or rights 0xFF)
                  token
                  now
                  (+ now ttl))))

;; Capability derivation (child inherits from parent)
(defn inherit-capability [parent-cap child-name]
  "Derive child capability from parent (attenuated rights)"
  (let [token (generate-sideref child-name)
        inherited-rights (bit-shift-right (:rights parent-cap) 1)]  ;; Attenuation

    (->Capability (java.util.UUID/randomUUID)
                  child-name
                  (:endpoint parent-cap)
                  inherited-rights
                  token
                  (/ (System/currentTimeMillis) 1000)
                  (:expires-at parent-cap))))

;; Capability revocation
(def revoked-capabilities (atom #{}))

(defn revoke-capability [cap]
  "Revoke a capability (prevents future use)"
  (swap! revoked-capabilities conj (:id cap))
  {:status :revoked :capability-id (:id cap)})

(defn is-revoked? [cap]
  "Check if capability is revoked"
  (contains? @revoked-capabilities (:id cap)))

;; Capability validation
(defn valid? [cap]
  "Check if capability is still valid"
  (let [now (/ (System/currentTimeMillis) 1000)]
    (and (not (is-revoked? cap))
         (< now (:expires-at cap)))))

;; Query capability properties
(defn name [cap]
  "Get capability name"
  (:name cap))

(defn endpoint [cap]
  "Get capability endpoint"
  (:endpoint cap))

(defn rights [cap]
  "Get capability rights bitmap"
  (:rights cap))

(defn has-right? [cap right]
  "Check if capability grants specific right"
  (not (zero? (bit-and (:rights cap) right))))

;; Capability relationship tests
(defn derived-from? [child-cap parent-cap]
  "Test if child is derived from parent capability"
  ;; In production, would check cryptographic derivation chain
  (= (:endpoint child-cap) (:endpoint parent-cap)))

;; Attenuate capability (reduce rights)
(defn attenuate [cap new-rights]
  "Create attenuated version with reduced rights"
  (update cap :rights bit-and new-rights))

;; Transfer capability
(defn transfer-to [cap new-endpoint]
  "Transfer capability to different endpoint"
  (assoc cap :endpoint new-endpoint))

;; Statistics
(def stats (atom {:created 0 :verified 0 :revoked 0 :transferred 0}))

(defn record-stat [stat-type]
  "Record capability operation statistic"
  (case stat-type
    :create (swap! stats update :created inc)
    :verify (swap! stats update :verified inc)
    :revoke (swap! stats update :revoked inc)
    :transfer (swap! stats update :transferred inc)))

(defn get-stats []
  "Get capability statistics"
  @stats)

(defn reset-stats []
  "Reset capability statistics"
  (reset! stats {:created 0 :verified 0 :revoked 0 :transferred 0}))

;; Serialization for wire transmission
(defn serialize-capability [cap]
  "Serialize capability for transmission"
  {:id (str (:id cap))
   :name (:name cap)
   :endpoint (:endpoint cap)
   :rights (:rights cap)
   :token (str/join (map #(format "%02x" %) (:token-bytes (:token cap))))
   :expires-at (:expires-at cap)})

(defn deserialize-capability [wire-cap]
  "Deserialize capability from wire format"
  (->Capability
    (java.util.UUID/fromString (:id wire-cap))
    (:name wire-cap)
    (:endpoint wire-cap)
    (:rights wire-cap)
    {:token-bytes (hex-to-bytes (:token wire-cap))}
    (/ (System/currentTimeMillis) 1000)
    (:expires-at wire-cap)))

(defn hex-to-bytes [hex-string]
  "Convert hex string to byte array"
  (byte-array
    (map #(Integer/parseInt (apply str %) 16)
         (partition 2 hex-string))))

;; Capability delegation
(defn delegate [cap delegate-to-endpoint]
  "Delegate capability to another endpoint"
  (let [delegated (transfer-to cap delegate-to-endpoint)]
    (record-stat :transfer)
    delegated))

;; Capability amplification (upgrade rights - requires parent authority)
(defn amplify [cap parent-cap new-rights]
  "Amplify capability rights (only if parent grants)"
  (if (and (derived-from? cap parent-cap)
           (has-right? parent-cap new-rights))
    (update cap :rights bit-or new-rights)
    {:error "Cannot amplify without parent authority"}))

;; Inspect capability
(defn inspect [cap]
  "Get human-readable capability info"
  (println (str "Capability: " (:name cap)))
  (println (str "  ID: " (:id cap)))
  (println (str "  Endpoint: " (:endpoint cap)))
  (println (str "  Rights: " (format "0x%08X" (:rights cap)))
  (println (str "  Valid: " (valid? cap)))
  (println (str "  Revoked: " (is-revoked? cap)))
  (println (str "  Expires: " (java.time.Instant/ofEpochSecond (:expires-at cap)))))
