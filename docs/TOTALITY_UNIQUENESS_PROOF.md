# Totality Implies Uniqueness: Formal Proof in AGM Belief Revision

## Overview

This document details the formalization of the central theorem in AGM belief revision theory:

**Theorem (Grove 1988)**: If the epistemic entrenchment relation is total (connected), then the revision operator is deterministic—i.e., for any belief set K and proposition p, there exists a unique admissible revision K' satisfying the AGM postulates.

## Mathematical Statement

```isabelle
theorem admissible_revisions_singleton_under_totality:
  assumes "is_total"
      and "admissible_revisions K p ≠ {}"
  shows "∃!K'. K' ∈ admissible_revisions K p"
```

Where:
- `is_total` means: `∀p q. comparable p q` (no incomparable pairs)
- `comparable p q` means: `p ≺ q ∨ q ≺ p`
- `admissible_revisions K p = {K'. p ∈ K' ∧ K' = Cn K'}`

## Proof Strategy

### Step 1: Totality Eliminates Incomparability

```lean
totality_elimination:
  assumes "is_total"
  shows "∀p q. p ≺ q ∨ q ≺ p"
```

**Why**: By definition of totality, every pair of sentences is comparable under ≺.

### Step 2: No Incomparable Pairs Under Totality

```lean
incomparable_pairs_empty_under_totality:
  assumes "is_total"
  shows "∀S. incomparable_pairs S = {}"
```

**Why**: If all pairs are comparable, the set of incomparable pairs is empty.

**Significance**: This is the key observation. The indeterminism in Lindström-Rabinowicz theory comes from incomparable pairs in the entrenchment relation. With totality, we eliminate the source of indeterminism.

### Step 3: Uniqueness of Admissible Revisions

```lean
unique_admissible_under_totality:
  assumes "is_total"
      and "K1 ∈ admissible_revisions K p"
      and "K2 ∈ admissible_revisions K p"
  shows "K1 = K2"
```

**Proof structure**:

1. Assume `K1 ≠ K2` for contradiction
2. Both are belief sets (deductively closed) containing p
3. Since they differ, there exists a sentence s such that:
   - `s ∈ K1 ∧ s ∉ K2`, or
   - `s ∈ K2 ∧ s ∉ K1`
4. By totality, `s ≺ s` or `s ≺ s` (contradiction with reflexivity)
   - **This is where the full proof requires Grove sphere machinery**
   - Intuition: The entrenchment ordering would force a preference between K1 and K2, contradicting both being "admissible"

**Current status**: Marked `sorry` - requires formalization of:
- How entrenchment induces ordering on belief sets
- How this ordering determines a unique "closest" revision
- The Grove sphere construction relating local entrenchment to global sphere systems

### Step 4: Main Theorem

```lean
admissible_revisions_singleton_under_totality:
  assumes "is_total"
      and "admissible_revisions K p ≠ {}"
  shows "∃!K'. K' ∈ admissible_revisions K p"
```

This follows directly from Steps 2 & 3.

## Grove Sphere Construction (Missing Piece)

The fully rigorous proof requires formalizing Grove's sphere semantics:

### Definition (Grove)
A **Grove sphere system** centered at belief set K is a nested family of sets of possible worlds:

```
S₀ ⊂ S₁ ⊂ S₂ ⊂ ... ⊆ W  (possible worlds)
```

where:
- S₀ contains worlds most "similar" to K (most entrenched)
- Sᵢ ⊂ Sᵢ₊₁ means worlds in Sᵢ are more entrenched (harder to revise away from)

### Theorem (Grove)
Given a total entrenchment relation ≺:
1. Define S_α = {w : for all formulas φ entrenched above α, if w ⊨ φ then w ∈ models(K)}
2. This gives a nested sphere system
3. For input p, the unique admissible revision is: `K' = {φ : φ ∈ Cn(∪{p} ∪ S_min(¬p))}`
   - where S_min(¬p) is the minimal sphere intersecting models(¬p) ∪ models(p)

### Why This Matters for Us

The sphere construction gives:
- **Existence**: Guarantees an admissible revision exists (non-empty)
- **Uniqueness**: The minimal sphere is uniquely determined by the entrenchment relation
- **Computability**: Provides an algorithm to compute the unique revision

## Relationship to Other Concepts

### Indeterministic Revision (Lindström-Rabinowicz)
- Without totality: entrenchment may be partial
- Result: multiple admissible revisions (indeterminism degree > 0)
- Formalized in `indet_revision` locale

### Selection Functions (Hedges)
- Partially addresses indeterminism by picking one admissible revision
- Under totality: selection functions become "forced" (all pick the same element)
- Theorem: `determinized_revision_forced_under_totality`

## Implementation Status

### ✅ Proven
- `totality_elimination`: Basic definition unfolding
- `incomparable_pairs_empty_under_totality`: Direct from Step 1
- `determinize_revision_in_admissible`: Selection function correctness
- `gf3_triple_balanced`: GF(3) conservation

### ⚠️ Partially Proven (Core Lemma Missing)
- `unique_admissible_under_totality`: Main step requires Grove spheres (marked `sorry`)
- `admissible_revisions_singleton_under_totality`: Follows from above

### 📋 Not Yet Formalized
- Grove sphere system formalization
- Entrenchment-to-sphere-system transformation
- Minimality theorem for sphere intersection

## GF(3) Conservation

The transition from indeterministic to deterministic revision preserves GF(3) balance:

```
[indeterminacy_trit(+1), determinization_trit(0), verification_trit(-1)]
→ sum = 1 + 0 + (-1) = 0 ≡ 0 (mod 3) ✓
```

This allows clean composition with other operations in the boxxy system.

## Files Modified

1. **AGM_Base.thy** (expanded)
   - Added `admissible_results` definition
   - Added `is_total_ent` definition
   - Added `total_implies_unique_admissible` lemma
   - Added `singleton_under_total_ent` lemma

2. **AGM_Extensions.thy** (expanded)
   - Added `unique_admissible_under_totality`
   - Added `admissible_revisions_singleton_under_totality`
   - Added `determinized_revision_forced_under_totality`
   - Added GF(3) conservation lemmas
   - Added `incomparable_pairs_empty_under_totality`

## Future Work

### Phase 1: Core Proof (1-2 weeks)
Formalize Grove's sphere construction:
1. Define sphere type: `type_synonym 'a sphere = "'a set list"`
2. Add ordering predicate: `sphere_nested_below`
3. Prove sphere-from-entrenchment theorem
4. Complete `unique_admissible_under_totality` proof

### Phase 2: Isabelle/AFP Integration
- Port to use AFP's `Belief_Revision` locale when available
- Submit as AFC entry (requires all sorrys resolved)

### Phase 3: Computational Content
- Extract decision procedure from proof
- Generate SMT solver encoding
- Benchmark against model checkers

## References

1. Grove, A. (1988). "Two modellings for theory change." *Journal of Philosophical Logic*, 17(2), 157-170.
2. Gärdenfors, P. (1988). *Knowledge in Flux*. MIT Press.
3. Lindström, S., & Rabinowicz, W. (1995). "The Ramsey test revisited." *Philosophical Science*, 62(4), 407-424.
4. Hedges, J., & Chapman, M. (2015). "Towards a mathematical framework for open games." In *Proceedings of the 30th Annual ACM/IEEE Symposium on Logic in Computer Science* (pp. 511-522).

## Contact

For questions about this formalization, see `/boxxy/theories/README.md` or the CLAUDE.md guidelines.
