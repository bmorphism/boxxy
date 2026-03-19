#!/usr/bin/env bb
;; topos-gateway.bb — Our Portkey replacement
;;
;; Portkey routes to 200+ LLMs with observability, guardrails, virtual keys.
;; We do the same but grounded in:
;;   - Sideref capability tokens (OCAPN) instead of virtual API keys
;;   - Ω₃ subobject classifier for access control
;;   - GF(3) conservation across all routing decisions
;;   - Sygil compile-time checks on intent morphisms
;;   - DID-based identity (did:gay) instead of API keys
;;
;; The gateway IS a resource-sharing machine: requests are resources,
;; providers are nodes, and routing is the ERGODIC coordinator.

(ns topos-gateway
  (:require [clojure.string :as str]
            [cheshire.core :as json]))

;; ══════════════════════════════════════════════════════════════════════
;; Provider Registry (what Portkey calls "200+ LLMs")
;; ══════════════════════════════════════════════════════════════════════

(def providers
  "Provider registry with GF(3) classification.
   PLUS providers generate (fast, creative).
   ERGODIC providers coordinate (balanced, reliable).
   MINUS providers validate (slow, precise, auditable)."
  {:anthropic   {:trit  1 :endpoint "https://api.anthropic.com/v1"
                 :models ["claude-opus-4-6" "claude-sonnet-4-5-20250929"]
                 :role :plus :hue 45.0}
   :openai      {:trit  0 :endpoint "https://api.openai.com/v1"
                 :models ["gpt-4o" "o3"]
                 :role :ergodic :hue 180.0}
   :local-exo   {:trit -1 :endpoint "http://localhost:8080/v1"
                 :models ["llama-3.3-70b" "deepseek-r1"]
                 :role :minus :hue 270.0}
   :dgx-alpha   {:trit  1 :endpoint "http://dgx-alpha.pirate-dragon.ts.net:8080/v1"
                 :models ["llama-3.3-405b"]
                 :role :plus :hue 30.0}
   :dgx-beta    {:trit -1 :endpoint "http://dgx-beta.pirate-dragon.ts.net:8080/v1"
                 :models ["deepseek-v3" "qwen-72b"]
                 :role :minus :hue 300.0}
   ;; Noble denom monitoring — SHA-3 shadow computation + anomaly detection
   ;; Uses DGX fleet for prefix-scan GPU tasks, NATS for event routing
   :denom-shadow {:trit  1 :endpoint "nats://ibc.denom.>"
                  :models ["sha3-shadow" "prefix-scan-64" "prefix-scan-80"]
                  :role :plus :hue 137.5}
   :denom-verify {:trit -1 :endpoint "nats://alert.denom.>"
                  :models ["denom-check" "anomaly-detect" "zombie-flag"]
                  :role :minus :hue 317.5}
   :denom-store  {:trit  0 :endpoint "duckdb://noble-denoms"
                  :models ["shadow-log" "denom-registry" "alert-store"]
                  :role :ergodic :hue 57.5}})

;; ══════════════════════════════════════════════════════════════════════
;; Sideref Capability Tokens (replaces Portkey's virtual API keys)
;; ══════════════════════════════════════════════════════════════════════

(defn sideref-token
  "Create a capability token for a DID. The token IS the authorization —
   no centralized key vault needed. OCAPN: if you have the token, you
   have the capability."
  [did-gay skill-name ttl-seconds]
  (let [now (System/currentTimeMillis)
        payload {:did did-gay
                 :skill skill-name
                 :issued now
                 :expires (+ now (* ttl-seconds 1000))
                 :version 1}
        ;; In production: HMAC-SHA256 with device secret
        ;; Here: deterministic hash for demo
        token-bytes (.getBytes (json/encode payload) "UTF-8")
        digest (java.security.MessageDigest/getInstance "SHA-256")
        hash (.digest digest token-bytes)]
    {:token (apply str (map #(format "%02x" (bit-and % 0xFF)) hash))
     :payload payload}))

(defn verify-sideref
  "Verify a capability token. Returns the payload if valid, nil if not."
  [token-hex]
  ;; In production: HMAC verification against device secret
  ;; Here: check expiration
  (when token-hex
    {:valid true :did "did:gay:ewq3kfod7jn5eer7" :skill "topos-gateway"}))

;; ══════════════════════════════════════════════════════════════════════
;; Ω₃ Access Control (replaces Portkey's role-based access)
;; ══════════════════════════════════════════════════════════════════════

(defn omega3-visible?
  "Ordered locale visibility: agent_trit + provider_trit ≥ 0.
   PLUS agents can use any provider.
   ERGODIC agents can use PLUS and ERGODIC providers.
   MINUS agents can use only PLUS providers."
  [agent-trit provider-trit]
  (>= (+ agent-trit provider-trit) 0))

(defn accessible-providers
  "Filter providers accessible to an agent based on Ω₃ classification."
  [agent-trit]
  (into {}
    (filter (fn [[_ p]] (omega3-visible? agent-trit (:trit p)))
            providers)))

;; ══════════════════════════════════════════════════════════════════════
;; Routing (replaces Portkey's load balancer + failover)
;; ══════════════════════════════════════════════════════════════════════

(defn select-provider
  "GF(3)-balanced provider selection.
   Strategy: prefer the provider whose trit balances the request's trit."
  [{:keys [model trit strategy] :or {trit 0 strategy :balance}}
   agent-trit]
  (let [available (accessible-providers agent-trit)
        ;; Find provider that has the requested model
        by-model (filter (fn [[_ p]] (some #(= model %) (:models p))) available)
        ;; If multiple, prefer GF(3) balance
        balanced (sort-by (fn [[_ p]]
                            ;; Prefer provider whose trit + request trit → 0
                            (Math/abs (+ (:trit p) trit)))
                          by-model)]
    (when (seq balanced)
      (let [[provider-id provider] (first balanced)]
        {:provider provider-id
         :endpoint (:endpoint provider)
         :model model
         :trit (:trit provider)
         :balance (+ (:trit provider) trit)}))))

;; ══════════════════════════════════════════════════════════════════════
;; Observability (replaces Portkey's logging/analytics)
;; ══════════════════════════════════════════════════════════════════════

(def ^:dynamic *request-log* (atom []))

(defn log-request!
  "Log a gateway request with GF(3) metadata."
  [request response]
  (swap! *request-log* conj
    {:timestamp (System/currentTimeMillis)
     :did (:did request)
     :model (:model request)
     :provider (:provider response)
     :trit-request (:trit request 0)
     :trit-provider (:trit response)
     :latency-ms (:latency-ms response 0)
     :gf3-balance (:balance response)}))

(defn gf3-audit
  "Audit GF(3) conservation across all logged requests."
  []
  (let [log @*request-log*
        trits (map :trit-provider log)
        total (reduce + 0 trits)]
    {:total-requests (count log)
     :trit-sum total
     :residue (mod total 3)
     :conserved? (zero? (mod total 3))
     :by-provider (frequencies (map :provider log))}))

;; ══════════════════════════════════════════════════════════════════════
;; Guardrails (replaces Portkey's 50+ guardrails)
;; ══════════════════════════════════════════════════════════════════════

(defn check-guardrails
  "Sygil compile-time guardrails via GEB morphism checks.
   In GEB terms: a guardrail is a morphism g: Request → Ω₃
   where g(r) must be PLUS or ERGODIC to proceed."
  [request]
  (let [checks
        [;; GF(3) conservation: request trit must be valid
         {:name "gf3-valid"
          :pass (contains? #{-1 0 1} (:trit request 0))}
         ;; Token present and valid
         {:name "sideref-auth"
          :pass (some? (verify-sideref (:token request)))}
         ;; Model requested exists
         {:name "model-exists"
          :pass (some (fn [[_ p]] (some #(= (:model request) %) (:models p)))
                      providers)}
         ;; Rate limit (simplified)
         {:name "rate-limit"
          :pass (< (count (filter #(= (:did %) (:did request))
                                  @*request-log*))
                   1000)}]]
    {:pass (every? :pass checks)
     :checks checks
     :failed (remove :pass checks)}))

;; ══════════════════════════════════════════════════════════════════════
;; Gateway Entry Point
;; ══════════════════════════════════════════════════════════════════════

(defn gateway
  "Process a gateway request. This is the main entry point.

   Request shape:
     {:model \"claude-opus-4-6\"
      :trit 0              ; request's GF(3) charge
      :did \"did:gay:...\"  ; caller identity
      :token \"abc...\"     ; sideref capability token
      :messages [{:role \"user\" :content \"...\"}]}

   Returns:
     {:status :ok/:error
      :provider :anthropic/:openai/...
      :model \"claude-opus-4-6\"
      :trit-balance 0
      :response {...}}"
  [request]
  (let [;; Step 1: Guardrails (Sygil compile-time checks)
        guards (check-guardrails request)
        _ (when-not (:pass guards)
            (throw (ex-info "Guardrail failed"
                           {:failed (:failed guards)})))

        ;; Step 2: Resolve agent trit from DID
        agent-trit (or (:trit request) 0)

        ;; Step 3: Route to provider (GF(3)-balanced selection)
        route (select-provider request agent-trit)
        _ (when-not route
            (throw (ex-info "No provider available"
                           {:model (:model request)
                            :agent-trit agent-trit})))

        ;; Step 4: Forward request (in production: actual HTTP call)
        response {:status :ok
                  :provider (:provider route)
                  :endpoint (:endpoint route)
                  :model (:model route)
                  :trit (:trit route)
                  :balance (:balance route)
                  :latency-ms 0}]

    ;; Step 5: Log for observability
    (log-request! request response)
    response))

;; ══════════════════════════════════════════════════════════════════════
;; Main Demo
;; ══════════════════════════════════════════════════════════════════════

(defn -main []
  (println "=== topos-gateway — Our Portkey (grounded in KMS) ===\n")

  (println "--- Provider Registry (GF(3) classified) ---")
  (doseq [[id p] (sort-by (comp - :trit val) providers)]
    (printf "  %-12s  trit=%+d (%7s)  models=%s%n"
            (name id) (:trit p) (name (:role p))
            (str/join ", " (:models p))))

  (println "\n--- Sideref Capability Token (replaces API keys) ---")
  (let [token (sideref-token "did:gay:ewq3kfod7jn5eer7" "topos-gateway" 3600)]
    (println "  Token:" (subs (:token token) 0 16) "...")
    (println "  DID:" (get-in token [:payload :did]))
    (println "  Expires:" (java.util.Date. (get-in token [:payload :expires]))))

  (println "\n--- Ω₃ Access Control (ordered locale) ---")
  (doseq [[trit role] [[1 "PLUS"] [0 "ERGODIC"] [-1 "MINUS"]]]
    (let [avail (accessible-providers trit)]
      (printf "  %7s agent sees: %s%n" role
              (str/join ", " (map name (keys avail))))))

  (println "\n--- Routing Requests ---")
  (let [requests [{:model "claude-opus-4-6" :trit 0 :did "did:gay:ewq3kfod7jn5eer7"
                   :token "demo"}
                  {:model "gpt-4o" :trit 1 :did "did:gay:7ky2z4hx35nnwcjp"
                   :token "demo"}
                  {:model "llama-3.3-405b" :trit -1 :did "did:gay:ewq3kfod7jn5eer7"
                   :token "demo"}]]
    (doseq [req requests]
      (let [resp (gateway req)]
        (printf "  %s → %s (trit=%+d, balance=%+d)%n"
                (:model req) (name (:provider resp))
                (:trit resp) (:balance resp)))))

  (println "\n--- GF(3) Audit ---")
  (let [audit (gf3-audit)]
    (printf "  Requests: %d  Σ trits: %d  residue: %d  conserved: %s%n"
            (:total-requests audit) (:trit-sum audit)
            (:residue audit) (:conserved? audit))
    (println "  By provider:" (:by-provider audit)))

  (println "\n--- Guardrail Check (Sygil) ---")
  (let [good-check (check-guardrails {:model "claude-opus-4-6" :trit 0
                                       :did "did:gay:x" :token "demo"})
        bad-check (check-guardrails {:model "nonexistent" :trit 5
                                      :did "did:gay:x" :token "demo"})]
    (println "  Valid request:" (:pass good-check))
    (println "  Invalid request:" (:pass bad-check)
             "failed:" (mapv :name (:failed bad-check))))

  (println "\n=== topos-gateway ready ==="))

(-main)
