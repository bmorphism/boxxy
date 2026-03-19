#!/usr/bin/env bb
;; Gadget 1: Harmonic Centrality → GF(3) Ordered Locale
;;
;; The ordered locale is a frame (complete Heyting algebra) where:
;;   Opens are up-sets in the centrality poset.
;;   The GF(3) coordinate partitions centrality into three bands.
;;   The subobject classifier Ω₃ assigns trit values via χ: Nodes → Ω₃.
;;
;; Visibility in the ordered locale:
;;   visible(agent, record) ⟺ agent_trit + record_conf ≥ 0
;;   This is the GF(3) inner product in Z, projected to the locale frame.

(ns gadget.harmonic-centrality-gf3
  (:require [clojure.string :as str]))

;; --- Graph (adjacency list) ---

(defn add-edge [g [u v & [attrs]]]
  (-> g
      (update-in [:adj u] (fnil conj #{}) v)
      (update-in [:adj v] (fnil conj #{}) u)
      (update-in [:edges] (fnil conj []) [u v (or attrs {})])))

(defn build-graph [edges]
  (let [g (reduce add-edge {:adj {} :edges []} edges)
        nodes (set (keys (:adj g)))]
    (assoc g :nodes nodes)))

(defn neighbors [g node]
  (get-in g [:adj node] #{}))

;; --- BFS shortest paths ---

(defn bfs-distances
  "BFS from source, returns {node distance} for all reachable nodes."
  [g source]
  (loop [queue (conj clojure.lang.PersistentQueue/EMPTY source)
         visited {source 0}]
    (if (empty? queue)
      visited
      (let [current (peek queue)
            d (get visited current)
            nbrs (remove visited (neighbors g current))
            new-visited (reduce #(assoc %1 %2 (inc d)) visited nbrs)
            new-queue (reduce conj (pop queue) nbrs)]
        (recur new-queue new-visited)))))

;; --- Harmonic Centrality ---
;; H(u) = Σ_{v≠u} 1/d(v,u)
;; where d(v,u) is shortest-path distance, and 1/∞ = 0

(defn harmonic-centrality
  "Compute harmonic centrality for all nodes.
   Returns {node centrality-value}."
  [g]
  (let [nodes (:nodes g)]
    (into {}
      (for [u nodes]
        (let [dists (bfs-distances g u)
              hc (reduce-kv
                   (fn [acc v d]
                     (if (and (not= v u) (pos? d))
                       (+ acc (/ 1.0 d))
                       acc))
                   0.0
                   dists)]
          [u hc])))))

;; --- GF(3) Ordered Locale ---

(defn centrality->trit
  "χ: [0,1] → Ω₃ = {-1, 0, +1} — the characteristic morphism.

   The ordered locale has three opens forming a chain:
     U₋ ⊂ U₋∪U₀ ⊂ U₋∪U₀∪U₊ = X"
  [normalized t-lo t-hi]
  (cond
    (> normalized t-hi) 1   ; PLUS: high centrality → generator
    (> normalized t-lo) 0   ; ERGODIC: medium → coordinator
    :else                -1)) ; MINUS: low centrality → validator

(defn trit->wire
  "FFI-safe wire encoding. Never signed integers across boundaries.
   Byte 0: SIGN (0x00=ERGODIC, 0x01=PLUS, 0x02=MINUS)
   Byte 1: SIGNIFICAND (hue sector)"
  [trit hue]
  (let [sign (case (int trit) 1 0x01, 0 0x00, -1 0x02)
        sig (bit-and (int (* (/ hue 360.0) 256)) 0xFF)]
    {:sign sign :significand sig}))

(defn visible?
  "Ordered locale visibility: a + r ≥ 0.

   PLUS(+1)    sees: PUBLIC(+1)✓ CONF(0)✓ SECRET(-1)✓
   ERGODIC(0)  sees: PUBLIC(+1)✓ CONF(0)✓ SECRET(-1)✗
   MINUS(-1)   sees: PUBLIC(+1)✓ CONF(0)✗ SECRET(-1)✗"
  [agent-trit record-conf]
  (>= (+ agent-trit record-conf) 0))

;; --- Gadget Pipeline ---

(defn harmonic-centrality-gadget
  "Complete gadget: graph → centrality → GF(3) ordered locale."
  [g]
  (let [hc (harmonic-centrality g)
        max-c (apply max (vals hc))
        max-c (if (zero? max-c) 1.0 max-c)
        ;; Adaptive thresholds from distribution
        sorted-vals (sort (map #(/ % max-c) (vals hc)))
        n (count sorted-vals)
        t-lo (if (>= n 3) (nth sorted-vals (quot n 3)) 0.33)
        t-hi (if (>= n 3) (nth sorted-vals (quot (* 2 n) 3)) 0.66)]
    (into {}
      (map-indexed
        (fn [i [node c]]
          (let [norm (/ c max-c)
                trit (centrality->trit norm t-lo t-hi)
                hue (mod (* i 137.508) 360.0)
                wire (trit->wire trit hue)]
            [node {:centrality (Math/round (* c 10000.0) )
                   :centrality-raw (/ (Math/round (* c 10000.0)) 10000.0)
                   :normalized (/ (Math/round (* norm 1000.0)) 1000.0)
                   :trit trit
                   :role ({1 "PLUS" 0 "ERGODIC" -1 "MINUS"} trit)
                   :hue (/ (Math/round (* hue 10.0)) 10.0)
                   :wire wire}]))
        (sort-by (comp - val) hc)))))

(defn gf3-conservation [results]
  (let [trits (map :trit (vals results))
        total (reduce + trits)]
    {:total total
     :residue (mod total 3)
     :conserved? (zero? (mod total 3))
     :counts {:PLUS (count (filter #(= 1 %) trits))
              :ERGODIC (count (filter #(= 0 %) trits))
              :MINUS (count (filter #(= -1 %) trits))}}))

(defn resource-sharing [results]
  (let [donors    (map first (filter #(= 1 (:trit (val %))) results))
        routers   (map first (filter #(= 0 (:trit (val %))) results))
        receivers (map first (filter #(= -1 (:trit (val %))) results))]
    {:donors (vec donors)
     :routers (vec routers)
     :receivers (vec receivers)
     :flows (when (and (seq routers) (seq receivers))
              (vec (map-indexed
                     (fn [i d]
                       {:from d
                        :via (nth (vec routers) (mod i (count routers)))
                        :to (nth (vec receivers) (mod i (count receivers)))})
                     donors)))}))

;; --- Main ---

(defn -main []
  (println "=== Gadget 1: Harmonic Centrality → GF(3) Ordered Locale ===\n")

  (let [edges [["did:gay:ewq3kfod7jn5eer7" "did:plc:abc123"           {:type "same_identity"}]
               ["did:gay:ewq3kfod7jn5eer7" "did:ens:bmorphism.eth"    {:type "same_identity"}]
               ["did:gay:ewq3kfod7jn5eer7" "did:gay:7ky2z4hx35nnwcjp" {:type "follows"}]
               ["did:gay:ewq3kfod7jn5eer7" "did:plc:def456"           {:type "follows"}]
               ["did:gay:ewq3kfod7jn5eer7" "did:plc:ghi789"           {:type "follows"}]
               ["did:gay:ewq3kfod7jn5eer7" "did:gay:validator01"      {:type "trusts"}]
               ["did:gay:ewq3kfod7jn5eer7" "did:gay:validator02"      {:type "trusts"}]
               ["did:gay:7ky2z4hx35nnwcjp" "did:ens:vitalik.eth"      {:type "follows"}]
               ["did:plc:def456"           "did:plc:ghi789"            {:type "follows"}]
               ["did:gay:validator01"      "did:gay:validator02"       {:type "peer"}]]
        g (build-graph edges)
        results (harmonic-centrality-gadget g)
        conservation (gf3-conservation results)
        allocation (resource-sharing results)]

    (println (format "Graph: %d nodes, %d edges\n"
                     (count (:nodes g)) (count (:edges g))))

    (println "--- Harmonic Centrality × GF(3) Classification ---")
    (doseq [[node r] (sort-by (comp - :centrality-raw val) results)]
      (printf "  %-35s  c=%.3f  trit=%+d (%7s)  hue=%5.1f°  wire=[0x%02X 0x%02X]%n"
              node (:normalized r) (:trit r) (:role r)
              (:hue r) (:sign (:wire r)) (:significand (:wire r))))

    (println "\n--- GF(3) Conservation ---")
    (printf "  Σ trits = %d  residue = %d  conserved = %s%n"
            (:total conservation) (:residue conservation)
            (:conserved? conservation))
    (printf "  PLUS: %d  ERGODIC: %d  MINUS: %d%n"
            (get-in conservation [:counts :PLUS])
            (get-in conservation [:counts :ERGODIC])
            (get-in conservation [:counts :MINUS]))

    (println "\n--- Resource-Sharing Allocation ---")
    (println "  Donors   (PLUS):"    (:donors allocation))
    (println "  Routers  (ERGODIC):" (:routers allocation))
    (println "  Receivers (MINUS):"  (:receivers allocation))
    (when (:flows allocation)
      (println "  Flows:")
      (doseq [f (:flows allocation)]
        (printf "    %s → %s → %s%n" (:from f) (:via f) (:to f))))

    (println "\n--- Ordered Locale Visibility (a + r ≥ 0) ---")
    (doseq [[a-trit a-name] [[1 "PLUS"] [0 "ERGODIC"] [-1 "MINUS"]]]
      (let [vis (for [r [1 0 -1]
                      :when (visible? a-trit r)]
                  ({1 "PUBLIC" 0 "CONFIDENTIAL" -1 "SECRET"} r))]
        (printf "  %7s (trit=%+d) sees: %s%n" a-name a-trit (vec vis))))

    (println "\n=== Gadget complete ===")))

(-main)
