# Boxxy AGM: Belief Revision Meets Game Theory

Isabelle/HOL formalization of AGM belief revision extended with:
- **Lindström-Rabinowicz indeterministic revision** (partial entrenchment)
- **Hedges-style selection functions** (compositional game theory)
- **Semi-reliable nashator** (ε-approximate Nash equilibria)
- **GF(3) trit conservation** (novel contribution)

## Theory Files

| File | Description |
|------|-------------|
| `AGM_Base.thy` | Basic AGM postulates, trit datatypes |
| `AGM_Extensions.thy` | Partial entrenchment, selection functions, indeterministic revision |
| `Boxxy_AGM_Bridge.thy` | Bridges AFP formalization with L&R and Hedges extensions |
| `OpticClass.thy` | Lenses and parametric lenses from open-games-hs |
| `SemiReliable_Nashator.thy` | Nash product with ε-slack (bounded rationality) |
| `Vibesnipe.thy` | Compositional belief revision game theory |

## Building

```bash
# Requires Isabelle2024 or later
isabelle build -D .
```

For AFP integration, install the [Archive of Formal Proofs](https://www.isa-afp.org/) and uncomment the AFP import in ROOT.

## Research Foundations

### AGM Belief Revision
- Alchourrón, Gärdenfors, Makinson (1985): "On the Logic of Theory Change"
- AFP Belief_Revision (Fouillard et al. 2021)

### Indeterministic Revision
- **Lindström & Rabinowicz (1991)**: "Epistemic entrenchment with incomparabilities"
  - Key insight: Partial ordering → multiple admissible revision results
  - Grove's spheres become "ellipses" when beliefs are incomparable

### Selection Functions & Game Theory
- **Ghani, Hedges et al. (2018)**: "Compositional game theory"
- **Capucci (2022)**: "Diegetic representation of feedback in open games"
  - Selection functions determinize indeterministic outcomes
  - Nash product composes selections for multi-agent scenarios

### Novel: GF(3) Conservation
Operations are tagged with trits from GF(3) = {-1, 0, +1}:
- **+1 (Plus)**: Generative (expansion, selection, play)
- **0 (Zero)**: Neutral (revision, composition, equilibrium)
- **-1 (Minus)**: Verificative (contraction, coplay, checking)

Conservation law: `∑ trits ≡ 0 (mod 3)`

## Architecture

```
                    ┌─────────────────────┐
                    │   AFP Belief_Revision│
                    │   (proven AGM core)  │
                    └──────────┬──────────┘
                               │ sublocale
                    ┌──────────▼──────────┐
                    │  Boxxy_AGM_Bridge   │
                    │  (integration layer) │
                    └──────────┬──────────┘
           ┌───────────────────┼───────────────────┐
           │                   │                   │
┌──────────▼────────┐ ┌───────▼───────┐ ┌────────▼────────┐
│  AGM_Extensions   │ │  OpticClass   │ │ SemiReliable_   │
│  (L&R partial     │ │  (lenses,     │ │ Nashator        │
│   entrenchment)   │ │  para_lens)   │ │ (Nash product)  │
└──────────┬────────┘ └───────┬───────┘ └────────┬────────┘
           └───────────────────┼───────────────────┘
                    ┌──────────▼──────────┐
                    │     Vibesnipe       │
                    │ (compositional      │
                    │  belief revision)   │
                    └─────────────────────┘
```

## Key Concepts

### Relational vs Functional Revision

**Standard AGM** (functional): `K * p = K'` — unique output

**Lindström-Rabinowicz** (relational): `admissible_results K p = {K₁', K₂', ...}` — set of admissible outputs

**Hedges determinization**: `selected_revision σ K p = σ (admissible_results K p)` — selection picks one

### The Vibesnipe Pattern

1. **Entrenchment** defines which beliefs are more/less entrenched
2. **Incomparability** yields multiple admissible revisions
3. **Selection function** picks one (player's strategy)
4. **Nash product** composes selections for multi-agent games
5. **Semi-reliable** allows ε-slack (bounded rationality)

## Documentation

- [Canonical Research](../docs/CANONICAL_RESEARCH.md) - Full bibliography and mappings
- [AFP Integration Plan](../docs/AFP_INTEGRATION_PLAN.md) - How to connect to AFP

## Status

- [x] GF(3) trit definitions and conservation lemmas
- [x] Partial entrenchment locale (Lindström-Rabinowicz)
- [x] Selection functions and determinization
- [x] Nash product and semi-reliable variant
- [x] Optic class (lenses, parametric lenses)
- [ ] Full AFP integration (requires AFP installation)
- [ ] Collapse theorem proof (total → functional)
- [ ] Complete semi-reliable approximation bounds

## License

MIT
