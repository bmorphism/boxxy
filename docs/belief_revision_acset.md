# Belief Revision ACSet with Time-Travel

## Overview

This DuckLake database models **indeterministic belief revision** following the 
Lindström-Rabinowicz framework, integrated with collaborative infrastructure 
(CatColab, Catlab.jl, ACSets.jl, automerge).

## Key Insight

**Non-connected epistemic entrenchment yields indeterminism.**

When the entrenchment preorder fails to be total (∃ p, q where neither p ≤ q nor q ≤ p),
multiple revision operators become admissible. This is modeled via:

- **Grove spheres** with partial (non-linear) ordering
- **Incomparable fallback pairs** creating branching
- **Choice functions** that determinize the indeterministic operators

## ACSet Schema

### Objects (Ob)
| Object | Cardinality | GF(3) Sum |
|--------|-------------|-----------|
| Repo | 4 | -1 |
| Author | 43 | 0 |
| Commit | 400 | 0 |
| BeliefSet | 4 | -1 |
| Sentence | 6 | 0 |
| Fallback | 4 | 0 |
| RevisionOp | 3 | 0 |
| AGMPostulate | 8 | 0 |

### Morphisms (Hom)
- Commit → Repo: 400 arrows
- Commit → Author: 400 arrows
- EntrenchmentRel: 9 arrows (preorder)
- FallbackOrder: 9 arrows (partial order)
- LeviIdentity + HarperIdentity: 6 bridges

## Indeterminism Structure

### Source
```
Fallback 2 (#7AD570) ∦ Fallback 3 (#6EE7F1)
```
No ordering between these spheres → indeterministic contraction

### Entropy Analysis
| Repo | Total Commits | Indeterministic | Entropy (bits) |
|------|---------------|-----------------|----------------|
| automerge | 100 | 55 | 0.688 |
| ACSets.jl | 100 | 64 | 0.653 |
| CatColab | 100 | 67 | 0.634 |
| Catlab.jl | 100 | 68 | 0.627 |

## Lean 4 Theorems

```lean
-- Membership witness
lemma indetResult_of_member {K : BeliefSet} {I : IndetRevisionOp} {p : Sentence}
    {op : RevisionOp} (hop : op ∈ I) :
    op K p ∈ indetResult K I p := ⟨op, hop, rfl⟩

-- Non-connectivity implies incomparable pair exists
theorem nonconnected_yields_indeterminism 
    (E : EpistemicEntrenchment Sentence) 
    (h : E.isIndeterministic) :
    ∃ p q, ¬E.rel p q ∧ ¬E.rel q p := by
  push_neg at h; exact h

-- Choice function determinizes
lemma determinize_mem (I : IndetRevisionOp) (choice : ChoiceFunction BeliefSet)
    (hvalid : choice.isValid) (hI : ∀ K p, (indetResult K I p).Nonempty) 
    (K : BeliefSet) (p : Sentence) :
    determinize I choice hI K p ∈ indetResult K I p :=
  hvalid (indetResult K I p) (hI K p)
```

## 27-Step Chromatic Walk

GF(3)-balanced walk through belief revision space:
- Seed: 27
- Total trit sum: 0 ✓

Color trajectory: `#9BAB34 → #32F1AE → #BB1187 → ... → #549B33`

## Isabelle Formalization

The `theories/` directory contains rigorous Isabelle/HOL formalization:

```
theories/
├── ROOT                      # Session configuration
├── AGM_Base.thy              # AGM postulates K*1-K*8, entrenchment
├── SemiReliable_Nashator.thy # Hedges-Capucci selection functions
├── Vibesnipe.thy             # Multi-agent belief revision games
└── document/root.tex         # LaTeX documentation
```

### Build

```bash
isabelle build -D theories/
```

### Key Theorems

- `nash_product_is_nash_eq`: Nashator produces Nash equilibria
- `determinize_mem`: Choice functions select from admissible set
- `vibesnipe_equilibrium`: Semi-reliable nashator yields ε-approximate equilibria
- `gf3_conserved`: GF(3) trit balance maintained

## Database Location

```
/tmp/catcolab_timetravel.duckdb (154 MB)
```

## Queries

```sql
-- Time-travel: belief state at any point
SELECT * FROM BeliefStateTimeline WHERE week = '2026-01-05';

-- Indeterministic clusters
SELECT * FROM IndeterministicCommitClusters;

-- Grove sphere incomparability
SELECT * FROM GroveSphereSystem WHERE sphere_status = 'INDETERMINISTIC';

-- Choice function rationality
SELECT * FROM ChoiceFunctionAnalysis WHERE satisfies_agm;
```
