;; seL4 Ecosystem — All Candidates for boxxy Integration
;;
;; Comprehensive map of seL4-adjacent systems, editors, OS personalities,
;; databases, and frameworks. Each entry is a potential integration target
;; for the boxxy/std/sel4/ stack.
;;
;; Sources: seL4 Summit 2025 (Prague), seL4 Foundation, web research 2026-03-24
;; GF(3) trit assignments follow provider convention:
;;   +1 = Generator (creates resources)
;;    0 = Coordinator (routes/composes)
;;   -1 = Verifier (attenuates/validates)

(ns sel4.ecosystem
  "seL4 ecosystem registry: OS personalities, editors, databases, frameworks."
  (:require [sel4.balance :as bal]))

;; ============================================================================
;; 1. OS Personalities — full operating systems on seL4
;; ============================================================================

(def os-personalities
  [{:name        "LionsOS"
    :role        :os-personality
    :trit        0
    :description "KISS-principle OS from Trustworthy Systems (UNSW). Static architecture."
    :components  {:microkit "Lean framework for statically-architected seL4 systems"
                  :sddf     "seL4 Device Driver Framework — ring buffers, zero-copy shared memory"
                  :libvmm   "Virtual machine monitor library"}
    :device-classes [:network :block :serial :i2c :timer]
    :architecture :aarch64
    :summit-talk "Splitting the seL4 Spec: Kernel-Userland Integration (Sewell, Sison)"
    :relevance   "sDDF ring buffer pattern maps to common.clj shared-memory + notification.
                  Microkit's static architecture = CAmkES successor. Verification bridge
                  uses concurrent separation logic (Iris) — same formalism target as
                  seL4_Bridge.thy."}

   {:name        "Sculpt OS (Genode)"
    :role        :os-personality
    :trit        0
    :description "Dynamic general-purpose OS. 100+ components, sandboxed drivers."
    :kernels     [:nova :sel4 :fiasco-oc :hw]
    :components  {:depot      "Package management for live component deployment"
                  :drivers    "Sandboxed drivers with capability delegation"
                  :vms        "VirtualBox, Linux guests"
                  :networking "TCP/IP, Wi-Fi, USB"}
    :release     "Sculpt 25.10 with experimental seL4 variant"
    :summit-talk "Sculpt OS (Boettcher, Sumpf)"
    :relevance   "Live component deployment = hot-reload pattern for boxxy providers.
                  Multi-kernel support means boxxy could target Genode generically
                  then specialize to seL4 for verified deployments."}

   {:name        "CellulOS"
    :role        :os-personality
    :trit        -1
    :description "Research OS for comparing isolation mechanisms. Osmosis resource model."
    :concept     "Protection domain state — formal reasoning about capability sharing"
    :summit-talk "CellulOS (Agrawal et al.)"
    :relevance   "Osmosis model could formalize boxxy provider trit boundaries.
                  'Protection domain state' is the runtime counterpart of
                  seL4_Bridge.thy's static capability partitioning."}

   {:name        "Magnetite"
    :role        :os-personality
    :trit        0
    :description "Rust-based OS services for seL4. NASA cFS (core Flight System) port."
    :services    17
    :features    [:shared-memory :networking :filesystem :device-drivers]
    :architectures [:aarch64 :x86-64 :armv7a]
    :summit-talk "Porting NASA's cFS to Magnetite (Furgala, Jero)"
    :relevance   "Proves seL4 is viable for satellite flight software.
                  cFS OSAL abstraction = model for boxxy's sel4-provider.edn API.
                  Magnetite's fault handling maps to common.clj fault patterns."}

   {:name        "Kry10 OS"
    :role        :commercial-os
    :trit        0
    :description "IoT OS on seL4. Elixir/BEAM stack. Self-healing, dynamic upgrades."
    :stack       [:sel4 :elixir :beam :nerves]
    :funding     "$6M (Folklore, Rotomā, intelligence-backed fund)"
    :summit-2026 "Martin Dehnel-Wild keynote (Chief Scientist)"
    :relevance   "BEAM on seL4 = capability-secured actor model.
                  Elixir's supervision trees map to our fault handling patterns.
                  Self-healing = automatic server-loop restart (common.clj)."}

   {:name        "Atoll (Neutrality)"
    :role        :hypervisor
    :trit        -1
    :description "Formally-verified cloud hosting platform. European-sovereign."
    :features    [:workload-isolation :mathematical-proof :commodity-hardware]
    :summit-talk "seL4 on Big Iron (David Cock)"
    :relevance   "Extends seL4 formal proof to multi-tenant cloud.
                  Isolation proof = GF(3) verifier at infrastructure scale.
                  DuckDB-on-Atoll = capability-gated analytics per tenant."}])

;; ============================================================================
;; 2. Editors — candidates for seL4-native editing
;; ============================================================================

(def editors
  [{:name        "Edwin (MIT/GNU Scheme)"
    :role        :editor
    :trit        0
    :description "Emacs v18 clone written entirely in MIT Scheme. Built into MIT/GNU Scheme."
    :language    :mit-scheme
    :features    [:scheme-mode :repl :buffer-management :keybindings]
    :architecture "Single-process, cooperative. Extension language = MIT Scheme."
    :relevance   "Edwin on seL4 via Microkit:
                  - MIT Scheme runtime as single protection domain
                  - Edwin buffers backed by seL4 shared memory (dataport)
                  - Scheme eval = capability-gated (each buffer gets a capability)
                  - IPC to window manager (our sel4.window namespace)
                  - SICP pedagogy: seL4 capabilities taught through Scheme
                  - Edwin's simplicity (Emacs 18 clone) fits seL4's minimal TCB
                  Edwin is the natural seL4 editor: Scheme for extension,
                  small enough to audit, no FFI surface."}

   {:name        "Gypsum (Guile Scheme)"
    :role        :editor
    :trit        1
    :description "Project by Ramin Honary. Emacs rewrite in Guile Scheme with Elisp interp."
    :language    :guile
    :features    [:elisp-parser :elisp-interpreter :gtk-gui :scheme-repl]
    :status      "Early stage — can parse Elisp, cannot fully evaluate"
    :relevance   "Guile = our goblins-adapter runtime. If Gypsum matures:
                  - Goblins CapTP capabilities native in the editor
                  - Elisp compatibility = existing Emacs packages
                  - GTK GUI could be replaced with seL4 framebuffer (sel4.window)
                  - OCapN protocol for editor-to-editor collaboration
                  Risk: 1400 Elisp builtins is a massive surface."}

   {:name        "GNU Emacs (via Emacs-in-seL4-VM)"
    :role        :editor
    :trit        0
    :description "Run full GNU Emacs as Linux guest in CAmkES VM."
    :approach    "CAmkES VMM with Linux guest, Emacs inside, IPC bridge out"
    :relevance   "Pragmatic path: don't port Emacs, just run it in a verified VM.
                  Cross-VM connector (seL4SharedDataWithCaps) for:
                  - Shared buffers between Emacs and native seL4 components
                  - Capability-gated file access via seL4 IPC
                  - sel4-mode.el already implements the JSON-RPC bridge
                  This is what our existing stack supports TODAY."}

   {:name        "Ewig (zig-syrup)"
    :role        :editor
    :trit        0
    :description "Append-only Merkle DAG editor from zig-syrup. Time-travel, CRDT sync."
    :language    :zig
    :features    [:merkle-dag :time-travel :branching :crdt]
    :relevance   "Already in our stack. Ewig on seL4:
                  - Zig compiles to seL4 userspace (no libc dependency)
                  - Merkle DAG = immutable audit log over seL4 IPC
                  - CRDT sync across protection domains via notifications
                  - Pairs with agm-worlds.el for belief revision display"}])

;; ============================================================================
;; 3. Databases — embedded analytics on verified kernel
;; ============================================================================

(def databases
  [{:name        "DuckDB (in-process)"
    :role        :database
    :trit        1
    :description "In-process OLAP. 'SQLite for analytics.' Zero-copy, columnar."
    :features    [:columnar :vectorized :csv-json-parquet :zero-copy :embedded]
    :relevance   "DuckDB on seL4 = capability-gated analytical queries.
                  Integration paths:
                  1. DuckDB in Linux guest VM, query via cross-VM dataport
                  2. DuckDB compiled to seL4 userspace (needs libc shim)
                  3. DuckDB WASM in Hoot/Goblins sandbox
                  Use cases:
                  - IPC audit log analytics (balance.clj -> DuckDB)
                  - Capability derivation tree queries (CDT as table)
                  - sDDF performance telemetry (ring buffer stats)
                  - Satellite telemetry on Magnetite (cFS -> DuckDB)
                  Our fswatch-duckdb pattern already exists in ~/i."}

   {:name        "DuckDB + seL4 IPC Monitor"
    :role        :analytics
    :trit        0
    :description "Pipe sel4.balance session data into DuckDB for trit analytics."
    :schema      {:ipc_messages [:timestamp :endpoint :operation :badge :trit]
                  :capabilities [:id :name :endpoint :rights :trit :revoked]
                  :faults       [:timestamp :type :address :fault_ip :badge :action]
                  :gf3_sessions [:session_id :message_count :trit_sum :balanced]}
    :relevance   "Turns sel4-mode.el IPC monitor into a queryable database.
                  SQL over capabilities: SELECT * FROM capabilities WHERE trit = -1
                  GF(3) violation detection: WHERE trit_sum % 3 != 0"}])

;; ============================================================================
;; 4. Verification Frameworks — formal methods ecosystem
;; ============================================================================

(def verification
  [{:name        "Isabelle/HOL + AutoCorres2"
    :role        :verification
    :trit        -1
    :description "seL4's native proof engine. AutoCorres2 lifts C to monadic HOL."
    :summit-talks ["Next 700 verified platforms (Klein)"
                   "Verified IPv6 Stack (Kägi)"
                   "Binary Verification Deep Dive (Spinale)"]
    :relevance   "Our seL4_Bridge.thy lives here. AutoCorres2 could verify
                  the C equivalent of common.clj patterns.
                  Binary verification = trust all the way to machine code."}

   {:name        "Iris (Concurrent Separation Logic)"
    :role        :verification
    :trit        -1
    :description "Higher-order concurrent separation logic framework in Coq."
    :summit-talk "Program logic for seL4-based system verification (Brecknell)"
    :relevance   "Iris verifies USER-SPACE components on seL4.
                  Could verify our server-loop, notification, shared-memory patterns.
                  Compositional: verify each boxxy provider independently."}

   {:name        "Verus (Rust verification)"
    :role        :verification
    :trit        -1
    :description "Automated verification for Rust. Used for seL4 Ethernet driver."
    :summit-talk "Rust-based drivers and verified Rust (VanVossen)"
    :relevance   "Verify Rust components that bridge to our Clojure layer.
                  Verus + sel4 = verified driver + verified kernel."}

   {:name        "Pancake + Viper"
    :role        :verification
    :trit        -1
    :description "Imperative language targeting CakeML. Verified device drivers."
    :summit-talk "Verifying Device Drivers with Pancake (Zhao)"
    :relevance   "Alternative to C for writing verifiable seL4 drivers.
                  Pancake -> CakeML -> seL4 = fully verified driver stack."}

   {:name        "HAMR (Model-based Development)"
    :role        :toolchain
    :trit        0
    :description "SysMLv2 models -> Rust code gen -> seL4 Microkit. Collins Aerospace."
    :summit-talk "HAMR for seL4 Microkit/Rust (Hatcliff et al.)"
    :relevance   "Model-based approach could generate boxxy provider specs
                  from SysMLv2. HAMR generates the boilerplate; we add GF(3)."}])

;; ============================================================================
;; 5. Hardware Targets — where seL4 runs
;; ============================================================================

(def hardware-targets
  [{:name "ARM Cortex-A (aarch32)"
    :trit -1
    :verification :complete
    :note "ONLY fully verified architecture (all 6 properties)"}

   {:name "ARM Cortex-A (aarch64)"
    :trit 0
    :verification :partial
    :note "Functional correctness + integrity. Missing: binary, confidentiality, timing"}

   {:name "x86-64 (PC99)"
    :trit 0
    :verification :minimal
    :note "Functional correctness only. CAmkES VMM target. Atoll hypervisor."}

   {:name "RISC-V (rv64)"
    :trit 1
    :verification :partial
    :note "Functional + binary + integrity + confidentiality. AI edge platform (Liang)."}

   {:name "FPGA (Zynq/ZynqMP)"
    :trit 1
    :verification :partial
    :note "Reconfigurable hardware acceleration. Microkit hypervisor (Zaeske)."}])

;; ============================================================================
;; 6. Integration Priority Matrix
;; ============================================================================

(def integration-priorities
  "Ordered by feasibility × impact for boxxy."
  [{:rank 1
    :target "GNU Emacs in CAmkES VM"
    :effort :low
    :impact :high
    :reason "Already works. sel4-mode.el bridges via JSON-RPC. Ship today."}

   {:rank 2
    :target "DuckDB IPC Monitor"
    :effort :medium
    :impact :high
    :reason "DuckDB in Linux guest, fswatch-duckdb pattern exists.
             SQL over IPC logs = instant observability."}

   {:rank 3
    :target "LionsOS / sDDF integration"
    :effort :medium
    :impact :high
    :reason "sDDF ring buffers = our shared-memory pattern.
             LionsOS is seL4's future (replaces CAmkES)."}

   {:rank 4
    :target "Edwin on seL4 Microkit"
    :effort :high
    :impact :very-high
    :reason "MIT Scheme runtime in single protection domain.
             Scheme-native capability model. SICP pedagogy.
             The 'right' editor for a formally verified kernel.
             Requires: MIT Scheme -> seL4 userspace port (no libc)."}

   {:rank 5
    :target "Ewig (Zig) on seL4"
    :effort :medium
    :impact :medium
    :reason "Zig has no libc dependency. Merkle DAG = immutable audit.
             Already in zig-syrup. Needs seL4 syscall shim."}

   {:rank 6
    :target "Iris proofs for common.clj patterns"
    :effort :very-high
    :impact :very-high
    :reason "Formally verify our server-loop, notification, shared-memory.
             Compositional: verify each boxxy provider independently.
             Connects Brecknell's program logic to our stack."}

   {:rank 7
    :target "Kry10/BEAM bridge"
    :effort :high
    :impact :high
    :reason "Elixir supervision trees = fault handling.
             BEAM actors = seL4 protection domains.
             OTP + OCapN = distributed capability system."}

   {:rank 8
    :target "Gypsum (Guile Emacs)"
    :effort :very-high
    :impact :medium
    :reason "Depends on Gypsum maturing. Elisp compat is massive surface.
             But: Guile = Goblins runtime, so CapTP for free."}

   {:rank 9
    :target "Magnetite/cFS bridge"
    :effort :high
    :impact :niche
    :reason "Satellite flight software. Cool but narrow use case.
             Proves seL4 viability for safety-critical."}

   {:rank 10
    :target "Atoll (Neutrality) cloud"
    :effort :external
    :impact :high
    :reason "European-sovereign verified cloud. DuckDB-on-Atoll =
             per-tenant capability-gated analytics. Wait for their API."}])

;; ============================================================================
;; Ecosystem summary
;; ============================================================================

(defn ecosystem-report []
  "Print ecosystem summary."
  (println "\n=== seL4 Ecosystem for boxxy ===\n")
  (println "OS Personalities:" (count os-personalities))
  (println "Editors:         " (count editors))
  (println "Databases:       " (count databases))
  (println "Verification:    " (count verification))
  (println "Hardware:        " (count hardware-targets))
  (println "\nIntegration priorities:")
  (doseq [p integration-priorities]
    (println (format "  #%d [%s effort, %s impact] %s"
                     (:rank p)
                     (name (:effort p))
                     (name (:impact p))
                     (:target p)))))
