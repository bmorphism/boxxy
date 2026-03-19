#!/usr/bin/env bb
;; aellith-tile.bb
;;
;; Boxxy stripped to essentials, rewritten in the Aellith ontology.
;;
;; MULTIVERSAL ACAUSAL frame (compact closed, self-dual generator):
;;   All tilings coexist. Decision = logical selection, not causal intervention.
;;   Cups/caps create correlated VM pairs without data flow.
;;
;; CAUSAL frame (symmetric monoidal, directed flow):
;;   Tilings are schedules. boot→run→stop is temporal. Wires carry data.
;;
;; AELLITH / aella mapping:
;;   The 9 Aellith domains ARE the tile's internal lifecycle:
;;     boot  = ATTACHMENT (+1)  — first contact, lips meeting
;;     run   = POWER (0)        — command, execution, authority
;;     stop  = SHAME (-1)       — the catch, the stop, glottal closure
;;   GF(3) conservation: +1 + 0 + (-1) = 0 ✓
;;
;; PHONETIC-SEMANTIC-CHROMATIC interface (topos of music):
;;   Each tile state encodes as IPA phoneme + Mazzola form + Gay.jl color.
;;   The tile's lifecycle IS a musical morphism: P→L→R in Neo-Riemannian space.
;;
;; NIPS2017 normal form:
;;   Path-invariant retrieval via jp2 color hash.
;;   content_hash ∥ wavelet_LL_coefficients ∥ metadata = canonical form.

(require '[clojure.string :as str])

;; ═══════════════════════════════════════════════════════════════════
;; I. THE TILE — stripped to essentials
;; ═══════════════════════════════════════════════════════════════════

;; The one generator. Everything else is tiling.
(def tile
  {:name "VM"
   :type :morphism
   :domain  [:guest-state :config]   ; ⊗ product
   :codomain [:guest-state' :result] ; ⊗ product
   :lifecycle [:boot :run :stop]
   :frame nil})                      ; set to :causal or :acausal

;; ═══════════════════════════════════════════════════════════════════
;; II. AELLITH DOMAINS — the tile's lifecycle as experience
;; ═══════════════════════════════════════════════════════════════════

(def aellith-domains
  [{:domain :attachment :trit +1 :place :bilabial  :voiced "b" :nasal "m" :voiceless "p"
    :lifecycle :boot :desc "first contact, lips meeting, nursing"
    :color "#DD3C3C" :hue 0 :plr :P}
   {:domain :desire     :trit +1 :place :labiodental :voiced "v" :nasal "ɱ" :voiceless "f"
    :lifecycle nil :desc "wanting gesture, lip-bite"}
   {:domain :trust      :trit  0 :place :dental :voiced "ð" :nasal "n̪" :voiceless "θ"
    :lifecycle nil :desc "vulnerability, tongue exposed"}
   {:domain :power      :trit  0 :place :alveolar :voiced "d" :nasal "n" :voiceless "t"
    :lifecycle :run :desc "command, percussion, authority"
    :color "#3C99DD" :hue 205 :plr :L}
   {:domain :trauma     :trit -1 :place :postalveolar :voiced "ʒ" :nasal "ɲ̊" :voiceless "ʃ"
    :lifecycle nil :desc "sibilant recoil, hissing withdrawal"}
   {:domain :status     :trit  0 :place :palatal :voiced "ɟ" :nasal "ɲ" :voiceless "c"
    :lifecycle nil :desc "elevated, aspiration"}
   {:domain :jealousy   :trit -1 :place :velar :voiced "g" :nasal "ŋ" :voiceless "k"
    :lifecycle nil :desc "guttural possession, swallowing"}
   {:domain :disgust    :trit -1 :place :uvular :voiced "ɢ" :nasal "ɴ" :voiceless "q"
    :lifecycle nil :desc "gag reflex, retching"}
   {:domain :shame      :trit -1 :place :glottal :voiced "ɦ" :nasal "∅" :voiceless "ʔ"
    :lifecycle :stop :desc "the catch, the stop, caught breath"
    :color "#DDC33C" :hue 50 :plr :R}])

(def lifecycle-domains
  (filterv :lifecycle aellith-domains))

;; ═══════════════════════════════════════════════════════════════════
;; III. PHONETIC ENCODING — tile lifecycle as pronounceable word
;; ═══════════════════════════════════════════════════════════════════

(defn phoneme-for [domain trit-mode]
  (let [d (first (filter #(= (:domain %) domain) aellith-domains))]
    (case trit-mode
      :plus     (:voiced d)
      :zero     (:nasal d)
      :minus    (:voiceless d)
      (:voiced d))))

(defn vowel-for [intensity temporality]
  (let [vowels {:overwhelming {:present "a" :persistent "ɐ" :past "ɑ"}
                :strong       {:present "ɛ" :persistent "ɜ" :past "ɔ"}
                :moderate     {:present "e" :persistent "ə" :past "o"}
                :mild         {:present "ɪ" :persistent "ɨ" :past "ʊ"}
                :subliminal   {:present "i" :persistent "ɯ" :past "u"}}]
    (get-in vowels [intensity temporality] "ə")))

(defn tile-word
  "Encode the tile lifecycle as an IPA word.
   boot(b) + vowel + run(n) + vowel + stop(ʔ)
   The word IS the tile."
  [intensity temporality]
  (let [boot-phoneme (phoneme-for :attachment :plus)   ; b
        run-phoneme  (phoneme-for :power :zero)        ; n
        stop-phoneme (phoneme-for :shame :minus)       ; ʔ
        v (vowel-for intensity temporality)]
    (str "/" boot-phoneme v "." run-phoneme v "." stop-phoneme v "/")))

;; ═══════════════════════════════════════════════════════════════════
;; IV. MAZZOLA FORMS — tile as musical morphism
;; ═══════════════════════════════════════════════════════════════════

;; Mazzola's Forms: Simple (Z,R), Limit (product), Colimit (sum), List (powerset)
(def tile-form
  {:type :limit
   :name :VMTile
   :factors [{:type :simple :name :GuestState :module :Z}
             {:type :simple :name :Config :module :Z}]})

(def tile-result-form
  {:type :limit
   :name :VMResult
   :factors [{:type :simple :name :GuestState' :module :Z}
             {:type :simple :name :Result :module :Z}]})

;; Neo-Riemannian PLR: the lifecycle IS a PLR chain
;; P (boot/attachment) → L (run/power) → R (stop/shame)
;; This traces the Tonnetz path: major → minor → relative minor
(def plr-chain [:P :L :R])

(defn plr-transform [triad op]
  (let [[r t f] triad]
    (case op
      :P [(mod r 12) (mod (if (= (mod (- t r) 12) 4) (dec t) (inc t)) 12) (mod f 12)]
      :L (if (= (mod (- t r) 12) 4)
           [(mod (dec f) 12) t f]
           [r t (mod (inc r) 12)])
      :R (if (= (mod (- t r) 12) 4)
           [r t (mod (+ r 9) 12)]
           [(mod (+ f 3) 12) t f])
      triad)))

(defn lifecycle-as-plr
  "Apply PLR chain to C-major triad, one step per lifecycle phase."
  []
  (reduce (fn [{:keys [triad history]} op]
            (let [new-triad (plr-transform triad op)]
              {:triad new-triad
               :history (conj history {:op op :from triad :to new-triad})}))
          {:triad [0 4 7] :history []}
          plr-chain))

;; ═══════════════════════════════════════════════════════════════════
;; V. FRAMES — causal vs acausal
;; ═══════════════════════════════════════════════════════════════════

(defn causal-tile
  "Symmetric monoidal category: directed flow, temporal ordering.
   boot THEN run THEN stop. Wires carry data downward."
  []
  {:frame :causal
   :structure :symmetric-monoidal
   :generator tile
   :composition [:sequential :parallel :swap]
   :equivalence :planar-isotopy
   :lifecycle-order [:boot :run :stop]
   :time-direction :downward
   :word (tile-word :moderate :present)})

(defn acausal-tile
  "Compact closed category: cups/caps, no preferred time direction.
   boot-run-stop is a logical relation, not temporal sequence."
  []
  {:frame :acausal
   :structure :compact-closed
   :generator (assoc tile :self-dual true)
   :composition [:sequential :parallel :swap :cup :cap]
   :equivalence :planar-isotopy+compact
   :lifecycle-order nil
   :time-direction nil
   :word (tile-word :moderate :persistent)})

(defn multiversal
  "Presheaf topos over tiling contexts. All tilings coexist.
   Decision = logical selection (FDT), not causal intervention (CDT)."
  [frame]
  {:multiverse true
   :topos [:presheaf :over (if (= frame :acausal)
                             :free-compact-closed
                             :free-symmetric-monoidal)]
   :logic :intuitionistic
   :truth :local
   :decision-theory (if (= frame :acausal) :FDT :CDT)
   :yoneda-faithful true})

;; ═══════════════════════════════════════════════════════════════════
;; VI. JP2 COLOR HASH — NIPS2017 path-invariant normal form
;; ═══════════════════════════════════════════════════════════════════

(defn rct
  "Reversible Color Transform (JPEG2000 lossless).
   [R G B] → [Y Cb Cr], integer exact."
  [[r g b]]
  (let [y  (quot (+ r (* 2 g) b) 4)
        cb (- b g)
        cr (- r g)]
    [y cb cr]))

(defn rct-inverse
  "RCT inverse: [Y Cb Cr] → [R G B]."
  [[y cb cr]]
  (let [g (- y (quot (+ cb cr) 4))
        r (+ cr g)
        b (+ cb g)]
    [r g b]))

(defn hex->rgb [hex]
  (let [h (str/replace hex "#" "")]
    [(Integer/parseInt (subs h 0 2) 16)
     (Integer/parseInt (subs h 2 4) 16)
     (Integer/parseInt (subs h 4 6) 16)]))

(defn content-hash
  "SHA-256 stub for content-addressable normal form."
  [s]
  (let [md (java.security.MessageDigest/getInstance "SHA-256")
        bytes (.digest md (.getBytes (str s) "UTF-8"))]
    (str/join (map #(format "%02x" (bit-and % 0xff)) bytes))))

(defn jp2-color-hash
  "JP2-style color hash: RGB → RCT → hash.
   Simulates the coarsest-level LL subband fingerprint."
  [hex-colors]
  (let [ycbcr-values (mapv (comp rct hex->rgb) hex-colors)
        flat (flatten ycbcr-values)]
    (content-hash (str/join "," flat))))

(def nips2017-papers
  [{:id "1706.03762" :title "Attention Is All You Need"     :authors "Vaswani et al."}
   {:id "1710.09829" :title "Dynamic Routing Between Capsules" :authors "Sabour et al."}
   {:id "1703.03400" :title "MAML"                          :authors "Finn et al."}
   {:id "1705.07874" :title "GraphSAGE"                     :authors "Hamilton et al."}
   {:id "1705.07215" :title "SHAP"                          :authors "Lundberg & Lee"}
   {:id "1705.08039" :title "Poincaré Embeddings"           :authors "Nickel & Kiela"}])

(defn normal-form
  "Path-invariant normal form: content ∥ jp2_color ∥ metadata."
  [paper tile-colors]
  (let [text-h (content-hash (:title paper))
        jp2-h  (jp2-color-hash tile-colors)
        meta-h (content-hash (str (:id paper) "|" (:authors paper)))]
    {:paper (:title paper)
     :text-hash (subs text-h 0 16)
     :jp2-hash  (subs jp2-h 0 16)
     :meta-hash (subs meta-h 0 16)
     :normal-form (content-hash (str text-h jp2-h meta-h))
     :path-invariant true}))

;; ═══════════════════════════════════════════════════════════════════
;; VII. MAIN — run all frames
;; ═══════════════════════════════════════════════════════════════════

(defn run []
  (println "═══════════════════════════════════════════════════════════")
  (println "   BOXXY → AELLITH TILE")
  (println "   stripped to essentials: one tile, two frames, nine domains")
  (println "═══════════════════════════════════════════════════════════")
  (println)

  ;; Lifecycle domains
  (println "── TILE LIFECYCLE AS AELLITH ──")
  (doseq [d lifecycle-domains]
    (println (format "  %-10s  trit=%+d  IPA=%-3s  PLR=%s  %s  %s"
                     (name (:lifecycle d))
                     (:trit d)
                     (:voiced d)
                     (name (:plr d))
                     (:color d)
                     (:desc d))))
  (println (format "  GF(3) sum: %d ✓" (reduce + (map :trit lifecycle-domains))))
  (println)

  ;; Phonetic encoding
  (println "── PHONETIC TILE WORD ──")
  (doseq [[intensity temp] [[:moderate :present] [:strong :past] [:subliminal :persistent]]]
    (println (format "  %s × %s → %s"
                     (name intensity) (name temp)
                     (tile-word intensity temp))))
  (println)

  ;; PLR chain
  (let [{:keys [history]} (lifecycle-as-plr)]
    (println "── NEO-RIEMANNIAN PLR CHAIN (Tonnetz path) ──")
    (doseq [h history]
      (println (format "  %s: %s → %s" (name (:op h)) (str (:from h)) (str (:to h)))))
    (println))

  ;; Causal frame
  (let [c (causal-tile)]
    (println "── CAUSAL FRAME (symmetric monoidal) ──")
    (println (format "  Structure: %s" (name (:structure c))))
    (println (format "  Lifecycle: %s" (str/join " → " (map name (:lifecycle-order c)))))
    (println (format "  Tile word: %s" (:word c)))
    (println (format "  Decision: CDT (interventionist)")))
  (println)

  ;; Acausal frame
  (let [a (acausal-tile)]
    (println "── ACAUSAL FRAME (compact closed) ──")
    (println (format "  Structure: %s" (name (:structure a))))
    (println (format "  Self-dual: %s" (get-in a [:generator :self-dual])))
    (println (format "  Tile word: %s" (:word a)))
    (println (format "  Decision: FDT (logical selection)")))
  (println)

  ;; Multiversal
  (doseq [frame [:causal :acausal]]
    (let [m (multiversal frame)]
      (println (format "── MULTIVERSAL %s ──" (str/upper-case (name frame))))
      (println (format "  Topos: %s" (str (:topos m))))
      (println (format "  Decision theory: %s" (name (:decision-theory m))))
      (println (format "  Yoneda faithful: %s" (:yoneda-faithful m)))))
  (println)

  ;; JP2 color hash / NIPS2017 normal forms
  (let [tile-colors (mapv :color lifecycle-domains)]
    (println "── JP2 COLOR HASH (RCT transform) ──")
    (doseq [c tile-colors]
      (let [rgb (hex->rgb c)
            ycbcr (rct rgb)]
        (println (format "  %s  RGB=%s  YCbCr=%s" c (str rgb) (str ycbcr)))))
    (println)

    (println "── NIPS2017 NORMAL FORMS (path-invariant) ──")
    (doseq [paper nips2017-papers]
      (let [nf (normal-form paper tile-colors)]
        (println (format "  %-40s NF=%s" (:paper nf) (subs (:normal-form nf) 0 24))))))
  (println)

  ;; All 9 domains phoneme table
  (println "── FULL AELLITH PHONEME TABLE ──")
  (println "  DOMAIN       +1(voiced)  0(nasal)  -1(voiceless)  place")
  (println "  ──────────── ────────── ──────── ─────────────── ─────────────")
  (doseq [d aellith-domains]
    (println (format "  %-12s  %-9s  %-7s  %-14s  %s"
                     (name (:domain d))
                     (:voiced d) (:nasal d) (:voiceless d)
                     (name (:place d)))))
  (println)
  (println "═══════════════════════════════════════════════════════════")
  (println "   TILE COMPLETE. Same generator. Two frames. Nine mouths.")
  (println "═══════════════════════════════════════════════════════════"))

(run)
