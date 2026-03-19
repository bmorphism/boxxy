# $BOXXY: Feature Lifecycle via tldraw SDK

## The Tile on Canvas

tldraw SDK provides an infinite canvas React component. boxxy has one generator:
the VM tile. Every boxxy program is a tiling. The tldraw canvas IS the tiling surface.

```
tldraw shape  <-->  boxxy concept
─────────────────────────────────
geo:rectangle <-->  VM tile (the brick)
arrow         <-->  wire (state flow between tiles)
frame         <-->  composition boundary (;, ⊗, σ)
note          <-->  config annotation (kernel, initrd, disk)
group         <-->  parallel composition (⊗)
line          <-->  swap wires (σ)
```

## $BOXXY Token Features (12 lifecycle stages)

Each feature below is a stage in the $BOXXY token lifecycle.
On the tldraw canvas, each stage maps to a specific shape arrangement.

---

### Stage 1: MINT — Tile Creation

**tldraw sketch:** Single `geo:rectangle` with label "VM" and 3 input wires (arrows from top)
and 3 output wires (arrows to bottom).

**boxxy:** `(vz/new-vm config)` creates a tile instance.

**$BOXXY:** Minting a $BOXXY token = creating a tile on canvas.
The token carries: boot mode, CPU/RAM config, disk references, GF(3) trit assignment.

```
Token fields:
  tile_id: u64           -- unique tile identifier
  boot_mode: u8          -- 0=EFI, 1=Linux, 2=macOS
  config_hash: [u8; 32]  -- SHA256 of full VM config
  trit: i8               -- GF(3) assignment (-1, 0, +1)
  color: u32             -- Gay.jl deterministic color (seed 1069)
  minted_at: u64         -- timestamp
```

---

### Stage 2: COMPOSE — Sequential Wiring (;)

**tldraw sketch:** Two rectangles stacked vertically. Arrow from bottom of tile-1
to top of tile-2. Frame around both labeled "sequential".

**boxxy:** `(-> tile-1 tile-2)` or `;` composition. Output state of first tile
becomes input state of second.

**$BOXXY:** Composing two tokens produces a new "composed" token.
The composed token's config_hash = SHA256(hash-1 || hash-2).
GF(3) conservation: trit of composed = trit_add(trit-1, trit-2).

---

### Stage 3: PARALLEL — Tensor Product (⊗)

**tldraw sketch:** Two rectangles side by side. No arrows between them.
Frame around both labeled "parallel". Arrows enter from top independently,
exit from bottom independently.

**boxxy:** Two VMs running simultaneously: `(boxxy/parallel tile-1 tile-2)`.

**$BOXXY:** Parallel composition token carries BOTH sub-token hashes.
No wire crossing = no state dependency. Independently verifiable.

---

### Stage 4: SWAP — Wire Crossing (σ)

**tldraw sketch:** Two rectangles side by side, with crossing arrows
between output of tile-1 and input of tile-2, and vice versa.
The only arrangement where arrows cross.

**boxxy:** `(boxxy/swap tile-1 tile-2)` exchanges disk states.

**$BOXXY:** Swap token records the permutation. This is the ONLY
source of nontrivial topology in the tiling. Every other composition
is planar. The swap is what makes it a symmetric monoidal category.

---

### Stage 5: BOOT — Lifecycle Start

**tldraw sketch:** Rectangle with internal progress indicator.
Color transitions from gray (none) → yellow (created) → green (running).
Arrow from "config" note to tile's top wire.

**boxxy:** `(vz/start-vm! vm)`. The MacOSLifecycle state machine:
None → Created → Installed → Running.

**$BOXXY:** Boot event emitted on-chain. Token state transitions.
The MacOSState enum (lifecycle.go) maps directly:
  MacOSStateNone=0, Created=1, Installed=2, Running=3.

---

### Stage 6: ATTEST — Sideref Token Binding

**tldraw sketch:** Rectangle with a small lock icon (note shape).
Arrow from lock to "HMAC" label. Dashed line to "device" external entity.

**boxxy:** `NewSiderefToken(skillName, deviceSecret)` from sideref.go.
HMAC-SHA256 binding of skill identity to device.

**$BOXXY:** Attestation = binding a $BOXXY tile token to a physical device
via sideref. The sideref token becomes a claim on the tile's execution.
Unforgeable without device secret. Constant-time verification.

---

### Stage 7: TRIAD — GF(3) Consensus

**tldraw sketch:** Three rectangles in a triangle arrangement.
Each labeled with trit value (+1, 0, -1). Arrows forming a cycle.
Center label: "Σ = 0 mod 3".

**boxxy:** `NewTriadicConsensus(generator, coordinator, verifier)` from
vibesnipe_runtime.go. Three agents with balanced trits.

**$BOXXY:** Three tile tokens form a consensus triad.
  Generator(+1): produces new tile compositions
  Coordinator(0): routes and mediates
  Verifier(-1): validates execution
Sum must be 0 mod 3. The GF(3) invariant from AGM_Extensions.thy.

---

### Stage 8: SNIPE — Vibesnipe Selection

**tldraw sketch:** Multiple translucent rectangles overlapping (admissible revisions).
One highlighted with a crosshair icon. Arrow from "selector" to highlighted tile.

**boxxy:** The vibesnipe selection function from Vibesnipe.thy —
semi-reliable selection over revision operators induced by sphere system.

**$BOXXY:** A vibesnipe market on tile execution outcomes.
Bet on how many tiles it takes to achieve a goal state.
commit-reveal using SHA3-256. The `vibesnipe::market` module.

---

### Stage 9: BRIDGE — Worldline Connection

**tldraw sketch:** Two rectangles in different colors (worldline colors).
A thick bridge-shaped connector between them. Label: "witness_hash".
Small chips showing WL-2, WL-7, etc.

**boxxy/vibesniping:** The worldline_topology.move `submit_bridge` function.
Each bridge witness connects two adjacent worldlines.

**$BOXXY:** Walking a bridge earns $BOXXY. The bridge witness hash is
the proof of comprehension — it demonstrates that the holder understood
the connection between two theoretical foundations. This is the
anti-cognitive-debt mechanism: bridges require comprehension to produce.

---

### Stage 10: REGRET — Exchange

**tldraw sketch:** Rectangle with a "$REGRET" label and bidirectional arrows
to a "$BOXXY" rectangle. Exchange rate displayed on the arrow.

**boxxy/vibesniping:** The regret_exchange.move `exchange_snipes_for_regret` function.

**$BOXXY:** $REGRET tokens (quantified cognitive debt) can be exchanged
for $BOXXY tokens (tile execution rights). The exchange rate encodes:
the more regret accumulated (more worldlines unwalked), the more $BOXXY
it costs to execute tiles. Walking bridges reduces regret, which reduces
the cost of tile execution.

---

### Stage 11: PINHOLE — Network Exposure

**tldraw sketch:** Rectangle (VM tile) with a small dot on its border (the pinhole).
Arrow from pinhole to external "network" cloud shape. Label: port number.

**boxxy:** pinhole.go — minimal network exposure for VM tiles.
Each pinhole is a carefully scoped port forward.

**$BOXXY:** Pinhole tokens represent network access rights.
A tile can only communicate through explicitly minted pinholes.
This is the capability-security model from seL4_Bridge.thy:
  SendCap → PinholeOut
  RecvCap → PinholeIn
  RevokeCap → PinholeClose

---

### Stage 12: ISOTOPY — Equivalence Proof

**tldraw sketch:** Two different tile arrangements side by side.
A wavy "≅" symbol between them. Both produce the same output wires.

**boxxy:** The brick diagram theorem — two tilings are equivalent iff
planar isotopic. This is the coherence theorem for symmetric monoidal categories.

**$BOXXY:** An isotopy proof token demonstrates that two tile compositions
compute the same function. This is the ultimate $BOXXY: proof that
your tiling refactoring preserved behavior. The proof IS the comprehension.

---

## tldraw SDK Integration Plan

```typescript
// boxxy-canvas/src/BoxxyTldraw.tsx
import { Tldraw, createShapeId, TLShapeId } from 'tldraw'
import 'tldraw/tldraw.css'

// Custom shape: BoxxyTile
// Extends geo:rectangle with VM lifecycle state
interface BoxxyTileShape {
  type: 'boxxy-tile'
  props: {
    tileId: number
    bootMode: 'efi' | 'linux' | 'macos'
    state: 'none' | 'created' | 'installed' | 'running' | 'stopped'
    trit: -1 | 0 | 1
    color: string        // Gay.jl hex color
    configHash: string
  }
}

// Custom shape: BoxxyWire
// Extends arrow with state-flow semantics
interface BoxxyWireShape {
  type: 'boxxy-wire'
  props: {
    fromTile: TLShapeId
    toTile: TLShapeId
    compositionType: 'sequential' | 'parallel' | 'swap'
    bridgeWitness?: string  // hash if this wire is a worldline bridge
  }
}

// Custom shape: BoxxyTriad
// Three tiles in GF(3) balance
interface BoxxyTriadShape {
  type: 'boxxy-triad'
  props: {
    generator: TLShapeId
    coordinator: TLShapeId
    verifier: TLShapeId
    balanced: boolean
    triadSum: number
  }
}
```

## Canvas → Token Pipeline

```
tldraw canvas
    │
    ├── User draws tile rectangles
    ├── User connects with arrows (wires)
    ├── User groups into frames (compositions)
    │
    ▼
Canvas export (JSON shapes + arrows)
    │
    ├── Extract tile configs from shape props
    ├── Extract composition topology from arrows
    ├── Verify GF(3) balance on triads
    │
    ▼
boxxy REPL (Joker)
    │
    ├── (vz/new-vm config) for each tile
    ├── Sequential/parallel/swap from arrow topology
    ├── Sideref binding from lock annotations
    │
    ▼
$BOXXY token mint (Move on Aptos)
    │
    ├── worldline_topology::submit_bridge for worldline wires
    ├── vibesnipe::market for prediction markets on tile outcomes
    ├── regret_market::exchange_snipes_for_regret for regret↔boxxy exchange
    │
    ▼
On-chain attestation
```

## Mapping to Isabelle Theories

| tldraw Feature | $BOXXY Stage | Isabelle Theory |
|----------------|-------------|-----------------|
| Rectangle | MINT | OpticClass (lens_id) |
| Arrow (vertical) | COMPOSE | OpticClass (lens_compose / >>>) |
| Side-by-side group | PARALLEL | OpticClass (lens_parallel / &&&) |
| Crossing arrows | SWAP | Nashator (boxtimes with σ) |
| Color transition | BOOT | EgoLocale_AGM (ego state machine) |
| Lock icon | ATTEST | seL4_Bridge (sideref_valid) |
| Triangle layout | TRIAD | SemiReliable_Nashator (GF(3) balance) |
| Crosshair selection | SNIPE | Vibesnipe (vibesnipe_selection) |
| Bridge connector | BRIDGE | Cognitive_Debt (ext 68, worldline_topology) |
| Bidirectional arrow | REGRET | AGM_Extensions (contraction_trit) |
| Dot on border | PINHOLE | seL4_Bridge (SendCap/RecvCap) |
| Wavy ≅ symbol | ISOTOPY | Grove_Spheres (equivalence proof) |

## Color Assignments (seed 1069)

| Stage | Index | Color | Hex |
|-------|-------|-------|-----|
| MINT | 1 | crimson | #E82D4A |
| COMPOSE | 2 | burnt sienna | #D06546 |
| PARALLEL | 3 | teal | #2BBDA5 |
| SWAP | 4 | gold | #C9A82B |
| BOOT | 5 | indigo | #4B35CC |
| ATTEST | 6 | forest | #1DB854 |
| TRIAD | 7 | periwinkle | #76B0F0 |
| SNIPE | 8 | rose | #E84B89 |
| BRIDGE | 9 | amber | #CAC828 |
| REGRET | 10 | coral | #E86E4B |
| PINHOLE | 11 | emerald | #1D9E7E |
| ISOTOPY | 12 | lavender | #A76BF0 |
