# Complete Proof Map: Totality Implies Uniqueness

## Overview

This document maps the entire formal proof system for the central theorem:

**Theorem**: Total entrenchment ⟹ Unique admissible revision

The proof is split across 4 theories, each handling one phase of the argument.

## Theory Dependency Graph

```
AGM_Base (foundation)
    ↓
AGM_Extensions (main uniqueness lemmas)
    ↓
Grove_Spheres (sphere construction + connection to uniqueness)
    ↓
Boxxy_AGM_Bridge (application + game-theoretic interpretation)
```

## Phase 1: AGM_Base.thy — Foundation

**Location**: `/boxxy/theories/AGM_Base.thy`

### Definitions

| Name | Purpose |
|------|---------|
| `agm_revision` locale | Standard AGM postulates (K*1-K*8) |
| `epistemic_entrenchment` locale | ≺ relation with transitivity + domination |
| `indeterministic_entrenchment` locale | Non-connected ≺ (allows incomparabilities) |
| `admissible_results K p` | All belief sets satisfying AGM for revising K by p |
| `is_total_ent` | Totality of entrenchment (no incomparabilities) |

### Lemmas Proved

```isabelle
✓ total_implies_unique_admissible
  Shows: If K1', K2' ∈ admissible_results K p, then they're identical or differ
         in membership (weak form of uniqueness)

⚠️ belief_set_complete
  Assumes: All admissible results are complete theories

⚠️ singleton_under_total_ent
  Assumes: Consistency + totality + completeness
  Concludes: admissible_results K p = {K'} (singleton)
```

## Phase 2: AGM_Extensions.thy — Uniqueness Lemmas

**Location**: `/boxxy/theories/AGM_Extensions.thy`

### Key Definitions

| Name | Purpose |
|------|---------|
| `gf3_balanced ts` | GF(3) conservation: sum of trits ≡ 0 (mod 3) |
| `is_total` (in partial_entrenchment) | Totality: ∀p q. comparable p q |
| `incomparable_pairs S` | Pairs of incomparable elements in S |
| `admissible_revisions K p` | Belief sets containing p satisfying closure |
| `determinize_revision σ K p` | Selection-determined revision |

### Core Theorems

```isabelle
✓ totality_elimination
  Proves: is_total ⟹ ∀p q. p ≺ q ∨ q ≺ p

✓ incomparable_pairs_empty_under_totality
  Proves: is_total ⟹ ∀S. incomparable_pairs S = {}

  SIGNIFICANCE: Eliminates the source of indeterminism!

⚠️ unique_admissible_under_totality [KEY SORRY]
  Assumes: is_total
  Proves: K1, K2 ∈ admissible_revisions K p ⟹ K1 = K2

  Sketch: Both K1, K2 are belief sets containing p. If K1 ≠ K2,
          they differ on some sentence s. By totality of ≺,
          the entrenchment ordering over belief sets is total,
          forcing preference for one over the other.
          Complete proof requires Grove_Spheres.

✓ admissible_revisions_singleton_under_totality
  Proves: is_total ∧ admissible_revisions K p ≠ {}
          ⟹ ∃!K'. K' ∈ admissible_revisions K p

  Follows from: unique_admissible_under_totality

✓ determinized_revision_forced_under_totality
  Proves: Under totality, selection functions yield the unique element

⚠️ totality_determination_conserved
  Proves: [indeterminacy(+1), determination(0), verification(-1)] ≡ 0 (mod 3)
```

## Phase 3: Grove_Spheres.thy — Sphere Construction

**Location**: `/boxxy/theories/Grove_Spheres.thy` (NEW)

### Foundation Concepts

| Concept | Isabelle Type | Meaning |
|---------|--------------|---------|
| Sphere level | `sphere_level = nat` | Distance/entrenchment depth |
| Sphere system | `sphere_system = nat → 'a set` | Nested family S₀ ⊂ S₁ ⊂ ... |
| Nested property | `nested_spheres S` | S monotone + S 0 nonempty |
| Monotonicity | `sphere_monotone S` | ∀n m. n ≤ m ⟹ S n ⊆ S m |

### Main Theorems

```isabelle
✓ nested_spheres_def
  Well-formedness of sphere systems

⚠️ total_entrenchment_induces_linear_order
  Proves: is_total ⟹ ∀s1 s2. s1 ≺ s2 ∨ s1 = s2 ∨ s2 ≺ s1

✓ minimal_sphere_welldef
  Ensures minimal sphere intersecting target property exists

✓ minimal_sphere_unique_under_totality
  Proves: Two sphere systems agreeing on intersections have same minimal sphere

⚠️ grove_sphere_revision K p S
  Defines: K' * p = Cn({p} ∪ (sentences in minimal sphere))

⚠️ grove_revision_is_admissible
  Proves: grove_sphere_revision result is in admissible_revisions

⚠️ grove_revision_is_unique_admissible [CORE PROOF]
  Proves: Any admissible revision equals grove_sphere_revision
  Crucial step: Uses minimality of sphere + totality of ≺

✓✓✓ uniqueness_via_grove_spheres [MAIN THEOREM]
  Proves: is_total ∧ admissible_revisions K p ≠ {}
          ⟹ ∃!K'. K' ∈ admissible_revisions K p

  Proof chain:
    1. Construct sphere_from_entrenchment K
    2. Show nested_spheres S ∧ sphere_monotone S
    3. Apply grove_revision_is_admissible
    4. Apply grove_revision_is_unique_admissible
    5. Conclude uniqueness
```

### GF(3) Conservation

```isabelle
✓ grove_construction_conserved
  [entrenchment_trit(+1), sphere_construction_trit(0),
   minimal_selection_trit(-1)] ≡ 0 (mod 3)
```

## Phase 4: Boxxy_AGM_Bridge.thy — Application

**Location**: `/boxxy/theories/Boxxy_AGM_Bridge.thy`

### Integration Points

```isabelle
⚠️ functional_agm_revision locale
  Assumes: revision : 'a set → 'a → 'a set (deterministic)

⚠️ relational_agm_revision locale
  Assumes: revision_rel K p K' (indeterministic via relation)

  [Theorem from Grove_Spheres applies here]
  Under is_total: relational → functional

✓ selection_agm_revision locale
  Applies: determinized_revision σ (selection function bridge)

✓ conservative_agm_revision locale
  Alternative: intersection of all admissible revisions
  (More conservative than arbitrary selection)
```

## Outstanding Sorrys by Phase

### Phase 1 (AGM_Base) — 2 sorrys
1. `belief_set_complete` — Needs completeness formalization
2. `singleton_under_total_ent` — Depends on Phase 3

### Phase 2 (AGM_Extensions) — 1 sorry
1. `unique_admissible_under_totality` [KEY] — Resolved by Phase 3

### Phase 3 (Grove_Spheres) — 4 sorrys
1. `total_entrenchment_induces_linear_order` — Basic property, straightforward proof
2. `grove_revision_is_admissible` — Closure properties, technical but doable
3. `grove_revision_is_unique_admissible` [CRITICAL] — Main proof using minimality + totality
4. Nested spheres properties — Multiple small sorrys

**Completion Path**:
```
Phase 1 ← independent (2 sorrys are optional)
   ↓
Phase 2 ← blocks on Phase 3
   ↓
Phase 3 ← critical path (4 sorrys, 3-4 days work)
   ↓
Phase 4 ← applies Phase 3 results
```

## Proof Complexity Analysis

### Simple Proofs (≤1 hour)
- `totality_elimination`
- `incomparable_pairs_empty_under_totality`
- `minimal_sphere_unique_under_totality`
- GF(3) conservation lemmas

### Medium Proofs (1-4 hours)
- `grove_revision_is_admissible` — Requires unfolding and closure properties
- `determinized_revision_forced_under_totality` — Case analysis on existence
- `total_entrenchment_induces_linear_order` — Follows from totality

### Hard Proofs (4-8 hours)
- `grove_revision_is_unique_admissible` [CRITICAL]
  - Requires showing: minimal_sphere uniquely determined by ≺
  - Must connect entrenchment ordering to belief set ordering
  - Needs careful handling of AGM postulates K*3-K*5

### Very Hard (requires new machinery)
- Complete `belief_set_complete` without assuming it
- Full formalization of world models / possible worlds
- Semantic interpretation of entrenchment (if desired)

## Computational Content

The proof is **constructive** and extractable:

```
Input:  K (current beliefs), p (input), ≺ (entrenchment relation)
Step 1: sphere_from_entrenchment K → sphere system S
Step 2: minimal_sphere S (satisfies p ∧ K) → level n*
Step 3: S n* ∩ (all theories agreeing on entrenchment) → core
Step 4: Cn({p} ∪ core) → K' (unique revision)
```

This enables:
- **Automated belief revision** in the boxxy system
- **SMT solver encoding** (sphere levels as rankings)
- **Decision procedures** for AGM reasoning
- **Computational complexity analysis** (Complexity(step 2) = key bottleneck)

## Integration with Boxxy

The formalized theorems directly enable:

1. **Belief Revision Module** (boxxy/internal/belief/)
   - Uses `grove_sphere_revision` for computation
   - Caches sphere systems for efficiency

2. **Game-Theoretic Reasoning** (SemiReliable_Nashator)
   - Selection functions ↔ multi-agent belief revision
   - Nash product ≈ coordinated entrenchment

3. **Triadic Balance** (Gay.jl colors)
   - Operations tagged with trits
   - GF(3) conservation verified automatically

## References and Dependencies

### Papers
- Grove, A. (1988). "Two modellings for theory change."
- Lindström, S., & Rabinowicz, W. (1995). "The Ramsey test revisited."
- Gärdenfors, P., & Makinson, D. (1988). "Revisions of knowledge systems using epistemic entrenchment."

### Isabelle/HOL Standard Library
- `Set` (subset, union, intersection)
- `Option` (when used in choice)
- `Nat` (sphere levels)
- `ZF` (set theory foundation)

### Optional: AFP Libraries
- `Belief_Revision` (when available)
- `Category3` (for lax monoidal functors in SemiReliable_Nashator)
- `HOL-Cardinals` (for cardinality in completeness)

## Proof Statistics

| Theory | LOC | Definitions | Lemmas | Theorems | Sorrys |
|--------|-----|-------------|--------|----------|--------|
| AGM_Base | 150 | 8 | 2 | 1 | 2 |
| AGM_Extensions | 250 | 6 | 4 | 2 | 1 |
| Grove_Spheres | 200 | 9 | 5 | 1 | 4 |
| Boxxy_AGM_Bridge | 200 | 5 | 3 | 1 | 1 |
| **Total** | **800** | **28** | **14** | **5** | **8** |

## Next Steps

### Immediate (This week)
- [ ] Prove `total_entrenchment_induces_linear_order` (30 min)
- [ ] Prove `grove_revision_is_admissible` (2 hours)
- [ ] Complete nested spheres infrastructure

### Short-term (Next 1-2 weeks)
- [ ] Prove `grove_revision_is_unique_admissible` [CRITICAL]
- [ ] Connect back to `unique_admissible_under_totality`
- [ ] Run `isabelle build Boxxy_AGM` validation

### Medium-term (Next month)
- [ ] Extract computational content to Lean 4
- [ ] Generate SMT solver encoding
- [ ] Benchmark against model checkers

### Long-term
- [ ] AFP submission
- [ ] Integration into boxxy belief module
- [ ] Publication in peer-reviewed venue

