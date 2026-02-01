# Implementation Guide: Complete Formalization of Totality → Uniqueness

## Executive Summary

This guide provides a complete roadmap for implementing the formal proof that **total entrenchment implies unique belief revision** in AGM theory, with integration into the boxxy system.

**Status**:
- ✅ Foundations laid (AGM_Base, AGM_Extensions)
- ✅ Grove sphere framework created (Grove_Spheres)
- ⚠️ 8 sorrys remaining, highest priority: `grove_revision_is_unique_admissible`

**Time estimate to completion**: 3-5 days of focused work

---

## Part 1: Understanding the System

### What We're Proving

**Theorem (Grove 1988)**:
```
Total entrenchment ⟹ Deterministic revision operator

Formally:
  ∀K p. is_total ∧ admissible_revisions K p ≠ {}
    ⟹ ∃!K'. K' ∈ admissible_revisions K p
```

### Why It Matters

AGM belief revision comes in two flavors:

| Aspect | Standard AGM | Lindström-Rabinowicz |
|--------|-------------|----------------------|
| Entrenchment | Total (connected) | Partial (may have incomparabilities) |
| Output | Single revision K' | Set of revisions {K'₁, K'₂, ...} |
| Determinism | Deterministic | Indeterministic |
| Application | Classical logic | Bounded rationality |

Grove's result bridges these: **totality of entrenchment forces determinism**.

### Where It Fits in boxxy

```
boxxy system architecture:

  Gay.jl (colors)
      ↓
  GF(3) Conservation
      ↓
  Belief Revision (← WE ARE HERE)
      ↓
  Game Theory (Nashator)
      ↓
  Triadic Skill Orchestration
```

The belief revision module uses:
- `grove_sphere_revision K p S` for computing revisions
- `unique_admissible_under_totality` to verify correctness
- GF(3) trits for system balance

---

## Part 2: Theory-by-Theory Implementation Plan

### Theory 1: AGM_Base.thy (DONE ✅)

**Status**: Foundation complete, optional enhancements

**What's proved**:
- AGM postulates K*1 through K*8
- Expansion operator ⊕
- Basic entrenchment structures

**What needs work**: None critical (foundational)

---

### Theory 2: AGM_Extensions.thy (DONE ✅ — mostly)

**Status**: Main structure complete, 1 critical sorry

**What's proved**:
- ✅ GF(3) trit operations
- ✅ Selection functions
- ✅ `totality_elimination`
- ✅ `incomparable_pairs_empty_under_totality`
- ⚠️ `unique_admissible_under_totality` [References Grove_Spheres]

**Next step**: Once Grove_Spheres is done, this sorry resolves automatically

**Work**: Document reference properly (already done)

---

### Theory 3: Grove_Spheres.thy (NEW — CRITICAL PATH)

**Status**: Structure laid out, 4 sorrys (2 easy, 2 hard)

**Priority**: HIGHEST — This unblocks everything

#### Easy Sorrys (Complete First)

**3a. `total_entrenchment_induces_linear_order`** (20 minutes)
```isabelle
lemma total_entrenchment_induces_linear_order:
  assumes "is_total"
  shows "∀s1 s2. s1 ≺ s2 ∨ s1 = s2 ∨ s2 ≺ s1"
```

**How**: Just unfold `is_total` definition and rearrange:
```
is_total = ∀p q. comparable p q
comparable = p ≺ q ∨ q ≺ p

So for any s1, s2: either s1 ≺ s2, or s2 ≺ s1
If s1 ≺ s2: done
If s2 ≺ s1: could have s1 = s2 only if s1 ≺ s1 and s2 ≺ s2
  (but transitivity rules this out unless equal)
```

**Proof sketch**:
```isabelle
proof
  fix s1 s2
  have h: "s1 ≺ s2 ∨ s2 ≺ s1" by (simp [assms is_total_def comparable_def])
  cases h
  · left; exact this  (* s1 ≺ s2 *)
  · right; right; exact this  (* s2 ≺ s1 *)
qed
```

**3b. `nested_spheres_from_entrenchment`** (30 minutes)
```isabelle
lemma nested_spheres_from_entrenchment:
  assumes "is_total"
  shows "nested_spheres (sphere_from_entrenchment K (λK'. belief_set K'))"
```

**How**: Show two properties:
1. S is monotone: n ≤ m ⟹ S n ⊆ S m
2. S 0 is nonempty

**Proof sketch**:
- S n = belief sets at entrenchment depth exactly n
- Depth is well-defined by: card {s. s ∈ K \ K' ∧ s is entrenched}
- Monotonicity: higher depth includes more revisions
- Nonemptiness: K itself is at depth 0

#### Hard Sorrys (Complete After Easy Ones)

**3c. `grove_revision_is_admissible`** (2-3 hours) [MEDIUM]

```isabelle
lemma grove_revision_is_admissible:
  assumes "is_total"
      and "nested_spheres S"
      and "S_structure_valid: ..."
  shows "p ∈ grove_sphere_revision K p S ∧
         grove_sphere_revision K p S = Cn (grove_sphere_revision K p S)"
```

**How**: By definition of `grove_sphere_revision`:
```
KG = Cn({p} ∪ {φ : ∀w ∈ S (minimal_sphere S ψ). φ ∈ w})
```

So:
- p ∈ KG: because p ∈ {p} ⊆ ... ⊆ Cn(...) by cn_incl
- KG = Cn KG: because Cn is idempotent

**Proof sketch**:
```isabelle
constructor
· simp [grove_sphere_revision_def, cn_incl]
· simp [grove_sphere_revision_def, cn_idem]
```

**3d. `grove_revision_is_unique_admissible`** (4-6 hours) [CRITICAL]

This is the **main theorem** — see GROVE_PROOF_STRATEGY.md for detailed guidance.

```isabelle
theorem grove_revision_is_unique_admissible:
  assumes "is_total"
      and "nested_spheres S"
      and "sphere_monotone S"
      and "S_canonical: S = sphere_from_entrenchment K (λK'. belief_set K')"
      and "K'' ∈ admissible_revisions K p"
  shows "K'' = grove_sphere_revision K p S"
```

**Strategy**: Prove K'' ⊆ KG and KG ⊆ K'' separately, then use Set.Subset_antisym.

**Proof structure** (see GROVE_PROOF_STRATEGY.md for full details):
1. Extract basic properties of K'' and KG
2. Prove minimal sphere is uniquely determined (uses totality)
3. Prove K'' ⊆ KG (uses AGM postulates K*3-K*5)
4. Prove KG ⊆ K'' (uses minimality + admissibility)
5. Conclude equality

---

### Theory 4: Boxxy_AGM_Bridge.thy (MOSTLY DONE ✅)

**Status**: Ready for use once Phase 3 completes

**What's proved**:
- ✅ Functional AGM revision locale
- ✅ Relational AGM revision locale
- ✅ Selection-determined revision
- ⚠️ Depends on Grove_Spheres for full proof

**Action**: No changes needed; will work once Grove_Spheres is complete

---

## Part 3: Work Schedule

### Day 1: Foundation & Easy Sorrys

**Morning (2 hours)**:
1. Read PROOF_MAP.md thoroughly
2. Review AGM_Base.thy and understand definitions
3. Read Grove (1988) paper (key sections: 2-3)

**Afternoon (3 hours)**:
1. Complete `total_entrenchment_induces_linear_order` (20 min)
2. Complete `nested_spheres_from_entrenchment` (30 min)
3. Test compilation: `isabelle build -b Boxxy_AGM`
4. Write unit tests for these lemmas (if desired)

**Evening (1 hour)**:
- Review GROVE_PROOF_STRATEGY.md
- Outline strategy for the hard sorrys
- Commit progress

### Day 2: Medium Sorry

**Morning (2 hours)**:
1. Complete `grove_revision_is_admissible`
2. Test that it compiles
3. Write documentation

**Afternoon (2 hours)**:
1. Set up proof skeleton for `grove_revision_is_unique_admissible`
2. Identify sub-lemmas that will be needed
3. Begin main proof

### Days 3-4: Critical Sorry

**Day 3 (6-8 hours)**:
1. Prove sub-lemmas:
   - `admissible_subset_grove` (1-2 hours)
   - `grove_subset_admissible` (1-2 hours)
   - Connection to minimal sphere (1-2 hours)
2. Sketch main proof outline

**Day 4 (6-8 hours)**:
1. Complete `grove_revision_is_unique_admissible`
2. Iterative debugging of proof (likely 2-3 attempts)
3. Test with `isabelle build -b Boxxy_AGM`
4. Run full regression tests

### Day 5: Integration & Polish

**Morning (3 hours)**:
1. Verify all sorrys are eliminated
2. Run `isabelle document` to generate proof document
3. Audit all proofs for clarity

**Afternoon (3 hours)**:
1. Write up final proof document
2. Create visualization of proof tree
3. Prepare for presentation/publication

---

## Part 4: Testing & Validation

### Automated Tests

**Test 1: Syntax & Type Checking**
```bash
isabelle check Boxxy_AGM
```
Should produce: ✓ All theories type-check

**Test 2: Build Verification**
```bash
isabelle build -b Boxxy_AGM
```
Should complete without errors and no sorrys remaining

**Test 3: Cross-Theory Verification**
```bash
# Make sure SemiReliable_Nashator still works
isabelle build -b SemiReliable_Nashator
```

### Manual Verification Checklist

- [ ] All sorry statements eliminated
- [ ] All lemmas compiles independently
- [ ] `grove_revision_is_unique_admissible` follows from assumptions
- [ ] Uniqueness theorem produces singleton for sample K, p, ≺
- [ ] GF(3) balance maintained throughout
- [ ] No orphaned definitions or lemmas
- [ ] Documentation is accurate and complete

### Example Verification (Concrete Test)

```isabelle
example : is_total → (
  let K = {"p ∨ q", "¬(p ∧ q)"}
  let p = "p"
  let S = sphere_from_entrenchment K belief_set
  in card (admissible_revisions K p) = 1  (* exactly one *)
)
```

---

## Part 5: Integration into boxxy

### Using the Proof

**In boxxy/internal/belief/revision.go**:
```go
// Use grove_sphere_revision to compute revisions
type BeliefReviser struct {
    entrenchment map[Sentence]map[Sentence]bool  // ≺ relation
    spheres      map[int]BeliefSet                // Cached S
}

func (br *BeliefReviser) Revise(K BeliefSet, p Sentence) BeliefSet {
    // Corresponds to: grove_sphere_revision K p S
    S := br.buildSpheres()
    minLevel := br.findMinimalSphere(S, p)
    base := S[minLevel]
    return br.logicalClosure(Singleton(p) ∪ base)
}
```

**In boxxy/internal/belief/validation.go**:
```go
// Verify uniqueness when total entrenchment is assumed
func ValidateUniqueness(K, p, entrenchment) bool {
    if !IsTotal(entrenchment) {
        return false  // Uniqueness not guaranteed
    }
    results := AllAdmissibleResults(K, p, entrenchment)
    return len(results) == 1  // Must be exactly one
}
```

### Documentation Updates Needed

1. Update `/boxxy/internal/belief/README.md` to reference the formal proof
2. Add CLAUDE.md note about theorem dependency
3. Include link to PROOF_MAP.md in docs

---

## Part 6: Troubleshooting Common Issues

### Issue 1: "undefined reference to `grove_sphere_revision`"
**Cause**: Isabelle not seeing Grove_Spheres.thy definition
**Fix**: Check ROOT file includes Grove_Spheres before Boxxy_AGM_Bridge

### Issue 2: "proof of `minimal_sphere_welldef` failed"
**Cause**: Least property not imported or not applied correctly
**Fix**: Use `Least_le` and `LeastI` from Nat order theory; check assumptions

### Issue 3: "Type mismatch: sphere_system expects 'a sphere_level → 'a set"
**Cause**: sphere_level defined as nat but used as something else
**Fix**: Ensure consistent type usage: sphere_level = nat throughout

### Issue 4: "proof state has 1 subgoal" (stuck on main proof)
**Cause**: Likely hitting the uniqueness core that requires Grove machinery
**Fix**: Review GROVE_PROOF_STRATEGY.md; may need to prove sub-lemmas first

### Issue 5: "Sorry: theorem grove_revision_is_unique_admissible still has sorry"
**Cause**: Using the lemma in other proofs before it's complete
**Fix**: Temporarily mark as `sorry` in other uses until this completes; use `declare [[ML_print_depth 100]]` for better error messages

---

## Part 7: Reference Materials

### Key Files

| File | Purpose |
|------|---------|
| PROOF_MAP.md | High-level overview of all 4 theories |
| GROVE_PROOF_STRATEGY.md | Detailed step-by-step for critical sorry |
| TOTALITY_UNIQUENESS_PROOF.md | Original proof exposition |
| Grove (1988) paper | Mathematical foundation (crucial!) |

### Key Lemmas by Theory

**AGM_Extensions.thy**:
- `incomparable_pairs_empty_under_totality` — Key step 1
- `unique_admissible_under_totality` — Main theorem (references Grove_Spheres)

**Grove_Spheres.thy**:
- `minimal_sphere_unique_under_totality` — Step 2
- `grove_revision_is_unique_admissible` — Step 3 (CRITICAL)
- `uniqueness_via_grove_spheres` — Main application

**Boxxy_AGM_Bridge.thy**:
- Uses results from above to implement belief module

### Isabelle Tactics to Know

| Tactic | Use |
|--------|-----|
| `simp [...]` | Simplification (expand definitions) |
| `by (rule ...)` | Apply named lemma |
| `ext` | Prove set equality via extensionality |
| `Set.Subset_antisym` | Prove set equality from both inclusions |
| `intro` | Introduce universal quantifiers |
| `cases h` | Case analysis on h |
| `by (exact ...)` | Provide direct proof term |

---

## Part 8: Expected Final State

### Metrics

```
Theory           Lines  Definitions  Lemmas  Theorems  Sorrys
AGM_Base         150    8            2       1         0
AGM_Extensions   250    6            4       2         0
Grove_Spheres    200    9            5       1         0
Boxxy_AGM_Bridge 200    5            3       1         0
────────────────────────────────────────────────────────────
TOTAL            800    28           14      5         0  ✅
```

### Proof Artifacts

- Main theorem: `uniqueness_via_grove_spheres` ✅
- Key lemmas: 5+ proven supporting lemmas ✅
- GF(3) conservation: Verified throughout ✅
- Documentation: Complete with examples ✅

### Integration Points

- boxxy/internal/belief: Ready for `grove_sphere_revision` implementation
- boxxy/internal/vm: GF(3) trits maintained
- boxxy/theories: All proofs validated in Isabelle

---

## Conclusion

By following this guide:

1. **Days 1-2**: Establish foundation (easy sorrys)
2. **Days 3-4**: Complete critical proof (grove_revision_is_unique_admissible)
3. **Day 5**: Polish, test, integrate

**Result**: Fully formalized proof that total entrenchment implies unique belief revision, with complete GF(3) integration into the boxxy system.

**Success**: When `isabelle build Boxxy_AGM` completes with **0 sorrys** and all theories compile ✅

