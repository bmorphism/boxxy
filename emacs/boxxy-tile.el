;;; boxxy-tile.el --- Boxxy tiling with GF(3) obstruction integration  -*- lexical-binding: t; -*-

;; Copyright (C) 2026  bmorphism
;; SPDX-License-Identifier: Apache-2.0

;; Author: bmorphism
;; Keywords: tiling, GF(3), boxxy, obstruction, ewig, immer

;;; Commentary:

;; Integrates boxxy's tiling model with the obstruction-compositionality
;; framework and ewig/immer concepts.  Each tile carries a GF(3) trit
;; assignment; the layout must satisfy trit-sum = 0 (mod 3) for balance.
;;
;; Four canonical tiles in a 2x2 grid:
;;   auctex  (+1) — LaTeX/xy-pic obstruction diagrams
;;   ncatlab  (0) — EWW browser for nCatLab pages
;;   editor  (-1) — prog-mode editing buffer (or shell)
;;   monitor  (0) — GF(3) balance monitor
;;
;; Keybindings (under C-c C-t prefix, mnemonic: "tile"):
;;   C-c C-t s  — boxxy-tile-setup
;;   C-c C-t o  — boxxy-tile-show-obstruction
;;   C-c C-t c  — boxxy-tile-cycle-content
;;   C-c C-t x  — boxxy-tile-open-shell
;;   C-c C-t g  — boxxy-tile-gf3-check

;;; Code:

(require 'cl-lib)
(require 'obstruction-compositionality)

;; ── Customization ──

(defgroup boxxy-tile nil
  "Boxxy tiling with GF(3) balance."
  :group 'tools
  :prefix "boxxy-tile-")

(defcustom boxxy-tile-layout
  '(("auctex"   :trit  1 :color "#D04040" :mode latex-mode   :content obstruction)
    ("ncatlab"   :trit  0 :color "#D08D69" :mode eww-mode     :content ncatlab)
    ("editor"    :trit -1 :color "#5968A6" :mode prog-mode    :content buffer)
    ("monitor"   :trit  0 :color "#93DB59" :mode special-mode :content gf3-monitor))
  "Alist mapping tile names to their properties.
Each entry is (NAME :trit TRIT :color COLOR :mode MODE :content CONTENT).
TRIT is an integer in {-1, 0, 1}.  The sum of all trits must be 0 mod 3."
  :type '(alist :key-type string :value-type plist)
  :group 'boxxy-tile)

(defcustom boxxy-tile-shell-command "boxxy repl shell"
  "Command to run in the shell tile."
  :type 'string
  :group 'boxxy-tile)

;; ── Internal state ──

(defvar boxxy-tile--windows nil
  "Alist of (NAME . window) for active tiles.")

(defvar boxxy-tile--content-sources
  '(obstruction ncatlab buffer gf3-monitor shell)
  "Available content sources for cycling.")

;; ── Tile property accessors ──

(defun boxxy-tile--get (name prop)
  "Get property PROP from tile NAME in `boxxy-tile-layout'."
  (let ((entry (assoc name boxxy-tile-layout)))
    (when entry
      (plist-get (cdr entry) prop))))

(defun boxxy-tile--trits ()
  "Return the list of trit values from `boxxy-tile-layout'."
  (mapcar (lambda (entry) (plist-get (cdr entry) :trit))
          boxxy-tile-layout))

;; ── GF(3) balance ──

(defun boxxy-tile-gf3-check ()
  "Return t if the current tiling is GF(3) balanced (trit sum = 0 mod 3).
When called interactively, display the result."
  (interactive)
  (let* ((trits (boxxy-tile--trits))
         (sum (mod (apply #'+ trits) 3))
         (balanced (zerop sum)))
    (when (called-interactively-p 'interactive)
      (message "[GF3] Trits: %s | Sum mod 3: %d | %s"
               (mapconcat (lambda (t) (format "%+d" t)) trits " ")
               sum
               (if balanced "BALANCED" "BROKEN")))
    balanced))

(defun boxxy-tile--gf3-status-string ()
  "Return a mode-line string showing GF(3) balance status."
  (let* ((trits (boxxy-tile--trits))
         (sum (mod (apply #'+ trits) 3))
         (balanced (zerop sum))
         (trit-str (mapconcat (lambda (tr) (format "%+d" tr)) trits " ")))
    (if balanced
        (propertize (format "[GF3: %s %s]" "ok" trit-str)
                    'face '(:foreground "#44CF6C"))
      (propertize (format "[GF3: !! %s]" trit-str)
                  'face '(:foreground "#FF4444")))))

;; ── Mode-line minor mode ──

(defvar boxxy-tile-modeline--string ""
  "Cached mode-line string for GF(3) status.")

(defun boxxy-tile-modeline--update ()
  "Update the cached GF(3) mode-line string."
  (setq boxxy-tile-modeline--string
        (concat " " (boxxy-tile--gf3-status-string))))

(define-minor-mode boxxy-tile-modeline
  "Minor mode showing GF(3) tiling balance in the mode line."
  :lighter boxxy-tile-modeline--string
  :global t
  (if boxxy-tile-modeline
      (progn
        (boxxy-tile-modeline--update)
        (add-hook 'window-configuration-change-hook #'boxxy-tile-modeline--update))
    (remove-hook 'window-configuration-change-hook #'boxxy-tile-modeline--update)))

;; ── Tile content providers ──

(defun boxxy-tile--create-gf3-monitor ()
  "Create and return a GF(3) monitor buffer."
  (let ((buf (get-buffer-create "*boxxy GF(3) monitor*")))
    (with-current-buffer buf
      (let ((inhibit-read-only t))
        (erase-buffer)
        (insert (propertize "boxxy GF(3) Tiling Monitor"
                            'face '(:weight bold :height 1.2))
                "\n")
        (insert (make-string 50 ?-) "\n\n")
        (dolist (entry boxxy-tile-layout)
          (let* ((name (car entry))
                 (trit (plist-get (cdr entry) :trit))
                 (color (plist-get (cdr entry) :color))
                 (face `(:foreground ,color :weight bold)))
            (insert (format "  %-12s  trit: %s  color: %s\n"
                            (propertize name 'face face)
                            (propertize (format "%+d" trit)
                                        'face face)
                            color))))
        (insert "\n" (make-string 50 ?-) "\n")
        (let* ((trits (boxxy-tile--trits))
               (sum (mod (apply #'+ trits) 3))
               (balanced (zerop sum)))
          (insert (format "\n  Sum of trits: %d  (mod 3 = %d)\n"
                          (apply #'+ trits) sum))
          (insert (format "  Status: %s\n"
                          (if balanced
                              (propertize "BALANCED" 'face '(:foreground "#44CF6C" :weight bold))
                            (propertize "BROKEN" 'face '(:foreground "#FF4444" :weight bold))))))
        (insert "\n\n  ewig: immutable structural sharing (immer)\n")
        (insert "  Each tile = persistent snapshot; undo = history tree\n"))
      (special-mode))
    buf))

(defun boxxy-tile--set-modeline-color (window color)
  "Set the mode-line background color for WINDOW's buffer to COLOR."
  (with-selected-window window
    (face-remap-add-relative 'mode-line :background color :foreground "#FFFFFF")))

(defun boxxy-tile--populate (name window)
  "Populate WINDOW with content for tile NAME."
  (let ((content (boxxy-tile--get name :content)))
    (select-window window)
    (pcase content
      ('obstruction
       (let ((buf (get-buffer-create (format "*boxxy-tile:%s*" name))))
         (with-current-buffer buf
           (let ((inhibit-read-only t))
             (erase-buffer)
             (insert "% boxxy-tile: auctex obstruction tile\n")
             (insert "% Use C-c C-t o to load an obstruction diagram\n\n")
             (insert "\\documentclass{article}\n")
             (insert "\\usepackage[all]{xy}\n")
             (insert "\\begin{document}\n")
             (insert "% awaiting obstruction selection...\n")
             (insert "\\end{document}\n"))
           (when (fboundp 'LaTeX-mode) (LaTeX-mode)))
         (set-window-buffer window buf)))
      ('ncatlab
       (let ((buf (get-buffer-create (format "*boxxy-tile:%s*" name))))
         (with-current-buffer buf
           (let ((inhibit-read-only t))
             (erase-buffer)
             (insert "nCatLab tile — use C-c C-t o to browse obstruction pages\n")))
         (set-window-buffer window buf)))
      ('buffer
       (let ((buf (or (cl-find-if
                       (lambda (b)
                         (with-current-buffer b
                           (derived-mode-p 'prog-mode)))
                       (buffer-list))
                      (get-buffer-create "*boxxy-tile:editor*"))))
         (set-window-buffer window buf)))
      ('gf3-monitor
       (set-window-buffer window (boxxy-tile--create-gf3-monitor)))
      ('shell
       (boxxy-tile--open-shell-in-window window)))))

(defun boxxy-tile--open-shell-in-window (window)
  "Open a shell running `boxxy-tile-shell-command' in WINDOW."
  (select-window window)
  (if (fboundp 'vterm)
      (progn
        (vterm (format "*boxxy-shell*"))
        (vterm-send-string (concat boxxy-tile-shell-command "\n")))
    (ansi-term "/bin/sh" "boxxy-shell")
    (term-send-raw-string (concat boxxy-tile-shell-command "\n"))))

;; ── Main setup ──

;;;###autoload
(defun boxxy-tile-setup ()
  "Set up the boxxy 2x2 tiled layout.
Deletes other windows, splits into a 2x2 grid, assigns each tile
its content, colors the mode-line per trit, and verifies GF(3) balance."
  (interactive)
  ;; Verify GF(3) balance before proceeding
  (let* ((trits (boxxy-tile--trits))
         (sum (mod (apply #'+ trits) 3)))
    (unless (zerop sum)
      (user-error "boxxy-tile: layout is not GF(3) balanced (sum=%d mod 3=%d)"
                  (apply #'+ trits) sum)))
  ;; Create 2x2 grid
  (delete-other-windows)
  (let* ((top-left (selected-window))
         (top-right (split-window-right))
         (_  (select-window top-left))
         (bot-left (split-window-below))
         (_  (select-window top-right))
         (bot-right (split-window-below))
         (windows (list top-left top-right bot-left bot-right))
         (names (mapcar #'car boxxy-tile-layout)))
    ;; Assign windows
    (setq boxxy-tile--windows
          (cl-mapcar #'cons names windows))
    ;; Populate each tile
    (cl-mapc
     (lambda (name win)
       (boxxy-tile--populate name win)
       (boxxy-tile--set-modeline-color win (boxxy-tile--get name :color)))
     names windows)
    ;; Activate modeline minor mode
    (boxxy-tile-modeline 1)
    ;; Focus top-left
    (select-window top-left)
    (message "boxxy-tile: 2x2 layout ready | %s"
             (boxxy-tile--gf3-status-string))))

;; ── Shell tile ──

;;;###autoload
(defun boxxy-tile-open-shell ()
  "Open a shell tile replacing the editor tile.
The shell tile gets trit -1 (Verifier)."
  (interactive)
  (let ((editor-win (cdr (assoc "editor" boxxy-tile--windows))))
    (unless (and editor-win (window-live-p editor-win))
      (user-error "boxxy-tile: no live editor tile; run boxxy-tile-setup first"))
    ;; Update layout: change editor content to shell
    (setf (plist-get (cdr (assoc "editor" boxxy-tile-layout)) :content) 'shell)
    (boxxy-tile--open-shell-in-window editor-win)
    (message "boxxy-tile: shell opened in editor position (trit -1, Verifier)")))

;; ── Cycle content ──

;;;###autoload
(defun boxxy-tile-cycle-content ()
  "Cycle the content source of the current tile."
  (interactive)
  (let* ((win (selected-window))
         (tile-entry (cl-find-if
                      (lambda (pair) (eq (cdr pair) win))
                      boxxy-tile--windows))
         (name (car tile-entry)))
    (unless name
      (user-error "boxxy-tile: current window is not a boxxy tile"))
    (let* ((layout-entry (assoc name boxxy-tile-layout))
           (current (plist-get (cdr layout-entry) :content))
           (idx (cl-position current boxxy-tile--content-sources))
           (next-idx (mod (1+ (or idx 0)) (length boxxy-tile--content-sources)))
           (next (nth next-idx boxxy-tile--content-sources)))
      (setf (plist-get (cdr layout-entry) :content) next)
      (boxxy-tile--populate name win)
      (boxxy-tile-modeline--update)
      (message "boxxy-tile: %s -> %s" name next))))

;; ── Obstruction integration ──

;;;###autoload
(defun boxxy-tile-show-obstruction (name)
  "Display obstruction NAME in the tiled layout.
Shows the xy-pic diagram in the auctex tile and the nCatLab page
in the ncatlab tile."
  (interactive
   (list (completing-read "Obstruction: "
                          (mapcar #'obstruction-name obstruction-catalog)
                          nil t)))
  (let ((obs (obstruction-find name)))
    (unless obs
      (user-error "Unknown obstruction: %s" name))
    ;; AUCTeX tile: write xy-pic
    (let ((auctex-win (cdr (assoc "auctex" boxxy-tile--windows))))
      (when (and auctex-win (window-live-p auctex-win))
        (let* ((tex-path (obstruction--write-tex-file obs))
               (buf (find-file-noselect tex-path)))
          (with-current-buffer buf
            (when (fboundp 'LaTeX-mode) (LaTeX-mode))
            (goto-char (point-min)))
          (set-window-buffer auctex-win buf))))
    ;; nCatLab tile: open in EWW
    (let ((ncatlab-win (cdr (assoc "ncatlab" boxxy-tile--windows))))
      (when (and ncatlab-win (window-live-p ncatlab-win))
        (select-window ncatlab-win)
        (eww (obstruction-ncatlab obs))))
    ;; Refresh monitor
    (let ((monitor-win (cdr (assoc "monitor" boxxy-tile--windows))))
      (when (and monitor-win (window-live-p monitor-win))
        (set-window-buffer monitor-win (boxxy-tile--create-gf3-monitor))))
    (message "boxxy-tile: showing %s (severity %+d, %s)"
             (obstruction-name obs)
             (obstruction-severity obs)
             (obstruction-severity-label (obstruction-severity obs)))))

;; ── Keymap ──

(defvar boxxy-tile-command-map
  (let ((map (make-sparse-keymap)))
    (define-key map (kbd "s") #'boxxy-tile-setup)
    (define-key map (kbd "o") #'boxxy-tile-show-obstruction)
    (define-key map (kbd "c") #'boxxy-tile-cycle-content)
    (define-key map (kbd "x") #'boxxy-tile-open-shell)
    (define-key map (kbd "g") #'boxxy-tile-gf3-check)
    map)
  "Keymap for boxxy-tile commands.")

(global-set-key (kbd "C-c C-t") boxxy-tile-command-map)

(provide 'boxxy-tile)

;;; boxxy-tile.el ends here
