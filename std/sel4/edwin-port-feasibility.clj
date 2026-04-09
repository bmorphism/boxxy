;; Edwin on seL4 — Port Feasibility Assessment
;;
;; Date: 2026-03-24
;; Question: Can MIT/GNU Scheme 12.1 + Edwin run on seL4?
;;
;; TL;DR: YES, via three paths (ranked by effort):
;;   Path A: Edwin in Linux guest VM (CAmkES VMM) — works NOW
;;   Path B: MIT Scheme SVM1 bytecode on seL4 Microkit — MEDIUM effort
;;   Path C: MIT Scheme native (aarch64) on seL4 — HARD (needs W^X workaround)

(ns sel4.edwin-port-feasibility
  "Feasibility analysis for porting Edwin/MIT-Scheme to seL4.")

;; ============================================================================
;; MIT/GNU Scheme 12.1 — What It Needs
;; ============================================================================

(def mit-scheme-requirements
  {:build
   {:compiler     "gcc or clang (C99)"
    :make         "GNU make"
    :m4           "m4 macro processor"
    :curses       "ncurses (for terminal Edwin; X11 optional)"
    :libc         "glibc or musl (musl compat confirmed)"
    :architectures ["i386" "x86-64" "aarch64" "svm1"]}

   :runtime
   {:native-code
    {:description "JIT-compiled Scheme. Fastest. Requires W+X memory."
     :issue       "seL4 enforces W^X (write XOR execute) on verified platforms.
                   Native code backend needs writable+executable pages for JIT.
                   This BREAKS on verified seL4 (aarch32 enforces W^X strictly)."
     :workaround  "Use SVM1 instead, or run on x86-64 where W^X is not yet
                   enforced in the seL4 proof."}

    :svm1
    {:description "Software Virtual Machine 1. Portable bytecode interpreter.
                   No JIT. No W+X requirement. Works everywhere."
     :performance "Slower than native (estimated 3-10x), but sufficient for
                   an editor. Edwin is not compute-bound."
     :advantage   "Pure interpreter. No executable code generation at runtime.
                   Compatible with seL4's W^X enforcement.
                   This is the path for verified seL4 platforms."}}

   :libc-surface
   {:description "MIT Scheme's C runtime uses standard POSIX libc."
    :critical    ["malloc/free"        ;; Memory allocation
                  "mmap/munmap"        ;; Memory mapping (for GC)
                  "read/write/open"    ;; File I/O
                  "select/poll"        ;; I/O multiplexing (Edwin needs this)
                  "signal"             ;; Signal handling (GC uses SIGALRM)
                  "setjmp/longjmp"     ;; Non-local exits
                  "dlopen/dlsym"       ;; Dynamic loading (plugins)
                  "ncurses"            ;; Terminal UI (Edwin without X11)
                  "getenv"             ;; Environment variables
                  "time/gettimeofday"] ;; Timestamps
    :optional    ["X11"                ;; Graphical Edwin (not needed on seL4)
                  "pthreads"           ;; Threads (MIT Scheme is mostly single-threaded)
                  "networking"]        ;; Socket API (nice-to-have, not required)
    :note        "musl libc compatibility means the POSIX surface is well-defined.
                  LionsOS and Magnetite both provide enough POSIX for this."}

   :edwin-specific
   {:description "Edwin's additional requirements beyond base MIT Scheme."
    :needs       ["Terminal I/O (ncurses or raw termios)"
                  "Keyboard input (blocking read with timeout)"
                  "Screen dimensions (TIOCGWINSZ ioctl)"
                  "File system access (open/read/write/stat)"
                  "Process spawning (optional — for M-x shell)"]
    :does-not-need ["X11 (terminal mode is fine)"
                    "GUI toolkit"
                    "Sound"
                    "Networking (unless you want comint modes)"]}})

;; ============================================================================
;; Path A: Edwin in Linux Guest VM (CAmkES VMM)
;; ============================================================================

(def path-a
  {:name        "Edwin in Linux Guest VM"
   :effort      :low
   :performance :native
   :verification :partial  ;; Kernel verified, guest is not
   :description
   "Run a minimal Linux (Buildroot) as CAmkES VM guest.
    Install MIT Scheme + Edwin in the guest.
    Bridge to seL4 native components via cross-VM connectors."

   :steps
   ["1. Buildroot image with MIT Scheme 12.1 (apt-get install mit-scheme)"
    "2. CAmkES VMM configuration (sel4-provider.edn already has this)"
    "3. Cross-VM shared memory dataport for buffer exchange"
    "4. seL4 notification for IPC events -> Edwin"
    "5. sel4-mode.el inside Edwin connects via JSON-RPC :9998"]

   :advantages
   ["Works TODAY with existing CAmkES VMM"
    "Full MIT Scheme with native code (fast)"
    "Full ncurses terminal (Linux provides termios)"
    "File system, networking, process spawning all work"
    "Edwin packages and Scheme libraries available"]

   :disadvantages
   ["Linux kernel is in the TCB (not verified)"
    "~128MB RAM for Linux guest"
    "Boot time ~2-5 seconds for guest"
    "Cross-VM latency for IPC (~microseconds, acceptable)"]

   :gf3-trit 0})  ;; Coordinator — bridges verified and unverified worlds

;; ============================================================================
;; Path B: MIT Scheme SVM1 on seL4 Microkit
;; ============================================================================

(def path-b
  {:name        "SVM1 Bytecode on seL4 Microkit"
   :effort      :medium
   :performance :acceptable  ;; 3-10x slower than native, fine for editor
   :verification :high  ;; No JIT, no W+X, compatible with verified seL4
   :description
   "Compile MIT Scheme's SVM1 bytecode interpreter as a seL4 Microkit
    component. Edwin runs on the bytecode VM. No Linux guest needed."

   :steps
   ["1. Cross-compile MIT Scheme SVM1 for aarch64-none-elf (bare metal)"
    "2. Provide minimal libc shim (musl-based, ~50 functions)"
    "3. Implement termios over seL4 IPC (Edwin needs terminal I/O)"
    "4. Implement file I/O over seL4 IPC to file server component"
    "5. Package as Microkit protection domain"
    "6. Connect to window manager via seL4 IPC (sel4.window)"
    "7. Connect to file server via seL4 IPC (capability-gated)"]

   :libc-shim
   {:description "Minimal POSIX shim routing to seL4 IPC."
    :functions
    {"malloc/free"     "seL4 untyped retype for pages, simple bump allocator"
     "read/write"      "seL4 IPC to file server or terminal server"
     "open/close"      "Capability request to file server"
     "select"          "seL4 notification poll (multiple endpoints)"
     "mmap"            "seL4 page mapping (no W+X pages!)"
     "signal"          "seL4 notification (SIGALRM -> periodic notification)"
     "setjmp/longjmp"  "Standard C implementation (no kernel involvement)"
     "ncurses"         "Minimal termios shim over seL4 IPC to terminal server"
     "getenv"          "Static config (no environment in bare-metal seL4)"
     "time"            "seL4 timer driver via sDDF"}
    :estimated-loc "~2000 lines of C for the shim"
    :reference     "Magnetite's POSIX shim is a good starting point (17 services)"}

   :advantages
   ["No Linux guest — pure seL4 protection domain"
    "W^X compatible (SVM1 is pure interpreter)"
    "Small TCB: SVM1 interpreter + libc shim + Edwin"
    "Capability-gated file access (not POSIX ambient authority)"
    "Each Edwin buffer = separate shared memory region"
    "~8-16MB RAM (vs 128MB for Linux guest)"]

   :disadvantages
   ["3-10x slower than native code (fine for editor, bad for compute)"
    "Need to write POSIX shim (~2000 LOC)"
    "Some Scheme libraries may not work (no dlopen)"
    "No process spawning (M-x shell won't work)"]

   :gf3-trit -1})  ;; Verifier — runs on verified platform, no W+X

;; ============================================================================
;; Path C: MIT Scheme Native on seL4 (aarch64/x86-64)
;; ============================================================================

(def path-c
  {:name        "Native Code on seL4"
   :effort      :hard
   :performance :fast
   :verification :compromised  ;; W+X breaks verified seL4 on ARM
   :description
   "Run MIT Scheme's native code compiler on seL4.
    Requires writable+executable memory pages for JIT."

   :problem
   "seL4's aarch32 proof enforces W^X. The native code backend
    allocates pages that are both writable and executable.
    On x86-64, the proof doesn't yet cover W^X, so this works
    but without full verification guarantees."

   :steps
   ["1. Same as Path B steps 1-7"
    "2. Additionally: configure seL4 to allow W+X pages (x86-64 only)"
    "3. Or: implement a two-phase JIT (write phase -> make executable)"]

   :advantages
   ["Full native speed"
    "All Scheme features work (including native-compiled libraries)"]

   :disadvantages
   ["Breaks W^X on verified ARM platforms"
    "x86-64 only (for now)"
    "Two-phase JIT is complex and may break MIT Scheme assumptions"
    "Larger attack surface (executable heap)"]

   :gf3-trit 1})  ;; Generator — creates executable code at runtime

;; ============================================================================
;; Recommendation
;; ============================================================================

(def recommendation
  {:short-term  {:path :A
                 :reason "Ship today. Edwin in CAmkES VM. sel4-mode.el bridges it.
                          Zero new code needed beyond what we already have."}

   :medium-term {:path :B
                 :reason "SVM1 on Microkit. ~2000 LOC POSIX shim.
                          The 'right' architecture: pure seL4, no Linux, W^X clean.
                          Edwin is slow but editors don't need speed.
                          Magnetite's POSIX shim is a reference implementation.
                          LionsOS sDDF provides timer + serial drivers.
                          Target: 3-6 months with 1 developer."}

   :long-term   {:path :iris-verification
                 :reason "Once Path B works, verify the POSIX shim with Iris.
                          This would make Edwin the first formally-verified editor
                          on a formally-verified kernel. SICP on verified iron."}

   :skip        {:path :C
                 :reason "Native code breaks the verification story.
                          SVM1 is good enough for an editor.
                          Only pursue if compute-heavy Scheme needed."}})

;; ============================================================================
;; Key references
;; ============================================================================

(def references
  [{:name "MIT/GNU Scheme 12.1"
    :url  "https://www.gnu.org/software/mit-scheme/"
    :note "SVM1 bytecode + portable C source available"}
   {:name "Edwin documentation"
    :url  "https://www.gnu.org/software/mit-scheme/documentation/stable/mit-scheme-user/Edwin.html"
    :note "Emacs v18 clone, needs ncurses, no X11 required"}
   {:name "seL4 Microkit"
    :url  "https://github.com/seL4/microkit"
    :note "Static protection domains, replaces CAmkES"}
   {:name "LionsOS"
    :url  "https://lionsos.org/"
    :note "sDDF drivers (serial, timer, network, block)"}
   {:name "Magnetite"
    :url  "https://www.ndss-symposium.org/wp-content/uploads/spacesec26-3.pdf"
    :note "17 POSIX services on seL4, reference for libc shim"}
   {:name "Sculpt OS"
    :url  "https://genode.org/"
    :note "100+ components on seL4, alternative to Microkit"}
   {:name "seL4 Summit 2025 — Splitting the Spec"
    :url  "https://sel4.systems/Summit/2025/abstracts2025.html"
    :note "Concurrent semantics for kernel-userland verification"}
   {:name "seL4 Summit 2025 — Program Logic (Iris)"
    :url  "https://sel4.systems/Summit/2025/abstracts2025.html"
    :note "Brecknell: Iris for user-space seL4 component verification"}])
