# boxxy

Clojure-flavored SCI for Apple Virtualization.framework. Run VMs on macOS via REPL, CLI, or scripts.

Dedication: [FOAR EVERYWUN FRUM BOXXY](https://www.youtube.com/watch?v=Yavx9yxTrsw)

Built on the upstream [Code-Hex/vz](https://github.com/Code-Hex/vz) Go bindings for Virtualization.framework, which makes the virtualization surface area scriptable from Joker and SCI.

Joker is a small Clojure interpreter with a linter/formatter, and SCI is a lightweight embeddable Clojure interpreter; boxxy uses both to provide a fast REPL and scriptable VM control.

## Features

- EFI boot (UEFI guests)
- Direct Linux kernel boot
- macOS guests on Apple Silicon
- REPL, CLI, and `.joke` scripts

## Requirements

- macOS 12+
- Apple Silicon or Intel with VT-x
- Xcode Command Line Tools

## Installation

```bash
git clone https://github.com/bmorphism/boxxy.git
cd boxxy
make install
```

Binary is signed with virtualization entitlements.

## Quick Start

Start REPL:

```bash
boxxy repl
```

Run HaikuOS (EFI):

```bash
boxxy -e '(vz/create-disk-image "haiku.img" 16)'
boxxy run --efi --iso haiku-r1beta5-x86_64-anyboot.iso --disk haiku.img
```

Run Linux (direct kernel):

```bash
boxxy run --linux \
  --kernel vmlinuz \
  --initrd initrd.img \
  --disk root.img \
  --memory 2 \
  --cpus 4
```

Run Guix (EFI installer or kernel boot):

```bash
boxxy run --guix --iso guix.iso --disk guix.img
boxxy run --guix --kernel vmlinuz --initrd initrd --disk guix.img
```

Run Guix with Rosetta for x86_64 Linux binaries (Apple Silicon):

```bash
boxxy run --guix --guix-arch x86_64 --rosetta \
  --kernel vmlinuz \
  --initrd initrd \
  --disk guix.img
```

Hardened sandbox (disable guest networking):

```bash
boxxy run --guix --iso guix.iso --disk guix.img --hardened
```

Run a script:

```bash
boxxy examples/haiku-vm.joke
```

## REPL Example

```clojure
boxxy=> (def store (vz/new-efi-variable-store "test.nvram" true))
#<EFIVariableStore>

boxxy=> (def boot (vz/new-efi-boot-loader store))
#<EFIBootLoader>

boxxy=> (def platform (vz/new-generic-platform))
#<GenericPlatform>

boxxy=> (def config (vz/new-vm-config 2 4 boot platform))
#<VMConfig>

boxxy=> (def iso-att (vz/new-disk-attachment "haiku.iso" true))
#<DiskAttachment>

boxxy=> (def iso (vz/new-usb-mass-storage iso-att))
#<USBMassStorage>

boxxy=> (vz/add-storage-devices config [iso])
nil

boxxy=> (vz/validate-config config)
true

boxxy=> (def vm (vz/new-vm config))
#<VM>

boxxy=> (vz/start-vm! vm)
true

boxxy=> (vz/vm-state vm)
"running"

boxxy=> (quit)
```

## API Reference

### Boot loaders

| Function | Description |
|----------|-------------|
| `(vz/new-efi-variable-store path create?)` | EFI NVRAM store |
| `(vz/new-efi-boot-loader store)` | EFI boot loader |
| `(vz/new-linux-boot-loader kernel initrd cmdline)` | Linux boot loader |
| `(vz/new-macos-boot-loader)` | macOS boot loader |

### Platform & storage

| Function | Description |
|----------|-------------|
| `(vz/new-generic-platform)` | Create generic platform config |
| `(vz/new-disk-attachment path read-only?)` | Attach disk/ISO image |
| `(vz/new-virtio-block-device attachment)` | Create virtio disk |
| `(vz/new-usb-mass-storage attachment)` | Create USB mass storage |

### Network

| Function | Description |
|----------|-------------|
| `(vz/new-nat-network)` | Create NAT network attachment |
| `(vz/new-virtio-network attachment)` | Create virtio network device |

### VM configuration

| Function | Description |
|----------|-------------|
| `(vz/new-vm-config cpus memory-gb boot platform)` | Create VM configuration |
| `(vz/add-storage-devices config [devices])` | Add storage devices |
| `(vz/add-network-devices config [devices])` | Add network devices |
| `(vz/validate-config config)` | Validate configuration |
| `(vz/new-vm config)` | Create VM instance |

### VM control

| Function | Description |
|----------|-------------|
| `(vz/start-vm! vm)` | Start VM |
| `(vz/stop-vm! vm)` | Stop VM |
| `(vz/pause-vm! vm)` | Pause VM |
| `(vz/resume-vm! vm)` | Resume VM |
| `(vz/vm-state vm)` | Get VM state |

### Utilities

| Function | Description |
|----------|-------------|
| `(vz/create-disk-image path size-gb)` | Create sparse disk image |

## Architecture

```
boxxy
├── cmd/boxxy/main.go       # CLI entrypoint
├── internal/
│   ├── lisp/               # S-expression parser & evaluator
│   │   ├── reader.go       # Lisp reader
│   │   └── eval.go         # Evaluator with standard functions
│   ├── repl/               # Interactive REPL
│   ├── runner/             # Script & CLI runner
│   └── vm/                 # VM abstraction layer
│       └── vm.go           # vz namespace bindings
├── std/vz/                 # vz namespace definitions
└── examples/               # Example scripts
    ├── haiku-vm.joke       # HaikuOS example
    ├── linux-vm.joke       # Linux example
    └── minimal.joke        # API demonstration
```

## Supported guests

| OS | Boot Mode | Status |
|----|-----------|--------|
| HaikuOS | EFI | ✅ Tested |
| FreeBSD | EFI | ✅ Tested |
| Alpine Linux | Linux | ✅ Tested |
| Ubuntu | Linux/EFI | ✅ Tested |
| Guix System | Linux/EFI | ✅ Supported |
| macOS | macOS | ⚠️ Apple Silicon only |
| Windows | EFI | ⚠️ Experimental |

## Development

```bash
# Run tests
make test

# Lint
make lint

make release
```

## Haiku analysis (ARM64 status)

From `docs/HAIKU-ARM64-STATUS.md`:

- HaikuOS ARM64 cannot currently run via boxxy (no prebuilt images, incomplete port, macOS toolchain issues).
- Apple Virtualization.framework only supports native ARM64 guests; Rosetta only applies to Linux x86_64 binaries inside an ARM64 Linux VM (Haiku uses different ABIs).

Concrete example in this repo:

- `scripts/build-haiku-arm64.bb` builds an ARM64 MMC image on Linux and writes `haiku-arm64-mmc.img`, intended for use with `cmd/haiku-gui/main.go`.

## Concurrency notes

Host-side VM control is guarded by Go mutexes (`internal/vm/vm.go`), and GUI runners lock the OS thread for Virtualization.framework (`cmd/haiku-gui/main.go`, `cmd/alpine-gui/main.go`). Expect edge cases and proceed carefully—this is an evolving codebase.

## License

MIT

## Credits

- [Code-Hex/vz](https://github.com/Code-Hex/vz) - Go bindings for Virtualization.framework
- [Apple Virtualization.framework](https://developer.apple.com/documentation/virtualization)
