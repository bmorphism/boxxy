# Canonical Research Foundations for Boxxy AGM Extensions

## Overview

This document maps the boxxy Isabelle formalization to established academic literature, providing the theoretical grounding for integration with the AFP Belief_Revision formalization.

---

## 1. Lindström-Rabinowicz Indeterministic Belief Revision

### Core Papers

1. **Lindström, S., and Rabinowicz, W. (1989)** "On probabilistic representation of non-probabilistic belief revision"  
   *Journal of Philosophical Logic*, 18: 69-101.  
   - Introduces the **non-uniqueness problem**: multiple probability functions can have the same belief set as their "top"
   - First proposal for **relational** (vs functional) belief revision

2. **Lindström, S., and Rabinowicz, W. (1991)** "Epistemic entrenchment with incomparabilities and relational belief revision"  
   In *The Logic of Theory Change*, Fuhrmann & Morreau (eds.): 93-126.  
   - **Key insight**: Non-connected (partial) entrenchment orderings yield indeterministic revision
   - Grove's spheres become "ellipses" when beliefs are incomparable
   - Multiple admissible revision results when `p ⊀ q ∧ q ⊀ p`

3. **Rabinowicz, W. and Lindström, S. (1994)** "How to model relational belief revision"  
   In *Logic and Philosophy of Science in Uppsala*, Kluwer: 69-84.

4. **Olsson, E. J. (2007)** "Lindström and Rabinowicz on relational belief revision"  
   Festschrift paper providing philosophical analysis.  
   - Argues relational revision is valid for **first stage** of revision process
   - Second stage uses tie-breaking rule → intersection of admissible states
   - **Connection to your formalization**: This two-stage model maps to `indet_revision` + `selection_fn`

### Mapping to Boxxy Theories

| L&R Concept | Boxxy Formalization | File |
|-------------|---------------------|------|
| Partial entrenchment | `partial_entrenchment` locale | AGM_Extensions.thy |
| Incomparable beliefs | `incomparable p q` definition | AGM_Extensions.thy |
| System of ellipses | `incomparable_pairs` function | AGM_Extensions.thy |
| Relational revision | `admissible_revisions` | AGM_Extensions.thy |
| Determinization | `determinized_revision` | AGM_Extensions.thy |

---

## 2. Hedges Compositional Game Theory & Selection Functions

### Core Papers

1. **Ghani, N., Hedges, J., Winschel, V., Zahn, P. (2018)** "Compositional game theory"  
   *Proceedings of LiCS 2018*. [arXiv:1603.04641](https://arxiv.org/abs/1603.04641)  
   - Introduces **open games** as morphisms in a symmetric monoidal category
   - **Coutility** concept: utility returned to environment
   - Sequential composition via categorical composition
   - Simultaneous moves via monoidal product

2. **Capucci, M., Gavranović, B., Hedges, J., Rischel, E.F. (2021)** "Towards Foundations of Categorical Cybernetics"  
   [arXiv:2105.06332](https://arxiv.org/abs/2105.06332)  
   - **Selection functions** as the core abstraction
   - **Lax monoidal structure** for compositionality
   - Connection to parametric lenses

3. **Capucci, M. (2022)** "Diegetic Representation of Feedback in Open Games"  
   *EPTCS 380*. [arXiv:2206.12338](https://arxiv.org/abs/2206.12338)  
   - **Diegetic** vs extra-diegetic game analysis
   - Nash product (**nashator**) decomposes into elementary parts
   - Formal analogy with **backpropagation**
   - Selection functions as "reparameterisation"

4. **Hedges, J. (2017)** "Coherence for lenses and open games"  
   [arXiv:1704.02230](https://arxiv.org/abs/1704.02230)

5. **Bolt, J., Hedges, J., Zahn, P. (2019)** "Sequential games and nondeterministic selection functions"  
   [arXiv:1811.06810](https://arxiv.org/abs/1811.06810)  
   - **Nondeterministic selection** over powerset monad
   - Connection to iterated removal of dominated strategies

### Mapping to Boxxy Theories

| Hedges Concept | Boxxy Formalization | File |
|----------------|---------------------|------|
| Selection function | `'a selection_fn` type | AGM_Base.thy, AGM_Extensions.thy |
| Valid selection | `valid_selection σ` | AGM_Extensions.thy |
| Nash product (⊠) | `nash_product` (⊠) | SemiReliable_Nashator.thy |
| Argmax selection | `argmax_rel` | SemiReliable_Nashator.thy |
| ε-approximate | `argmax_approx`, `semi_reliable` | SemiReliable_Nashator.thy |
| Parametric lens | `para_lens` record | OpticClass.thy |
| Lens parallel (&&&&) | `lens_parallel` (∥) | OpticClass.thy |
| Lens sequential (>>>>) | `lens_compose` (⋙) | OpticClass.thy |
| Galois connection | `galois_unit`, `galois_counit` | OpticClass.thy |

---

## 3. AFP Belief_Revision Integration

### AFP Entry

**Fouillard, V., Taha, S., Boulanger, F., Sabouret, N. (2021)** "Belief Revision Theory"  
[Archive of Formal Proofs](https://www.isa-afp.org/entries/Belief_Revision.html)

### Structure

```
AFP/Belief_Revision/
├── AGM_Logic.thy      -- Tarskian, Supraclassical, Compact logics
├── AGM_Remainder.thy  -- Remainder sets
├── AGM_Contraction.thy -- K*1-K*6 contraction postulates
└── AGM_Revision.thy   -- K*1-K*8 revision postulates + Harper/Levi identities
```

### Key Locales

- `Tarskian_logic`: Cn operator with monotonicity, inclusion, transitivity
- `Supraclassical_logic`: Adds propositional connectives (deep embedding)
- `AGM_Contraction`: 6 contraction postulates
- `AGM_Revision`: 8 revision postulates
- Uses sublocale to derive contraction ↔ revision equivalence via Harper/Levi

### Integration Strategy

```isabelle
theory Boxxy_AGM_Integration
  imports 
    "Belief_Revision.AGM_Revision"  (* AFP *)
    AGM_Extensions                   (* Boxxy *)
begin

(* Extend AFP's AGM_Revision with partial entrenchment *)
locale indet_agm_revision = 
  AGM_Revision +                     (* Inherit AFP proofs *)
  partial_entrenchment +             (* Add incomparability *)
  fixes selection :: "'a set set ⇒ 'a set"
  assumes valid_sel: "valid_selection selection"
begin

(* When entrenchment is total, collapse to AFP's functional revision *)
lemma total_entrenchment_functional:
  assumes "is_total"
  shows "∃!K'. (K, p, K') ∈ revision_rel"
  using assms AFP_AGM_theorems...

end
```

---

## 4. The Bridge: Selection Functions Connect Both Domains

### Key Insight

Both Lindström-Rabinowicz and Hedges use **selection functions** to determinize indeterminacy:

| Domain | Indeterminacy Source | Selection Function Role |
|--------|---------------------|------------------------|
| Belief Revision | Incomparable entrenchment | Pick one admissible revision |
| Game Theory | Multiple Nash equilibria | Pick one equilibrium |

### Formal Connection

```
L&R Relational Revision          Hedges Open Games
        ↓                               ↓
  admissible_revisions           nash_product(σ₁, σ₂)
        ↓                               ↓
   Set of outcomes              Set of equilibria
        ↓                               ↓
    selection_fn σ               selection_fn σ  
        ↓                               ↓
  Single revision              Single equilibrium
```

### Your `vibesnipe` Concept

The `vibesnipe` record in Vibesnipe.thy captures exactly this bridge:
- `spheres`: Grove-style fallback structure (from AGM)
- `epsilon`: Slack parameter (from semi-reliable nashator)
- Selection from indeterministic outcomes

---

## 5. GF(3) Trit Conservation: Novel Contribution

This appears to be a **novel integration** not present in the canonical literature. Your contribution:

1. **Trit tagging** of operations (+1, 0, -1) 
2. **Conservation law**: `gf3_balanced` ensures sum ≡ 0 (mod 3)
3. **Semantic assignment**:
   - `Plus (+1)`: Generative operations (selection, get/play)
   - `Zero (0)`: Neutral operations (compose, nash equilibrium)
   - `Minus (-1)`: Verificative operations (put/coplay, check)

This mirrors:
- Physics: CPT conservation
- Category theory: Adjunction unit/counit balance
- Game theory: Play/coplay bidirectionality

---

## 6. Recommended Reading Order

### For AGM Background
1. Stanford Encyclopedia: [Logic of Belief Revision](https://plato.stanford.edu/entries/logic-belief-revision/)
2. Fermé & Hansson (2011): "AGM 25 Years" survey paper
3. AFP Belief_Revision proof document

### For Indeterministic Extension
4. Lindström & Rabinowicz (1991) on entrenchment incomparabilities
5. Levi (2004) *Mild Contraction* for value conflict explanation
6. Rott (1992) on generalized epistemic entrenchment

### For Game-Theoretic Bridge
7. Ghani et al. (2018) "Compositional game theory"
8. Capucci (2022) "Diegetic Representation"
9. Hedges (2017) "Coherence for lenses and open games"

---

## 7. Bibliography

```bibtex
@article{AGM1985,
  author = {Alchourrón, Carlos E. and Gärdenfors, Peter and Makinson, David},
  title = {On the Logic of Theory Change: Partial Meet Contraction and Revision Functions},
  journal = {Journal of Symbolic Logic},
  volume = {50},
  pages = {510--530},
  year = {1985}
}

@incollection{LindstromRabinowicz1991,
  author = {Lindström, Sten and Rabinowicz, Wlodek},
  title = {Epistemic entrenchment with incomparabilities and relational belief revision},
  booktitle = {The Logic of Theory Change},
  editor = {Fuhrmann, A. and Morreau, M.},
  pages = {93--126},
  year = {1991}
}

@inproceedings{GhaniHedges2018,
  author = {Ghani, Neil and Hedges, Jules and Winschel, Viktor and Zahn, Philipp},
  title = {Compositional game theory},
  booktitle = {Proceedings of LiCS 2018},
  year = {2018}
}

@article{Capucci2022,
  author = {Capucci, Matteo},
  title = {Diegetic Representation of Feedback in Open Games},
  journal = {EPTCS},
  volume = {380},
  pages = {145--158},
  year = {2022}
}

@article{AFP_Belief_Revision,
  author = {Fouillard, Valentin and Taha, Safouan and Boulanger, Frédéric and Sabouret, Nicolas},
  title = {Belief Revision Theory},
  journal = {Archive of Formal Proofs},
  year = {2021},
  note = {\url{https://isa-afp.org/entries/Belief_Revision.html}}
}
```
