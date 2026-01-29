# Boxxy Examples

## Quick Start

```bash
# Build boxxy
cd ~/i/boxxy && make install

# Verify
boxxy --version
```

## 1. Linux VM (Direct Kernel Boot)

Boot a minimal Linux VM with kernel + initrd. Fastest path.

```bash
boxxy examples/linux-vm.joke
```

```clojure
;; linux-vm.joke
(def boot (vz/new-linux-boot-loader
            "vmlinuz"           ; kernel
            "initramfs.cpio.gz" ; initrd
            "console=hvc0"))    ; serial console
(def platform (vz/new-generic-platform))
(def config (vz/new-vm-config 2 4 boot platform))

;; 16GB disk
(vz/create-disk-image "linux.img" 16)
(def disk-att (vz/new-disk-attachment "linux.img" false))
(def disk (vz/new-virtio-block-device disk-att))
(vz/add-storage-devices config [disk])

;; NAT network
(def nat (vz/new-nat-network))
(def net (vz/new-virtio-network nat))
(vz/add-network-devices config [net])

;; Boot
(vz/validate-config config)
(def vm (vz/new-vm config))
(vz/start-vm! vm)
(println "State:" (vz/vm-state vm))
(vz/wait-for-shutdown vm)
```

## 2. EFI VM (HaikuOS, FreeBSD, etc.)

UEFI boot with NVRAM persistence.

```bash
boxxy examples/haiku-vm.joke
```

```clojure
;; haiku-vm.joke
(def store (vz/new-efi-variable-store "haiku.nvram" true))
(def boot (vz/new-efi-boot-loader store))
(def platform (vz/new-generic-platform))
(def config (vz/new-vm-config 2 4 boot platform))

;; ISO + disk
(def iso-att (vz/new-disk-attachment "haiku.iso" true))
(def iso (vz/new-usb-mass-storage iso-att))
(vz/create-disk-image "haiku.img" 16)
(def disk-att (vz/new-disk-attachment "haiku.img" false))
(def disk (vz/new-virtio-block-device disk-att))
(vz/add-storage-devices config [iso disk])

;; Network
(def nat (vz/new-nat-network))
(def net (vz/new-virtio-network nat))
(vz/add-network-devices config [net])

;; Validate & boot
(if (= (vz/validate-config config) true)
  (do
    (def vm (vz/new-vm config))
    (vz/start-vm! vm)
    (vz/wait-for-shutdown vm))
  (println "Config invalid"))
```

## 3. Interactive REPL

Explore the vz API interactively.

```bash
boxxy repl
```

```
boxxy=> (def platform (vz/new-generic-platform))
boxxy=> (type platform)
:external
boxxy=> (vz/create-disk-image "/tmp/test.img" 8)
true
boxxy=> (def att (vz/new-disk-attachment "/tmp/test.img" false))
boxxy=> (type att)
:external
```

## 4. Pause / Resume (Rewind)

Checkpoint a running VM and resume later.

```clojure
;; Start VM
(vz/start-vm! vm)
(println (vz/vm-state vm))  ;=> "running"

;; Pause (checkpoint memory state)
(vz/pause-vm! vm)
(println (vz/vm-state vm))  ;=> "paused"

;; Resume from checkpoint
(vz/resume-vm! vm)
(println (vz/vm-state vm))  ;=> "running"
```

## 5. Soft-Machine Provider (TypeScript)

Use boxxy as a compute provider in soft-machine.

```bash
# Set env vars
export BOXXY_PATH=~/.local/bin/boxxy
export BOXXY_STATE_PATH=~/.boxxy

# In soft-machine server
COMPUTE_PROVIDER=boxxy bun start
```

```typescript
import { getProviderRegistry } from "@/cluster/providers";

const registry = getProviderRegistry();
const boxxy = registry.get("boxxy");

await boxxy.initialize();

// Start a VM
const instance = await boxxy.startInstance({
  machineSize: "medium",
  metadata: {
    bootMode: "linux",
    kernelPath: "/path/to/vmlinuz",
  },
});

console.log(instance.id);    // boxxy-a1b2c3d4
console.log(instance.status); // running

// Branch for parallel agents
const branches = await boxxy.branchInstance(instance.id, 2);
// branches[0] = codex workspace
// branches[1] = copilot workspace
```

## 6. Browser via Squint Bridge

Use boxxy from the browser by connecting to a local WebSocket proxy.

```bash
# Start the boxxy WebSocket proxy (TODO: implement in boxxy)
boxxy serve --ws-port 7888
```

```clojure
;; In browser (squint/SCI)
(require '[vz.bridge :as vz])

(def provider (vz/browser-provider))

;; Start a VM via WebSocket
(-> ((:start-instance provider)
     {:machine-size :medium
      :kernel-path  "/path/to/vmlinuz"
      :disk-size-gb 16})
    (.then #(println "VM started:" %)))
```

## 7. EDN Normal Form

The provider's shape is defined as EDN data (`std/provider.edn`).
This is the metacircular property: the evaluator's spec is data
that the evaluator can read.

```clojure
;; Read the normal form
(def spec (edn/read-string (slurp "std/provider.edn")))

(:provider/name spec)       ;=> "boxxy"
(:provider/color spec)      ;=> "#0BC68E"
(:capabilities spec)        ;=> {:labels [:local :rewind :branch :suspend] ...}

;; The vz namespace is self-describing
(-> spec :vz/namespace :control)
;=> [{:name :vz/start-vm!, :args [:vm], :return :bool, :color "#A3C343"} ...]

;; Roundtrip: EDN → provider → EDN = identical
(= spec (edn/read-string (provider->edn (create-provider spec))))
;=> true
```

## Colors (seed=42)

Every depth in the vz S-expression tree gets a deterministic color
from gay MCP's SplitMix64 golden-angle spiral:

| Depth | Role     | Color     | Spin |
|-------|----------|-----------|------|
| 0     | root     | `#0BC68E` | +1   |
| 1     | config   | `#91BE25` | -1   |
| 2     | instance | `#1533EA` | +1   |
| 3     | snapshot | `#D822A5` | -1   |
| 4     | exec     | `#B09A11` | +1   |
| 5     | branch   | `#E2799D` | -1   |
| 6     | volume   | `#A4DE31` | +1   |

Magnetization: 0.143 (near-balanced, 4 up / 3 down)

## Provider Color Palette (all 9 providers, seed=42)

| # | Provider   | Color     | Trit | Role        |
|---|------------|-----------|------|-------------|
| 1 | Fly.io     | `#91BE25` | +1   | Production  |
| 2 | ix         | `#1533EA` |  0   | STW/local   |
| 3 | Vers       | `#D822A5` | -1   | Branching   |
| 4 | Morph      | `#B09A11` | +1   | Exploration |
| 5 | Cloud Run  | `#E2799D` | +1   | GPU         |
| 6 | Modal      | `#A4DE31` | +1   | GPU/ML      |
| 7 | Verifiers  | `#23B78B` |  0   | Training    |
| 8 | CheerpX    | `#8CE2F0` |  0   | Browser     |
| 9 | **Boxxy**  | `#A3C343` |  0   | Local VM    |
