# boxxy

```
┌─────────────────────────────────────────────────┐
│                                                 │
│   ┌───┐   ┌───┐   ┌───┐   ┌───┐   ┌───┐      │
│   │ V │───│ M │───│ ⊗ │───│ V │───│ M │      │
│   └─┬─┘   └─┬─┘   └─┬─┘   └─┬─┘   └─┬─┘      │
│     │       │       │       │       │          │
│   ┌─┴─┐   ┌─┴─┐   ┌─┴─┐   ┌─┴─┐   ┌─┴─┐      │
│   │ V │───│ M │───│ ⊗ │───│ V │───│ M │      │
│   └───┘   └───┘   └───┘   └───┘   └───┘      │
│                     THE                         │
│                    TILE                          │
│                                                 │
└─────────────────────────────────────────────────┘
```

One tile. Every composition of VMs -- sequential, parallel, nested --
is a tiling of this tile.

Dedication: [FOAR EVERYWUN FRUM BOXXY](https://www.youtube.com/watch?v=Yavx9yxTrsw)

## The Tile

A brick diagram (Hedges & Herold 2019, arXiv:1908.10660) is a variant of
a string diagram where morphisms are axis-aligned rectangles ("bricks")
and wires are the edges between them. Brick diagrams tile the plane.
Two tilings are equivalent if and only if they are related by planar isotopy --
continuous deformation that doesn't cross wires or change the height-ordering
of bricks.

boxxy has one tile:

```
        in_wires (guest state)
            │ │ │
       ┌────┴─┴─┴────┐
       │              │
       │     VM       │
       │              │
       │  boot ───→ ● │
       │  run  ───→ ● │
       │  stop ───→ ● │
       │              │
       └────┬─┬─┬────┘
            │ │ │
        out_wires (guest state')
```

The tile is a morphism in a monoidal category:

```
tile : GuestState ⊗ Config → GuestState' ⊗ Result
```

It has:
- **Input wires** (top): guest disk state, kernel/initrd, config
- **Output wires** (bottom): modified guest state, execution result
- **Internal state**: the VM lifecycle (boot → run → stop)

**Claim.** Every boxxy program is a tiling of this tile, and two programs
are equivalent if and only if their tilings are related by planar isotopy.

### Sequential composition (`;`)

Run one VM, feed its output state to the next:

```
    │ │ │              │ │ │
┌───┴─┴─┴───┐     ┌───┴─┴─┴───┐
│   build   │     │   build   │
│   (Guix)  │     │   (Guix)  │ ;  test (Alpine)
└───┬─┬─┬───┘     └───┬─┬─┬───┘
    │ │ │              │ │ │
┌───┴─┴─┴───┐     ┌───┴─┴─┴───┐
│   test    │  =  │   test    │
│  (Alpine) │     │  (Alpine) │
└───┬─┬─┬───┘     └───┬─┬─┬───┘
    │ │ │              │ │ │
```

### Parallel composition (`⊗`)

Run two VMs side by side, no wires cross:

```
    │ │ │    │ │ │
┌───┴─┴─┴───┐┌───┴─┴─┴───┐
│   Guix    ││  FreeBSD  │
│           ││           │
└───┬─┬─┬───┘└───┬─┬─┬───┘
    │ │ │    │ │ │
```

### The swap (σ)

Exchange outputs between two VMs. The only place wires cross:

```
    │ │ │    │ │ │
┌───┴─┴─┴───┐┌───┴─┴─┴───┐
│   Guix    ││  FreeBSD  │
└───┬─┬─┬───┘└───┬─┬─┬───┘
     ╲ ╲ ╲  ╱ ╱ ╱
      ╲ ╲ ╲╱ ╱ ╱
       ╲ ╲╱╲ ╱
        ╲╱  ╲╱
        ╱╲  ╱╲
       ╱  ╲╱  ╲
      ╱   ╱╲   ╲
┌───┴─┴─┴───┐┌───┴─┴─┴───┐
│  FreeBSD  ││   Guix    │
│  (gets    ││  (gets    │
│  Guix     ││  FreeBSD  │
│  state)   ││  state)   │
└───┬─┬─┬───┘└───┬─┬─┬───┘
    │ │ │    │ │ │
```

### Why one tile is enough

The coherence theorem for monoidal categories (Joyal & Street 1991,
strengthened by Delpeuch & Vicary 2022 settling Selinger's conjecture)
says: two diagrams built from generators by `;`, `⊗`, and `σ` are equal
if and only if they are planar isotopic. Planar isotopy means you can
continuously deform one tiling into the other without tearing wires or
passing bricks through each other.

boxxy has one generator: the VM tile. Every program is a tiling.
Equivalence of programs = planar isotopy of tilings. The categorical
structure (associativity, interchange, unit laws) comes for free from
the geometry.

This is the brick diagram claim:

> **Theorem (informal).** The free monoidal category generated by the
> boxxy VM tile, quotiented by planar isotopy, is the category of all
> boxxy programs. Two programs compute the same function if and only if
> their brick diagrams are isotopic.

The one tile connects across all tilings because it IS the generator.
Every tiling is a word in the free monoid on {tile, `;`, `⊗`, `σ`}.
Isotopy is the only equivalence. There is nothing else.

## What boxxy is

Clojure-flavored (SCI/Joker) scriptable VM manager for Apple's
Virtualization.framework. Go binary, signed with virtualization
entitlements. The VM tile instantiated on metal.

## Quick Start

```bash
boxxy repl
```

```clojure
;; The tile: one VM
(def vm (-> (vz/new-vm-config 2 4
              (vz/new-efi-boot-loader
                (vz/new-efi-variable-store "test.nvram" true))
              (vz/new-generic-platform))
            (vz/add-storage-devices
              [(vz/new-usb-mass-storage
                 (vz/new-disk-attachment "haiku.iso" true))])
            (vz/new-vm)))

;; Sequential composition: boot ; run
(vz/start-vm! vm)
(vz/vm-state vm) ;=> "running"

;; Stop = complete the tile
(vz/stop-vm! vm)
```

```bash
# Parallel composition: two VMs side by side
boxxy run --efi --iso haiku.iso --disk haiku.img &
boxxy run --linux --kernel vmlinuz --initrd initrd.img --disk root.img &
```

## Guests

| OS | Boot | Tile configuration |
|----|------|--------------------|
| HaikuOS | EFI | `(vz/new-efi-boot-loader store)` |
| FreeBSD | EFI | `(vz/new-efi-boot-loader store)` |
| Alpine Linux | Linux | `(vz/new-linux-boot-loader kernel initrd cmdline)` |
| Ubuntu | Linux/EFI | either boot loader |
| Guix System | Linux/EFI | `--guix` flag, optional `--rosetta` |
| macOS | macOS | `(vz/new-macos-boot-loader)` Apple Silicon only |
| Windows | EFI | experimental |

Each row is the same tile with different input wires.

## API

The tile's interface:

| Wire | Function | Direction |
|------|----------|-----------|
| boot | `(vz/new-efi-boot-loader store)` | in |
| boot | `(vz/new-linux-boot-loader kernel initrd cmdline)` | in |
| boot | `(vz/new-macos-boot-loader)` | in |
| disk | `(vz/new-disk-attachment path read-only?)` | in |
| disk | `(vz/new-virtio-block-device attachment)` | in |
| disk | `(vz/new-usb-mass-storage attachment)` | in |
| net | `(vz/new-nat-network)` | in |
| net | `(vz/new-virtio-network attachment)` | in |
| config | `(vz/new-vm-config cpus memory-gb boot platform)` | in |
| config | `(vz/add-storage-devices config [devices])` | in |
| config | `(vz/add-network-devices config [devices])` | in |
| validate | `(vz/validate-config config)` | in → out |
| vm | `(vz/new-vm config)` | out |
| control | `(vz/start-vm! vm)` | in |
| control | `(vz/stop-vm! vm)` | in |
| control | `(vz/pause-vm! vm)` | in |
| control | `(vz/resume-vm! vm)` | in |
| state | `(vz/vm-state vm)` | out |
| disk | `(vz/create-disk-image path size-gb)` | in |

## Across all tilings

The tile connects across all tilings because of these isotopies:

**boxxy on metal** (Virtualization.framework):
```
tile = VM on Apple Silicon
tiling = orchestrated fleet of local VMs
isotopy = refactoring the orchestration
```

**boxxy in browser** (WebVM / CheerpX):
```
tile = x86 VM in a browser tab via WASM JIT
tiling = multiple tabs, each a VM
isotopy = same program, different layout
shared Merkle root = same state across tilings
```

**boxxy in proof** (zkVM / SP1 / Jolt):
```
tile = RISC-V execution with succinct proof
tiling = composed proofs (IVC / folding)
isotopy = equivalent proofs, same verification
shared Merkle root = the state commitment
```

**boxxy in market** (Plurigrid / Alkahest):
```
tile = one energy bid clearing
tiling = the full market (all bids, all clearings)
isotopy = equivalent market outcomes
shared Merkle root = consensus on clearing price
settlement via Stripe Bridge
```

**boxxy in broadcast** (145km AM gospel station):
```
tile = one commitment beacon pulse
tiling = all receivers × all timestamps
isotopy = same signal, different positions
shared Merkle root = atmospheric fingerprint
```

**boxxy in body** (passport.gay / neonatal):
```
tile = one moment of engaged perception
tiling = the session (brain × position × time)
isotopy = same invariant, different profiles
shared Merkle root = the 3-torus commitment
```

Same tile. Same laws. Same isotopy. The generator doesn't change.
Only the category of interpretation changes.

## Architecture

```
boxxy
├── cmd/boxxy/main.go       # CLI: the REPL that manipulates tiles
├── internal/
│   ├── lisp/               # Reader/evaluator: the term language
│   ├── repl/               # Interactive tiling
│   ├── runner/             # Script/CLI runner
│   └── vm/                 # The tile (vz namespace bindings)
├── src/gf3_vm_isolation.rs # GF(3) isolation: sum(trits) = 0 mod 3
├── std/vz/                 # vz namespace: tile interface
└── examples/               # Example tilings
```

## Requirements

- macOS 12+, Apple Silicon or Intel with VT-x
- Xcode Command Line Tools

```bash
git clone https://github.com/bmorphism/boxxy.git
cd boxxy && make install
```

Binary is signed with virtualization entitlements.

## License

MIT

## References

- Hedges & Herold, [Foundations of brick diagrams](https://arxiv.org/abs/1908.10660), 2019
- Delpeuch & Vicary, [Normalization for planar string diagrams](https://lmcs.episciences.org/8960), 2022
- Joyal & Street, The geometry of tensor calculus I, 1991
- Ghani, Hedges, Winschel & Zahn, [Compositional Game Theory](https://arxiv.org/abs/1603.04641), 2018
- [Code-Hex/vz](https://github.com/Code-Hex/vz) - Go bindings for Virtualization.framework
- [Apple Virtualization.framework](https://developer.apple.com/documentation/virtualization)
