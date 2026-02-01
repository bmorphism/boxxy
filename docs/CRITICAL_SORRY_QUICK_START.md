# Critical Sorry: Quick Start Guide

**File**: `theories/Grove_Spheres.thy` line 197
**Sorry name**: `grove_revision_is_unique_admissible`
**Estimated time**: 4-6 hours
**Blocking**: Main theorem + all downstream proofs
**Difficulty**: Hard (requires deep understanding of Grove spheres)

---

## The Problem (In Plain English)

**What we need to prove**:
```
If the epistemic entrenchment relation is total (every two beliefs are comparable),
then any two admissible revisions of the same belief set by the same proposition
must be identical.
```

**Why it's hard**:
- We need to show that total entrenchment + AGM postulates uniquely determine the revision
- The proof requires understanding how Grove sphere systems work
- We need to prove that the minimal sphere containing p is unique under totality

---

## The Formal Statement

```isabelle
lemma grove_revision_is_unique_admissible:
  assumes is_total: "is_total"
      and K1_admissible: "K1 ∈ admissible_results K p"
      and K2_admissible: "K2 ∈ admissible_results K p"
  shows "K1 = K2"
```

**What this means**:
- `is_total`: The entrenchment relation ≺ is total (connected, no incomparabilities)
- `K1, K2 ∈ admissible_results K p`: Both K1 and K2 satisfy the AGM postulates for revising K by p
- The conclusion: K1 and K2 must be the same belief set

---

## Why This Matters

Once this single lemma is proven, everything else falls into place:

```
grove_revision_is_unique_admissible
         ↓ (enables)
admissible_subset_grove (K' ∈ admissible_results ⟹ K' ⊆ grove_sphere_revision)
         ↓ (enables)
grove_subset_admissible (grove_sphere_revision ⊆ ...all admissible_results)
         ↓ (enables)
Main theorem: admissible_revisions_singleton_under_totality
```

---

## The Proof Strategy (6 Steps)

### Step 1: Assume Two Different Admissible Revisions
```isabelle
by_contra h  -- Assume K1 ≠ K2 for contradiction
```

### Step 2: Identify Their Difference
```isabelle
obtain s where hs: "s ∈ K1 ∧ s ∉ K2" by (...)
```

### Step 3: Use Admissibility Constraints
Both K1 and K2 satisfy:
- `K1 ⊆ K ⊕ p` (inclusion)
- `p ∈ K1` (success)
- `belief_set K1` (closure)

Same for K2.

### Step 4: Derive Contradiction from Totality
Since is_total holds, every two sentences are comparable under ≺.

The key insight:
```
For the sentence s where K1 and K2 differ:
- Either s ≺ t for all t in difference between K1 and K2 (s most entrenched)
- Or some t is more entrenched than s

Total entrenchment forces a unique "cut-off" point.
```

### Step 5: Show This Cut-off is Unique
Using Grove's theorem: The minimal sphere containing p determines uniquely the revision result.

### Step 6: Conclude
The unique cut-off point determines a unique revision, so K1 = K2. Contradiction! ✓

---

## Implementation Checklist

### Before You Start (30 minutes)
- [ ] Read GROVE_PROOF_STRATEGY.md section "Filling In the Sorry" (critical details)
- [ ] Review AGM_Base.thy lines 1-80 (understand admissible_results definition)
- [ ] Review Grove_Spheres.thy lines 1-100 (understand sphere definitions)
- [ ] Read Grove (1988) paper, Section 3 (sphere semantics) - 20 minutes

### Setup Phase (30 minutes)
- [ ] Create new file `grove_proof_scratch.txt` with proof outline
- [ ] List all available lemmas you can use
- [ ] Write out step 1 in plain English
- [ ] Identify which lemmas handle step 1

### Phase 1: Foundation (1 hour)
- [ ] Implement steps 1-2 (assume contradiction, identify difference)
- [ ] Test compilation: `isabelle build Boxxy_AGM 2>&1 | grep -C2 grove_revision_is_unique`
- [ ] Any errors should give hints about missing lemmas

### Phase 2: Using AGM (1.5 hours)
- [ ] Implement step 3 (unfold admissibility constraints)
- [ ] Apply `admissible_results_def` to both K1 and K2
- [ ] Derive consequences about closure, success, inclusion
- [ ] Test compilation

### Phase 3: Totality Insight (2 hours)
- [ ] Implement step 4 (use totality to eliminate K2 possibility)
- [ ] This is the hardest part - you'll need:
  - `total_entrenchment_induces_linear_order` lemma (already proven)
  - Understanding of how totality eliminates indeterminism
- [ ] Reference: GROVE_PROOF_STRATEGY.md "Step 4: The Totality Cut"
- [ ] Test compilation frequently

### Phase 4: Sphere Mechanics (1 hour)
- [ ] Implement step 5 (sphere determines unique revision)
- [ ] Use `minimal_sphere_unique_under_totality` (already proven)
- [ ] Apply `grove_sphere_revision_def`
- [ ] Show resulting sphere matches what totality forces
- [ ] Test compilation

### Phase 5: The Contradiction (30 minutes)
- [ ] Implement step 6 (derive final contradiction)
- [ ] Your assumption `h: "K1 ≠ K2"` should lead to `False`
- [ ] You've shown K1 and K2 must be identical
- [ ] The `by_contra` framework will complete the proof
- [ ] Full compilation: `isabelle build -b Boxxy_AGM`

### Phase 6: Iteration & Polish (1 hour)
- [ ] Test compilation after each proof segment
- [ ] Fix any tactic failures
- [ ] Expected failures: `apply` doesn't work → use `calc` or `have`
- [ ] If stuck: compare with GROVE_PROOF_STRATEGY.md templates
- [ ] Final test: No errors, no sorrys in this lemma

---

## Common Pitfalls & How to Avoid Them

### Pitfall 1: "apply failed"
**Symptom**: `apply (tactic)` fails with "goal is not of required form"
**Fix**: Use `have` instead to set up intermediate result first
```isabelle
have h_key: "property" by (...)
apply (tactic)  -- Now it knows about h_key
```

### Pitfall 2: "Missing lemma"
**Symptom**: Need a lemma that doesn't exist
**Fix**:
1. Check if it's already proven in Grove_Spheres.thy or AGM_Extensions.thy
2. If not, you may need to add it as a helper lemma first
3. Common helpers you might need:
   - `admissible_subset_grove` (what you're trying to prove!)
   - `grove_subset_admissible` (what you're trying to prove!)
   - Sphere monotonicity lemmas

### Pitfall 3: "Can't unify types"
**Symptom**: `belief_set` vs `'a set` mismatch
**Fix**: Check the definitions in AGM_Base.thy
- `belief_set K` means `K = Cn K` (K is closed under consequence)
- `admissible_results K p` returns `'a set set` (sets of sentences)

### Pitfall 4: "Lost in the proof"
**Symptom**: You've written 50 lines and still don't see the path to contradiction
**Fix**:
1. Step back and review GROVE_PROOF_STRATEGY.md "Pseudo-Code" section
2. Your proof should follow that structure exactly
3. If not, restart: your approach is probably too complicated
4. Grove's proof is elegant - if you're struggling, you're likely overcomplicating

---

## Testing Strategy

### After Each Phase
```bash
cd /Users/bob/i/boxxy/theories
isabelle build Boxxy_AGM 2>&1 | grep -A5 "grove_revision_is_unique_admissible"
```

### Expected Progression
- **Phase 1**: `sorry` still there, code compiles
- **Phase 2**: `sorry` partially filled, code compiles
- **Phase 3**: More substantial proof, might have red squiggles, code compiles
- **Phase 4**: Proof nearly complete, code compiles
- **Phase 5**: Proof complete, `sorry` removed, code compiles
- **Phase 6**: Full build succeeds: `Finished Boxxy_AGM`

### Final Verification
```bash
isabelle build -b Boxxy_AGM
grep -n "sorry" theories/Grove_Spheres.thy  # Should be empty or only 3 remaining (other sorrys)
```

---

## Critical Resources

### Must Read (Required)
1. GROVE_PROOF_STRATEGY.md - Complete implementation guide
2. Grove (1988) paper Section 3 - The original theorem
3. AGM_Base.thy - Definition of admissible_results

### Reference During Proof
1. PROOF_MAP.md "Phase 3" section - What lemmas are available
2. Isabelle/HOL documentation on `by_contra` and `False.elim`
3. AGM_Extensions.thy - Similar proof patterns

### If You Get Stuck
1. IMPLEMENTATION_GUIDE.md "Part 6: Troubleshooting"
2. Re-read GROVE_PROOF_STRATEGY.md "Template for Filling In"
3. Check if helper lemmas need to be proven first
4. Consider stepping back to a simpler sorry (medium priority) first

---

## Expected Proof Size

**Your completed proof should be**: 200-300 lines of Isabelle

```isabelle
lemma grove_revision_is_unique_admissible:
  assumes is_total: "is_total"
      and K1_admissible: "K1 ∈ admissible_results K p"
      and K2_admissible: "K2 ∈ admissible_results K p"
  shows "K1 = K2"
proof (rule ccontr)
  assume h: "K1 ≠ K2"

  (* Phase 1: Identify difference ~20 lines *)
  obtain s where hs: "s ∈ K1 ∧ s ∉ K2" by (...)

  (* Phase 2: Use admissibility ~40 lines *)
  have h_K1_belief_set: "belief_set K1" by (...)
  have h_K1_includes_p: "p ∈ K1" by (...)
  have h_K1_inclusion: "K1 ⊆ K ⊕ p" by (...)

  have h_K2_belief_set: "belief_set K2" by (...)
  have h_K2_includes_p: "p ∈ K2" by (...)
  have h_K2_inclusion: "K2 ⊆ K ⊕ p" by (...)

  (* Phase 3: Apply totality ~60 lines *)
  have h_total: "is_total" by fact
  have h_lin_order: "∀x y. x ≺ y ∨ y ≺ x ∨ x = y" by (...)

  (* Here's where you use totality to show K2 is impossible *)
  have h_contradiction: "False" by (...)

  (* Phases 4-5: Complete contradiction *)
  exact h_contradiction
qed
```

The proof won't be exactly this size, but 200-300 lines is the right ballpark.

---

## Success Indicators

✅ **You've succeeded when**:
1. No more `sorry` in `grove_revision_is_unique_admissible`
2. `isabelle build Boxxy_AGM` produces "Finished Boxxy_AGM" with exit code 0
3. You understand why totality forces uniqueness
4. The other admissible subset lemmas can now be proven (they're easier)

---

## Next After This Sorry

Once `grove_revision_is_unique_admissible` is proven:
1. Prove `admissible_subset_grove` (1-2 hours) - much easier
2. Prove `grove_subset_admissible` (1-2 hours) - similar difficulty
3. Main theorem is automatically proven via `uniqueness_via_grove_spheres`
4. All 8 sorrys eliminated

**Timeline**: Day 1-2 (easy) → Day 3-4 (this sorry) → Day 5 (medium) → Done!

---

**Good luck! This is the hardest but most rewarding part of the formalization.**

💪 Remember: You have Grove's original proof, detailed implementation guides, and a complete proof framework. You're not inventing from scratch—you're formalizing something that's been mathematically proven since 1988.

The hard work is translating mathematical intuition to Isabelle syntax, not inventing the proof itself.

