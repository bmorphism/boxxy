# AGM Postulate Mapping (What We Follow)

This document annotates which parts of the Isabelle development explicitly follow AGM postulates, and where those postulates are defined or referenced.

## Source of AGM Postulates
All eight AGM revision postulates are defined in `theories/AGM_Base.thy` inside the `agm_revision` locale:

- K*1 (Closure): `agm_closure`
- K*2 (Success): `agm_success`
- K*3 (Inclusion): `agm_inclusion`
- K*4 (Vacuity): `agm_vacuity`
- K*5 (Consistency): `agm_consistency`
- K*6 (Extensionality): `agm_extensionality`
- K*7 (Superexpansion): `agm_superexpansion`
- K*8 (Subexpansion): `agm_subexpansion`

The bundle predicate `satisfies_agm` is defined as the conjunction of K*1–K*8 in the same file.

## Where AGM Is Followed (By File)

### `theories/AGM_Base.thy`
- Defines the AGM postulates and the `agm_revision` locale.
- Defines `admissible_results` in the indeterministic setting using AGM-style conditions (closure, success, inclusion) as core constraints.

### `theories/Grove_Spheres.thy`
- Establishes uniqueness of revision under total entrenchment (Grove-style spheres).
- Proof strategy follows AGM constraints, especially **K*3–K*5** (inclusion, vacuity, consistency) to show admissibility minimality and uniqueness.

### `theories/AGM_Extensions.thy`
- Extends the AGM base with partial entrenchment and selection functions.
- Integrates AGM constraints via the determinization step (selection from admissible results), preserving AGM behavior.

### `theories/Boxxy_AGM_Bridge.thy`
- Bridge layer that documents the connection to the AFP `AGM_Revision` locale (when available) and explicitly states the AGM postulates are inherited.

### `theories/OpticClass.thy`, `theories/SemiReliable_Nashator.thy`, `theories/Vibesnipe.thy`
- Use AGM as a semantic baseline for game-theoretic or approximate revision (commentary-level references; does not redefine postulates).

## AGM Postulate Usage Notes

From `docs/GROVE_PROOF_STRATEGY.md`:
- **K*3 (Inclusion)**: `K * p ⊆ K ⊕ p` used to derive minimality of admissible results.
- **K*4 (Vacuity)**: If `¬p ∉ K`, then expansion is included in revision.
- **K*5 (Consistency)**: Ensures admissible results are consistent when `p` is not contradictory.

These are the key AGM postulates that drive the totality ⇒ uniqueness argument in the Grove spheres proof chain.

## Quick Pointers
- Formal postulate definitions: `theories/AGM_Base.thy`
- Uniqueness under total entrenchment: `theories/Grove_Spheres.thy`
- Determinization (selection functions): `theories/AGM_Extensions.thy`
- Bridge to AFP AGM locale: `theories/Boxxy_AGM_Bridge.thy`

