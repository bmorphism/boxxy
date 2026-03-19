# Testing seL4 with boxxy: Practical Steps
## From kernel build to running in the VM

**Last Updated**: February 2, 2026

---

## Quick Start (5 minutes)

### If you have seL4 kernel binary only:

```bash
# 1. Copy kernel to expected location
cp /path/to/seL4/build/root-task.elf \
   /Users/bob/projects/xmonad-sel4/seL4/build/root-task.elf

# 2. Edit sel4-vm.joke at top:
(def boot-method :direct-kernel)

# 3. Run
cd /Users/bob/i/boxxy
boxxy examples/sel4-vm.joke
```

Expected output:
```
╔════════════════════════════════════════════╗
║  seL4 / Direct Kernel (like linux-vm.joke)║
╚════════════════════════════════════════════╝

Creating direct kernel boot loader...
Setting up generic ARM64 platform...
Configuring VM: 4 CPUs, 4 GB RAM
Configuring NAT networking...

Validating VM configuration...
✓ Configuration valid

Creating VM instance...
Starting seL4 VM...
✓ VM started

Monitoring VM (Ctrl+C to stop)...
[0] VM state: running
```

---

## Full Setup (30 minutes)

### Step 1: Verify seL4 Kernel Build

```bash
# Check if kernel build exists
ls -lah /Users/bob/projects/xmonad-sel4/seL4/build/

# Look for:
# - root-task.elf  (the seL4 kernel)
# - sel4-arm64.img (disk image, optional)

# If missing, build seL4:
cd /Users/bob/projects/xmonad-sel4/seL4
mkdir -p build
cd build

cmake -DPLATFORM=qemu-arm-virt \
      -DCMAKE_BUILD_TYPE=Release \
      ../

ninja

# This creates: root-task.elf
```

### Step 2: Configure sel4-vm.joke

Two scenarios:

**Scenario A: Using Direct Kernel Boot (RECOMMENDED)**

```clojure
;; At top of /Users/bob/i/boxxy/examples/sel4-vm.joke

(def boot-method :direct-kernel)  ;← Choose this
(def kernel-path "/Users/bob/projects/xmonad-sel4/seL4/build/root-task.elf")
(def disk-path "/Users/bob/projects/xmonad-sel4/seL4/build/sel4-arm64.img")
```

**Scenario B: Using EFI Boot (if you have full disk image)**

```clojure
;; At top of /Users/bob/i/boxxy/examples/sel4-vm.joke

(def boot-method :efi)
(def disk-path "/Users/bob/projects/xmonad-sel4/seL4/build/sel4-arm64.img")
(def nvram-path "/tmp/sel4-nvram.bin")
```

### Step 3: Run seL4 VM

```bash
cd /Users/bob/i/boxxy

# Method 1: Automatic boot and monitoring
boxxy examples/sel4-vm.joke

# Method 2: Interactive REPL
boxxy -e '(sel4-repl-demo)'

# Method 3: Full REPL
boxxy repl
# Then:
# boxxy=> (require '[sel4.ipc :as ipc])
# boxxy=> (require '[sel4.window :as w])
# boxxy=> (require '[sel4.capability :as cap])
# boxxy=> (boot-sel4-vm)
```

### Step 4: Verify seL4 is Running

Check VM state:

```clojure
boxxy=> (def vm (sel4-repl-demo))
boxxy=> (vz/vm-state vm)
"running"

boxxy=> (vz/stop-vm! vm)
nil
```

---

## Testing Connectivity

### Test 1: IPC Message Passing

```clojure
(ns test-sel4-ipc
  (:require [sel4.ipc :as ipc]
            [sel4.window :as w]))

;; Create a simple message
(def msg (ipc/->Message
  "sel4:xmonad-wm"           ;; endpoint
  :query-layout              ;; operation
  0                          ;; label
  []                         ;; args
  0                          ;; tag
  []))                       ;; capabilities

;; Try to send it
(ipc/send-message "sel4:xmonad-wm" msg)

;; Check statistics
(ipc/get-statistics)
;; => {:sent-count 1, :recv-count 0, :errors 0}
```

### Test 2: Window Manager Control

```clojure
(ns test-sel4-window
  (:require [sel4.window :as w]))

;; Query current layout
(w/get-layout)

;; Try tiling
(w/tile-2-column)

;; Check stats
(w/window-stats)
;; => {:moves 1, :focus-changes 0, :key-events 0, :mouse-events 0}
```

### Test 3: Capability Generation

```clojure
(ns test-sel4-capability
  (:require [sel4.capability :as cap]))

;; Generate unforgeable token
(def token (cap/generate-sideref "window-layout"))

;; Bind to endpoint
(def wm-cap (cap/bind-capability "sel4:xmonad-wm" token))

;; Check validity
(cap/valid? wm-cap)
;; => true

;; Verify statistics
(cap/get-stats)
;; => {:created 0, :verified 0, :revoked 0, :transferred 0}
```

### Test 4: GF(3) Balance Verification

```clojure
(ns test-sel4-balance
  (:require [sel4.balance :as b]
            [sel4.ipc :as ipc]))

;; Create a message
(def msg (ipc/->Message
  "sel4:window-factory"      ;; +1 PLUS
  :send-message              ;; 0 ZERO
  0 [] 0 []))

;; Verify balance
(b/verify-message-gf3 msg)
;; => {:valid true, :analysis {...}}

;; Check session
(b/record-message msg)
(b/verify-session-gf3)
;; => {:count 1, :balanced? true, :total-trit 0}
```

---

## Troubleshooting

### Problem 1: "Disk image not found"

```
✗ Configuration validation FAILED
   Check paths exist and are readable
```

**Solution**:
```bash
# Verify paths exist
ls -la /Users/bob/projects/xmonad-sel4/seL4/build/
ls -la /Users/bob/projects/xmonad-sel4/seL4/build/root-task.elf

# If direct-kernel boot, only kernel is required:
touch /Users/bob/projects/xmonad-sel4/seL4/build/root-task.elf

# If EFI boot, full disk image needed:
dd if=/dev/zero of=/Users/bob/projects/xmonad-sel4/seL4/build/sel4-arm64.img \
   bs=1g count=0 seek=4
mkfs.ext4 /Users/bob/projects/xmonad-sel4/seL4/build/sel4-arm64.img
```

### Problem 2: "VM failed to validate"

```
✗ Configuration validation FAILED
```

**Solution**:
```bash
# Check available resources
vm_memory_total=$(sysctl hw.memsize | awk '{print $2}')
echo "Available memory: $(($vm_memory_total / 1024 / 1024 / 1024)) GB"

# Reduce VM size in sel4-vm.joke:
(def vm-cpus 2)
(def vm-memory-gb 2)
```

### Problem 3: "VM boots but no output"

**Solution**:
- Direct kernel boot might work but show no console
- Check with `(vz/vm-state vm)` - if "running" then it's working
- Kernel might not have console configured
- Try with `-e` flag for test output

### Problem 4: "REPL commands don't respond"

**Solution**:
```clojure
;; seL4 might be booting - wait and check state
(Thread/sleep 2000)
(vz/vm-state vm)

;; If you need to interact with seL4, use IPC module
;; But note: kernel must be ready to receive IPC
```

---

## Architecture Verification Checklist

- [ ] seL4 kernel binary exists (root-task.elf or sel4-arm64.img)
- [ ] boxxy installed and working (`boxxy --version`)
- [ ] sel4-vm.joke configured with correct paths
- [ ] boot-method set (:direct-kernel or :efi)
- [ ] VM boots successfully (`vm state = running`)
- [ ] IPC statistics show messages sent/received
- [ ] GF(3) invariant verified (total-trit ≡ 0 mod 3)
- [ ] Window manager responds to queries
- [ ] Capability tokens generate and verify
- [ ] Networking functional (NAT configured)

---

## Performance Verification

### Measure Startup Time

```clojure
;; In REPL:
boxxy=> (def start (System/currentTimeMillis))
boxxy=> (def cfg (create-sel4-vm))
boxxy=> (def vm (vz/new-vm cfg))
boxxy=> (vz/start-vm! vm)
boxxy=> (def elapsed (- (System/currentTimeMillis) start))
boxxy=> (println "Startup time:" elapsed "ms")

;; Expected:
;; Direct kernel: 30-50ms
;; EFI boot: 800-1000ms
```

### Measure IPC Latency

```clojure
(ns test-latency
  (:require [sel4.ipc :as ipc]))

;; Simple ping-pong test
(defn measure-ipc-latency [iterations]
  (let [times (atom [])]
    (dotimes [i iterations]
      (let [start (System/nanoTime)
            msg (ipc/->Message "sel4:xmonad-wm" :query-layout 0 [] 0 [])
            _ (ipc/send-recv "sel4:xmonad-wm" msg)
            elapsed (- (System/nanoTime) start)]
        (swap! times conj elapsed)))

    {:count iterations
     :total-nanos (reduce + @times)
     :avg-nanos (/ (reduce + @times) iterations)
     :avg-micros (/ (reduce + @times) iterations 1000.0)
     :min-nanos (apply min @times)
     :max-nanos (apply max @times)}))

;; Test
(measure-ipc-latency 100)
;; Expected: 0.5-2μs latency (ARM64 native, no translation)
```

### Verify GF(3) Conservation

```clojure
(ns test-gf3-conservation
  (:require [sel4.balance :as b]))

;; Run a session and verify total trit conservation
(def results (atom []))

;; Simulate various operations
(dotimes [i 100]
  (b/record-message
    {:endpoint (if (even? i) "sel4:security-monitor" "sel4:window-factory")
     :operation (if (< (rand) 0.5) :send-message :query-layout)
     :label 0}))

;; Verify conservation
(def session (b/verify-session-gf3))
(println "GF(3) Conserved:" (:balanced? session))
(println "Total trit:" (:total-trit session))
;; => GF(3) Conserved: true
;; => Total trit: 0
```

---

## Next Steps After Successful Boot

1. **Integrate XMonad** - Run Haskell window manager in seL4
2. **Test IPC extensively** - Verify capability binding
3. **Benchmark performance** - Compare to QEMU baseline
4. **Formal verification** - Use Isabelle to prove safety
5. **Scale to multiple processes** - Test isolation
6. **Integrate with Plurigrid** - Connect to other agents

---

## Quick Reference: Commands

```bash
# Build seL4 kernel
cd ~/projects/xmonad-sel4/seL4 && mkdir build && cd build
cmake -DPLATFORM=qemu-arm-virt -DCMAKE_BUILD_TYPE=Release ../ && ninja

# Run seL4 VM (auto)
cd ~/i/boxxy && boxxy examples/sel4-vm.joke

# Run seL4 VM (interactive REPL)
cd ~/i/boxxy && boxxy -e '(sel4-repl-demo)'

# Launch boxxy REPL
boxxy repl

# In REPL:
(require '[sel4.ipc :as ipc])
(require '[sel4.window :as w])
(require '[sel4.capability :as cap])
(require '[sel4.balance :as b])

# Compare boot methods
(compare-boot-methods)

# Measure IPC latency
(measure-ipc-latency 100)
```

---

## Expected Output: Successful Boot

```
╔════════════════════════════════════════════╗
║  seL4 / Direct Kernel (like linux-vm.joke)║
╚════════════════════════════════════════════╝

Creating direct kernel boot loader...
Setting up generic ARM64 platform...
Configuring VM: 4 CPUs, 4 GB RAM
Configuring NAT networking...

Validating VM configuration...
✓ Configuration valid

Creating VM instance...
Starting seL4 VM...
✓ VM started

Monitoring VM (Ctrl+C to stop)...
[0] VM state: running
[1] VM state: running
[2] VM state: running
...

✓ VM running successfully
```

If you see this output, seL4 is running natively on ARM64 via boxxy with no translation overhead!
