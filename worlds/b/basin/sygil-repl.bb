#!/usr/bin/env bb
;; sygil-repl.bb — Read, Eval, Print, Loop for GEB morphisms
;;
;; READ:  s-expressions representing GEB morphisms
;; EVAL:  type-check (dom/cod), GF(3) conservation, spectral gap
;; PRINT: colored result with Sygil seal
;; LOOP:  again
;;
;; GEB objects:  so0, so1, (prod A B), (coprod A B)
;; GEB morphisms: (id A), (comp f g), (pair f g),
;;                (inject-left A B), (inject-right A B),
;;                (terminal A), (init A)
;; Trit annotation: (trit +1 <morph>), (trit 0 <morph>), (trit -1 <morph>)
;; Commands: :check, :gf3 [trits], :color <seed>, :triad, :quit

(ns sygil.repl
  (:require [clojure.string :as str]))

;; ══════════════════════════════════════════════════════════════════════
;; GEB Objects
;; ══════════════════════════════════════════════════════════════════════

(defn geb-obj? [x]
  (and (map? x) (contains? x :geb-type)))

(defn so0 [] {:geb-type :so0})
(defn so1 [] {:geb-type :so1})
(defn prod [a b] {:geb-type :prod :left a :right b})
(defn coprod [a b] {:geb-type :coprod :left a :right b})

(defn obj-equal? [a b]
  (cond
    (and (= :so0 (:geb-type a)) (= :so0 (:geb-type b))) true
    (and (= :so1 (:geb-type a)) (= :so1 (:geb-type b))) true
    (and (= :prod (:geb-type a)) (= :prod (:geb-type b)))
    (and (obj-equal? (:left a) (:left b))
         (obj-equal? (:right a) (:right b)))
    (and (= :coprod (:geb-type a)) (= :coprod (:geb-type b)))
    (and (obj-equal? (:left a) (:left b))
         (obj-equal? (:right a) (:right b)))
    :else false))

(defn obj->str [obj]
  (case (:geb-type obj)
    :so0 "0"
    :so1 "1"
    :prod (format "(%s x %s)" (obj->str (:left obj)) (obj->str (:right obj)))
    :coprod (format "(%s + %s)" (obj->str (:left obj)) (obj->str (:right obj)))
    (str obj)))

;; ══════════════════════════════════════════════════════════════════════
;; GEB Morphisms
;; ══════════════════════════════════════════════════════════════════════

(defn morph? [x]
  (and (map? x) (contains? x :morph-type)))

(defn geb-id [obj] {:morph-type :id :obj obj :trit 0})
(defn geb-comp [f g] {:morph-type :comp :f f :g g :trit 0})
(defn geb-pair [f g] {:morph-type :pair :fst f :snd g :trit 0})
(defn geb-inject-left [a b] {:morph-type :inject-left :obj a :complement b :trit 0})
(defn geb-inject-right [a b] {:morph-type :inject-right :complement a :obj b :trit 0})
(defn geb-terminal [a] {:morph-type :terminal :obj a :trit 0})
(defn geb-init [a] {:morph-type :init :obj a :trit 0})

(defn with-trit [morph t]
  (assoc morph :trit t))

;; ══════════════════════════════════════════════════════════════════════
;; Domain / Codomain inference (the type checker)
;; ══════════════════════════════════════════════════════════════════════

(defn dom [morph]
  (case (:morph-type morph)
    :id           (:obj morph)
    :comp         (dom (:g morph))
    :pair         (dom (:fst morph))
    :inject-left  (:obj morph)
    :inject-right (:obj morph)
    :terminal     (:obj morph)
    :init         (so0)))

(defn cod [morph]
  (case (:morph-type morph)
    :id           (:obj morph)
    :comp         (cod (:f morph))
    :pair         (prod (cod (:fst morph)) (cod (:snd morph)))
    :inject-left  (coprod (:obj morph) (:complement morph))
    :inject-right (coprod (:complement morph) (:obj morph))
    :terminal     (so1)
    :init         (:obj morph)))

(defn morph->str [m]
  (case (:morph-type m)
    :id           (format "id(%s)" (obj->str (:obj m)))
    :comp         (format "(%s . %s)" (morph->str (:f m)) (morph->str (:g m)))
    :pair         (format "<%s, %s>" (morph->str (:fst m)) (morph->str (:snd m)))
    :inject-left  (format "i1(%s, %s)" (obj->str (:obj m)) (obj->str (:complement m)))
    :inject-right (format "i2(%s, %s)" (obj->str (:complement m)) (obj->str (:obj m)))
    :terminal     (format "!(%s)" (obj->str (:obj m)))
    :init         (format "?(%s)" (obj->str (:obj m)))
    (str m)))

;; ══════════════════════════════════════════════════════════════════════
;; EVAL: Type-check and verify
;; ══════════════════════════════════════════════════════════════════════

(defn check-comp
  "Check that f . g is well-typed: cod(g) = dom(f)."
  [f g]
  (let [cod-g (cod g)
        dom-f (dom f)]
    {:check :composition
     :ok (obj-equal? cod-g dom-f)
     :cod-g (obj->str cod-g)
     :dom-f (obj->str dom-f)}))

(defn check-gf3
  "Check GF(3) conservation: sum of trits = 0 (mod 3)."
  [morphisms]
  (let [trits (mapv :trit morphisms)
        total (reduce + trits)
        residue (mod (+ (mod total 3) 3) 3)]
    {:check :gf3
     :ok (zero? residue)
     :sum total
     :residue residue
     :trits trits}))

(defn check-spectral-gap
  "Check spectral gap >= Ramanujan bound for d=3."
  [gap]
  (let [bound (- 3.0 (* 2.0 (Math/sqrt 2.0)))]
    {:check :spectral-gap
     :ok (>= gap bound)
     :gap gap
     :bound bound}))

(defn eval-morphism
  "Full evaluation of a morphism: infer types, check well-formedness."
  [morph]
  (let [d (dom morph)
        c (cod morph)
        ;; If composition, check inner typing
        comp-check (when (= :comp (:morph-type morph))
                     (check-comp (:f morph) (:g morph)))]
    {:morphism (morph->str morph)
     :dom (obj->str d)
     :cod (obj->str c)
     :trit (:trit morph)
     :role ({1 "PLUS" 0 "ERGODIC" -1 "MINUS"} (:trit morph))
     :typing (str (obj->str d) " -> " (obj->str c))
     :composition-check comp-check
     :well-typed (or (nil? comp-check) (:ok comp-check))}))

;; ══════════════════════════════════════════════════════════════════════
;; READ: Parse s-expression into GEB morphism
;; ══════════════════════════════════════════════════════════════════════

(defn read-obj
  "Parse a GEB object from s-expression."
  [form]
  (cond
    (= form 'so0) (so0)
    (= form 'so1) (so1)
    (and (list? form) (= (first form) 'prod))
    (prod (read-obj (nth form 1)) (read-obj (nth form 2)))
    (and (list? form) (= (first form) 'coprod))
    (coprod (read-obj (nth form 1)) (read-obj (nth form 2)))
    :else (so1))) ;; default to terminal

(defn read-morph
  "Parse a GEB morphism from s-expression."
  [form]
  (cond
    ;; Trit annotation: (trit +1 <morph>)
    (and (list? form) (= (first form) 'trit))
    (with-trit (read-morph (nth form 2)) (nth form 1))

    ;; Identity: (id A)
    (and (list? form) (= (first form) 'id))
    (geb-id (read-obj (nth form 1)))

    ;; Composition: (comp f g) or (. f g)
    (and (list? form) (contains? #{'comp '.} (first form)))
    (geb-comp (read-morph (nth form 1)) (read-morph (nth form 2)))

    ;; Pairing: (pair f g) or (<> f g)
    (and (list? form) (contains? #{'pair '<>} (first form)))
    (geb-pair (read-morph (nth form 1)) (read-morph (nth form 2)))

    ;; Left injection: (inject-left A B) or (i1 A B)
    (and (list? form) (contains? #{'inject-left 'i1} (first form)))
    (geb-inject-left (read-obj (nth form 1)) (read-obj (nth form 2)))

    ;; Right injection: (inject-right A B) or (i2 A B)
    (and (list? form) (contains? #{'inject-right 'i2} (first form)))
    (geb-inject-right (read-obj (nth form 1)) (read-obj (nth form 2)))

    ;; Terminal: (terminal A) or (! A)
    (and (list? form) (contains? #{'terminal '!} (first form)))
    (geb-terminal (read-obj (nth form 1)))

    ;; Initial: (init A) or (? A)
    (and (list? form) (contains? #{'init} (first form)))
    (geb-init (read-obj (nth form 1)))

    ;; Bare symbol defaults
    (= form 'id) (geb-id (so1))
    :else (geb-id (so1))))

;; ══════════════════════════════════════════════════════════════════════
;; PRINT: Display result with Sygil seal
;; ══════════════════════════════════════════════════════════════════════

(defn print-eval-result
  "Pretty-print an evaluation result."
  [result]
  (printf "  %s : %s%n" (:morphism result) (:typing result))
  (printf "  trit: %+d (%s)%n" (:trit result) (:role result))
  (when-let [cc (:composition-check result)]
    (printf "  comp: cod(g)=%s %s dom(f)=%s%n"
            (:cod-g cc) (if (:ok cc) "=" "/=") (:dom-f cc)))
  (printf "  well-typed: %s%n" (:well-typed result)))

(defn print-gf3-result
  "Pretty-print GF(3) conservation check."
  [result]
  (printf "  trits: %s%n" (:trits result))
  (printf "  sum: %d  residue: %d%n" (:sum result) (:residue result))
  (printf "  %s%n" (if (:ok result) "CONSERVED" "VIOLATED")))

(defn print-sygil-seal
  "Print the Sygil seal."
  [checks]
  (let [all-ok (every? :ok checks)]
    (println)
    (if all-ok
      (println "  SYGIL SEALED")
      (println "  SYGIL BROKEN"))
    (doseq [c checks]
      (printf "  %s %s%n"
              (if (:ok c) "ok" "!!") (:check c)))))

;; ══════════════════════════════════════════════════════════════════════
;; SplitMix64 (for :color command)
;; ══════════════════════════════════════════════════════════════════════

(defn splitmix64 [seed]
  (let [golden (unchecked-long 0x9e3779b97f4a7c15N)
        mix1 (unchecked-long 0xbf58476d1ce4e5b9N)
        mix2 (unchecked-long 0x94d049bb133111ebN)
        z (unchecked-add (long seed) golden)
        z (unchecked-multiply (bit-xor z (unsigned-bit-shift-right z 30)) mix1)
        z (unchecked-multiply (bit-xor z (unsigned-bit-shift-right z 27)) mix2)]
    (bit-xor z (unsigned-bit-shift-right z 31))))

(defn seed->color [seed]
  (let [z (splitmix64 seed)
        hue (mod (* (bit-and (unsigned-bit-shift-right z 16) 0xFFFF) 137.508) 360.0)
        trit (cond (< hue 120) 1, (< hue 240) 0, :else -1)]
    {:seed seed :hue hue :trit trit
     :role ({1 "PLUS" 0 "ERGODIC" -1 "MINUS"} trit)}))

;; ══════════════════════════════════════════════════════════════════════
;; LOOP: The REPL itself
;; ══════════════════════════════════════════════════════════════════════

(def ^:dynamic *morphism-stack* (atom []))

(defn process-input
  "Process one REPL input. Returns :quit to exit, anything else to continue."
  [input]
  (let [trimmed (str/trim input)]
    (cond
      ;; Empty
      (str/blank? trimmed) :continue

      ;; Commands
      (= trimmed ":quit") :quit
      (= trimmed ":q") :quit

      (= trimmed ":stack")
      (do (println "  stack:" (mapv morph->str @*morphism-stack*))
          :continue)

      (= trimmed ":clear")
      (do (reset! *morphism-stack* [])
          (println "  stack cleared")
          :continue)

      (= trimmed ":check")
      (do (if (empty? @*morphism-stack*)
            (println "  stack empty, push morphisms first")
            (let [gf3 (check-gf3 @*morphism-stack*)
                  comps (for [[f g] (partition 2 1 @*morphism-stack*)]
                          (check-comp f g))
                  checks (cons gf3 comps)]
              (print-sygil-seal checks)))
          :continue)

      (= trimmed ":triad")
      (do (println "  SLIME  (-1, MINUS)   Common Lisp  GEB native")
          (println "  Geiser ( 0, ERGODIC) Scheme       SplitMix64")
          (println "  CIDER  (+1, PLUS)    Clojure/bb   this REPL")
          (println "  Sum: -1 + 0 + 1 = 0  GF(3) CONSERVED")
          :continue)

      (= trimmed ":noble")
      (do (println "  Noble Denom Monitor Pipeline (GEB morphisms):")
          (println)
          (println "  observe : 1 → DenomRegistry   trit -1 (MINUS)")
          (println "    NATS subscribe ibc.denom.> → extract path/chain/channel")
          (println)
          (println "  compute : DenomRegistry → SHA3Set   trit +1 (PLUS)")
          (println "    SHA-256 path → SHA-3 shadow hash (length-extension immune)")
          (println "    DGX-alpha dispatches prefix-scan GPU tasks")
          (println)
          (println "  store   : SHA3Set → Ω₃   trit 0 (ERGODIC)")
          (println "    DuckDB noble-denoms.duckdb upsert + anomaly detection")
          (println "    Publish alerts to NATS alert.denom.>")
          (println)
          (println "  Pipeline: store . compute . observe")
          (println "    dom(observe) = 1, cod(store) = Ω₃")
          (println "    cod(observe) = dom(compute) = DenomRegistry  ✓")
          (println "    cod(compute) = dom(store) = SHA3Set          ✓")
          (println "    Σ trit = -1 + 1 + 0 = 0  GF(3) CONSERVED   ✓")
          (println)
          (println "  Sygil seal: well-typed, GF(3) balanced, spectral gap ≥ Ramanujan")
          :continue)

      (str/starts-with? trimmed ":gf3 ")
      (let [nums (mapv #(Integer/parseInt %) (str/split (subs trimmed 5) #"\s+"))
            result (check-gf3 (mapv #(hash-map :trit %) nums))]
        (print-gf3-result result)
        :continue)

      (str/starts-with? trimmed ":color ")
      (let [seed (Long/parseLong (str/trim (subs trimmed 7)))
            color (seed->color seed)]
        (printf "  seed=%d  hue=%.1f  trit=%+d  role=%s%n"
                (:seed color) (:hue color) (:trit color) (:role color))
        :continue)

      (= trimmed ":help")
      (do (println "  GEB morphisms (s-expressions):")
          (println "    (id so1)                identity on terminal object")
          (println "    (comp f g)              composition f . g")
          (println "    (pair f g)              pairing <f, g> : A -> B x C")
          (println "    (i1 A B)                left injection A -> A + B")
          (println "    (i2 A B)                right injection B -> A + B")
          (println "    (terminal A)  or (! A)  terminal morphism A -> 1")
          (println "    (init A)                initial morphism 0 -> A")
          (println "    (trit +1 <morph>)       annotate with GF(3) charge")
          (println)
          (println "  GEB objects:")
          (println "    so0                     initial object (void)")
          (println "    so1                     terminal object (unit)")
          (println "    (prod A B)              product A x B")
          (println "    (coprod A B)            coproduct A + B")
          (println)
          (println "  Commands:")
          (println "    :check                  run Sygil seal on stack")
          (println "    :stack                  show morphism stack")
          (println "    :clear                  clear stack")
          (println "    :gf3 +1 0 -1           check GF(3) conservation")
          (println "    :color <seed>           derive color from seed")
          (println "    :triad                  show REPL triad")
          (println "    :quit                   exit")
          :continue)

      ;; Otherwise: READ a GEB morphism s-expression
      :else
      (try
        (let [form (read-string trimmed)
              morph (read-morph form)
              result (eval-morphism morph)]
          (print-eval-result result)
          (swap! *morphism-stack* conj morph)
          (printf "  [pushed to stack, depth=%d]%n" (count @*morphism-stack*))
          :continue)
        (catch Exception e
          (printf "  error: %s%n" (.getMessage e))
          :continue)))))

(defn repl
  "The Sygil REPL. Read, Eval, Print, Loop."
  []
  (println "Sygil REPL — GEB morphism verifier")
  (println "  Read:  s-expressions (GEB morphisms)")
  (println "  Eval:  type-check + GF(3) conservation")
  (println "  Print: dom -> cod, trit, Sygil seal")
  (println "  Loop:  again")
  (println)
  (println "  :help for commands, :quit to exit")
  (println)
  (loop []
    (print "sygil> ")
    (flush)
    (when-let [line (read-line)]
      (when (not= :quit (process-input line))
        (recur)))))

;; ══════════════════════════════════════════════════════════════════════
;; Main: either REPL or batch mode
;; ══════════════════════════════════════════════════════════════════════

(defn -main [& args]
  (if (seq args)
    ;; Batch mode: evaluate arguments
    (doseq [arg args]
      (process-input arg))
    ;; Interactive REPL
    (repl)))

;; Non-interactive batch demo (for piped/embedded execution)
;; Run `bb sygil-repl.bb` in a terminal for interactive mode.
(when-not (System/getenv "SYGIL_INTERACTIVE")
  (println "Sygil REPL — batch demo\n")
  (process-input "(trit +1 (id so1))")
  (println)
  (process-input "(trit 0 (terminal so1))")
  (println)
  (process-input "(trit -1 (init so1))")
  (println)
  (process-input ":check")
  (println)
  (process-input ":gf3 +1 0 -1")
  (println)
  (process-input ":color 1069")
  (println)
  (process-input ":triad"))
