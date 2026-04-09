;;; sel4-bridge.el --- Connect sel4-mode to the seL4 bridge relay  -*- lexical-binding: t; -*-

;; Copyright (C) 2026  bmorphism
;; SPDX-License-Identifier: Apache-2.0

;; Author: bmorphism
;; Keywords: seL4, bridge, IPC, monitor

;;; Commentary:

;; Connects sel4-mode.el to the Clojure/babashka bridge on :9998.
;; The bridge relays seL4 JSON-RPC methods to/from Nashator on :9999.
;;
;; Wire protocol: 4-byte BE length prefix + JSON-RPC 2.0
;; (same as zig-syrup/src/message_frame.zig and nashator-captp.zig)
;;
;; Usage:
;;   M-x sel4-bridge-connect       — connect to bridge on :9998
;;   M-x sel4-bridge-subscribe     — subscribe to live IPC feed
;;   M-x sel4-bridge-unsubscribe   — unsubscribe from live feed
;;   M-x sel4-bridge-disconnect    — disconnect from bridge
;;   M-x sel4-bridge-status        — show connection status
;;
;; Hooks into sel4--rpc from sel4-mode.el for transparent bridging.

;;; Code:

(require 'sel4-mode)

;; ── Connection state ──

(defvar sel4-bridge--process nil
  "Network process for the bridge connection.")

(defvar sel4-bridge--subscribed nil
  "Non-nil when subscribed to the live IPC monitor feed.")

(defvar sel4-bridge--subscription-id nil
  "JSON-RPC request ID used for the monitor subscription.")

;; ── Connect / Disconnect ──

;;;###autoload
(defun sel4-bridge-connect ()
  "Connect to the seL4 bridge on :9998.
Replaces the default sel4--connection with a bridge-aware one."
  (interactive)
  ;; Use the existing sel4-mode wire protocol machinery
  (sel4--connect)
  (if (and sel4--connection (process-live-p sel4--connection))
      (progn
        (setq sel4-bridge--process sel4--connection)
        (message "sel4-bridge: connected to %s:%d"
                 sel4-bridge-host sel4-bridge-port))
    (message "sel4-bridge: connection failed")))

(defun sel4-bridge-disconnect ()
  "Disconnect from the seL4 bridge."
  (interactive)
  (when sel4-bridge--subscribed
    (sel4-bridge-unsubscribe))
  (when (and sel4-bridge--process (process-live-p sel4-bridge--process))
    (delete-process sel4-bridge--process))
  (setq sel4-bridge--process nil
        sel4--connection nil)
  (message "sel4-bridge: disconnected"))

;; ── Monitor subscription ──

(defun sel4-bridge-subscribe ()
  "Subscribe to the live IPC monitor feed from the bridge.
New IPC messages will automatically refresh the *seL4 IPC Monitor* buffer."
  (interactive)
  (unless (and sel4--connection (process-live-p sel4--connection))
    (sel4-bridge-connect))
  (when (and sel4--connection (process-live-p sel4--connection))
    (setq sel4-bridge--subscription-id
          (sel4--rpc "sel4.monitor.subscribe"
                     '((filters . []))
                     #'sel4-bridge--on-subscribe-response))
    (setq sel4-bridge--subscribed t)
    (message "sel4-bridge: subscribing to IPC monitor feed...")))

(defun sel4-bridge-unsubscribe ()
  "Unsubscribe from the live IPC monitor feed."
  (interactive)
  (when (and sel4--connection (process-live-p sel4--connection)
             sel4-bridge--subscribed)
    (sel4--rpc "sel4.monitor.unsubscribe" '()
               #'sel4-bridge--on-unsubscribe-response)
    (setq sel4-bridge--subscribed nil)
    (message "sel4-bridge: unsubscribed from IPC feed")))

(defun sel4-bridge--on-subscribe-response (json)
  "Handle subscription confirmation."
  (let ((result (alist-get 'result json)))
    (message "sel4-bridge: subscribed (client=%s)"
             (alist-get 'client_id result))))

(defun sel4-bridge--on-unsubscribe-response (_json)
  "Handle unsubscription confirmation."
  (message "sel4-bridge: unsubscribed"))

;; ── Notification handling ──
;;
;; The bridge pushes sel4.monitor.event notifications when subscribed.
;; We intercept these in the recv filter to update the IPC monitor.

(defun sel4-bridge--handle-notification (json)
  "Handle an incoming JSON-RPC notification from the bridge.
Updates the IPC monitor buffer if the notification is a monitor event."
  (let ((method (alist-get 'method json))
        (params (alist-get 'params json)))
    (when (string= method "sel4.monitor.event")
      (sel4-bridge--process-monitor-event params))))

(defun sel4-bridge--process-monitor-event (event)
  "Process a monitor event and update the IPC log + buffer.
EVENT is an alist with type, direction, method, endpoint, trit, etc."
  (let ((direction (intern (or (alist-get 'direction event) "recv")))
        (endpoint  (or (alist-get 'endpoint event) "bridge"))
        (method    (or (alist-get 'method event) "unknown"))
        (trit      (or (alist-get 'trit event) 0)))
    ;; Log into sel4-mode's IPC log
    (sel4--log-ipc direction endpoint
                   (intern (car (last (split-string method "\\."))))
                   0 nil)
    ;; Force refresh if monitor buffer is visible
    (when-let ((buf (get-buffer "*seL4 IPC Monitor*")))
      (when (get-buffer-window buf)
        (sel4--refresh-monitor buf)))))

;; ── Enhanced recv filter ──
;;
;; Override sel4--recv-filter to also handle notifications (no id field).

(defun sel4-bridge--recv-filter (_proc data)
  "Enhanced receive filter that handles both responses and notifications.
Notifications (no `id') are routed to the monitor event handler."
  (setq sel4--recv-buffer (concat sel4--recv-buffer data))
  (let ((frame (sel4--decode-frame sel4--recv-buffer)))
    (while frame
      (let* ((payload (decode-coding-string (car frame) 'utf-8))
             (json (json-read-from-string payload))
             (id (alist-get 'id json)))
        (if id
            ;; Response — dispatch to pending callback
            (let ((callback (gethash id sel4--pending-callbacks)))
              (when callback
                (remhash id sel4--pending-callbacks)
                (funcall callback json)))
          ;; Notification — handle monitor events
          (sel4-bridge--handle-notification json)))
      (setq sel4--recv-buffer (cdr frame))
      (setq frame (sel4--decode-frame sel4--recv-buffer)))))

;; Install the enhanced filter when connecting
(advice-add 'sel4--connect :after #'sel4-bridge--install-filter)

(defun sel4-bridge--install-filter (&rest _)
  "Install the enhanced recv filter that handles notifications."
  (when (and sel4--connection (process-live-p sel4--connection))
    (set-process-filter sel4--connection #'sel4-bridge--recv-filter)))

;; ── Bridge-aware RPC wrappers ──

(defun sel4-bridge-ipc-send (endpoint args)
  "Send an IPC message via the bridge."
  (interactive "sEndpoint: \nsArgs (JSON): ")
  (sel4--rpc "sel4.ipc.send"
             `((endpoint . ,endpoint) (args . ,args))
             (lambda (json)
               (message "sel4-bridge: sent -> %s" (alist-get 'result json)))))

(defun sel4-bridge-cap-query (&optional cap-id)
  "Query capabilities via the bridge."
  (interactive "sCapability ID (or * for all): ")
  (sel4--rpc "sel4.cap.query"
             `((id . ,(or cap-id "*")))
             (lambda (json)
               (let ((result (alist-get 'result json)))
                 (message "sel4-bridge: caps -> %s" result)
                 ;; Update sel4--capabilities for the inspector
                 (when-let ((caps (alist-get 'capabilities result)))
                   (setq sel4--capabilities caps)
                   ;; Refresh capability inspector if open
                   (when (get-buffer "*seL4 Capabilities*")
                     (sel4-capability-inspector)))))))

(defun sel4-bridge-balance-report ()
  "Request GF(3) balance report from the bridge."
  (interactive)
  (sel4--rpc "sel4.balance.report" '()
             (lambda (json)
               (let ((result (alist-get 'result json)))
                 (message "sel4-bridge: trit_sum=%s balanced=%s msgs=%s"
                          (alist-get 'session_trit_sum result)
                          (alist-get 'balanced result)
                          (alist-get 'message_count result))))))

(defun sel4-bridge-fault-log ()
  "Request fault log from the bridge."
  (interactive)
  (sel4--rpc "sel4.fault.log" '()
             (lambda (json)
               (let* ((result (alist-get 'result json))
                      (faults (alist-get 'faults result)))
                 (setq sel4--fault-log faults)
                 (sel4-fault-log)))))

;; ── Status ──

(defun sel4-bridge-status ()
  "Show bridge connection status."
  (interactive)
  (message "sel4-bridge: %s | subscribed: %s | pending: %d"
           (if (and sel4--connection (process-live-p sel4--connection))
               (format "connected to %s:%d" sel4-bridge-host sel4-bridge-port)
             "disconnected")
           (if sel4-bridge--subscribed "yes" "no")
           (hash-table-count sel4--pending-callbacks)))

;; ── Keymap additions ──
;; Extend sel4-command-map with bridge-specific bindings

(define-key sel4-command-map (kbd "C") #'sel4-bridge-connect)
(define-key sel4-command-map (kbd "S") #'sel4-bridge-subscribe)
(define-key sel4-command-map (kbd "U") #'sel4-bridge-unsubscribe)
(define-key sel4-command-map (kbd "D") #'sel4-bridge-disconnect)
(define-key sel4-command-map (kbd "?") #'sel4-bridge-status)

(provide 'sel4-bridge)

;;; sel4-bridge.el ends here
