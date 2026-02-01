# Structural Progression: Boxxy AGM Formalization

## Key Events (Structural, Not Temporal)

### Layer 0: Type Foundation
```
sentence = string
belief_set = sentence set
trit = Minus | Zero | Plus
```
**Event**: Type unification failure in `determinized_revision`
**Resolution**: Selection function domain corrected from `'a` to `'a set`

### Layer 1: Algebraic Structure (GF(3))
```isabelle
trit_add : trit → trit → trit    (* closed binary operation *)
trit_neg : trit → trit           (* inverse *)
Zero                              (* identity *)
```
**Proven**: Commutativity, associativity, identity, inverse laws → **Abelian group**

### Layer 2: Order Structure (Entrenchment)
```
epistemic_entrenchment: total order → deterministic revision
partial_entrenchment: preorder → indeterministic revision (L&R)
```
**Key locale hierarchy**:
```
partial_entrenchment ⊂ indet_revision ⊂ relational_agm_revision
```

### Layer 3: Selection/Determinization
```
valid_selection σ ⟺ ∀S. S ≠ {} → σ S ∈ S
determinize: (A → B → C set) → C selection_fn → A → B → C
```
**Bridge**: `selection_agm_revision` extends `relational_agm_revision` with σ

### Layer 4: Game-Theoretic Composition
```
nash_product (⊠): selection_rel → selection_rel → selection_rel
semi_reliable ε: allows ε-slack in optimization
```
**Proven**: `nash_product_is_nash_eq` characterizes Nash equilibria

### Layer 5: Optic/Lens Structure
```
lens: get (forward) + put (backward)
para_lens: parameterized version for game arenas
∥ (parallel) = nashator
⋙ (sequential) = composition
```
**Proven**: `lens_id_lawful`, `optic_gf3_conserved`

### Layer 6: Conservation Law
```
gf3_balanced ops ⟺ Σ(trit_val ∘ op_trit) ≡ 0 (mod 3)
```
**Trit assignments**:
- Plus (+1): expansion, selection, get/play
- Zero (0): revision, compose, equilibrium
- Minus (-1): contraction, verification, put/coplay

---

## Dependency DAG (Not Linear)

```
                    Main.thy
                       │
         ┌─────────────┼─────────────┐
         ▼             ▼             ▼
    HOL-Library    AGM_Base    (AFP_Belief_Revision)
         │             │             │
         └──────┬──────┴─────────────┘
                ▼
         AGM_Extensions
         ├── trit datatype + group laws
         ├── partial_entrenchment locale
         └── indet_revision locale
                │
         ┌──────┴──────┐
         ▼             ▼
    OpticClass   Boxxy_AGM_Bridge
         │       ├── functional_agm_revision
         │       ├── relational_agm_revision
         │       └── selection_agm_revision
         │             │
         └──────┬──────┘
                ▼
       SemiReliable_Nashator
       ├── nash_product (⊠)
       └── semi_reliable ε
                │
                ▼
           Vibesnipe
           └── vibesnipe_equilibrium [sorry]
```

---

## What Maximum Success Looks Like

### 1. Zero `sorry` Statements

| Current Sorry | Resolution Path |
|---------------|-----------------|
| `total_implies_unique` | Formalize Grove spheres as locale; prove sphere nesting → uniqueness |
| `semi_reliable_approx` | Instantiate `ordered_ab_group_add` for ε arithmetic; unfold nash_product |
| `nashator_assoc_exists` | Import AFP's Category3 or define monoidal coherence directly |
| `vibesnipe_equilibrium` | Compose above three + `selected_is_admissible` |

### 2. AFP Sublocale Chain

```isabelle
sublocale indet_agm_revision ⊆ AGM_Revision  (* inherit AFP theorems *)
sublocale selection_agm_revision ⊆ indet_agm_revision
sublocale vibesnipe ⊆ selection_agm_revision
```

### 3. Representation Theorem

```isabelle
theorem representation:
  "satisfies_agm r ⟺ 
   (∃ent σ. is_total ent ∧ valid_selection σ ∧ 
            r = determinize_revision σ ∘ admissible_revisions)"
```

### 4. Game-Theoretic Completeness

```isabelle
theorem nash_existence:
  assumes "finite (admissible_results K p)"
      and "valid_selection σ₁" and "valid_selection σ₂"
  shows "∃r₁ r₂. revision_nash_eq agent₁ agent₂ K p r₁ r₂"
```

### 5. GF(3) End-to-End Conservation

```isabelle
theorem full_stack_conservation:
  "gf3_balanced (agm_ops @ optic_ops @ game_ops @ verification_ops)"
```

---

## Lean Techniques to Import

### 1. `decide` / `simp` for Finite Cases

Isabelle equivalent: Define `trit` as `datatype` enables `(cases t; simp)`.

**Lean pattern**:
```lean
example : trit_add Minus Minus = Plus := by decide
```

**Isabelle already uses**: `by (cases a; cases b; simp)`

**Improvement**: Create custom simp set `gf3_simps` collecting all trit lemmas:
```isabelle
lemmas gf3_simps = trit_add.simps trit_neg.simps trit_val.simps
                   trit_add_comm trit_add_assoc trit_add_zero_left
```

### 2. `aesop` / `auto` for Locale Obligations

Lean's `aesop` uses goal-directed proof search. Isabelle's `auto` is similar but less directed.

**Improvement**: Use `intro_locales` + `simp add: locale_defs` pattern:
```isabelle
interpretation my_partial_entrenchment: partial_entrenchment my_rel
  by intro_locales (auto simp: my_rel_def)
```

### 3. `field_simp` for Arithmetic

The `semi_reliable_approx` proof requires tracking ε through inequalities.

**Lean pattern**:
```lean
field_simp
ring
```

**Isabelle equivalent**: `algebra` tactic + `ordered_ab_group_add` instances:
```isabelle
lemma semi_reliable_approx:
  fixes ε :: "'a::{ordered_ab_group_add}"
  shows "..."
  by (unfold defs) algebra
```

### 4. Tactic Combinators

Lean's `<;>` applies tactic to all goals. Isabelle's `ALLGOALS` is similar.

**Improvement**: Use method combinators more aggressively:
```isabelle
proof (induction rule: trit.induct; simp add: gf3_simps)
```

### 5. `calc` for Equational Reasoning

Lean's `calc` chains equalities readably.

**Isabelle equivalent**: `also/finally` chains:
```isabelle
have "trit_add (trit_add a b) c = trit_add a (trit_add b c)"
proof -
  have "trit_add (trit_add a b) c = ..." by simp
  also have "... = ..." by simp
  finally show ?thesis .
qed
```

### 6. Instance Search for Typeclass Resolution

Lean's instance search automatically finds `ordered_ab_group_add` for numeric types.

**Isabelle pattern**: Use `where` clause in locale instantiation:
```isabelle
interpretation real_nashator: semi_reliable_nashator 
  where ε = "(ε::real)" and σ = argmax_rel
  by standard (auto simp: ordered_ab_group_add_class.axioms)
```

---

## Next Iteration Checklist

- [ ] Create `gf3_simps` simp set for one-shot trit proofs
- [ ] Instantiate `ordered_ab_group_add` for `semi_reliable_approx`
- [ ] Formalize Grove spheres as locale with nesting property
- [ ] Prove `total_implies_unique` using sphere nesting
- [ ] Import AFP Belief_Revision and create sublocale chain
- [ ] Prove `vibesnipe_equilibrium` by composing components
- [ ] Add `Nitpick` checks to all conjectures before proof attempts
- [ ] Document novel GF(3) conservation for potential AFP submission
