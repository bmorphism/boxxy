;;; basin-headless.el --- Self-operating magit with peer discovery  -*- lexical-binding: t; -*-

;; Copyright (C) 2026 bmorphism
;; Author: bmorphism
;; Keywords: vc, tools, network
;; Package-Requires: ((emacs "29.1") (magit "4.0") (crdt "0.3") (geiser "0.28.1") (transient "0.3"))

;;; Commentary:

;; Basin headless mode: self-operating magit with Tailscale peer discovery.
;;
;; Architecture:
;;   causality (self)     — this machine, macOS, the operator
;;   2-monad              — online peer, macOS
;;   raspberrypi           — linux peer (offline)
;;   hatchery             — macOS peer (offline)
;;
;; Workflow:
;;   1. Discover peers via `tailscale status --json`
;;   2. Auto-add online peers as magit remotes
;;   3. Fetch from all reachable peers
;;   4. Optionally connect geiser-hoot REPL to remote Hoot server
;;   5. CRDT collaborative editing across mesh
;;
;; GF(3) Assignment (SplitMix64 from GAY_SEED):
;;   causality    → trit  0 (ERGODIC/coordinator)
;;   2-monad      → trit +1 (PLUS/generator)
;;   raspberrypi  → trit -1 (MINUS/validator)
;;   Sum: 0 + 1 + (-1) = 0 CONSERVED
;;
;; Wireworld integration:
;;   Peer mesh topology maps to wireworld circuit.
;;   Online peers = electron heads (@), offline = wire (.),
;;   recently-disconnected = electron tails (*).
;;   The mesh IS a cellular automaton.

;;; Code:

(require 'cl-lib)
(require 'json)

;; ══════════════════════════════════════════════════════════════════════
;; Peer Discovery
;; ══════════════════════════════════════════════════════════════════════

(defvar basin-mesh-name "pirate-dragon"
  "Tailscale mesh (tailnet) name.")

(defvar basin-repo-path "/Users/bob/i/boxxy"
  "Local path to the boxxy repository.")

(defvar basin-remote-repo-paths
  '(("2-monad" . "~/i/boxxy")
    ("raspberrypi" . "~/i/boxxy")
    ("hatchery" . "~/i/boxxy"))
  "Alist of peer hostname to repo path on that peer.")

(defvar basin-peer-trit-assignments
  '(("causality" . 0)
    ("2-monad" . 1)
    ("raspberrypi" . -1)
    ("hatchery" . 0)
    ("MacBook Air (2)" . 1)
    ("localhost" . -1))
  "GF(3) trit assignments for each peer. Sum should be 0 mod 3.")

(defvar basin--peer-cache nil
  "Cached peer discovery result.")

(defun basin--parse-peer (peer-table)
  "Extract fields from a peer hash-table PEER-TABLE."
  (let* ((host (gethash "HostName" peer-table))
         (ips (gethash "TailscaleIPs" peer-table))
         (online (eq (gethash "Online" peer-table) t))
         (os (gethash "OS" peer-table))
         (trit (or (cdr (assoc host basin-peer-trit-assignments)) 0))
         (ip (cond ((vectorp ips) (aref ips 0))
                   ((listp ips) (car ips))
                   (t nil))))
    `((host . ,host)
      (ip . ,ip)
      (online . ,online)
      (os . ,os)
      (trit . ,trit))))

(defun basin-discover-peers ()
  "Discover Tailscale mesh peers. Returns alist of peers with status."
  (interactive)
  (let* ((json-str (shell-command-to-string "tailscale status --json 2>/dev/null"))
         (data (condition-case nil
                   (json-parse-string json-str :object-type 'hash-table)
                 (error nil))))
    (when data
      (let* ((peers-ht (gethash "Peer" data))
             (peers nil))
        (when (hash-table-p peers-ht)
          (maphash (lambda (_key val)
                     (push (basin--parse-peer val) peers))
                   peers-ht))
        (setq basin--peer-cache (nreverse peers))
        (when (called-interactively-p 'interactive)
          (message "basin: %d peers (%d online)"
                   (length basin--peer-cache)
                   (cl-count-if (lambda (p) (alist-get 'online p)) basin--peer-cache)))
        basin--peer-cache))))

(defun basin-online-peers ()
  "Return only online peers."
  (cl-remove-if-not (lambda (p) (alist-get 'online p))
                     (or basin--peer-cache (basin-discover-peers))))

;; ══════════════════════════════════════════════════════════════════════
;; Self-Operating Magit
;; ══════════════════════════════════════════════════════════════════════

(defun basin-magit-sync-remotes ()
  "Auto-add online Tailscale peers as git remotes and fetch."
  (interactive)
  (require 'magit)
  (let ((default-directory basin-repo-path)
        (peers (basin-online-peers))
        (existing-remotes (magit-list-remotes))
        (added 0)
        (fetched 0))
    ;; Add remotes for online peers that have repo paths configured
    (dolist (peer peers)
      (let* ((host (alist-get 'host peer))
             (ip (alist-get 'ip peer))
             (remote-path (cdr (assoc host basin-remote-repo-paths)))
             (remote-name (replace-regexp-in-string "[^a-zA-Z0-9_-]" "-" host)))
        (when (and remote-path ip)
          ;; Add remote if not exists
          (unless (member remote-name existing-remotes)
            (let ((url (format "%s:%s" ip remote-path)))
              (magit-call-git "remote" "add" remote-name url)
              (cl-incf added)
              (message "basin: added remote %s -> %s" remote-name url)))
          ;; Fetch from this remote
          (condition-case err
              (progn
                (magit-call-git "fetch" remote-name "--quiet")
                (cl-incf fetched)
                (message "basin: fetched %s (trit=%+d)" remote-name (alist-get 'trit peer)))
            (error (message "basin: fetch %s failed: %s" remote-name (error-message-string err)))))))
    (message "basin: sync complete — added %d remotes, fetched %d" added fetched)
    (when (called-interactively-p 'interactive)
      (magit-status))))

(defun basin-magit-push-to-peer (peer-name)
  "Push current branch to a specific peer remote."
  (interactive
   (list (completing-read "Push to peer: "
                          (mapcar (lambda (p) (alist-get 'host p))
                                  (basin-online-peers)))))
  (require 'magit)
  (let ((default-directory basin-repo-path)
        (remote-name (replace-regexp-in-string "[^a-zA-Z0-9_-]" "-" peer-name))
        (branch (magit-get-current-branch)))
    (magit-call-git "push" remote-name branch)
    (message "basin: pushed %s to %s" branch remote-name)))

;; ══════════════════════════════════════════════════════════════════════
;; Wireworld Mesh Visualization
;; ══════════════════════════════════════════════════════════════════════

(defun basin-mesh-to-wireworld ()
  "Render the Tailscale peer mesh as a wireworld circuit.
Online peers are electron heads (@), offline are wire (.),
self is electron tail (*) showing recent activity."
  (interactive)
  (let* ((peers (or basin--peer-cache (basin-discover-peers)))
         (buf (get-buffer-create "*Basin Mesh*")))
    (with-current-buffer buf
      (erase-buffer)
      (insert "Basin Mesh Topology (wireworld)\n")
      (insert "================================\n\n")
      ;; Self at center
      (insert "                  * causality (self, trit=0)\n")
      (insert "                 /|\\\n")
      ;; Fan out to peers
      (let ((i 0))
        (dolist (peer peers)
          (let* ((host (alist-get 'host peer))
                 (online (alist-get 'online peer))
                 (trit (alist-get 'trit peer))
                 (cell (if online "@" "."))
                 (pad (make-string (* i 2) ?\s)))
            (insert (format "       %s.........%s %s (trit=%+d, %s)\n"
                            pad cell host trit
                            (alist-get 'os peer)))
            (cl-incf i))))
      (insert "\n")
      (insert "@ = online (electron head)\n")
      (insert ". = offline (wire)\n")
      (insert "* = self (electron tail)\n")
      (wireworld-mode))
    (switch-to-buffer buf)))

;; ══════════════════════════════════════════════════════════════════════
;; CRDT Peer Collaboration
;; ══════════════════════════════════════════════════════════════════════

(defun basin-crdt-share-buffer ()
  "Start a CRDT session for collaborative editing with mesh peers."
  (interactive)
  (require 'crdt)
  (if (fboundp 'crdt-share-buffer)
      (progn
        (crdt-share-buffer)
        (message "basin: CRDT session started — share URL with mesh peers"))
    (message "basin: crdt package not available")))

;; ══════════════════════════════════════════════════════════════════════
;; Geiser-Hoot Remote REPL
;; ══════════════════════════════════════════════════════════════════════

(defun basin-connect-remote-hoot (peer-name)
  "Connect to a Hoot dev server running on a mesh peer."
  (interactive
   (list (completing-read "Connect to Hoot on: "
                          (mapcar (lambda (p) (alist-get 'host p))
                                  (basin-online-peers)))))
  (require 'geiser-hoot nil t)
  (if (fboundp 'connect-to-hoot)
      (let* ((peer (cl-find-if (lambda (p) (string= (alist-get 'host p) peer-name))
                               (basin-online-peers)))
             (ip (alist-get 'ip peer)))
        (message "basin: connecting to hoot on %s (%s)..." peer-name ip)
        (connect-to-hoot))
    (message "basin: geiser-hoot not available")))

;; ══════════════════════════════════════════════════════════════════════
;; Headless Entry Point
;; ══════════════════════════════════════════════════════════════════════

(defun basin-headless-full ()
  "Complete headless startup: discover, sync, report."
  (interactive)
  (message "basin: === headless full startup ===")
  ;; 1. Discover peers
  (let ((peers (basin-discover-peers)))
    (message "basin: %d peers on %s mesh" (length peers) basin-mesh-name)
    (dolist (peer peers)
      (let ((host (or (alist-get 'host peer) "???"))
            (ip (or (alist-get 'ip peer) "n/a"))
            (online (alist-get 'online peer))
            (trit (or (alist-get 'trit peer) 0))
            (os (or (alist-get 'os peer) "")))
        (message "  %s %-18s %s  trit=%+d  %s"
                 (if online "@" ".") host ip trit os))))
  ;; 2. Sygil GF(3) check on peer trits
  (when (fboundp 'sygil-check-gf3)
    (let* ((online (basin-online-peers))
           (trits (mapcar (lambda (p) (alist-get 'trit p)) online))
           (sum (apply #'+ trits))
           (conserved (zerop (mod (+ (mod sum 3) 3) 3))))
      (message "basin: online peer trits=%S sum=%d %s"
               trits sum (if conserved "GF(3) CONSERVED" "GF(3) BROKEN"))))
  ;; 3. Sync magit remotes (skip in batch mode to avoid blocking)
  (if noninteractive
      (message "basin: magit sync skipped (batch mode)")
    (condition-case err
        (basin-magit-sync-remotes)
      (error (message "basin: magit sync skipped: %s" (error-message-string err)))))
  ;; 4. Report persistence homology if results exist
  (basin-report-persistence)
  (message "basin: === headless startup complete ==="))

;; ══════════════════════════════════════════════════════════════════════
;; Persistence Homology Integration
;; ══════════════════════════════════════════════════════════════════════

(defvar basin-persistence-result-file
  (expand-file-name "worlds/b/basin/persistence/boxxy-persistence-result.json"
                    basin-repo-path)
  "Path to the latest ripser persistence result JSON.")

(defun basin-report-persistence ()
  "Report GF(3) trit from latest persistence computation."
  (interactive)
  (if (file-exists-p basin-persistence-result-file)
      (condition-case err
          (let* ((json-str (with-temp-buffer
                             (insert-file-contents basin-persistence-result-file)
                             (buffer-string)))
                 (data (json-parse-string json-str :object-type 'hash-table))
                 (betti (gethash "betti_numbers" data))
                 (euler (gethash "euler_characteristic" data))
                 (n-pts (gethash "n_points" data))
                 (backend (gethash "backend" data))
                 (time-ms (gethash "computation_time_ms" data))
                 (b0 (if (> (length betti) 0) (aref betti 0) 0))
                 (b1 (if (> (length betti) 1) (aref betti 1) 0))
                 (b2 (if (> (length betti) 2) (aref betti 2) 0))
                 (trit (cond ((> b0 b1) 1) ((< b0 b1) -1) (t 0)))
                 (role (cond ((= trit 1) "PLUS/generator")
                             ((= trit -1) "MINUS/validator")
                             (t "ERGODIC/coordinator"))))
            (message "basin: persistence β=(%d,%d,%d) χ=%d trit=%+d (%s) [%d pts, %s, %.0fms]"
                     b0 b1 b2 euler trit role n-pts backend time-ms))
        (error (message "basin: persistence parse error: %s" (error-message-string err))))
    (message "basin: no persistence results (run worlds/b/basin/persistence/run-persistence.sh)")))

(defun basin-run-persistence ()
  "Dispatch persistence computation to dgx-spark asynchronously."
  (interactive)
  (let ((script (expand-file-name "worlds/b/basin/persistence/run-persistence.sh"
                                  basin-repo-path)))
    (if (file-exists-p script)
        (progn
          (message "basin: dispatching persistence to dgx-spark...")
          (async-shell-command script "*basin-persistence*"))
      (message "basin: run-persistence.sh not found"))))

(provide 'basin-headless)
;;; basin-headless.el ends here
