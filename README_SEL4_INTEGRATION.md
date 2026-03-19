# seL4 + boxxy Integration Guide

**Status**: Ready for implementation
**Date**: February 2, 2026

## Quick Start

### 1. Boot seL4 VM

```bash
cd /Users/bob/i/boxxy
boxxy examples/sel4-vm.joke
```

Or from REPL:

```clojure
boxxy=> (load-file "examples/sel4-vm.joke")
boxxy=> (sel4-repl-demo)
```

### 2. Control Window Manager

```clojure
(require '[sel4.window :as w])

;; Get current layout
(w/get-layout)

;; Move windows in 2-column layout
(w/tile-2-column)

;; Focus a window
(w/focus-window 1)

;; Send keyboard input
(w/send-key 65)  ;; 'A' key
```

### 3. Manage Capabilities

```clojure
(require '[sel4.capability :as cap])

;; Generate unforgeable token
(def token (cap/generate-sideref "window-manager"))

;; Bind to endpoint
(def wm-cap (cap/bind-capability "sel4:xmonad-wm" token))

;; Verify still valid
(cap/valid? wm-cap)

;; Revoke when done
(cap/revoke-capability wm-cap)
```

### 4. Verify GF(3) Balance

```clojure
(require '[sel4.balance :as b])

;; Verify message balances
(b/verify-round-trip
  (b/->MessageRoundTrip
    "sel4:window-factory"   ;; PLUS
    :send-message           ;; ZERO
    "sel4:xmonad-wm"        ;; ZERO
    :send-message))         ;; ZERO

;; Verify session preserves balance
(b/verify-session-gf3)

;; Print report
(b/print-gf3-report)
```

## Architecture

### File Structure

```
/Users/bob/i/boxxy/
├── examples/
│   ├── sel4-vm.joke              # VM boot script
│   └── sel4-interactive.joke     # Interactive demo (TBD)
│
├── std/sel4/
│   ├── ipc.clj                   # Message passing
│   ├── window.clj                # Window manager control
│   ├── capability.clj            # Capability binding
│   ├── balance.clj               # GF(3) verification
│   └── protocol.clj              # Message serialization (TBD)
│
├── internal/
│   └── sel4/                     # Go backend (TBD)
│       ├── vm.go
│       ├── ipc.go
│       └── capability.go
│
└── test/
    └── sel4_test.joke            # Integration tests (TBD)
```

### Modules

#### `sel4.ipc` - Inter-Process Communication

Low-level message passing to seL4 endpoints.

```clojure
(require '[sel4.ipc :as ipc])

;; Send message (non-blocking)
(ipc/send-message "sel4:xmonad-wm" message)

;; Wait for response (blocking)
(ipc/recv-message "sel4:xmonad-wm" :timeout 5000)

;; Request-response pattern
(ipc/send-recv "sel4:xmonad-wm" message)

;; Record statistics
(ipc/get-statistics)
```

#### `sel4.window` - Window Manager

High-level window operations via XMonad/seL4.

```clojure
(require '[sel4.window :as w])

;; Layout queries
(w/get-layout)
(w/get-focused-window)
(w/list-windows)

;; Layout operations
(w/move-window wm-id x y width height)
(w/focus-window wm-id)
(w/close-window wm-id)

;; Predefined layouts
(w/tile-2-column)
(w/tile-3-column)
(w/fullscreen wm-id)
(w/maximize wm-id)

;; Input delegation
(w/send-key keycode :shift? true)
(w/send-mouse-move x y)
(w/send-mouse-click button)

;; Batch operations
(w/arrange-windows [[1 0 0 960 1080] [2 960 0 960 1080]])
```

#### `sel4.capability` - Cryptographic Capabilities

Bind seL4 capabilities to boxxy Sideref tokens.

```clojure
(require '[sel4.capability :as cap])

;; Generate unforgeable token (OCAPN spec)
(def token (cap/generate-sideref "window-layout"))

;; Create capability with rights bitmap
(def cap (cap/make-capability "window-manager"
                              :endpoint "sel4:xmonad-wm"
                              :rights 0xFF))

;; Verify token signature (constant-time)
(cap/verify-sideref token device-id)

;; Derive child capability (attenuated rights)
(def child (cap/inherit-capability parent-cap "child-wm"))

;; Revoke capability
(cap/revoke-capability cap)

;; Query capability properties
(cap/valid? cap)
(cap/derived-from? child parent)
(cap/has-right? cap 0x01)
```

#### `sel4.balance` - GF(3) Conservation

Verify all messages preserve GF(3) trit balance.

```clojure
(require '[sel4.balance :as b])

;; Analyze single message
(b/analyze-message msg)

;; Verify message is balanced
(b/verify-message-gf3 msg)

;; Verify round-trip (request + response)
(b/verify-round-trip
  (b/->MessageRoundTrip
    sender operation receiver response timestamp))

;; Verify sequence
(b/verify-message-sequence messages)

;; Track session
(b/record-message msg)
(b/verify-session-gf3)

;; Strict enforcement (optional)
(b/enable-gf3-enforcement)
(b/check-or-warn msg)
```

## GF(3) Balance Semantics

Every seL4 IPC operation must preserve GF(3) (mod 3 arithmetic):

### Skill Trits (seL4 Components)

| Skill | Trit | Role |
|-------|------|------|
| `sel4:capability-verifier` | -1 | Validation |
| `sel4:security-monitor` | -1 | Verification |
| `sel4:xmonad-wm` | 0 | Coordination |
| `sel4:ipc-router` | 0 | Message brokering |
| `sel4:process-creator` | +1 | Generation |
| `sel4:window-factory` | +1 | Creation |

### Operation Trits

| Operation | Trit | Type |
|-----------|------|------|
| `:verify-capability` | -1 | Verification |
| `:audit-message` | -1 | Validation |
| `:send-message` | 0 | Coordination |
| `:query-layout` | 0 | Coordination |
| `:create-endpoint` | +1 | Generation |
| `:allocate-capability` | +1 | Creation |

### Balanced Round-Trip Example

```clojure
;; Window factory (+1) creates endpoint (+1) via ipc-router (0)
;; Balanced if: (+1) + (+1) + (0) + (??) ≡ 0 (mod 3)
;; Therefore: -1 operation required to balance
;; Solution: Use security-monitor (-1) to verify/audit

(b/verify-round-trip
  (b/->MessageRoundTrip
    "sel4:window-factory"      ;; +1
    :create-endpoint           ;; +1
    "sel4:ipc-router"          ;; 0
    :audit-message))           ;; -1
;; Total: (+1) + (+1) + (0) + (-1) = 1 ≡ 1 (mod 3) -- UNBALANCED

;; Correct version:
(b/verify-round-trip
  (b/->MessageRoundTrip
    "sel4:window-factory"      ;; +1
    :send-message              ;; 0
    "sel4:ipc-router"          ;; 0
    :verify-capability))       ;; -1
;; Total: (+1) + (0) + (0) + (-1) = 0 ≡ 0 (mod 3) -- BALANCED ✓
```

## Implementation Progress

### Phase 1: Boot Script ✅
- [x] boxxy Clojure script to boot seL4 VM
- [x] EFI boot loader configuration
- [x] Network and storage device setup
- [x] Interactive REPL demo mode

### Phase 2: IPC Bindings ✅
- [x] Message serialization/deserialization
- [x] Endpoint management
- [x] Send/recv/send-recv operations
- [ ] (Backend) Go bindings to native seL4 syscalls

### Phase 3: Window Manager ✅
- [x] Window layout queries
- [x] Window move/resize operations
- [x] Focus management
- [x] Input device delegation
- [x] Batch operations
- [ ] (Backend) Integration with XMonad

### Phase 4: Capabilities ✅
- [x] HMAC-SHA256 Sideref token generation
- [x] Constant-time verification
- [x] Capability inheritance (attenuation)
- [x] Revocation tracking
- [x] Capability serialization
- [ ] (Backend) seL4 kernel integration

### Phase 5: GF(3) Balance ✅
- [x] Trit value system
- [x] Message analysis
- [x] Round-trip verification
- [x] Session tracking
- [x] Runtime enforcement (optional)
- [ ] (Backend) Automatic violation detection

### Phase 6: Testing & Integration (Next Week)
- [ ] Create sel4_test.joke test suite
- [ ] Integration tests with QEMU
- [ ] Performance benchmarks
- [ ] Documentation examples

## Building seL4 Disk Image

### Prerequisites

```bash
# Install ARM cross-compiler
brew install arm-none-eabi-gcc

# Or use Nix flakes
cd /Users/bob/projects/xmonad-sel4
nix develop
```

### Build ARM64 EFI Image

```bash
cd /Users/bob/projects/xmonad-sel4/seL4

# Configure for ARM64
mkdir -p build/arm64-efi
cd build/arm64-efi
cmake -DPLATFORM=qemu-arm-virt \
      -DCMAKE_BUILD_TYPE=Release \
      ../../

# Build
ninja

# Create disk image (4GB sparse)
dd if=/dev/zero of=sel4-arm64.img bs=1g count=0 seek=4
mkfs.ext4 sel4-arm64.img

# Mount and install kernel
mkdir -p /tmp/sel4-mount
sudo mount -o loop sel4-arm64.img /tmp/sel4-mount
sudo mkdir -p /tmp/sel4-mount/boot
sudo cp ./root-task.elf /tmp/sel4-mount/boot/sel4-kernel
sudo umount /tmp/sel4-mount
```

## Next Steps

1. **This Week**:
   - Create seL4 disk image (build script above)
   - Run `boxxy examples/sel4-vm.joke` to boot VM
   - Test window manager connectivity

2. **Next Week**:
   - Implement Go backend (internal/sel4/)
   - Connect to real seL4 IPC
   - Write test suite (test/sel4_test.joke)

3. **Week 3-4**:
   - Full integration testing
   - Performance optimization
   - Formal verification (Isabelle)

## References

- **boxxy**: [GitHub](https://github.com/bmorphism/boxxy) | [Local README](/Users/bob/i/boxxy/README.md)
- **seL4**: [Docs](https://docs.sel4.systems/) | [Kernel Source](/Users/bob/sel4/kernel)
- **OCAPN**: [Object Capability Network](https://github.com/spritely/ocapn)
- **XMonad**: [Haskell Window Manager](https://xmonad.org/)
- **Apple Virtualization**: [Framework Docs](https://developer.apple.com/documentation/virtualization)

## Contributing

To extend seL4/boxxy integration:

1. Add new window operations to `sel4.window`
2. Add new capabilities to `sel4.capability`
3. Extend GF(3) semantics in `sel4.balance`
4. Create tests in `test/sel4_test.joke`
5. Document in this README

## License

MIT (inherits from boxxy)
