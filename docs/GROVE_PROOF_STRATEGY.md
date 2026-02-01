# Grove Proof Strategy: Completing the Core Sorry

## Problem Statement

**Lemma** (in Grove_Spheres.thy):
```isabelle
theorem grove_revision_is_unique_admissible:
  assumes "is_total"
      and "nested_spheres S"
      and "sphere_monotone S"
      and "S_canonical: S = sphere_from_entrenchment K (λK'. belief_set K')"
      and "K'' ∈ admissible_revisions K p"
  shows "K'' = grove_sphere_revision K p S"
```

This is the **critical sorry** that blocks the entire uniqueness proof.

## High-Level Proof Strategy

The proof works by showing that both `K''` and `grove_sphere_revision K p S` are **minimal models** of a certain set of constraints, and that totality of entrenchment forces this set to have a unique minimal element.

### Three Constraints

Any admissible revision K' must satisfy:

1. **Success**: `p ∈ K'` (the input is believed)
2. **Closure**: `K' = Cn K'` (belief set must be closed under consequence)
3. **Minimality**: There is no other belief set K'' ≠ K' with K'' ⊂ K' and K'' ⊨ p

The last constraint comes from AGM postulate K*3 (inclusion): `K * p ⊆ K ⊕ p`

## Step-by-Step Proof Sketch

### Step 1: Extract Properties of K''

```isabelle
have K''_p: "p ∈ K''" by (simp [K'' ∈ admissible_revisions K p])
have K''_closed: "K'' = Cn K''" by (simp [K'' ∈ admissible_revisions K p])
```

Immediate from the definition of `admissible_revisions K p`.

### Step 2: Extract Properties of grove_sphere_revision

```isabelle
have KG_p: "p ∈ grove_sphere_revision K p S" by (simp [grove_sphere_revision_def])
have KG_closed: "grove_sphere_revision K p S = Cn (grove_sphere_revision K p S)"
  by (simp [grove_sphere_revision_def, cn_idem])
```

Also immediate from the definition and closure properties.

### Step 3: Key Observation About Minimal Sphere

The grove revision is constructed as:
```
KG = Cn({p} ∪ {φ : ∀w ∈ S(minimal_sphere S ψ). φ ∈ w})
```

Where ψ = `satisfies_prop_and_theory K p`, i.e., worlds where:
- p is true
- All of K is true

**Claim**: This minimal sphere is **uniquely determined** by K, p, and the total entrenchment ≺.

**Why**: Because:
- S is built from ≺ via `sphere_from_entrenchment`
- ≺ is total (by assumption)
- Total ≺ means every two sentences are comparable
- This induces a total order on "levels" of worlds
- Hence the minimal level intersecting ψ is unique

### Step 4: Show Any Admissible K'' Must Be Contained in KG

**Key Lemma**:
```isabelle
lemma admissible_subset_grove:
  assumes "is_total"
      and "nested_spheres S"
      and "K'' ∈ admissible_revisions K p"
  shows "K'' ⊆ grove_sphere_revision K p S"
```

**Proof idea**:
1. Suppose φ ∈ K'' for some φ
2. We want to show φ ∈ KG = `grove_sphere_revision K p S`
3. By closure of KG, it suffices to show φ ∈ KG's base set
4. Since K'' must be minimal (AGM-compliant), and ≺ is total...
5. ... any sentence in K'' must be either:
   - Entailed by p (hence in all revisions)
   - In the minimal sphere (hence in KG)

**Technical detail**: This step requires showing that AGM postulates K*3-K*5 force K'' to be "maximally minimal" in a way that agrees with the sphere structure.

### Step 5: Show Any Admissible K'' Must Contain KG

**Key Lemma**:
```isabelle
lemma grove_subset_admissible:
  assumes "is_total"
      and "nested_spheres S"
      and "K'' ∈ admissible_revisions K p"
  shows "grove_sphere_revision K p S ⊆ K''"
```

**Proof idea**:
1. Suppose φ ∈ KG = `grove_sphere_revision K p S`
2. By definition, φ is in the closure of {p} ∪ (base of KG)
3. To show φ ∈ K'', it suffices to show the base set ⊆ K''
4. The base set consists of sentences true in all worlds in the minimal sphere
5. Since K'' contains p and is minimal, it must contain all such sentences
6. (This requires: totality forces a unique minimal sphere; K'' being admissible means it includes everything true in that sphere)

### Step 6: Combine for Equality

```isabelle
have subset1: "K'' ⊆ KG" by (exact admissible_subset_grove assms)
have subset2: "KG ⊆ K''" by (exact grove_subset_admissible assms)
show "K'' = KG" by (exact Set.Subset_antisym subset1 subset2)
```

This is the standard trick: two sets are equal if they contain each other.

## Critical Technical Details

### Why Totality Is Needed

Each step above relies on totality in the following ways:

1. **Step 3**: Minimal sphere is unique
   - Without totality: multiple levels could equally "reach" ψ (incomparability issue)
   - With totality: linear order on levels ⟹ unique minimum

2. **Step 4**: Admissible K'' ⊆ KG
   - Without totality: K'' could contain sentences at "incomparable levels" to KG
   - With totality: Every sentence is at a definite level ⟹ K'' stays within KG

3. **Step 5**: KG ⊆ K''
   - Without totality: KG could contain sentences K'' "rejects" via incomparability
   - With totality: KG includes exactly the "forced" sentences ⟹ K'' must agree

### Where AGM Postulates Are Used

- **K*1** (closure): Ensures Cn properties work correctly
- **K*2** (success): Guarantees p ∈ K'' and p ∈ KG
- **K*3** (inclusion): Forces minimality ⟹ K'' ⊆ K ⊕ p
- **K*4** (vacuity): Handles case where p is consistent with K
- **K*5** (consistency): Ensures consistent revisions exist

The proof sketch above implicitly uses all of K*1-K*5, particularly K*3 and K*4 for showing the subset relations.

## Pseudo-Code for the Proof

```
FUNCTION prove_grove_revision_is_unique_admissible(is_total, nested_spheres S, K'', p):

  /* Step 1-2: Extract basic properties */
  ASSERT p ∈ K'' AND K'' = Cn K''                    /* from admissible_revisions *)
  ASSERT p ∈ KG AND KG = Cn KG                      /* from grove_sphere_revision *)

  /* Step 3: Establish minimal sphere uniqueness */
  LET min_level = LEAST n WHERE (∃w ∈ S n. ψ(w))    /* ψ = satisfies_prop_and_theory K p *)
  PROVE min_level is unique under is_total            /* Use totality *)

  /* Step 4: Show K'' ⊆ KG */
  ASSUME φ ∈ K''
  SHOW φ ∈ KG BY:
    - Case 1: φ is entailed by {p} → φ ∈ Cn({p}) ⊆ KG
    - Case 2: φ is not entailed by {p} →
      φ must come from minimality of K'' (AGM K*3)
      → φ is in all worlds at level min_level
      → φ ∈ KG
  END SHOW
  CONCLUDE K'' ⊆ KG

  /* Step 5: Show KG ⊆ K'' */
  ASSUME ψ ∈ base(KG)  /* ψ is in the base before closure *)
  SHOW ψ ∈ K'' BY:
    - ψ is true in all worlds at min_level
    - K'' contains p and is minimal (admissible)
    - By AGM K*4 (vacuity): if neg(p) ∉ K, then K ⊕ p ⊆ K*p
    - Hence all "mandatory" consequences (those in min_level) ∈ K''
    - Hence ψ ∈ K''
  END SHOW
  CONCLUDE KG ⊆ K''

  /* Step 6: Conclude equality */
  RETURN K'' = KG

END FUNCTION
```

## Filling In the Sorry

To complete `grove_revision_is_unique_admissible`, follow this template:

```isabelle
theorem grove_revision_is_unique_admissible:
  assumes "is_total"
      and "nested_spheres S"
      and "sphere_monotone S"
      and "S_canonical: S = sphere_from_entrenchment K (λK'. belief_set K')"
      and "K'' ∈ admissible_revisions K p"
  shows "K'' = grove_sphere_revision K p S"
proof -
  (* Extract properties *)
  have K''_p: "p ∈ K''" by (simp [assms(5) admissible_revisions_def])
  have K''_closed: "K'' = Cn K''" by (simp [assms(5) admissible_revisions_def])

  (* Extract grove revision properties *)
  have KG_p: "p ∈ grove_sphere_revision K p S" by (simp [grove_sphere_revision_def])
  have KG_closed: "grove_sphere_revision K p S = Cn (grove_sphere_revision K p S)"
    by (simp [grove_sphere_revision_def cn_idem])

  (* Show equality via subset in both directions *)
  have subset1: "K'' ⊆ grove_sphere_revision K p S" by
    (PROVE admissible_subset_grove using assms(1,2,5))
  have subset2: "grove_sphere_revision K p S ⊆ K''" by
    (PROVE grove_subset_admissible using assms(1,2,5))

  exact Set.Subset_antisym subset1 subset2
qed
```

## Auxiliary Lemmas to Prove First

Before tackling the main proof, establish these lemmas:

### Lemma 1: Minimal Sphere Property
```isabelle
lemma minimal_sphere_satisfies_ψ:
  assumes "nested_spheres S"
      and "∃w ∈ ⋃ (range S). ψ w"
  shows "∃w ∈ S (minimal_sphere S ψ). ψ w"
```
*Effort*: 30 minutes. Straightforward from definition of minimal_sphere using `LeastI`.

### Lemma 2: Uniqueness Under Totality
```isabelle
lemma minimal_sphere_unique_when_total:
  assumes "is_total"
      and "nested_spheres S"
      and "sphere_from_entrenchment_complete: ∀n. (∃w ∈ S n. ψ w) ∨ (∀w ∈ S n. ¬ψ w)"
  shows "∃!n. ∃w ∈ S n. ψ w"
```
*Effort*: 1 hour. Use totality to show decisiveness of ψ at each level.

### Lemma 3: AGM K*3 Implies Minimality
```isabelle
lemma admissible_is_minimal:
  assumes "K'' ∈ admissible_revisions K p"
      and "K''' ⊂ K''"
      and "K''' ⊆ Cn (insert p K)"
  shows "¬(p ∈ K''')"
```
*Effort*: 30 minutes. From definition of admissible_revisions and AGM postulate K*3.

### Lemma 4: Closure of KG Base Set
```isabelle
lemma grove_base_entails:
  assumes "nested_spheres S"
      and "φ ∈ S (minimal_sphere S ψ)"
  shows "φ ∈ grove_sphere_revision K p S"
```
*Effort*: 15 minutes. Unfold grove_sphere_revision_def and apply closure.

## Expected Proof Size

- Main theorem proof body: 50-80 lines
- Auxiliary lemmas: 150-200 lines total
- Comments/explanation: 50 lines
- **Total new code**: ~300 lines to complete the sorry

## Testing Strategy

After completing the proof:

1. **Syntax check**: `isabelle check Boxxy_AGM`
2. **Type check**: Verify all lemmas type-check
3. **Proof verification**: `isabelle build -b Boxxy_AGM`
4. **Dependency analysis**: Ensure no circular imports
5. **Regression test**: Run SemiReliable_Nashator proofs (should still work)

## Success Criterion

The proof is complete when:

```
✓ Grove_Spheres.thy compiles without sorrys
✓ uniqueness_via_grove_spheres is proven
✓ unique_admissible_under_totality (in AGM_Extensions) is resolved
✓ admissible_revisions_singleton_under_totality works
✓ All dependencies satisfied in Boxxy_AGM session
```

## References for Implementation

- **Isabelle proof patterns**: HOL/ex/Ackermann.thy, HOL/Proofs/Inductive.thy
- **Set theory proofs**: HOL/Set.thy (Set.Subset_antisym, Set.ext)
- **Order theory**: HOL/Order.thy (Least_le, LeastI)
- **Closure properties**: Example in AGM_Base.thy (cn_idem, cn_mono)

## Contact & Questions

If stuck on a substep:
1. Check PROOF_MAP.md for theory-level context
2. Review AGM_Base.thy for similar proofs (selection functions, etc.)
3. Consult Grove (1988) paper for intuition
4. Post specific sorry context to /boxxy/docs/STUCK.md

