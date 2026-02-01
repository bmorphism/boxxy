# Sorry Statement Audit

This document categorizes all `sorry` statements in the Boxxy AGM formalization.

## Summary

| File | Count | Provable | Research Frontier |
|------|-------|----------|-------------------|
| Boxxy_AGM_Bridge.thy | 1 | ⚠️ Hard | ✓ |
| SemiReliable_Nashator.thy | 2 | ⚠️ Hard | ✓ |
| Vibesnipe.thy | 1 | ✗ | ✓ |
| **Total** | **4** | | |

---

## Detailed Analysis

### 1. `Boxxy_AGM_Bridge.thy:95` — `total_implies_unique`

```isabelle
lemma total_implies_unique:
  assumes "is_total"
  shows "∃K'. admissible_results K p = {K'} ∨ admissible_results K p = {}"
  sorry ⟨Requires Grove sphere nesting argument⟩
```

**Category**: Research frontier (provable with more infrastructure)

**Why it's hard**:
- Requires formalizing Grove's system of spheres (nested sets of possible worlds)
- Need to show: total entrenchment → nested spheres → unique minimal sphere intersecting A
- This is proven in Grove (1988) "Two modellings for theory change"

**Proof sketch**:
1. Total entrenchment induces a linear order on fallback theories
2. Fallback theories form nested spheres around the current belief set
3. For any consistent input p, exactly one sphere is the smallest that intersects p
4. This sphere determines the unique revision result

**Effort estimate**: 2-3 days with proper sphere formalization

---

### 2. `SemiReliable_Nashator.thy:79` — `semi_reliable_approx`

```isabelle
lemma semi_reliable_approx:
  assumes "(xy, k) ∈ semi_reliable_nashator ε argmax_rel argmax_rel"
  shows "epsilon_nash_eq (2 * ε) xy k"
  sorry ⟨Proof requires more infrastructure⟩
```

**Category**: Provable with effort

**Why it's hard**:
- Need to unfold `semi_reliable_nashator` definition
- Track ε through both players' selections
- Show that ε-slack in each player compounds to 2ε total

**Proof sketch**:
1. By definition, `semi_reliable ε σ` means within ε of some element in σ
2. Nash product preserves this: each player is within ε of their argmax
3. Combined: each player's deviation costs at most ε, so total slack is 2ε

**Effort estimate**: 1 day

---

### 3. `SemiReliable_Nashator.thy:94` — `nashator_assoc_exists`

```isabelle
lemma nashator_assoc_exists:
  fixes ε1 :: "('a, 'r) selection_rel"
    and ε2 :: "('b, 's) selection_rel"
    and ε3 :: "('c, 't) selection_rel"
  shows "∃f. bij f"
  sorry ⟨Coherence proof - requires proper category theory setup⟩
```

**Category**: Research frontier (needs category theory library)

**Why it's hard**:
- This is a coherence theorem for lax monoidal functors
- Needs: `(ε1 ⊠ ε2) ⊠ ε3 ≅ ε1 ⊠ (ε2 ⊠ ε3)` via coherent isomorphism
- Standard in category theory but requires proper categorical setup

**Proof approach**:
- Import a category theory library (e.g., Category3 from AFP)
- Define selection as a functor
- Show nashator satisfies laxator axioms
- Apply general coherence theorem

**Effort estimate**: 1 week (mostly library integration)

**Alternative**: Prove directly for concrete cases without full categorical machinery

---

### 4. `Vibesnipe.thy:119` — `vibesnipe_equilibrium`

```isabelle
theorem vibesnipe_equilibrium:
  assumes "epsilon v1 = ε"
      and "epsilon v2 = ε"
      and "gf3_balanced [trit v1, trit v2, Zero]"
  shows "∃r1 r2. revision_nash_eq ⟨...⟩ ⟨...⟩ K p r1 r2"
  sorry
```

**Category**: Research frontier (main theorem, requires everything else)

**Why it's hard**:
- This is the main synthesis theorem
- Requires: sphere theory + selection determinization + Nash product correctness
- The `undefined` fields in the record indicate incomplete specification

**Proof dependencies**:
1. ✗ `total_implies_unique` (sorry)
2. ✗ `semi_reliable_approx` (sorry)
3. ✓ `selected_is_admissible` (proven)
4. ✓ `nash_product_is_nash_eq` (proven)

**Effort estimate**: After other sorrys are resolved, 2-3 days

---

## Priority Order for Proof Completion

### Phase 1: Low-hanging fruit (1-2 days)
1. `semi_reliable_approx` — straightforward unfolding

### Phase 2: Core theory (1 week)
2. `total_implies_unique` — requires Grove spheres

### Phase 3: Category theory (1-2 weeks)
3. `nashator_assoc_exists` — requires categorical infrastructure

### Phase 4: Main theorem (after Phase 1-3)
4. `vibesnipe_equilibrium` — assembles everything

---

## Proven Lemmas (no sorry)

These lemmas are complete and can be trusted:

| Lemma | File | Significance |
|-------|------|--------------|
| `gf3_triple_balanced` | AGM_Extensions | GF(3) conservation |
| `trit_add_comm` | AGM_Extensions | Commutativity |
| `trit_add_assoc` | AGM_Extensions | Associativity |
| `determinize_mem` | AGM_Extensions | Selection correctness |
| `nash_product_is_nash_eq` | SemiReliable_Nashator | Nash characterization |
| `lens_id_lawful` | OpticClass | Lens laws |
| `optic_gf3_conserved` | OpticClass | Optic GF(3) |
| `selected_is_admissible` | Boxxy_AGM_Bridge | Selection correctness |
| `selected_satisfies_K2` | Boxxy_AGM_Bridge | AGM success postulate |
| `conservative_satisfies_K2` | Boxxy_AGM_Bridge | Conservative revision |
| `identity_bridge_balanced` | Vibesnipe | Levi/Harper GF(3) |

---

## Recommendations

1. **For publication**: Focus on Phase 1-2, leave Phase 3-4 as "future work"
2. **For AFP submission**: All sorrys must be resolved
3. **For practical use**: Current state is sufficient for experimentation
