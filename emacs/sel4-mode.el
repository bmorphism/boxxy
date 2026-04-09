;;; sel4-mode.el --- seL4 IPC + Capability Management for Emacs  -*- lexical-binding: t; -*-

;; Copyright (C) 2026  bmorphism
;; SPDX-License-Identifier: Apache-2.0

;; Author: bmorphism
;; Keywords: seL4, capabilities, IPC, microkernel, GF(3)

;;; Commentary:

;; Emacs interface to the boxxy seL4 stack.
;; Connects Emacs to seL4 IPC via the Nashator wire protocol (4-byte BE + JSON-RPC 2.0).
;;
;; Three modes of operation:
;;   1. Navigation — browse seL4 source, theories, provider spec
;;   2. IPC monitor — live view of seL4 IPC messages and GF(3) balance
;;   3. Capability inspector — browse/attenuate/revoke capabilities
;;
;; Wire protocol: JSON-RPC 2.0 over TCP :9998 (sel4 bridge port)
;;   Request:  4-byte BE length + JSON payload
;;   Response: 4-byte BE length + JSON payload
;;
;; Keybindings (under C-c C-4 prefix, mnemonic: seL"4"):
;;   C-c C-4 m  — IPC monitor (live message view)
;;   C-c C-4 c  — Capability inspector
;;   C-c C-4 b  — GF(3) balance report
;;   C-c C-4 f  — Fault log
;;   C-c C-4 t  — Run seL4 tests
;;   C-c C-4 s  — Open seL4 source file
;;   C-c C-4 i  — Open Isabelle theory
;;   C-c C-4 p  — Provider spec (sel4-provider.edn)
;;   C-c C-4 a  — Architecture diagram
;;   C-c C-4 v  — Verification matrix

;;; Code:

(require 'json)
(require 'compile)

;; ── Customization ──

(defgroup sel4-mode nil
  "seL4 IPC and capability management."
  :group 'tools
  :prefix "sel4-")

(defcustom sel4-boxxy-dir
  (expand-file-name "~/i/boxxy/")
  "Path to boxxy project root."
  :type 'directory
  :group 'sel4-mode)

(defcustom sel4-bridge-host "127.0.0.1"
  "seL4 bridge host (Nashator relay)."
  :type 'string
  :group 'sel4-mode)

(defcustom sel4-bridge-port 9998
  "seL4 bridge port."
  :type 'integer
  :group 'sel4-mode)

(defcustom sel4-auto-balance-check t
  "Automatically check GF(3) balance after each IPC operation."
  :type 'boolean
  :group 'sel4-mode)

;; ── Source locations ──

(defconst sel4--clj-sources
  '((ipc        . "std/sel4/ipc.clj")
    (capability . "std/sel4/capability.clj")
    (balance    . "std/sel4/balance.clj")
    (window     . "std/sel4/window.clj")
    (common     . "std/sel4/common.clj")
    (provider   . "std/sel4-provider.edn"))
  "Clojure source files in the seL4 stack.")

(defconst sel4--isabelle-theories
  '((bridge       . "theories/seL4_Bridge.thy")
    (agm-base     . "theories/AGM_Base.thy")
    (agm-ext      . "theories/AGM_Extensions.thy")
    (cognitive    . "theories/Cognitive_Debt.thy"))
  "Isabelle theory files for seL4 verification.")

;; ── GF(3) trit display ──

(defface sel4-trit-plus
  '((t :foreground "#44CF6C" :weight bold))
  "Face for +1 (Generator) trit."
  :group 'sel4-mode)

(defface sel4-trit-zero
  '((t :foreground "#2E86AB" :weight normal))
  "Face for 0 (Coordinator) trit."
  :group 'sel4-mode)

(defface sel4-trit-minus
  '((t :foreground "#A23B72" :weight bold))
  "Face for -1 (Verifier) trit."
  :group 'sel4-mode)

(defface sel4-cap-valid
  '((t :foreground "#44CF6C"))
  "Face for valid capabilities."
  :group 'sel4-mode)

(defface sel4-cap-revoked
  '((t :foreground "#FF4444" :strike-through t))
  "Face for revoked capabilities."
  :group 'sel4-mode)

(defface sel4-fault
  '((t :foreground "#FF8800" :weight bold))
  "Face for fault messages."
  :group 'sel4-mode)

(defun sel4--trit-string (trit)
  "Return propertized string for TRIT value (-1, 0, +1)."
  (pcase trit
    (1  (propertize "+1" 'face 'sel4-trit-plus))
    (0  (propertize " 0" 'face 'sel4-trit-zero))
    (-1 (propertize "-1" 'face 'sel4-trit-minus))
    (_  (propertize " ?" 'face 'default))))

;; ── Wire protocol (4-byte BE + JSON-RPC 2.0) ──

(defvar sel4--connection nil
  "Network process for seL4 bridge connection.")

(defvar sel4--request-id 0
  "JSON-RPC request ID counter.")

(defvar sel4--pending-callbacks (make-hash-table :test 'equal)
  "Pending JSON-RPC callbacks keyed by request ID.")

(defvar sel4--recv-buffer ""
  "Partial receive buffer for length-prefixed frames.")

(defun sel4--connect ()
  "Connect to seL4 bridge via TCP."
  (when (and sel4--connection (process-live-p sel4--connection))
    (delete-process sel4--connection))
  (condition-case err
      (setq sel4--connection
            (make-network-process
             :name "sel4-bridge"
             :host sel4-bridge-host
             :service sel4-bridge-port
             :coding 'binary
             :filter #'sel4--recv-filter
             :sentinel #'sel4--sentinel
             :noquery t))
    (error
     (message "sel4: cannot connect to %s:%d — %s"
              sel4-bridge-host sel4-bridge-port (error-message-string err))
     nil)))

(defun sel4--sentinel (proc event)
  "Handle connection state changes."
  (cond
   ((string-match-p "open" event)
    (message "sel4: connected to %s:%d" sel4-bridge-host sel4-bridge-port))
   ((string-match-p "\\(closed\\|failed\\|broken\\)" event)
    (message "sel4: connection lost — %s" (string-trim event)))))

(defun sel4--encode-frame (json-string)
  "Encode JSON-STRING with 4-byte big-endian length prefix."
  (let* ((payload (encode-coding-string json-string 'utf-8))
         (len (length payload))
         (header (unibyte-string
                  (logand (ash len -24) #xFF)
                  (logand (ash len -16) #xFF)
                  (logand (ash len -8) #xFF)
                  (logand len #xFF))))
    (concat header payload)))

(defun sel4--decode-frame (data)
  "Decode a 4-byte BE length-prefixed frame from DATA.
Returns (payload . remaining) or nil if incomplete."
  (when (>= (length data) 4)
    (let ((len (logior (ash (aref data 0) 24)
                       (ash (aref data 1) 16)
                       (ash (aref data 2) 8)
                       (aref data 3))))
      (when (>= (length data) (+ 4 len))
        (cons (substring data 4 (+ 4 len))
              (substring data (+ 4 len)))))))

(defun sel4--recv-filter (_proc data)
  "Process incoming length-prefixed JSON-RPC frames."
  (setq sel4--recv-buffer (concat sel4--recv-buffer data))
  (let ((frame (sel4--decode-frame sel4--recv-buffer)))
    (while frame
      (let* ((payload (decode-coding-string (car frame) 'utf-8))
             (json (json-read-from-string payload))
             (id (alist-get 'id json))
             (callback (gethash id sel4--pending-callbacks)))
        (when callback
          (remhash id sel4--pending-callbacks)
          (funcall callback json)))
      (setq sel4--recv-buffer (cdr frame))
      (setq frame (sel4--decode-frame sel4--recv-buffer)))))

(defun sel4--rpc (method params &optional callback)
  "Send JSON-RPC 2.0 request to seL4 bridge.
CALLBACK receives the parsed JSON response."
  (unless (and sel4--connection (process-live-p sel4--connection))
    (sel4--connect))
  (when (and sel4--connection (process-live-p sel4--connection))
    (let* ((id (cl-incf sel4--request-id))
           (request (json-encode
                     `((jsonrpc . "2.0")
                       (method . ,method)
                       (params . ,params)
                       (id . ,id)))))
      (when callback
        (puthash id callback sel4--pending-callbacks))
      (process-send-string sel4--connection (sel4--encode-frame request))
      id)))

;; ── seL4 operations ──

(defconst sel4--operations
  '(;; IPC ops
    (send          . #x01) (recv      . #x02) (call   . #x03)
    (reply         . #x04) (reply-recv . #x05) (nb-send . #x06)
    (signal        . #x07) (wait      . #x08) (poll   . #x09)
    ;; Capability ops
    (cnode-copy    . #x30) (cnode-mint  . #x31) (cnode-move   . #x32)
    (cnode-delete  . #x33) (cnode-revoke . #x34) (untyped-retype . #x35)
    ;; Window ops
    (query-layout  . #x40) (move-window . #x41) (focus-window . #x42))
  "seL4 operation codes.")

(defconst sel4--operation-trits
  '((send . 0) (recv . 0) (call . 0) (reply . 0) (reply-recv . 0)
    (nb-send . 0) (signal . 0) (wait . 0) (poll . 0)
    (cnode-copy . -1) (cnode-mint . -1) (cnode-move . 0)
    (cnode-delete . -1) (cnode-revoke . -1) (untyped-retype . 1)
    (query-layout . 0) (move-window . 0) (focus-window . 0))
  "GF(3) trit assignments for operations.")

;; ── IPC Monitor ──

(defvar sel4--ipc-log nil
  "Ring buffer of recent IPC messages.")

(defvar sel4--ipc-log-max 500
  "Maximum number of IPC messages to retain.")

(defvar sel4--ipc-trit-sum 0
  "Running GF(3) trit sum for the session.")

(defun sel4--log-ipc (direction endpoint op badge args)
  "Record an IPC message in the log."
  (let* ((trit (or (alist-get op sel4--operation-trits) 0))
         (entry `((time . ,(format-time-string "%H:%M:%S.%3N"))
                  (dir . ,direction)
                  (endpoint . ,endpoint)
                  (op . ,op)
                  (badge . ,badge)
                  (args . ,args)
                  (trit . ,trit))))
    (setq sel4--ipc-trit-sum (mod (+ sel4--ipc-trit-sum trit) 3))
    (push entry sel4--ipc-log)
    (when (> (length sel4--ipc-log) sel4--ipc-log-max)
      (setq sel4--ipc-log (seq-take sel4--ipc-log sel4--ipc-log-max)))
    ;; Refresh monitor buffer if visible
    (when-let ((buf (get-buffer "*seL4 IPC Monitor*")))
      (when (get-buffer-window buf)
        (sel4--refresh-monitor buf)))))

(defun sel4--refresh-monitor (buf)
  "Refresh the IPC monitor buffer."
  (with-current-buffer buf
    (let ((inhibit-read-only t)
          (pos (point)))
      (erase-buffer)
      (insert (propertize "seL4 IPC Monitor" 'face '(:weight bold :height 1.2))
              "\n")
      (insert (format "Session trit sum: %s  Messages: %d\n"
                      (sel4--trit-string sel4--ipc-trit-sum)
                      (length sel4--ipc-log)))
      (insert (make-string 72 ?─) "\n")
      (insert (format "%-12s %-4s %-24s %-12s %s  %s\n"
                      "Time" "Dir" "Endpoint" "Operation" "Trit" "Badge"))
      (insert (make-string 72 ?─) "\n")
      (dolist (entry (seq-take (reverse sel4--ipc-log) 50))
        (let-alist entry
          (insert (format "%-12s %-4s %-24s %-12s %s   0x%04X\n"
                          .time
                          (if (eq .dir 'send) " >>" " <<")
                          (truncate-string-to-width
                           (format "%s" .endpoint) 24 nil nil "…")
                          .op
                          (sel4--trit-string .trit)
                          (or .badge 0)))))
      (goto-char (min pos (point-max))))))

(defun sel4-ipc-monitor ()
  "Open the seL4 IPC monitor."
  (interactive)
  (let ((buf (get-buffer-create "*seL4 IPC Monitor*")))
    (with-current-buffer buf
      (special-mode)
      (sel4--refresh-monitor buf))
    (display-buffer buf)))

;; ── Capability Inspector ──

(defvar sel4--capabilities nil
  "Known capabilities in the current session.")

(defun sel4--format-rights (rights)
  "Format a rights bitmap as a string."
  (let ((flags '((#x01 . "R") (#x02 . "W") (#x04 . "G")
                 (#x08 . "P") (#x10 . "D"))))
    (mapconcat (lambda (pair)
                 (if (> (logand rights (car pair)) 0)
                     (cdr pair) "-"))
               flags "")))

(defun sel4-capability-inspector ()
  "Open the capability inspector."
  (interactive)
  (let ((buf (get-buffer-create "*seL4 Capabilities*")))
    (with-current-buffer buf
      (let ((inhibit-read-only t))
        (erase-buffer)
        (insert (propertize "seL4 Capability Inspector" 'face '(:weight bold :height 1.2))
                "\n")
        (insert (make-string 72 ?─) "\n")
        (insert (format "%-36s %-20s %-6s %s  %s\n"
                        "ID" "Name" "Rights" "Trit" "Status"))
        (insert (make-string 72 ?─) "\n")
        (if (null sel4--capabilities)
            (insert "\n  No capabilities loaded.\n"
                    "  Connect to seL4 bridge (C-c C-4 m) to populate.\n")
          (dolist (cap sel4--capabilities)
            (let-alist cap
              (let ((face (if .revoked 'sel4-cap-revoked 'sel4-cap-valid)))
                (insert (propertize
                         (format "%-36s %-20s %-6s %s  %s\n"
                                 (truncate-string-to-width .id 36 nil nil "…")
                                 .name
                                 (sel4--format-rights .rights)
                                 (sel4--trit-string .trit)
                                 (if .revoked "REVOKED" "active"))
                         'face face))))))
        (insert (make-string 72 ?─) "\n")
        (insert (format "\nRights: R=Read W=Write G=Grant P=Passthrough D=Delete\n"))
        (insert "GF(3): Provider trit sel4 = -1 (Verifier)\n"))
      (special-mode))
    (display-buffer buf)))

;; ── GF(3) Balance Report ──

(defun sel4-balance-report ()
  "Show GF(3) balance report for the IPC session."
  (interactive)
  (let ((buf (get-buffer-create "*seL4 GF(3) Balance*")))
    (with-current-buffer buf
      (let ((inhibit-read-only t)
            (trit-counts (make-vector 3 0))
            (total (length sel4--ipc-log)))
        (erase-buffer)
        (insert (propertize "seL4 GF(3) Balance Report" 'face '(:weight bold :height 1.2))
                "\n\n")
        ;; Count trits
        (dolist (entry sel4--ipc-log)
          (let ((trit (alist-get 'trit entry)))
            (pcase trit
              (-1 (cl-incf (aref trit-counts 0)))
              (0  (cl-incf (aref trit-counts 1)))
              (1  (cl-incf (aref trit-counts 2))))))
        ;; Display
        (insert "  Provider Triad:\n")
        (insert (format "    hv  %s  Generator  (creates VMs, allocates)\n"
                        (sel4--trit-string 1)))
        (insert (format "    vz  %s  Coordinator (routes, coordinates)\n"
                        (sel4--trit-string 0)))
        (insert (format "    sel4 %s  Verifier   (attenuates, validates)\n"
                        (sel4--trit-string -1)))
        (insert (format "\n  Conservation: hv(+1) + vz(0) + sel4(-1) = 0 (mod 3)\n"))
        (insert (make-string 50 ?─) "\n\n")
        (insert "  Session IPC Statistics:\n")
        (insert (format "    Total messages:  %d\n" total))
        (insert (format "    %s Verifier ops:  %d\n"
                        (sel4--trit-string -1) (aref trit-counts 0)))
        (insert (format "    %s Coordinator:   %d\n"
                        (sel4--trit-string 0) (aref trit-counts 1)))
        (insert (format "    %s Generator ops: %d\n"
                        (sel4--trit-string 1) (aref trit-counts 2)))
        (insert (format "\n    Running trit sum: %s (mod 3)\n"
                        (sel4--trit-string sel4--ipc-trit-sum)))
        (insert (format "    Balanced: %s\n"
                        (if (zerop sel4--ipc-trit-sum)
                            (propertize "YES" 'face 'sel4-trit-plus)
                          (propertize "NO — VIOLATION" 'face 'sel4-fault))))
        ;; Bar chart
        (when (> total 0)
          (insert "\n  Distribution:\n")
          (let ((bar-width 40))
            (dolist (pair `((-1 ,(aref trit-counts 0) sel4-trit-minus "Verify")
                           (0  ,(aref trit-counts 1) sel4-trit-zero  "Coord")
                           (1  ,(aref trit-counts 2) sel4-trit-plus  "Create")))
              (let* ((count (nth 1 pair))
                     (face (nth 2 pair))
                     (label (nth 3 pair))
                     (pct (/ (* 100.0 count) total))
                     (bar-len (round (* bar-width (/ (float count) total)))))
                (insert (format "    %-6s %s%s %3.0f%%\n"
                                label
                                (propertize (make-string bar-len ?█) 'face face)
                                (make-string (- bar-width bar-len) ? )
                                pct)))))))
      (special-mode))
    (display-buffer buf)))

;; ── Fault Log ──

(defvar sel4--fault-log nil
  "Recorded seL4 faults.")

(defun sel4-fault-log ()
  "Display seL4 fault log."
  (interactive)
  (let ((buf (get-buffer-create "*seL4 Faults*")))
    (with-current-buffer buf
      (let ((inhibit-read-only t))
        (erase-buffer)
        (insert (propertize "seL4 Fault Log" 'face '(:weight bold :height 1.2))
                "\n")
        (insert (make-string 72 ?─) "\n")
        (if (null sel4--fault-log)
            (insert "\n  No faults recorded.\n")
          (dolist (fault sel4--fault-log)
            (let-alist fault
              (insert (propertize
                       (format "[%s] %s at 0x%X (IP=0x%X, badge=0x%X)\n"
                               .time .type .address .fault-ip .badge)
                       'face 'sel4-fault))
              (when .action
                (insert (format "       Action: %s\n" .action)))
              (when .reason
                (insert (format "       Reason: %s\n" .reason))))))
        (insert (make-string 72 ?─) "\n")
        (insert (format "\nFault types: vm-fault cap-fault user-exception unknown-syscall\n")))
      (special-mode))
    (display-buffer buf)))

;; ── Verification Matrix ──

(defun sel4-verification-matrix ()
  "Display seL4 formal verification status per architecture."
  (interactive)
  (let ((buf (get-buffer-create "*seL4 Verification*")))
    (with-current-buffer buf
      (let ((inhibit-read-only t))
        (erase-buffer)
        (insert (propertize "seL4 Verification Matrix" 'face '(:weight bold :height 1.2))
                "\n")
        (insert "Source: seL4 Foundation / Proofcraft\n\n")
        (insert (format "%-24s %7s %7s %7s %7s\n"
                        "Property" "ARMv7" "AArch64" "x86_64" "RISC-V"))
        (insert (make-string 60 ?─) "\n")
        (let ((props '(("Functional correctness" t    t     t     t)
                       ("Fast-path verified"     t    t     nil   nil)
                       ("Binary correctness"     t    nil   nil   t)
                       ("Integrity+availability" t    t     nil   t)
                       ("Confidentiality"        t    nil   nil   t)
                       ("Timing analysis"        t    nil   nil   nil))))
          (dolist (row props)
            (insert (format "%-24s %7s %7s %7s %7s\n"
                            (car row)
                            (sel4--check-mark (nth 1 row))
                            (sel4--check-mark (nth 2 row))
                            (sel4--check-mark (nth 3 row))
                            (sel4--check-mark (nth 4 row))))))
        (insert (make-string 60 ?─) "\n")
        (insert "\nARMv7 (aarch32) is the only fully verified architecture.\n")
        (insert "Isabelle bridge: boxxy/theories/seL4_Bridge.thy\n"))
      (special-mode))
    (display-buffer buf)))

(defun sel4--check-mark (val)
  "Return check or cross mark."
  (if val
      (propertize "  ✓" 'face 'sel4-cap-valid)
    (propertize "  ✗" 'face '(:foreground "#666666"))))

;; ── Architecture Diagram ──

(defun sel4-show-architecture ()
  "Display seL4/boxxy architecture."
  (interactive)
  (let ((buf (get-buffer-create "*seL4 Architecture*")))
    (with-current-buffer buf
      (let ((inhibit-read-only t))
        (erase-buffer)
        (insert (propertize "seL4 × boxxy × Emacs Architecture" 'face '(:weight bold :height 1.2))
                "\n\n")
        (insert "  ┌─────────────────────────────────────────────────────┐\n")
        (insert "  │  Emacs (sel4-mode.el)                              │\n")
        (insert "  │  IPC Monitor ⋅ Cap Inspector ⋅ GF(3) Balance      │\n")
        (insert "  │  Fault Log ⋅ Verification Matrix                   │\n")
        (insert "  └────────────────────┬────────────────────────────────┘\n")
        (insert "                       │ JSON-RPC 2.0 (4-byte BE + TCP :9998)\n")
        (insert "                       ↓\n")
        (insert "  ┌─────────────────────────────────────────────────────┐\n")
        (insert "  │  Nashator Bridge (:9999)                           │\n")
        (insert "  │  sel4 relay (:9998) ⟷ goblins relay (:9997)       │\n")
        (insert "  └────────────────────┬────────────────────────────────┘\n")
        (insert "                       │ seL4 IPC (capability invocation)\n")
        (insert "                       ↓\n")
        (insert "  ┌─────────────────────────────────────────────────────┐\n")
        (insert "  │  boxxy/std/sel4/         (Clojure/babashka)        │\n")
        (insert "  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │\n")
        (insert "  │  │ ipc.clj  │ │ cap.clj  │ │ common.clj       │   │\n")
        (insert "  │  │ msg pass │ │ HMAC-256 │ │ 6 patterns:      │   │\n")
        (insert "  │  │ ser/de   │ │ sideref  │ │  server loop     │   │\n")
        (insert "  │  └──────────┘ └──────────┘ │  notification    │   │\n")
        (insert "  │  ┌──────────┐ ┌──────────┐ │  shared memory   │   │\n")
        (insert "  │  │ balance  │ │ window   │ │  fault handling  │   │\n")
        (insert "  │  │ GF(3)   │ │ XMonad   │ │  memory mgmt    │   │\n")
        (insert "  │  │ triad   │ │ tiling   │ │  thread mgmt    │   │\n")
        (insert "  │  └──────────┘ └──────────┘ └──────────────────┘   │\n")
        (insert "  └────────────────────┬────────────────────────────────┘\n")
        (insert "                       │ CAmkES VMM\n")
        (insert "                       ↓\n")
        (insert "  ┌─────────────────────────────────────────────────────┐\n")
        (insert "  │  seL4 Microkernel (formally verified)              │\n")
        (insert "  │  Capabilities ⋅ IPC ⋅ Scheduling ⋅ Memory          │\n")
        (insert "  │  GF(3) trit: -1 (VERIFIER)                        │\n")
        (insert "  └─────────────────────────────────────────────────────┘\n")
        (insert "\n")
        (insert "  ┌─────────────────────────────────────────────────────┐\n")
        (insert "  │  Isabelle/HOL (boxxy/theories/)                    │\n")
        (insert "  │  seL4_Bridge.thy: 7 cap types → GF(3) partition    │\n")
        (insert "  │  Guillotine escalation → monotone capability map   │\n")
        (insert "  └─────────────────────────────────────────────────────┘\n"))
      (special-mode))
    (display-buffer buf)))

;; ── Navigation ──

(defun sel4-open-source (key)
  "Open a seL4 source file by KEY."
  (interactive
   (list (intern (completing-read "Source: "
                                  (mapcar #'car sel4--clj-sources)))))
  (let ((file (alist-get key sel4--clj-sources)))
    (find-file (expand-file-name file sel4-boxxy-dir))))

(defun sel4-open-isabelle (key)
  "Open an Isabelle theory file by KEY."
  (interactive
   (list (intern (completing-read "Theory: "
                                  (mapcar #'car sel4--isabelle-theories)))))
  (let ((file (alist-get key sel4--isabelle-theories)))
    (find-file (expand-file-name file sel4-boxxy-dir))))

(defun sel4-open-provider ()
  "Open the sel4-provider.edn spec."
  (interactive)
  (find-file (expand-file-name "std/sel4-provider.edn" sel4-boxxy-dir)))

;; ── Build / Test ──

(defun sel4-run-tests ()
  "Run seL4 parallel test suite (Nushell)."
  (interactive)
  (let ((default-directory sel4-boxxy-dir))
    (compile "nu sel4-parallel-test.nu")))

;; ── Keymap ──

(defvar sel4-command-map
  (let ((map (make-sparse-keymap)))
    (define-key map (kbd "m") #'sel4-ipc-monitor)
    (define-key map (kbd "c") #'sel4-capability-inspector)
    (define-key map (kbd "b") #'sel4-balance-report)
    (define-key map (kbd "f") #'sel4-fault-log)
    (define-key map (kbd "t") #'sel4-run-tests)
    (define-key map (kbd "s") #'sel4-open-source)
    (define-key map (kbd "i") #'sel4-open-isabelle)
    (define-key map (kbd "p") #'sel4-open-provider)
    (define-key map (kbd "a") #'sel4-show-architecture)
    (define-key map (kbd "v") #'sel4-verification-matrix)
    map)
  "Keymap for seL4 commands.")

(global-set-key (kbd "C-c C-4") sel4-command-map)

(provide 'sel4-mode)

;;; sel4-mode.el ends here
