#!/usr/bin/env python3
"""
Gadget 1: Harmonic Centrality → GF(3) Ordered Locale

The ordered locale is a frame (complete Heyting algebra) where:
  - Opens are up-sets in the centrality poset
  - The GF(3) coordinate partitions centrality into three bands
  - The subobject classifier Ω₃ assigns trit values via characteristic morphism

The centrality-to-trit map IS the characteristic morphism χ: Nodes → Ω₃
of the "well-connected" subobject in the locale of the DID graph.

Visibility in the ordered locale:
  visible(agent, record) ⟺ agent_trit + record_conf ≥ 0
  This is the GF(3) inner product: inclusion in the locale frame.
"""

import json
import sys
from typing import Any

import networkx as nx


# --- GF(3) Ordered Locale ---

def centrality_to_trit(c: float, thresholds: tuple[float, float] = (0.33, 0.66)) -> int:
    """χ: [0,1] → Ω₃ = {-1, 0, +1}  — the characteristic morphism.

    The ordered locale has three opens:
      U₊ = {x : c(x) > t_high}  → PLUS  (+1, generator, high centrality)
      U₀ = {x : t_low < c(x) ≤ t_high} → ERGODIC (0, coordinator, medium)
      U₋ = {x : c(x) ≤ t_low}   → MINUS (-1, validator, low centrality)

    The opens form a chain: U₋ ⊂ U₋∪U₀ ⊂ U₋∪U₀∪U₊ = X
    This chain IS the frame of the ordered locale on 3 elements.
    """
    lo, hi = thresholds
    if c > hi:
        return 1   # PLUS: high centrality → generator
    elif c > lo:
        return 0   # ERGODIC: medium → coordinator
    else:
        return -1  # MINUS: low centrality → validator


def trit_to_wire(trit: int, hue: float) -> dict:
    """FFI-safe wire encoding: never signed integers across boundaries.

    Byte 0: SIGN (0x00=ERGODIC, 0x01=PLUS, 0x02=MINUS)
    Byte 1: SIGNIFICAND (hue sector, 0-255)
    """
    sign = {1: 0x01, 0: 0x00, -1: 0x02}[trit]
    significand = int((hue / 360.0) * 256) & 0xFF
    return {"sign": sign, "significand": significand,
            "bytes": [sign, significand]}


def visible(agent_trit: int, record_conf: int) -> bool:
    """Ordered locale visibility: a + r ≥ 0.

    This is the correct access control:
      Bob(+1)    sees: PUBLIC(+1)✓ CONF(0)✓ SECRET(-1)✓  (all)
      Arbiter(0) sees: PUBLIC(+1)✓ CONF(0)✓ SECRET(-1)✗
      Alice(-1)  sees: PUBLIC(+1)✓ CONF(0)✗ SECRET(-1)✗  (public only)

    The sum a+r ≥ 0 IS the GF(3) inner product ⟨a,r⟩ in Z,
    projected to {True, False} via the locale's order.
    """
    return agent_trit + record_conf >= 0


# --- Harmonic Centrality Gadget ---

def harmonic_centrality_gadget(G: nx.Graph, sources=None) -> dict[Any, dict]:
    """Compute harmonic centrality and classify into GF(3) ordered locale.

    Returns for each node:
      - centrality: float in [0, n-1]
      - normalized: float in [0, 1]
      - trit: GF(3) classification
      - role: PLUS/ERGODIC/MINUS
      - hue: deterministic color (golden angle from node index)
      - wire: FFI-safe [sign, significand] encoding
    """
    hc = nx.harmonic_centrality(G, sources=sources)

    # Normalize to [0, 1]
    max_c = max(hc.values()) if hc.values() else 1.0
    if max_c == 0:
        max_c = 1.0

    # Compute adaptive thresholds from the distribution
    values = sorted(hc.values())
    n = len(values)
    if n >= 3:
        t_lo = values[n // 3]
        t_hi = values[2 * n // 3]
        # Normalize thresholds
        t_lo_norm = t_lo / max_c
        t_hi_norm = t_hi / max_c
    else:
        t_lo_norm, t_hi_norm = 0.33, 0.66

    results = {}
    for i, (node, c) in enumerate(hc.items()):
        norm = c / max_c
        trit = centrality_to_trit(norm, (t_lo_norm, t_hi_norm))
        # Golden angle hue: self-similar at every other index
        hue = (i * 137.508) % 360.0
        wire = trit_to_wire(trit, hue)

        results[node] = {
            "centrality": round(c, 4),
            "normalized": round(norm, 4),
            "trit": trit,
            "role": {1: "PLUS", 0: "ERGODIC", -1: "MINUS"}[trit],
            "hue": round(hue, 1),
            "wire": wire,
        }

    return results


def gf3_conservation_check(results: dict) -> dict:
    """Verify GF(3) conservation: Σ trits ≡ 0 (mod 3)."""
    trits = [r["trit"] for r in results.values()]
    total = sum(trits)
    residue = total % 3
    return {
        "total": total,
        "residue": residue,
        "conserved": residue == 0,
        "counts": {
            "PLUS": trits.count(1),
            "ERGODIC": trits.count(0),
            "MINUS": trits.count(-1),
        }
    }


def resource_sharing_allocation(results: dict) -> dict:
    """Derive resource-sharing allocation from centrality trits.

    PLUS nodes (high centrality) = resource DONORS (they have connections)
    ERGODIC nodes (medium)       = resource ROUTERS (they coordinate)
    MINUS nodes (low centrality) = resource RECEIVERS (they need help)
    """
    donors = [n for n, r in results.items() if r["trit"] == 1]
    routers = [n for n, r in results.items() if r["trit"] == 0]
    receivers = [n for n, r in results.items() if r["trit"] == -1]

    return {
        "donors": donors,
        "routers": routers,
        "receivers": receivers,
        "flow": [
            {"from": d, "via": routers[i % len(routers)] if routers else None,
             "to": receivers[i % len(receivers)] if receivers else None}
            for i, d in enumerate(donors)
        ] if routers and receivers else [],
    }


# --- Demo: DID Relationship Graph ---

def build_did_graph() -> nx.Graph:
    """Build a sample DID relationship graph for the passport.gay collective."""
    G = nx.Graph()

    # Core nodes: DID identifiers
    dids = [
        "did:gay:ewq3kfod7jn5eer7",   # bmorphism (hub)
        "did:gay:7ky2z4hx35nnwcjp",   # contributor
        "did:plc:abc123",              # bluesky user
        "did:ens:bmorphism.eth",       # ENS holder
        "did:ens:vitalik.eth",         # ENS holder
        "did:plc:def456",              # bluesky user
        "did:plc:ghi789",              # bluesky user
        "did:gay:validator01",         # validator
        "did:gay:validator02",         # validator
    ]

    G.add_nodes_from(dids)

    # Edges: identity bindings and social connections
    edges = [
        ("did:gay:ewq3kfod7jn5eer7", "did:plc:abc123", {"type": "same_identity"}),
        ("did:gay:ewq3kfod7jn5eer7", "did:ens:bmorphism.eth", {"type": "same_identity"}),
        ("did:gay:ewq3kfod7jn5eer7", "did:gay:7ky2z4hx35nnwcjp", {"type": "follows"}),
        ("did:gay:ewq3kfod7jn5eer7", "did:plc:def456", {"type": "follows"}),
        ("did:gay:ewq3kfod7jn5eer7", "did:plc:ghi789", {"type": "follows"}),
        ("did:gay:ewq3kfod7jn5eer7", "did:gay:validator01", {"type": "trusts"}),
        ("did:gay:ewq3kfod7jn5eer7", "did:gay:validator02", {"type": "trusts"}),
        ("did:gay:7ky2z4hx35nnwcjp", "did:ens:vitalik.eth", {"type": "follows"}),
        ("did:plc:def456", "did:plc:ghi789", {"type": "follows"}),
        ("did:gay:validator01", "did:gay:validator02", {"type": "peer"}),
    ]

    G.add_edges_from(edges)
    return G


def main():
    print("=== Gadget 1: Harmonic Centrality → GF(3) Ordered Locale ===\n")

    G = build_did_graph()
    print(f"Graph: {G.number_of_nodes()} nodes, {G.number_of_edges()} edges\n")

    # Compute gadget
    results = harmonic_centrality_gadget(G)

    print("--- Harmonic Centrality × GF(3) Classification ---")
    for node, r in sorted(results.items(), key=lambda x: -x[1]["centrality"]):
        sign_hex = f"0x{r['wire']['sign']:02X}"
        sig_hex = f"0x{r['wire']['significand']:02X}"
        print(f"  {node:<35s}  c={r['normalized']:.3f}  "
              f"trit={r['trit']:+d} ({r['role']:>7s})  "
              f"hue={r['hue']:5.1f}°  wire=[{sign_hex} {sig_hex}]")

    # GF(3) conservation
    print("\n--- GF(3) Conservation ---")
    check = gf3_conservation_check(results)
    print(f"  Σ trits = {check['total']}  residue = {check['residue']}  "
          f"conserved = {check['conserved']}")
    print(f"  PLUS: {check['counts']['PLUS']}  "
          f"ERGODIC: {check['counts']['ERGODIC']}  "
          f"MINUS: {check['counts']['MINUS']}")

    # Resource sharing
    print("\n--- Resource-Sharing Allocation ---")
    alloc = resource_sharing_allocation(results)
    print(f"  Donors   (PLUS):    {alloc['donors']}")
    print(f"  Routers  (ERGODIC): {alloc['routers']}")
    print(f"  Receivers (MINUS):  {alloc['receivers']}")
    if alloc["flow"]:
        print("  Flows:")
        for f in alloc["flow"]:
            print(f"    {f['from']} → {f['via']} → {f['to']}")

    # Ordered locale visibility demo
    print("\n--- Ordered Locale Visibility (a + r ≥ 0) ---")
    conf_labels = {1: "PUBLIC", 0: "CONFIDENTIAL", -1: "SECRET"}
    for a_trit, a_name in [(1, "PLUS"), (0, "ERGODIC"), (-1, "MINUS")]:
        vis = [conf_labels[r] for r in [1, 0, -1] if visible(a_trit, r)]
        print(f"  {a_name:>7s} (trit={a_trit:+d}) sees: {vis}")

    # JSON output for pipeline
    output = {
        "gadget": "harmonic-centrality-gf3",
        "graph": {"nodes": G.number_of_nodes(), "edges": G.number_of_edges()},
        "results": {str(k): v for k, v in results.items()},
        "conservation": check,
        "allocation": {k: [str(x) for x in v] if isinstance(v, list) else v
                       for k, v in alloc.items()},
    }
    print(f"\n--- JSON (pipe to next gadget) ---")
    print(json.dumps(output, indent=2, default=str))


if __name__ == "__main__":
    main()
