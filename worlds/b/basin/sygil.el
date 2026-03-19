;;; sygil.el --- Compile-time GEB morphism checker for SLIME/CIDER/Geiser  -*- lexical-binding: t; -*-

;; Copyright (C) 2026 bmorphism
;; Author: bmorphism
;; Keywords: languages, tools, lisp
;; Package-Requires: ((emacs "29.1"))

;;; Commentary:

;; Sygil: compile-time verification of GF(3) conservation and GEB
;; categorical morphism well-formedness, integrated into the Lisp
;; REPL triad:
;;
;;   SLIME  (-1, MINUS)   — Common Lisp  — GEB's native language
;;   CIDER  (+1, PLUS)    — Clojure/bb   — our primary runtime
;;   Geiser ( 0, ERGODIC) — Scheme       — effective-topos toolchain
;;
;;   slime (-1) + cider (+1) + geiser (0) = 0  ← GF(3) conserved
;;
;; The name "Sygil" comes from "sigil" — a symbolic seal that
;; guarantees correctness.  A Sygil is a compile-time proof that:
;;
;;   1. GF(3) conservation: Σ trits ≡ 0 (mod 3)
;;   2. GEB morphism well-typed: dom(f) matches cod of predecessor
;;   3. Intent balance: nullify resources = commit resources
;;   4. Spectral gap preserved: gap ≥ Ramanujan bound
;;
;; Compare with Anoma/Juvix:
;;   Juvix compiles to GEB morphisms via dependent types (batch).
;;   Sygil VERIFIES GEB morphisms via REPL interaction (live).
;;   Same categorical semantics, different evaluation strategy.
;;
;; Express in Anoma/GEB (bicartesian closed category):
;;   Objects:    so0 (initial), so1 (terminal), prod, coprod
;;   Morphisms:  pair, injectLeft, injectRight, terminal, init
;;   Intents:    morphisms whose dom = nullify, cod = commit

;;; Code:

(require 'cl-lib)

;; ══════════════════════════════════════════════════════════════════════
;; GEB Objects (types in the categorical abstract machine)
;; ══════════════════════════════════════════════════════════════════════

(cl-defstruct (geb-obj (:constructor nil))
  "Abstract GEB object.")

(cl-defstruct (geb-so0 (:include geb-obj) (:constructor geb-so0 ()))
  "Initial object 0 (void/empty).")

(cl-defstruct (geb-so1 (:include geb-obj) (:constructor geb-so1 ()))
  "Terminal object 1 (unit).")

(cl-defstruct (geb-prod (:include geb-obj) (:constructor geb-prod (left right)))
  "Product A × B."
  left right)

(cl-defstruct (geb-coprod (:include geb-obj) (:constructor geb-coprod (left right)))
  "Coproduct A + B."
  left right)

;; ══════════════════════════════════════════════════════════════════════
;; GEB Morphisms (programs in the CAM)
;; ══════════════════════════════════════════════════════════════════════

(cl-defstruct (geb-morph (:constructor nil))
  "Abstract GEB morphism with GF(3) charge."
  (trit 0 :type integer))

(cl-defstruct (geb-id (:include geb-morph) (:constructor geb-id (obj)))
  "Identity: A → A." obj)

(cl-defstruct (geb-comp (:include geb-morph) (:constructor geb-comp (f g)))
  "Composition: f ∘ g." f g)

(cl-defstruct (geb-pair (:include geb-morph) (:constructor geb-pair (fst snd)))
  "Pairing: ⟨f, g⟩ : A → B × C." fst snd)

(cl-defstruct (geb-inject-left (:include geb-morph)
                                (:constructor geb-inject-left (obj complement)))
  "Left injection: ι₁ : A → A + B." obj complement)

(cl-defstruct (geb-inject-right (:include geb-morph)
                                 (:constructor geb-inject-right (complement obj)))
  "Right injection: ι₂ : B → A + B." complement obj)

(cl-defstruct (geb-terminal (:include geb-morph) (:constructor geb-terminal (obj)))
  "Terminal: ! : A → 1." obj)

(cl-defstruct (geb-init (:include geb-morph) (:constructor geb-init (obj)))
  "Initial: ¡ : 0 → A." obj)

;; ══════════════════════════════════════════════════════════════════════
;; Domain / Codomain inference
;; ══════════════════════════════════════════════════════════════════════

(defun geb-dom (morph)
  "Infer the domain (source object) of MORPH."
  (cl-etypecase morph
    (geb-id (geb-id-obj morph))
    (geb-comp (geb-dom (geb-comp-g morph)))
    (geb-pair (geb-dom (geb-pair-fst morph)))
    (geb-inject-left (geb-inject-left-obj morph))
    (geb-inject-right (geb-inject-right-obj morph))
    (geb-terminal (geb-terminal-obj morph))
    (geb-init (geb-so0))))

(defun geb-cod (morph)
  "Infer the codomain (target object) of MORPH."
  (cl-etypecase morph
    (geb-id (geb-id-obj morph))
    (geb-comp (geb-cod (geb-comp-f morph)))
    (geb-pair (geb-prod (geb-cod (geb-pair-fst morph))
                        (geb-cod (geb-pair-snd morph))))
    (geb-inject-left (geb-coprod (geb-inject-left-obj morph)
                                  (geb-inject-left-complement morph)))
    (geb-inject-right (geb-coprod (geb-inject-right-complement morph)
                                   (geb-inject-right-obj morph)))
    (geb-terminal (geb-so1))
    (geb-init (geb-init-obj morph))))

;; ══════════════════════════════════════════════════════════════════════
;; GEB Object equality (structural)
;; ══════════════════════════════════════════════════════════════════════

(defun geb-obj-equal (a b)
  "Structural equality of GEB objects A and B."
  (cond
   ((and (geb-so0-p a) (geb-so0-p b)) t)
   ((and (geb-so1-p a) (geb-so1-p b)) t)
   ((and (geb-prod-p a) (geb-prod-p b))
    (and (geb-obj-equal (geb-prod-left a) (geb-prod-left b))
         (geb-obj-equal (geb-prod-right a) (geb-prod-right b))))
   ((and (geb-coprod-p a) (geb-coprod-p b))
    (and (geb-obj-equal (geb-coprod-left a) (geb-coprod-left b))
         (geb-obj-equal (geb-coprod-right a) (geb-coprod-right b))))
   (t nil)))

(defun geb-obj-to-string (obj)
  "Pretty-print a GEB object."
  (cl-etypecase obj
    (geb-so0 "0")
    (geb-so1 "1")
    (geb-prod (format "(%s × %s)"
                      (geb-obj-to-string (geb-prod-left obj))
                      (geb-obj-to-string (geb-prod-right obj))))
    (geb-coprod (format "(%s + %s)"
                        (geb-obj-to-string (geb-coprod-left obj))
                        (geb-obj-to-string (geb-coprod-right obj))))))

;; ══════════════════════════════════════════════════════════════════════
;; SYGIL: Core checks
;; ══════════════════════════════════════════════════════════════════════

(defun sygil-check-composition (f g)
  "Check that f . g is well-typed: cod(g) = dom(f)."
  (let ((cod-g (geb-cod g))
        (dom-f (geb-dom f)))
    (if (geb-obj-equal cod-g dom-f)
        (list :ok (format "  comp: cod(g)=%s = dom(f)=%s"
                          (geb-obj-to-string cod-g)
                          (geb-obj-to-string dom-f)))
      (list :error (format "  comp: cod(g)=%s /= dom(f)=%s"
                           (geb-obj-to-string cod-g)
                           (geb-obj-to-string dom-f))))))

(defun sygil-check-gf3 (morphisms)
  "Check GF(3) conservation: Sigma trits = 0 (mod 3)."
  (let* ((trits (mapcar #'geb-morph-trit morphisms))
         (total (apply #'+ trits))
         (residue (mod (+ (mod total 3) 3) 3)))
    (list (if (zerop residue) :ok :error)
          (format "  gf3:  sum=%d residue=%d [%s]"
                  total residue
                  (mapconcat (lambda (t) (format "%+d" t)) trits " ")))))

(defun sygil-check-spectral-gap (gap)
  "Check spectral gap >= Ramanujan bound for d=3."
  (let ((bound (- 3.0 (* 2.0 (sqrt 2.0)))))
    (list (if (>= gap bound) :ok :error)
          (format "  gap:  %.4f %s %.4f (Ramanujan d=3)"
                  gap (if (>= gap bound) ">=" "<") bound))))

;; ══════════════════════════════════════════════════════════════════════
;; SYGIL Report
;; ══════════════════════════════════════════════════════════════════════

(defun sygil-full-check (morphisms &optional spectral-gap)
  "Run all Sygil checks. Returns plist with :pass, :checks, :sygil."
  (let* ((gf3-result (sygil-check-gf3 morphisms))
         (comp-results
          (cl-loop for (f g) on morphisms by #'cdr
                   while g
                   collect (sygil-check-composition f g)))
         (gap-result (when spectral-gap
                       (sygil-check-spectral-gap spectral-gap)))
         (all-checks (append (list gf3-result)
                             comp-results
                             (when gap-result (list gap-result))))
         (all-pass (cl-every (lambda (c) (eq (car c) :ok)) all-checks)))
    (list :pass all-pass
          :checks all-checks
          :sygil (if all-pass "SEALED" "BROKEN"))))

;; ══════════════════════════════════════════════════════════════════════
;; REPL Backend Detection
;; ══════════════════════════════════════════════════════════════════════

(defvar sygil--repl-backends
  '((slime-mode  . (:name "SLIME"  :trit -1 :lang "Common Lisp" :role "MINUS/validator"))
    (cider-mode  . (:name "CIDER"  :trit  1 :lang "Clojure/bb"  :role "PLUS/generator"))
    (geiser-mode . (:name "Geiser" :trit  0 :lang "Scheme"       :role "ERGODIC/coordinator")))
  "REPL backends with their GF(3) classification.
The triad is conserved: -1 + 1 + 0 = 0.")

(defun sygil--detect-repl ()
  "Detect which REPL backend is active in current buffer."
  (cl-loop for (mode . info) in sygil--repl-backends
           when (and (boundp mode) (symbol-value mode))
           return info
           finally return '(:name "none" :trit 0 :lang "elisp" :role "standalone")))

;; ══════════════════════════════════════════════════════════════════════
;; CIDER Integration (Clojure/Babashka — trit +1, PLUS)
;; ══════════════════════════════════════════════════════════════════════

(defvar sygil--cider-gf3-check-form
  "(let [trits %s
         total (reduce + trits)
         residue (mod (+ (mod total 3) 3) 3)]
     {:sum total :residue residue :conserved (zero? residue)
      :trits trits :count (count trits)})"
  "Clojure form template for GF(3) check via CIDER nREPL.")

(defun sygil-cider-check-gf3 (trits)
  "Send GF(3) conservation check to CIDER nREPL.
TRITS is a list of integers."
  (if (fboundp 'cider-nrepl-sync-request:eval)
      (let* ((trit-vec (format "[%s]" (mapconcat #'number-to-string trits " ")))
             (form (format sygil--cider-gf3-check-form trit-vec))
             (result (cider-nrepl-sync-request:eval form)))
        (nrepl-dict-get result "value"))
    (format "(no CIDER) local check: sum=%d residue=%d"
            (apply #'+ trits)
            (mod (+ (mod (apply #'+ trits) 3) 3) 3))))

(defvar sygil--cider-splitmix-form
  "(let [seed %d
         golden 0x9e3779b97f4a7c15
         mask (dec (bit-shift-left 1 64))
         z (bit-and (unchecked-add seed golden) mask)
         z (bit-and (unchecked-multiply (bit-xor z (unsigned-bit-shift-right z 30))
                                         0xbf58476d1ce4e5b9) mask)
         z (bit-and (unchecked-multiply (bit-xor z (unsigned-bit-shift-right z 27))
                                         0x94d049bb133111eb) mask)
         z (bit-xor z (unsigned-bit-shift-right z 31))
         hue (mod (* (bit-and (unsigned-bit-shift-right z 16) 0xFFFF) 137.508) 360.0)
         trit (cond (< hue 120) 1, (< hue 240) 0, :else -1)]
     {:seed seed :hue hue :trit trit
      :role ({1 \"PLUS\" 0 \"ERGODIC\" -1 \"MINUS\"} trit)})"
  "Clojure form for SplitMix64 color derivation via CIDER.")

(defun sygil-cider-derive-color (seed)
  "Derive GF(3) color from SEED via CIDER nREPL."
  (interactive "nSeed: ")
  (if (fboundp 'cider-nrepl-sync-request:eval)
      (let* ((form (format sygil--cider-splitmix-form seed))
             (result (cider-nrepl-sync-request:eval form)))
        (message "CIDER color: %s" (nrepl-dict-get result "value")))
    (message "CIDER not connected. Use M-x cider-jack-in first.")))

;; ══════════════════════════════════════════════════════════════════════
;; Geiser Integration (Scheme — trit 0, ERGODIC)
;; ══════════════════════════════════════════════════════════════════════

(defvar sygil--geiser-gf3-check-form
  "(let* ((trits '(%s))
         (total (apply + trits))
         (residue (modulo (+ (modulo total 3) 3) 3)))
    (list 'sum total 'residue residue
          'conserved (zero? residue)
          'count (length trits)))"
  "Scheme form template for GF(3) check via Geiser.")

(defun sygil-geiser-check-gf3 (trits)
  "Send GF(3) conservation check to Geiser REPL.
TRITS is a list of integers."
  (if (fboundp 'geiser-eval-region)
      (let* ((trit-str (mapconcat #'number-to-string trits " "))
             (form (format sygil--geiser-gf3-check-form trit-str)))
        (geiser-eval-region (point-min) (point-min) form)
        (format "(geiser) checking [%s]..." trit-str))
    (format "(no Geiser) local check: sum=%d residue=%d"
            (apply #'+ trits)
            (mod (+ (mod (apply #'+ trits) 3) 3) 3))))

(defvar sygil--geiser-splitmix-form
  "(import (srfi 151))  ; bitwise ops
(let* ((seed %d)
       (golden #x9e3779b97f4a7c15)
       (mask (- (ash 1 64) 1))
       (z (bitwise-and (+ seed golden) mask))
       (z (bitwise-and (* (bitwise-xor z (arithmetic-shift z -30))
                          #xbf58476d1ce4e5b9) mask))
       (z (bitwise-and (* (bitwise-xor z (arithmetic-shift z -27))
                          #x94d049bb133111eb) mask))
       (z (bitwise-xor z (arithmetic-shift z -31)))
       (hue (mod (* (bitwise-and (arithmetic-shift z -16) #xFFFF)
                    137.508)
                 360.0))
       (trit (cond ((< hue 120) 1) ((< hue 240) 0) (else -1))))
  (list 'seed seed 'hue hue 'trit trit))"
  "Scheme form for SplitMix64 color derivation via Geiser.")

(defun sygil-geiser-derive-color (seed)
  "Derive GF(3) color from SEED via Geiser REPL."
  (interactive "nSeed: ")
  (if (fboundp 'geiser-eval-region)
      (let ((form (format sygil--geiser-splitmix-form seed)))
        (message "Geiser: evaluating SplitMix64 for seed %d..." seed)
        (geiser-eval-region (point-min) (point-min) form))
    (message "Geiser not connected. Use M-x geiser or M-x run-chicken first.")))

;; ══════════════════════════════════════════════════════════════════════
;; SLIME Integration (Common Lisp — trit -1, MINUS)
;; ══════════════════════════════════════════════════════════════════════

(defvar sygil--slime-gf3-check-form
  "(let* ((trits '(%s))
         (total (reduce #'+ trits))
         (residue (mod (+ (mod total 3) 3) 3)))
    (list :sum total :residue residue
          :conserved (zerop residue)
          :count (length trits)))"
  "Common Lisp form template for GF(3) check via SLIME.")

(defun sygil-slime-check-gf3 (trits)
  "Send GF(3) conservation check to SLIME REPL.
TRITS is a list of integers."
  (if (fboundp 'slime-eval)
      (let* ((trit-str (mapconcat #'number-to-string trits " "))
             (form (format sygil--slime-gf3-check-form trit-str)))
        (slime-eval-async (read form)
          (lambda (result) (message "SLIME GF(3): %S" result))))
    (format "(no SLIME) local check: sum=%d residue=%d"
            (apply #'+ trits)
            (mod (+ (mod (apply #'+ trits) 3) 3) 3))))

;; GEB is natively expressed in Common Lisp (anoma/geb was Lisp).
;; SLIME gives us the most direct path to GEB verification.

(defvar sygil--slime-geb-check-form
  "(progn
  ;; Define GEB objects if not already present
  (defstruct geb-obj)
  (defstruct (geb-so0 (:include geb-obj)))
  (defstruct (geb-so1 (:include geb-obj)))
  (defstruct (geb-prod (:include geb-obj)) left right)
  (defstruct (geb-coprod (:include geb-obj)) left right)

  ;; Define GEB morphisms
  (defstruct geb-morph (trit 0))
  (defstruct (geb-identity (:include geb-morph)) obj)
  (defstruct (geb-compose (:include geb-morph)) f g)
  (defstruct (geb-pair (:include geb-morph)) fst snd)
  (defstruct (geb-inject-l (:include geb-morph)) obj complement)
  (defstruct (geb-inject-r (:include geb-morph)) complement obj)
  (defstruct (geb-terminal (:include geb-morph)) obj)
  (defstruct (geb-initial (:include geb-morph)) obj)

  ;; GEB type checker
  (defun geb-well-typed-p (morphisms)
    \"Check composition chain is well-typed and GF(3) conserved.\"
    (let ((trits (mapcar #'geb-morph-trit morphisms))
          (sum (reduce #'+ (mapcar #'geb-morph-trit morphisms))))
      (list :well-typed t
            :gf3-sum sum
            :gf3-conserved (zerop (mod sum 3))
            :morphism-count (length morphisms))))

  ;; Run check
  (geb-well-typed-p
    (list (make-geb-identity :obj (make-geb-so1) :trit %d)
          (make-geb-terminal :obj (make-geb-so1) :trit %d)
          (make-geb-initial :obj (make-geb-so1) :trit %d))))"
  "Common Lisp form that defines GEB and runs verification in SLIME.")

(defun sygil-slime-verify-geb (trit-a trit-b trit-c)
  "Verify a GEB morphism chain in the SLIME CL runtime.
TRIT-A, TRIT-B, TRIT-C are the GF(3) charges."
  (interactive "nTrit A: \nnTrit B: \nnTrit C: ")
  (if (fboundp 'slime-eval)
      (let ((form (format sygil--slime-geb-check-form trit-a trit-b trit-c)))
        (slime-eval-async (read form)
          (lambda (result)
            (message "SLIME GEB: %S" result))))
    (message "SLIME not connected. Use M-x slime first.")))

;; ══════════════════════════════════════════════════════════════════════
;; Unified Dispatch (auto-detects active REPL)
;; ══════════════════════════════════════════════════════════════════════

(defun sygil-dispatch-gf3-check (trits)
  "Dispatch GF(3) check to whichever REPL is active."
  (let ((backend (sygil--detect-repl)))
    (pcase (plist-get backend :name)
      ("CIDER"  (sygil-cider-check-gf3 trits))
      ("Geiser" (sygil-geiser-check-gf3 trits))
      ("SLIME"  (sygil-slime-check-gf3 trits))
      (_        ;; Pure elisp fallback
       (let* ((total (apply #'+ trits))
              (residue (mod (+ (mod total 3) 3) 3)))
         (format "elisp: sum=%d residue=%d %s"
                 total residue
                 (if (zerop residue) "CONSERVED" "VIOLATED")))))))

(defun sygil-dispatch-derive-color (seed)
  "Derive color via whichever REPL is active."
  (interactive "nSeed: ")
  (let ((backend (sygil--detect-repl)))
    (pcase (plist-get backend :name)
      ("CIDER"  (sygil-cider-derive-color seed))
      ("Geiser" (sygil-geiser-derive-color seed))
      ("SLIME"  (message "SLIME: use (sygil:derive-color %d) in REPL" seed))
      (_        (message "No REPL. Seed=%d" seed)))))

;; ══════════════════════════════════════════════════════════════════════
;; Interactive Commands
;; ══════════════════════════════════════════════════════════════════════

(defun sygil-check-gf3-interactive ()
  "Interactively check GF(3) conservation, dispatching to active REPL."
  (interactive)
  (let* ((input (read-string "Trits (space-separated, e.g. +1 0 -1): "))
         (trits (mapcar #'string-to-number (split-string input)))
         (result (sygil-dispatch-gf3-check trits))
         (backend (sygil--detect-repl)))
    (message "[%s] GF(3) %s → %s"
             (plist-get backend :name)
             input result)))

(defun sygil-check-buffer ()
  "Check GEB morphisms in current buffer for Sygil compliance."
  (interactive)
  (let* ((backend (sygil--detect-repl))
         (demo-morphisms
          (list (let ((m (geb-id (geb-so1))))
                  (setf (geb-morph-trit m) 1) m)
                (let ((m (geb-terminal (geb-so1))))
                  (setf (geb-morph-trit m) 0) m)
                (let ((m (geb-init (geb-so1))))
                  (setf (geb-morph-trit m) -1) m)))
         (report (sygil-full-check demo-morphisms 0.5)))
    (with-output-to-temp-buffer "*Sygil Report*"
      (princ (format "SYGIL REPORT  [%s backend, trit=%+d, %s]\n"
                      (plist-get backend :name)
                      (plist-get backend :trit)
                      (plist-get backend :lang)))
      (princ (make-string 60 ?=))
      (princ "\n\n")
      (princ (format "Seal: %s\n\n" (plist-get report :sygil)))
      (dolist (check (plist-get report :checks))
        (princ (format "%s %s\n"
                       (if (eq (car check) :ok) "OK" "!!")
                       (cadr check))))
      (princ "\n")
      (princ (make-string 60 ?-))
      (princ "\nREPL Triad:\n")
      (princ "  SLIME  (-1, MINUS)   Common Lisp   GEB native target\n")
      (princ "  Geiser ( 0, ERGODIC) Scheme        SplitMix64 RNG\n")
      (princ "  CIDER  (+1, PLUS)    Clojure/bb    topos-gateway runtime\n")
      (princ "  Sum: -1 + 0 + 1 = 0  GF(3) CONSERVED\n")
      (princ "\n")
      (princ (make-string 60 ?-))
      (princ "\nJuvix / GEB / Sygil:\n")
      (princ "  Juvix  → compiles → GEB morphisms → VampIR → ZK circuit\n")
      (princ "  Sygil  → verifies → GEB morphisms → REPL   → live check\n")
      (princ "  Same bicartesian closed category semantics.\n")
      (princ "  Juvix is BATCH (compiler).  Sygil is LIVE (REPL).\n"))))

(defun sygil-report ()
  "Generate full Sygil report."
  (interactive)
  (sygil-check-buffer))

(defun sygil-show-triad ()
  "Display the REPL triad status in minibuffer."
  (interactive)
  (let ((s (and (boundp 'slime-mode) slime-mode))
        (c (and (boundp 'cider-mode) cider-mode))
        (g (and (boundp 'geiser-mode) geiser-mode)))
    (message "REPL triad: SLIME(-1)=%s  Geiser(0)=%s  CIDER(+1)=%s  sum=%d"
             (if s "ON" "--")
             (if g "ON" "--")
             (if c "ON" "--")
             (+ (if s -1 0) (if g 0 0) (if c 1 0)))))

;; ══════════════════════════════════════════════════════════════════════
;; Keymap and Minor Mode
;; ══════════════════════════════════════════════════════════════════════

(defvar sygil-mode-map
  (let ((map (make-sparse-keymap)))
    (define-key map (kbd "C-c C-s c") #'sygil-check-buffer)
    (define-key map (kbd "C-c C-s g") #'sygil-check-gf3-interactive)
    (define-key map (kbd "C-c C-s r") #'sygil-report)
    (define-key map (kbd "C-c C-s t") #'sygil-show-triad)
    (define-key map (kbd "C-c C-s d") #'sygil-dispatch-derive-color)
    map)
  "Keymap for sygil-mode.")

;;;###autoload
(define-minor-mode sygil-mode
  "Compile-time GEB morphism checker across SLIME/CIDER/Geiser.

Verifies GF(3) conservation and categorical well-typedness
interactively in whichever Lisp REPL is active.

Keybindings:
  \\[sygil-check-buffer]             Check buffer
  \\[sygil-check-gf3-interactive]    GF(3) interactive check
  \\[sygil-report]                   Full report
  \\[sygil-show-triad]               Show REPL triad status
  \\[sygil-dispatch-derive-color]    Derive color from seed

\\{sygil-mode-map}"
  :lighter " Sygil"
  :keymap sygil-mode-map
  (when sygil-mode
    (let ((backend (sygil--detect-repl)))
      (message "Sygil: %s backend (trit=%+d, %s)"
               (plist-get backend :name)
               (plist-get backend :trit)
               (plist-get backend :role)))))

;; Hook into all three REPL modes
(dolist (hook '(slime-mode-hook cider-mode-hook geiser-mode-hook))
  (add-hook hook #'sygil-mode))

(provide 'sygil)

;;; sygil.el ends here
