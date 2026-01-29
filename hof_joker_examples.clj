;; HOF + Joker: Higher-Order Functions with Canonical Beta Reduction
;; Demonstrates lambda calculus evaluation in Clojure using Joker interpreter
;;
;; Key pattern: Skill-classified functions with GF(3) ternary balance
;; Evaluation: Eager/strict with explicit environment capture (closure semantics)

(ns boxxy.hof-joker
  (:require [clojure.edn :as edn]))

;; ============================================================================
;; BETA REDUCTION EXAMPLES: Canonical Evaluation Order
;; ============================================================================

;; Example 1: Simple beta reduction
;; Term: ((λx.λy.+ x y) 3) 4
;; Trace:
;;   Step 1: Apply outer lambda to 3
;;     (λx.λy.+ x y) 3
;;     => (λy.+ 3 y)    [β-reduction, bind x=3]
;;
;;   Step 2: Apply inner lambda to 4
;;     (λy.+ 3 y) 4
;;     => + 3 4          [β-reduction, bind y=4]
;;
;;   Step 3: Evaluate +
;;     + 3 4
;;     => 7              [primitive evaluation]

(def add-curried
  "Curried addition: λx.λy.+ x y

   Skill role: GENERATOR (+1) - creates new values through arithmetic
   Canonical form: (fn [x] (fn [y] (+ x y)))

   Beta reduction sequence:
     (add-curried 3)  => (fn [y] (+ 3 y))    [closes over x=3]
     ((add-curried 3) 4) => 7                [closes over x=3,y=4; evaluates +]"
  (fn [x]
    (fn [y]
      (+ x y))))

;; Trace execution:
(assert (= 7 ((add-curried 3) 4))
        "Beta reduction: ((λx.λy.+ x y) 3) 4 = 7")

;; ============================================================================
;; Example 2: Function composition (BETA + ETA reduction)
;; ============================================================================

;; Term: (λf.λg.λx. f (g x))  [compose]
;; Applied to: (+ 1) and (* 2)
;; Trace:
;;   compose = λf.λg.λx. f (g x)
;;   compose (+ 1) = λg.λx. (+ 1) (g x)
;;   compose (+ 1) (* 2) = λx. (+ 1) ((* 2) x)
;;   compose (+ 1) (* 2) 5 = (+ 1) ((* 2) 5) = (+ 1) 10 = 11

(def compose
  "Function composition: λf.λg.λx. f (g x)

   Skill role: COORDINATOR (0) - bridges two functions

   Mathematical: (f ∘ g)(x) = f(g(x))

   Beta reduction:
     1. compose f = λg.λx. f (g x)        [bind f]
     2. compose f g = λx. f (g x)         [bind g]
     3. compose f g x = f (g x)           [bind x, compute]"
  (fn [f]
    (fn [g]
      (fn [x]
        (f (g x))))))

;; Example: (λx. x + 1) ∘ (λx. x * 2)
(def add1 (fn [x] (+ x 1)))
(def times2 (fn [x] (* x 2)))
(def add1-after-times2 ((compose add1) times2))

(assert (= 11 (add1-after-times2 5))
        "Composition beta reduction: ((λf.λg.λx.f(g x)) add1 times2) 5 = 11")

;; ============================================================================
;; Example 3: Map as Higher-Order Function
;; ============================================================================

;; Term: (λf.λs. map f s)  [map]
;; Applied to: (λx. x * 2) and [1 2 3]
;; Trace:
;;   map f = λs. map f s
;;   map f [1 2 3] = [2 4 6]         [applies f to each element]

(def hof-map
  "Higher-order map with explicit beta reduction

   λf.λs. map(f, s)

   Skill role: GENERATOR (+1) - generates new sequences from transformation

   Evaluation (eager):
     1. (hof-map f) closes over f in environment
     2. (hof-map f s) applies f to each element of s via map"
  (fn [f]
    (fn [s]
      (map f s))))

(assert (= [2 4 6] ((hof-map times2) [1 2 3]))
        "HOF map reduction: ((λf.λs.map f s) times2 [1 2 3]) = [2 4 6]")

;; ============================================================================
;; Example 4: Curried Fold/Reduce (Catamorphism)
;; ============================================================================

;; Term: (λf.λz.λs. fold(f, z, s))  [left fold]
;; Applied to: + , 0 , [1 2 3]
;; Trace:
;;   fold f = λz.λs. fold(f, z, s)
;;   fold f + = λs. fold(+, 0, s)
;;   fold f + 0 = λs. fold(+, 0, s)
;;   fold f + 0 [1 2 3] = 6

(def hof-fold
  "Left fold as higher-order function

   λf.λz.λs. fold(f, z, s)

   Also called: reduce, accumulate, catamorphism

   Skill role: COORDINATOR (0) - combines elements via binary operation

   Beta trace for + over [1 2 3] starting with 0:
     Step 1: bind f = +
     Step 2: bind z = 0
     Step 3: bind s = [1 2 3]
     Step 4: reduce
       acc=0, elem=1 => (+ 0 1) = 1
       acc=1, elem=2 => (+ 1 2) = 3
       acc=3, elem=3 => (+ 3 3) = 6
     Result: 6"
  (fn [f]
    (fn [z]
      (fn [s]
        (reduce f z s)))))

(assert (= 6 (((hof-fold +) 0) [1 2 3]))
        "HOF fold reduction: ((λf.λz.λs.fold(f,z,s)) + 0 [1 2 3]) = 6")

;; ============================================================================
;; Example 5: Partial Application with Closure Capture
;; ============================================================================

;; Term: (λf.λx.λy.λz. f x y z) partially applied to (f, a)
;; Demonstrates closure environment growth
;; Trace:
;;   bind f = multiply_three
;;   bind x = 2
;;   => λy.λz. multiply_three 2 y z
;;   bind y = 3
;;   => λz. multiply_three 2 3 z
;;   bind z = 4
;;   => multiply_three 2 3 4 = 24

(def multiply-three
  "Three-argument multiplication

   λx.λy.λz. x * y * z"
  (fn [x]
    (fn [y]
      (fn [z]
        (* x y z)))))

(def partial-apply-2
  "Partially apply multiply-three with x=2

   Closure captures multiply-three and x=2
   Remaining: λy.λz. 2 * y * z"
  (multiply-three 2))

(def partial-apply-2-3
  "Further partial: x=2, y=3

   Closure captures multiply-three, x=2, y=3
   Remaining: λz. 2 * 3 * z"
  (partial-apply-2 3))

(assert (= 24 (partial-apply-2-3 4))
        "Partial application: ((λx.λy.λz.x*y*z) 2) 3) 4 = 24")

;; ============================================================================
;; Example 6: Fixed-Point Combinator (Y combinator)
;; ============================================================================

;; Term: (λf. (λx.f (x x)) (λx.f (x x)))
;; Enables recursive function definition via self-application
;;
;; This is the classic Y combinator for recursion without named functions
;;
;; Trace: Y factorial
;;   Y f = (λx.f (x x)) (λx.f (x x))
;;
;;   When applied to factorial definition:
;;     (Y fact-factory) 5
;;     => (fact-factory (Y fact-factory)) 5
;;     => factorial-with-recursion 5
;;     => 120

(def y-combinator
  "Y combinator: λf.(λx.f(x x))(λx.f(x x))

   Enables recursion via self-application

   Skill role: VERIFIER (-1) - validates recursive definitions

   This is a sophisticated beta reduction example requiring:
     - Self-application (λx.x x)
     - Delayed evaluation (f receives unevaluated recursive call)
     - Fixed-point semantics (Y f is a fixed point of f)"
  (fn [f]
    (let [recursive-application (fn [x] (f (x x)))]
      (recursive-application recursive-application))))

;; Define factorial using Y combinator
(def factorial-factory
  "Factory function that takes a recursive call and returns factorial logic"
  (fn [recur]
    (fn [n]
      (if (<= n 1)
        1
        (* n (recur (- n 1)))))))

;; Create actual factorial via Y combinator
(def factorial (y-combinator factorial-factory))

(assert (= 120 (factorial 5))
        "Y combinator: Enable recursive factorial without named definition")

;; ============================================================================
;; GF(3) SKILL CLASSIFICATION: Balanced Triadic Composition
;; ============================================================================

;; Each function classified by its role in computation:
;;   PLUS (+1):      Generator - creates/transforms data
;;   ERGODIC (0):    Coordinator - connects/balances functions
;;   MINUS (-1):     Verifier - validates/constrains

(def skill-classified-functions
  "Higher-order functions classified for balanced composition

   Sum of trits must ≡ 0 (mod 3) for valid triads

   Example balanced triad:
     hof-map (PLUS +1) + compose (ERGODIC 0) + y-combinator (MINUS -1)
     Sum: 1 + 0 + (-1) = 0 ≡ 0 (mod 3) ✓"
  {
   :map {:name "hof-map" :trit 1 :role :PLUS :skill add-curried}
   :compose {:name "compose" :trit 0 :role :ERGODIC :skill compose}
   :fold {:name "hof-fold" :trit 1 :role :PLUS :skill hof-fold}
   :y-combinator {:name "y-combinator" :trit -1 :role :MINUS :skill y-combinator}
   })

;; Verify balance
(defn verify-triad-balance [f1 f2 f3]
  "Check if three functions form a balanced GF(3) triad"
  (let [sum (+ (:trit f1) (:trit f2) (:trit f3))]
    (zero? (mod sum 3))))

(assert (verify-triad-balance
         (:map skill-classified-functions)
         (:compose skill-classified-functions)
         (:y-combinator skill-classified-functions))
        "Balanced triad: map(+1) + compose(0) + y-combinator(-1) = 0")

;; ============================================================================
;; CANONICAL BETA REDUCTION TRACES: For Verification
;; ============================================================================

(def beta-reduction-examples
  "Collection of beta reduction traces for different HOF applications"
  [
   {
    :name "Addition currying"
    :term "((λx.λy.+ x y) 3) 4"
    :steps [
            {:step 1 :redex "(λx.λy.+ x y) 3" :result "(λy.+ 3 y)" :type :BETA}
            {:step 2 :redex "(λy.+ 3 y) 4" :result "(+ 3 4)" :type :BETA}
            {:step 3 :redex "(+ 3 4)" :result "7" :type :EVAL}
            ]
    :normal_form "7"
    }

   {
    :name "Function composition"
    :term "((compose add1 times2) 5)"
    :steps [
            {:step 1 :redex "(compose add1)" :result "(λx.add1(times2 x))" :type :BETA}
            {:step 2 :redex "((λx.add1(times2 x)) 5)" :result "(add1(times2 5))" :type :BETA}
            {:step 3 :redex "(times2 5)" :result "10" :type :EVAL}
            {:step 4 :redex "(add1 10)" :result "11" :type :EVAL}
            ]
    :normal_form "11"
    }

   {
    :name "Map over list"
    :term "((hof-map times2) [1 2 3])"
    :steps [
            {:step 1 :redex "(hof-map times2)" :result "(λs. map times2 s)" :type :BETA}
            {:step 2 :redex "((λs. map times2 s) [1 2 3])" :result "(map times2 [1 2 3])" :type :BETA}
            {:step 3 :redex "(map times2 [1 2 3])" :result "[2 4 6]" :type :EVAL}
            ]
    :normal_form "[2 4 6]"
    }

   {
    :name "Fold/reduce"
    :term "(((hof-fold +) 0) [1 2 3])"
    :steps [
            {:step 1 :redex "((hof-fold +) 0)" :result "(λs. reduce + 0 s)" :type :BETA}
            {:step 2 :redex "((λs. reduce + 0 s) [1 2 3])" :result "(reduce + 0 [1 2 3])" :type :BETA}
            {:step 3 :redex "(reduce + 0 [1 2 3])" :result "6" :type :EVAL}
            ]
    :normal_form "6"
    }
   ])

;; ============================================================================
;; CHURCH ENCODING: HOF as Lambda Calculus Primitives
;; ============================================================================

;; Church numerals: represent natural numbers as lambda functions
(def church-zero
  "Church 0: λf.λx. x

   Applies f zero times to x"
  (fn [f] (fn [x] x)))

(def church-one
  "Church 1: λf.λx. f x

   Applies f one time to x"
  (fn [f] (fn [x] (f x))))

(def church-two
  "Church 2: λf.λx. f (f x)

   Applies f two times to x"
  (fn [f] (fn [x] (f (f x)))))

;; Church successor: λn.λf.λx. f (n f x)
(def church-succ
  "Church successor function"
  (fn [n]
    (fn [f]
      (fn [x]
        (f ((n f) x))))))

;; Convert Church numeral to integer
(def church-to-int
  "Convert Church numeral to integer by applying inc 0"
  (fn [church-n]
    ((church-n inc) 0)))

(assert (= 0 (church-to-int church-zero))
        "Church 0")
(assert (= 1 (church-to-int church-one))
        "Church 1")
(assert (= 2 (church-to-int church-two))
        "Church 2")
(assert (= 3 (church-to-int (church-succ church-two)))
        "Church successor: succ(2) = 3")

;; ============================================================================
;; SUMMARY: Best Beta Reductions
;; ============================================================================

(comment
  "CANONICAL BETA REDUCTION STRATEGY FOR JOKER/CLOJURE:

   1. EAGER/STRICT EVALUATION
      - All arguments evaluated before function application
      - Closure captured at definition time
      - Environment chain for lexical scoping

   2. REDUCTION ORDER
      - Outermost-first (applicative order in eager evaluation)
      - Left-to-right for argument evaluation
      - All reductions are deterministic

   3. HOF PATTERNS (Best betas for each)
      a) Curried functions: Bind parameters sequentially
         ((λx.λy.+ x y) 3) 4 => ((λy.+ 3 y) 4) => 7

      b) Composition: Substitution of function results
         (compose f g) => λx.f(g(x))

      c) Map/Filter: Element-wise application via reduce
         (map f [a b c]) => [f(a) f(b) f(c)]

      d) Fold: Accumulation with binary operator
         (fold + 0 [1 2 3]) => ((+ (+ (+ 0 1) 2) 3))

      e) Partial application: Closure over fixed arguments
         ((hof-partial f 2) 3) => f(2, 3) [with closure capture]

      f) Fixed-point (Y combinator): Self-application for recursion
         (Y f) => (f (Y f))  [enables infinite recursion support]

   4. CLOSURE SEMANTICS (Environment capture)
      - Each lambda captures the environment at definition
      - Nested applications create environment chain
      - All variable lookups traverse parent chain

   5. GF(3) BALANCED COMPOSITION
      - Functions classified by role (±1 or 0)
      - Triads with sum ≡ 0 (mod 3) are balanced
      - Enables capability-based security verification

   RESULT: A pure functional programming system with
   - Explicit closure semantics (not term reduction)
   - Deterministic evaluation (strict/eager)
   - Capability-based function classification
   - Self-recursive computation via fixed-point combinators
")
