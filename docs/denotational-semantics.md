# Shared Denotational Semantics

The boxxy polyglot repo converges on a single semantic pipeline that every
language fragment implements or consumes. This document names the pipeline
and maps each stage to its concrete implementations.

## The Pipeline

```
seed : u64
  │
  ├─ splitmix64(seed) ──→ value : u64            (bijective hash)
  │
  ├─ Color{R,G,B}     ──→ from low 24 bits       (deterministic identity)
  │
  ├─ Trit              ──→ (R+G+B) mod 3 ∈ {0,1,2}  (GF(3) classification)
  │
  ├─ Syrup wire        ──→ <'gay:color R G B>     (serialization)
  │
  └─ Rainbow parens    ──→ hue + golden angle     (presentation)
```

Every VM, skill, bridge, and REPL backend enters this pipeline at `seed`
and exits at whichever stage it needs. The pipeline is the denotational
semantics: two components are equivalent iff they produce the same
Color from the same seed.

## Stage Implementations

### Stage 1: SplitMix64

| Language | File | Function |
|----------|------|----------|
| Go | `internal/tile/tile.go` | `splitmix64(state uint64)` |
| Zig | `color_losses.zig` | `pub fn splitmix64(state: u64)` |
| Zig | `winding.zig` | `pub fn splitmix64(state: u64)` |
| Emacs Lisp | `worlds/b/boris/sygil.el` | `(sygil-splitmix64)` |
| Babashka | `worlds/b/boris/sygil-repl.bb` | `(splitmix64)` |

All use identical constants: `γ = 0x9e3779b97f4a7c15`, mix1 = `0xbf58476d1ce4e5b9`,
mix2 = `0x94d049bb133111eb`. Cross-language test: seed `1069` → same hex in every impl.

### Stage 2: Color

| Language | File | Type |
|----------|------|------|
| Go | `internal/tile/tile.go` | `Color{R,G,B uint8}` |
| Go | `internal/color/nats_color.go` | `ColorMsg{Hex,Trit}` |
| Zig | `color_losses.zig` | `RGB{r,g,b: u8}` |
| Zig | `winding.zig` | `Color{r,g,b: u8}` |
| Python | `scripts/color-losses.py` | `(r,g,b)` tuple |

### Stage 3: Trit (GF(3))

| Language | File | Computation |
|----------|------|-------------|
| Go | `internal/tile/tile.go` | `(R+G+B) % 3` |
| Go | `internal/gf3/gf3.go` | `Trit` type with `Add`, `Mul`, `Neg` |
| Zig | `winding.zig` | `WindingAccumulator.trit()` |
| Zig | `color_losses.zig` | `ColorLossAccumulator.recordTrit()` |
| Python | `gadgets/harmonic-centrality-gf3.py` | `(r+g+b) % 3` |
| Isabelle | `theories/*.thy` | formal `GF3` type |
| Dafny | `verified/GF3.dfy` | verified `GF3` arithmetic |
| Move | `contracts/vibesnipe-arena/` | on-chain trit in agent struct |
| Rust | `src/gf3_vm_isolation.rs` | `GF3VM` isolation sketch |

**Conservation law**: any triplet of trits in a balanced operation must sum to 0 mod 3.

### Stage 4: Syrup Wire Format

| Language | File | Format |
|----------|------|--------|
| Go | `internal/tile/tile.go` | `SyrupEncode()` → `<'gay:color R G B>` |
| Zig | (planned) | Syrup canonical encoding |
| C++ | `docs/haiku-color-bridge.cpp` | Haiku BMessage with `trit` field |
| QMD | `docs/color-quatro.qmd` | Quarto doc with msg.AddInt8("trit") |

### Stage 5: Rainbow Parens / Presentation

| Language | File | Method |
|----------|------|--------|
| Go | `internal/repl/tile_repl.go` | Golden-angle palette from Color.Hue() |
| Go | `internal/repl/polyglot.go` | Per-backend Gay MCP hex in `Backend.Color` |

## VM Backend Mapping

Every VM target gets its identity from the same pipeline:

| Backend | File | Identity Source |
|---------|------|----------------|
| VZ (local macOS) | `internal/vm/vm.go` | Config → seed → RegisterVM(name) |
| Parallels | `internal/vm/parallels.go` | prlctl name |
| Windows | `internal/vm/windows.go` | EFI NVRAM path |
| **Morphcloud** | `internal/vm/morphcloud.go` | `snapshot_id` → REST API |
| **Vers.sh** | `internal/vm/vers.go` | `vm_id` + `branch` → CLI |
| **Mautrix Bridge** | `internal/vm/bridge.go` | `BridgeType` + name → matrix appservice |

## Lisp Namespace Summary

The unified Lisp environment exposes all backends through the same pattern:

```
;; Local VM (Virtualization.framework)
(vz/new-vm (vz/config :cpus 4 :memory 8))

;; Cloud VM (Morphcloud REST API)
(morphcloud/start! (morphcloud/new "dev" 4 8))

;; Branching VM (vers.sh Git-like)
(vers/run! (vers/new "experiment") "ubuntu:24.04")
(vers/branch! vm "feature-x")

;; Messaging bridge (mautrix)
(bridge/start! (bridge/new "signal" "my-signal"))
```

## The Invariant

For any component `C` with seed `s`:

```
∀ s : u64,
  splitmix64_go(s) = splitmix64_zig(s) = splitmix64_el(s) = splitmix64_bb(s)
  ∧ color(s).trit = (R+G+B) mod 3
  ∧ syrup(color(s)) is the canonical wire encoding
```

This is what makes the repo polyglot but coherent: the denotational
semantics is the color pipeline, and every language fragment either
produces or consumes it identically.
