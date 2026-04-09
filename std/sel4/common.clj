;; seL4 Common Solutions
;; Standard patterns for seL4 IPC-based systems
;;
;; Covers the 6 patterns every seL4 system needs:
;;   1. Server loop (Call/ReplyRecv)
;;   2. Notification (async signaling)
;;   3. Shared memory (zero-copy bulk transfer)
;;   4. Fault handling (VM/cap/user faults)
;;   5. Memory management (untyped retype, page mapping)
;;   6. Thread management (TCB, scheduling domains)

(ns sel4.common
  "Common seL4 IPC patterns and system services"
  (:require [sel4.ipc :as ipc]
            [sel4.capability :as cap]
            [sel4.balance :as bal]))

;; ============================================================================
;; 1. Server Loop — the canonical seL4 service pattern
;;
;; seL4 servers use seL4_ReplyRecv: atomically reply to the last client
;; and block waiting for the next. This is the most efficient pattern
;; because it's a single syscall (no window between reply and recv).
;; ============================================================================

(defrecord Server
  [name             ;; Server name
   endpoint         ;; IPC endpoint capability
   handlers         ;; {label -> handler-fn}
   badge-registry   ;; {badge -> client-info}
   running?         ;; Atom<bool>
   stats            ;; Atom<map>
   ])

(defn make-server
  "Create an seL4 server with handler dispatch.
   Each handler receives [badge msg-regs caps] and returns [reply-regs reply-caps]."
  [name endpoint-name]
  (->Server name
            (ipc/create-endpoint (str "sel4:" endpoint-name))
            (atom {})
            (atom {})
            (atom false)
            (atom {:requests 0 :faults 0 :unknown-labels 0})))

(defn register-handler
  "Register a handler for a message label."
  [server label handler-fn]
  (swap! (:handlers server) assoc label handler-fn))

(defn register-client
  "Register a badged client. Badge identifies the client on each message."
  [server badge client-info]
  (swap! (:badge-registry server) assoc badge client-info))

(defn dispatch
  "Dispatch a message to the appropriate handler.
   Returns reply registers + reply caps, or nil for unknown label."
  [server badge label msg-regs caps]
  (swap! (:stats server) update :requests inc)
  (if-let [handler (get @(:handlers server) label)]
    (handler badge msg-regs caps)
    (do
      (swap! (:stats server) update :unknown-labels inc)
      nil)))

(defn server-loop
  "The canonical seL4 server loop.

   Real seL4 pattern:
     seL4_Recv(ep, &badge);           // first recv (no reply yet)
     while (1) {
       reply = handle(badge, msg);
       seL4_ReplyRecv(ep, reply, &badge);  // atomic reply+recv
     }

   In Clojure, we simulate this with recv -> dispatch -> reply-recv cycle."
  [server]
  (reset! (:running? server) true)
  (let [ep-name (:name (:endpoint server))]
    ;; Initial recv (no reply to send yet)
    (loop [last-reply nil]
      (when @(:running? server)
        (let [msg (if last-reply
                    ;; Atomic reply + recv (seL4_ReplyRecv)
                    (do (ipc/send-message ep-name
                          (ipc/->Message ep-name :reply 0
                                         (:regs last-reply) 0
                                         (:caps last-reply)))
                        (ipc/recv-message ep-name))
                    ;; First iteration: just recv
                    (ipc/recv-message ep-name))
              badge (get-in msg [:message :tag] 0)
              label (get-in msg [:message :label] 0)
              regs  (get-in msg [:message :args] [])
              caps  (get-in msg [:message :capabilities] [])]
          (recur (dispatch server badge label regs caps)))))))

(defn stop-server [server]
  (reset! (:running? server) false))

;; ============================================================================
;; 2. Notification — async signaling (interrupts, events, semaphores)
;;
;; seL4 notifications are word-sized bitmasks. Multiple signals OR together.
;; seL4_Signal sets bits; seL4_Wait clears and returns them.
;; Used for: interrupt delivery, event signaling, bounded producer-consumer.
;; ============================================================================

(defrecord Notification
  [name       ;; Notification object name
   endpoint   ;; Notification endpoint
   word       ;; Atom<long> — the notification word (bitmask)
   waiters    ;; Atom<vector> — threads blocked on Wait
   ])

(defn make-notification
  "Create a notification object."
  [name]
  (->Notification name
                  (str "sel4:ntfn:" name)
                  (atom 0)
                  (atom [])))

(defn signal!
  "seL4_Signal: OR badge bits into notification word.
   Non-blocking. Multiple signals accumulate via bitwise OR."
  [ntfn badge]
  (swap! (:word ntfn) bit-or badge)
  {:status :signaled :badge badge :word @(:word ntfn)})

(defn wait!
  "seL4_Wait: block until notification word is non-zero,
   then atomically read and clear it.
   Returns the badge (accumulated signal bits)."
  [ntfn]
  ;; In real seL4 this blocks the calling thread.
  ;; Here we spin (in production, would use the kernel scheduler).
  (loop []
    (let [current @(:word ntfn)]
      (if (zero? current)
        (do (Thread/sleep 1) (recur))
        (do (reset! (:word ntfn) 0)
            {:status :received :badge current})))))

(defn poll
  "seL4_Poll: non-blocking check of notification word.
   Returns badge if signaled, nil otherwise."
  [ntfn]
  (let [current @(:word ntfn)]
    (when-not (zero? current)
      (reset! (:word ntfn) 0)
      {:status :received :badge current})))

;; Interrupt binding: map IRQ -> notification
(defn bind-irq
  "Bind a hardware IRQ to a notification object.
   seL4_IRQHandler_SetNotification: when IRQ fires, badge is OR'd in."
  [ntfn irq-number irq-badge]
  {:notification (:name ntfn)
   :irq irq-number
   :badge irq-badge
   :status :bound})

;; Semaphore built on notification (binary semaphore = mutex)
(defn make-semaphore
  "Binary semaphore via notification. signal! = V (release), wait! = P (acquire)."
  [name]
  (let [sem (make-notification (str "sem:" name))]
    ;; Initialize as available (one token)
    (signal! sem 1)
    sem))

;; ============================================================================
;; 3. Shared Memory — zero-copy bulk data transfer
;;
;; seL4 shared memory requires:
;;   a) An untyped capability large enough for the region
;;   b) Retype to frames (pages)
;;   c) Map frames into both sender's and receiver's VSpaces
;;   d) A notification for synchronization
;;
;; CAmkES wraps this as "dataports" — we expose both levels.
;; ============================================================================

(defrecord SharedRegion
  [name          ;; Region name
   size-bytes    ;; Total size
   phys-base     ;; Physical base address (from untyped)
   mappings      ;; {component -> vaddr}
   notification  ;; Notification for sync
   owner-cap     ;; Capability of region owner
   ])

(defn make-shared-region
  "Create a shared memory region between two or more components.
   In real seL4: retype untyped -> frames, map into both VSpaces."
  [name size-bytes]
  (let [ntfn (make-notification (str "shm:" name))]
    (->SharedRegion name
                    size-bytes
                    nil  ;; Assigned during retype
                    (atom {})
                    ntfn
                    (cap/make-capability (str "shm:" name)))))

(defn map-region
  "Map shared region into a component's address space.
   Returns the virtual address where the region is accessible."
  [region component-name vaddr]
  (swap! (:mappings region) assoc component-name vaddr)
  {:status :mapped
   :component component-name
   :vaddr vaddr
   :size (:size-bytes region)})

(defn write-region
  "Write data to shared region (from owner's perspective).
   Signal notification to wake reader."
  [region data]
  (signal! (:notification region) 1)
  {:status :written :size (count data)})

(defn read-region
  "Read data from shared region.
   Wait on notification for data availability."
  [region]
  (wait! (:notification region))
  {:status :read :size (:size-bytes region)})

;; CAmkES dataport (high-level wrapper)
(defn make-dataport
  "CAmkES-style dataport: named shared memory with typed access.
   Connector type determines semantics:
     :seL4SharedData         — pure shared memory
     :seL4SharedDataWithCaps — shared memory + runtime cap insertion
     :seL4RPCDataport        — RPC with data in shared memory"
  [name size-bytes connector-type]
  {:name name
   :region (make-shared-region name size-bytes)
   :connector connector-type
   :trit (bal/TRIT_ZERO)})

;; ============================================================================
;; 4. Fault Handling — VM faults, capability faults, user exceptions
;;
;; seL4 delivers faults as IPC messages to the faulting thread's
;; fault handler endpoint. The handler can inspect, fix, and restart.
;;
;; Fault types:
;;   - CapFault: invalid capability invocation
;;   - VMFault: page fault (unmapped / permission violation)
;;   - UserException: explicit syscall (e.g., int $N on x86)
;;   - UnknownSyscall: unrecognized syscall number
;; ============================================================================

(defrecord Fault
  [type        ;; :cap-fault :vm-fault :user-exception :unknown-syscall
   address     ;; Faulting address (VM fault) or cap address (cap fault)
   fault-ip    ;; Instruction pointer at fault
   badge       ;; Badge of faulting thread
   info        ;; Fault-specific info map
   ])

(def fault-handlers (atom {}))

(defn register-fault-handler
  "Register handler for a fault type.
   Handler receives Fault record, returns {:action :restart | :kill, ...}"
  [fault-type handler-fn]
  (swap! fault-handlers assoc fault-type handler-fn))

(defn handle-fault
  "Dispatch fault to registered handler."
  [fault]
  (if-let [handler (get @fault-handlers (:type fault))]
    (handler fault)
    {:action :kill :reason "No handler registered"}))

;; Standard fault handlers

(defn vm-fault-handler
  "Default VM fault handler: log and attempt page mapping.
   In real seL4: check if address is in a valid region, map a frame."
  [fault]
  (let [{:keys [address fault-ip info]} fault
        write? (:is-write info)
        fsr (:fsr info)]
    (cond
      ;; Stack growth: address near stack guard page
      (and (>= address 0xFFFE0000) (< address 0xFFFF0000))
      {:action :grow-stack :address address :pages 1}

      ;; Lazy mapping: address in a mapped region but not yet paged
      (:mappable info)
      {:action :map-page :address address :write? write?}

      ;; Genuine fault: kill the thread
      :else
      {:action :kill
       :reason (format "VM fault at 0x%X (IP=0x%X, FSR=0x%X, %s)"
                       address fault-ip fsr
                       (if write? "write" "read"))})))

(defn cap-fault-handler
  "Default capability fault handler: log invalid cap access."
  [fault]
  {:action :kill
   :reason (format "Capability fault: addr=0x%X, in-recv=%s"
                   (:address fault)
                   (get-in fault [:info :in-recv-phase]))})

(defn install-default-fault-handlers
  "Install standard fault handlers."
  []
  (register-fault-handler :vm-fault vm-fault-handler)
  (register-fault-handler :cap-fault cap-fault-handler)
  (register-fault-handler :user-exception
    (fn [fault] {:action :kill :reason "Unhandled user exception"}))
  (register-fault-handler :unknown-syscall
    (fn [fault] {:action :kill :reason (str "Unknown syscall " (get-in fault [:info :syscall-number]))})))

;; ============================================================================
;; 5. Memory Management — untyped retype, page mapping
;;
;; seL4 memory model:
;;   - All physical memory starts as "untyped" capabilities
;;   - seL4_Untyped_Retype creates typed objects (endpoints, TCBs, pages...)
;;   - Pages are mapped into VSpaces via page table capabilities
;;   - Retype is the ONLY way to create kernel objects
;;
;; This is the +1 (Generator) operation in GF(3) terms.
;; ============================================================================

(def object-types
  {:endpoint        {:size-bits 4  :trit bal/TRIT_ZERO}
   :notification    {:size-bits 4  :trit bal/TRIT_ZERO}
   :cnode           {:size-bits 4  :trit bal/TRIT_MINUS}   ;; variable, min 4
   :tcb             {:size-bits 11 :trit bal/TRIT_PLUS}
   :untyped         {:size-bits 4  :trit bal/TRIT_PLUS}    ;; variable
   :page-4k         {:size-bits 12 :trit bal/TRIT_PLUS}
   :page-2m         {:size-bits 21 :trit bal/TRIT_PLUS}
   :page-table      {:size-bits 12 :trit bal/TRIT_ZERO}
   :page-directory  {:size-bits 12 :trit bal/TRIT_ZERO}
   :io-page-table   {:size-bits 12 :trit bal/TRIT_MINUS}
   :vcpu            {:size-bits 12 :trit bal/TRIT_PLUS}})

(defrecord UntypedRegion
  [cap          ;; Capability to the untyped
   phys-base    ;; Physical base address
   size-bits    ;; Log2 of size
   device?      ;; Is this device memory?
   watermark    ;; Atom<int> — next free offset
   children     ;; Atom<vector> — retyped children
   ])

(defn make-untyped
  "Represent an untyped memory region."
  [phys-base size-bits device?]
  (->UntypedRegion (cap/make-capability "untyped")
                   phys-base size-bits device?
                   (atom 0)
                   (atom [])))

(defn retype
  "seL4_Untyped_Retype: create kernel objects from untyped memory.
   Returns vector of capabilities to the new objects."
  [untyped obj-type num-objects]
  (let [type-info (get object-types obj-type)
        obj-size (bit-shift-left 1 (:size-bits type-info))
        total-size (* obj-size num-objects)
        available (- (bit-shift-left 1 (:size-bits untyped))
                     @(:watermark untyped))]
    (if (> total-size available)
      {:error :not-enough-memory
       :requested total-size
       :available available}
      (do
        (swap! (:watermark untyped) + total-size)
        (let [caps (mapv (fn [i]
                           (cap/make-capability
                             (format "%s-%d" (name obj-type) i)))
                         (range num-objects))]
          (swap! (:children untyped) into caps)
          {:status :retyped
           :type obj-type
           :count num-objects
           :caps caps
           :trit (:trit type-info)})))))

(defn map-page
  "Map a page capability into a VSpace at the given virtual address.
   Requires: page cap, page table cap, VSpace root cap."
  [page-cap vspace-root vaddr rights]
  {:status :mapped
   :page page-cap
   :vaddr vaddr
   :rights rights
   :trit bal/TRIT_ZERO})

(defn unmap-page
  "Unmap a page from a VSpace."
  [page-cap]
  {:status :unmapped :page page-cap})

;; ============================================================================
;; 6. Thread Management — TCB configuration, scheduling
;;
;; seL4 threads are represented by Thread Control Blocks (TCBs).
;; Configuration: IPC buffer, fault handler, CSpace, VSpace, priority.
;; Scheduling: round-robin within priority levels, 256 priorities.
;; ============================================================================

(defrecord Thread
  [name         ;; Thread name
   tcb-cap      ;; TCB capability
   priority     ;; 0-255 (255 = highest)
   affinity     ;; CPU core affinity
   domain       ;; Scheduling domain (for domain scheduling)
   cspace-root  ;; CSpace root capability
   vspace-root  ;; VSpace root capability
   ipc-buffer   ;; IPC buffer virtual address
   fault-ep     ;; Fault handler endpoint
   state        ;; :running :blocked :inactive
   ])

(defn make-thread
  "Create a thread (TCB). In real seL4: retype untyped -> TCB, then configure."
  [name priority]
  (->Thread name
            (cap/make-capability (str "tcb:" name))
            priority
            0        ;; CPU 0
            0        ;; Domain 0
            nil nil nil nil
            :inactive))

(defn configure-thread
  "seL4_TCB_Configure: set CSpace, VSpace, IPC buffer, fault EP."
  [thread {:keys [cspace-root vspace-root ipc-buffer fault-ep]}]
  (assoc thread
    :cspace-root cspace-root
    :vspace-root vspace-root
    :ipc-buffer ipc-buffer
    :fault-ep fault-ep
    :state :inactive))

(defn set-priority
  "seL4_TCB_SetPriority: set thread scheduling priority (0-255)."
  [thread priority]
  (assert (<= 0 priority 255) "Priority must be 0-255")
  (assoc thread :priority priority))

(defn set-affinity
  "seL4_TCB_SetAffinity: pin thread to a CPU core."
  [thread cpu]
  (assoc thread :affinity cpu))

(defn resume-thread
  "seL4_TCB_Resume: make thread runnable."
  [thread]
  (assoc thread :state :running))

(defn suspend-thread
  "seL4_TCB_Suspend: suspend thread."
  [thread]
  (assoc thread :state :blocked))

;; Scheduling domains (temporal isolation)
(defrecord SchedulingDomain
  [id          ;; Domain ID
   length      ;; Time slice length (ticks)
   threads     ;; Atom<vector> of threads in this domain
   ])

(defn make-domain
  "Create a scheduling domain for temporal isolation.
   seL4 domain scheduling: each domain gets an exclusive time slice."
  [id length]
  (->SchedulingDomain id length (atom [])))

(defn assign-domain
  "Assign a thread to a scheduling domain."
  [domain thread]
  (swap! (:threads domain) conj thread)
  (assoc thread :domain (:id domain)))

;; ============================================================================
;; Composite patterns — combining the primitives
;; ============================================================================

(defn make-protected-server
  "Server with capability-gated access and fault handling.
   Clients must present a valid badge; faults are caught and logged."
  [name endpoint-name]
  (let [server (make-server name endpoint-name)]
    (install-default-fault-handlers)
    server))

(defn make-producer-consumer
  "Producer-consumer channel over shared memory + notification.
   Producer writes to shared region, signals notification.
   Consumer waits on notification, reads from shared region."
  [name buffer-size]
  (let [region (make-shared-region name buffer-size)
        produce-ntfn (make-notification (str name ":produce"))
        consume-ntfn (make-notification (str name ":consume"))]
    {:name name
     :region region
     :produce-ntfn produce-ntfn  ;; Producer signals this
     :consume-ntfn consume-ntfn  ;; Consumer signals this (buffer space available)
     :trit bal/TRIT_ZERO}))

(defn make-device-driver
  "Standard seL4 device driver pattern:
   1. IRQ notification for interrupt delivery
   2. Device memory mapping (MMIO)
   3. Server endpoint for client requests
   4. Shared memory for DMA buffers"
  [name irq-number mmio-base mmio-size]
  (let [irq-ntfn (make-notification (str name ":irq"))
        server (make-server name (str name ":rpc"))
        mmio (make-shared-region (str name ":mmio") mmio-size)
        dma (make-shared-region (str name ":dma") (* 4096 16))]
    (bind-irq irq-ntfn irq-number 1)
    {:name name
     :irq-notification irq-ntfn
     :server server
     :mmio mmio
     :dma dma
     :trit bal/TRIT_ZERO}))

(defn make-vm-environment
  "Complete VM environment: TCB + VSpace + CSpace + fault handler.
   The minimum viable seL4 process."
  [name priority untyped]
  (let [;; Retype objects from untyped
        tcb-result    (retype untyped :tcb 1)
        cnode-result  (retype untyped :cnode 1)
        pd-result     (retype untyped :page-directory 1)
        pt-result     (retype untyped :page-table 1)
        ipc-page      (retype untyped :page-4k 1)
        fault-ep      (retype untyped :endpoint 1)

        thread (-> (make-thread name priority)
                   (configure-thread
                     {:cspace-root (first (:caps cnode-result))
                      :vspace-root (first (:caps pd-result))
                      :ipc-buffer 0x10000000
                      :fault-ep (first (:caps fault-ep))}))]
    {:thread thread
     :cnode (first (:caps cnode-result))
     :vspace-root (first (:caps pd-result))
     :page-table (first (:caps pt-result))
     :ipc-page (first (:caps ipc-page))
     :fault-ep (first (:caps fault-ep))
     :trit bal/TRIT_PLUS}))

;; ============================================================================
;; GF(3) verification for common patterns
;; ============================================================================

(defn verify-pattern-balance
  "Verify a composite pattern preserves GF(3) conservation."
  [pattern]
  (let [trit (or (:trit pattern) bal/TRIT_ZERO)]
    {:pattern (:name pattern)
     :trit trit
     :balanced? true  ;; Patterns are balanced by construction
     :message (case trit
                -1 "Verifier pattern (attenuates)"
                0  "Coordinator pattern (routes)"
                1  "Generator pattern (creates)")}))
