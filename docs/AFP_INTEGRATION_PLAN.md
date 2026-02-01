# AFP Belief_Revision Integration Plan

## Goal

Layer boxxy's extensions (indeterministic revision, selection functions, GF(3), nashator) **on top of** the AFP's proven AGM foundations.

---

## Phase 1: Setup AFP Session Import

### 1.1 Install AFP Locally

```bash
# Download AFP
curl -LO https://www.isa-afp.org/release/afp-current.tar.gz
tar xzf afp-current.tar.gz

# Register with Isabelle
isabelle components -u /path/to/afp-2024-xx-xx/thys
```

### 1.2 Update ROOT to Import AFP

```isabelle
session Boxxy_AGM = HOL +
  description \<open>AGM Belief Revision Extensions\<close>
  options [document = false, quick_and_dirty = true]
  sessions
    "HOL-Library"
    "AFP-Belief_Revision"  (* Add AFP session *)
  theories
    AGM_Extensions
    OpticClass
    SemiReliable_Nashator
    Vibesnipe
```

---

## Phase 2: Refactor AGM_Extensions for AFP Compatibility

### 2.1 Remove Duplicate Definitions

The AFP already provides:
- `Tarskian_logic` locale with `Cn`
- `Supraclassical_logic` with connectives
- `AGM_Revision` and `AGM_Contraction` locales
- Harper/Levi identities

**Action**: Remove duplicates from `AGM_Base.thy`, keep only:
- GF(3) trit definitions
- Selection function types
- Indeterministic entrenchment locale

### 2.2 Create Bridging Locale

```isabelle
theory Boxxy_AGM_Bridge
  imports "AFP-Belief_Revision.AGM_Revision" AGM_Extensions
begin

(* Extend AFP's proven AGM with partial entrenchment *)
locale agm_with_partial_entrenchment = 
  AGM_Revision +                              (* AFP's functional revision *)
  partial_entrenchment ent_rel                (* Our partial order *)
  for ent_rel :: "'a \<Rightarrow> 'a \<Rightarrow> bool"
begin

(* When entrenchment has incomparabilities, revision becomes set-valued *)
definition indet_revision :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set set" where
  "indet_revision K p = {K *\<^sub>e p | e. e linearizes ent_rel}"

(* Key theorem: when entrenchment is total, collapse to AFP's functional revision *)
theorem total_implies_deterministic:
  assumes "\<forall>p q. comparable p q"
  shows "\<exists>!K'. K' \<in> indet_revision K p"
  sorry (* Requires proof that AFP's axioms + totality → uniqueness *)

end
end
```

---

## Phase 3: Connect Selection Functions to AFP

### 3.1 Selection as Determinization

```isabelle
locale agm_with_selection = 
  agm_with_partial_entrenchment +
  fixes \<sigma> :: "'a set selection_fn"
  assumes valid_sel: "valid_selection \<sigma>"
begin

(* Determinize set-valued revision *)
definition sel_revision :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set" where
  "sel_revision K p = \<sigma> (indet_revision K p)"

(* This satisfies AFP's AGM postulates when σ picks "good" revisions *)
lemma sel_revision_satisfies_agm:
  assumes "indet_revision K p \<noteq> {}"
      and "\<forall>K' \<in> indet_revision K p. satisfies_agm_postulates K'"
  shows "satisfies_agm_postulates (sel_revision K p)"
  using assms valid_sel
  unfolding sel_revision_def valid_selection_def
  by auto

end
```

### 3.2 Two-Stage Revision (per Olsson's Analysis)

```isabelle
(* Stage 1: Compute admissible revisions (relational) *)
(* Stage 2: Take intersection (tie-breaking rule) *)

definition conservative_revision :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set" where
  "conservative_revision K p = \<Inter> (indet_revision K p)"

(* This is the "rational" choice when beliefs tie for optimality *)
```

---

## Phase 4: Integrate Hedges Game Theory

### 4.1 Multi-Agent Belief Revision as Open Game

```isabelle
(* Each agent has their own entrenchment → selection function *)
record revision_agent =
  agent_ent :: "'a \<Rightarrow> 'a \<Rightarrow> bool"  (* Entrenchment *)
  agent_sel :: "'a set selection_fn"  (* How they choose *)

(* Nash equilibrium: neither agent wants to change given the other's revision *)
definition belief_nash_eq :: 
  "revision_agent \<Rightarrow> revision_agent \<Rightarrow> 'a set \<Rightarrow> 'a \<Rightarrow> 
   'a set \<Rightarrow> 'a set \<Rightarrow> bool" where
  "belief_nash_eq a1 a2 K p K1 K2 \<longleftrightarrow>
   K1 = determinize_revision (agent_sel a1) K p \<and>
   K2 = determinize_revision (agent_sel a2) K p \<and>
   (* Neither wants to deviate given the other's choice *)
   (\<forall>K1'. K1' \<in> indet_revision K p \<longrightarrow> 
          utility a1 K1 K2 \<ge> utility a1 K1' K2) \<and>
   (\<forall>K2'. K2' \<in> indet_revision K p \<longrightarrow> 
          utility a2 K1 K2 \<ge> utility a2 K1 K2')"
```

### 4.2 Connect to OpticClass

The `para_lens` structure in OpticClass.thy models:
- `para_get`: Forward pass (play / belief state observation)
- `para_put`: Backward pass (coplay / utility propagation)

```isabelle
(* Belief revision as parametric lens *)
definition revision_lens :: 
  "('a set, 'a set, 'a set, 'a, 'a set, utility) para_lens" where
  "revision_lens = \<lparr>
    para_get = \<lambda>K p. determinize_revision \<sigma> K p,
    para_put = \<lambda>K p u. (utility_feedback u, intrinsic_reward K p)
  \<rparr>"
```

---

## Phase 5: GF(3) Conservation Across Stack

### 5.1 Trit Assignment Rationale

| Layer | Operation | Trit | Justification |
|-------|-----------|------|---------------|
| AGM | Expansion (+) | +1 | Adding information |
| AGM | Contraction (÷) | -1 | Removing information |
| AGM | Revision (*) | 0 | Balanced (Levi = ÷ then +) |
| Optics | get/play | +1 | Forward flow |
| Optics | put/coplay | -1 | Backward flow |
| Optics | compose | 0 | Neutral |
| Games | Selection | +1 | Agent choosing |
| Games | Nash product | 0 | Equilibrium |
| Games | Verification | -1 | Checking |

### 5.2 Conservation Theorem

```isabelle
theorem full_stack_gf3_conserved:
  assumes "tagged_balanced agm_ops"
      and "tagged_balanced optic_ops"  
      and "tagged_balanced game_ops"
  shows "gf3_balanced (map op_trit (agm_ops @ optic_ops @ game_ops))"
  (* Sum of all trits ≡ 0 mod 3 *)
```

---

## Phase 6: Verification Goals

### Must Prove (to match AFP quality)

1. **Determinization correctness**: Selection from admissible set preserves AGM postulates
2. **Collapse theorem**: Total entrenchment → AFP's functional revision
3. **Levi/Harper with selection**: Identities still hold when determinizing
4. **Nash existence**: Under suitable conditions, belief Nash equilibria exist

### May Use `sorry` (research frontier)

1. Semi-reliable ε-approximate equilibria bounds
2. Full nashator coherence (requires proper category theory setup)
3. Detailed utility function specifications

---

## File Structure After Integration

```
theories/
├── AGM_Extensions.thy       (* GF(3), partial entrenchment, selection fns *)
├── Boxxy_AGM_Bridge.thy     (* NEW: Connects to AFP *)
├── OpticClass.thy           (* Lenses, parametric lenses *)
├── SemiReliable_Nashator.thy (* Nash product, ε-approximate *)
├── Vibesnipe.thy            (* Compositional belief revision game *)
└── ROOT                     (* Updated to import AFP *)

docs/
├── CANONICAL_RESEARCH.md    (* Research bibliography *)
└── AFP_INTEGRATION_PLAN.md  (* This file *)
```

---

## Next Steps

1. **Install Isabelle + AFP** on development machine
2. **Create Boxxy_AGM_Bridge.thy** with sublocale interpretations
3. **Remove duplicates** from AGM_Base.thy (or deprecate entirely)
4. **Verify build** with AFP imported
5. **Prove collapse theorem** (total entrenchment → functional)
6. **Document novel GF(3) contribution** for potential publication
