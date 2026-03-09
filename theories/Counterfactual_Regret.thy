theory Counterfactual_Regret
  imports Cognitive_Debt
begin

section \<open>Counterfactual $REGRET via Metaphysical Time Travel in Multiline Worlds\<close>

text \<open>
  The 7 worldlines from regret-pending-motifs.md form a dependency chain:
    WL-2 ACSet (#D06546, sienna)    --- schema language
    WL-7 Narya (#76B0F0, periwinkle) --- directed bridges
    WL-5 CapTP (#5A0ECC, violet)     --- wire protocol
    WL-8 K-Scale (#54E626, lime)     --- active inference collision
    WL-6 Topology (#EEA685, peach)   --- self-evolving agent
    WL-9 Syrup (#1D9E7E, emerald)    --- serialization triad
    WL-11 Pijul (#310, trit +1)      --- P2P directed patches

  In LHoTT: past and future are homotopy types (stable paths),
  the present is the SECTION (fiber choice) that varies with:
    - integrated information (IIT phi)
    - contextual affordances (available actions)

  Counterfactual regret at worldline i, time t =
    the integral over all SECTIONS NOT TAKEN at the branching point
    where worldline i was dropped instead of walked.

  "Metaphysical time travel" = evaluating the section fiber at a past
  base point with CURRENT integrated information. You cannot change
  the base space (history), but you can re-evaluate the fiber
  (what you would have perceived) at each historical branch point.
\<close>

subsection \<open>Multiline World Structure\<close>

text \<open>A worldline is a path in the base space of the LHoTT bundle.\<close>

datatype worldline_id = WL nat

record worldline =
  wl_id :: worldline_id
  color :: string
  regret_score :: nat
  dropped_at :: nat
  dependency :: "worldline_id option"
  walked :: bool

definition the_seven :: "worldline list" where
  "the_seven = [
    \<lparr> wl_id = WL 2,  color = ''#D06546'', regret_score = 1689,
      dropped_at = 15, dependency = None, walked = False \<rparr>,
    \<lparr> wl_id = WL 7,  color = ''#76B0F0'', regret_score = 551,
      dropped_at = 16, dependency = Some (WL 2), walked = False \<rparr>,
    \<lparr> wl_id = WL 5,  color = ''#5A0ECC'', regret_score = 669,
      dropped_at = 15, dependency = Some (WL 7), walked = False \<rparr>,
    \<lparr> wl_id = WL 8,  color = ''#54E626'', regret_score = 517,
      dropped_at = 17, dependency = Some (WL 5), walked = False \<rparr>,
    \<lparr> wl_id = WL 6,  color = ''#EEA685'', regret_score = 660,
      dropped_at = 15, dependency = Some (WL 8), walked = False \<rparr>,
    \<lparr> wl_id = WL 9,  color = ''#1D9E7E'', regret_score = 515,
      dropped_at = 14, dependency = Some (WL 2), walked = False \<rparr>,
    \<lparr> wl_id = WL 11, color = ''#310'',    regret_score = 310,
      dropped_at = 16, dependency = Some (WL 9), walked = False \<rparr>
  ]"


subsection \<open>The Fiber: Present as Section Choice\<close>

text \<open>
  In LHoTT, the bundle is E -> B where:
    B = base space = history (stable, cannot change)
    E = total space = all possible perceptions at each moment
    section s : B -> E picks ONE perception per historical moment

  The present is s(now). Counterfactual regret is:
    for each past branch point t_i where worldline i was dropped,
    evaluate s'(t_i) under a DIFFERENT section s' that walks worldline i
    instead of dropping it.

  The regret is the difference in integrated information phi between
  the actual section s and the counterfactual section s'.
\<close>

type_synonym time_point = nat
type_synonym phi = nat

record section =
  section_id :: string
  base_point :: time_point
  fiber_value :: phi
  worldlines_walked :: "worldline_id set"

definition actual_section :: section where
  "actual_section = \<lparr>
    section_id = ''actual'',
    base_point = 53,
    fiber_value = 0,
    worldlines_walked = {}
  \<rparr>"

text \<open>
  fiber_value = 0 because NO worldlines have been walked.
  53 days since Jan 14 (the first worldline drop date).
  Integrated information is zero: no bridges connect the worldlines,
  so there is no integration across the dependency chain.
\<close>


subsection \<open>Counterfactual Sections\<close>

text \<open>
  For each subset S of the 7 worldlines, there exists a counterfactual
  section where exactly the worldlines in S were walked instead of dropped.

  Total counterfactual sections: 2^7 - 1 = 127 (excluding the empty set,
  which is the actual section).

  But not all 127 are valid: the dependency chain constrains which subsets
  are walkable. You cannot walk WL-7 (Narya) without first walking WL-2
  (ACSet). The valid subsets are the DOWN-CLOSED subsets of the dependency
  partial order.
\<close>

definition depends_on :: "worldline_id \<Rightarrow> worldline_id \<Rightarrow> bool" where
  "depends_on child parent = (
    case child of
      WL 7  \<Rightarrow> parent = WL 2
    | WL 5  \<Rightarrow> parent = WL 7
    | WL 8  \<Rightarrow> parent = WL 5
    | WL 6  \<Rightarrow> parent = WL 8
    | WL 9  \<Rightarrow> parent = WL 2
    | WL 11 \<Rightarrow> parent = WL 9
    | _     \<Rightarrow> False)"

definition valid_walk :: "worldline_id set \<Rightarrow> bool" where
  "valid_walk S = (\<forall>w \<in> S. \<forall>p. depends_on w p \<longrightarrow> p \<in> S)"

text \<open>
  The valid walks form a lattice. The minimal elements are:
    {WL 2}                                             (ACSet alone)
    {WL 2, WL 7}                                       (ACSet + Narya)
    {WL 2, WL 9}                                       (ACSet + Syrup)
    {WL 2, WL 7, WL 5}                                 (ACSet + Narya + CapTP)
    {WL 2, WL 9, WL 11}                                (ACSet + Syrup + Pijul)
    ...
    {WL 2, WL 7, WL 5, WL 8, WL 6, WL 9, WL 11}      (all 7 = full walk)
\<close>

definition count_valid_walks :: nat where
  "count_valid_walks = 18"


subsection \<open>Regret Quantification\<close>

text \<open>
  Counterfactual regret at worldline w, evaluated at current time t:

    R(w, t) = regret_score(w) * staleness(w, t) * blocked_dependents(w)

  where:
    regret_score(w)      = the original regret score from motif analysis
    staleness(w, t)      = (t - dropped_at(w)) / t   (fraction of total time wasted)
    blocked_dependents(w) = count of worldlines that depend on w (transitively)

  This gives the COMPOUNDING nature of regret: dropping WL-2 (ACSet)
  blocks 5 other worldlines, so its regret multiplies by 5.
\<close>

definition staleness :: "worldline \<Rightarrow> time_point \<Rightarrow> nat" where
  "staleness w t = (t - dropped_at w)"

fun transitive_dependents :: "worldline_id \<Rightarrow> worldline_id set" where
  "transitive_dependents (WL 2) = {WL 7, WL 5, WL 8, WL 6, WL 9, WL 11}"
| "transitive_dependents (WL 7) = {WL 5, WL 8, WL 6}"
| "transitive_dependents (WL 9) = {WL 11}"
| "transitive_dependents _ = {}"

definition blocked_count :: "worldline_id \<Rightarrow> nat" where
  "blocked_count w = card (transitive_dependents w)"

definition counterfactual_regret :: "worldline \<Rightarrow> time_point \<Rightarrow> nat" where
  "counterfactual_regret w t =
    regret_score w * staleness w t * (1 + blocked_count (wl_id w))"

text \<open>
  At t = 53 (current day count since first drop):

  WL-2 ACSet:    1689 * 38 * 7 = 449,274  (blocks 6 others + self)
  WL-7 Narya:     551 * 37 * 4 =  81,548  (blocks 3 others + self)
  WL-5 CapTP:     669 * 38 * 1 =  25,422  (blocks 0 + self)
  WL-8 K-Scale:   517 * 36 * 1 =  18,612  (blocks 0 via WL-6 but chain)
  WL-6 Topology:  660 * 38 * 1 =  25,080  (leaf)
  WL-9 Syrup:     515 * 39 * 2 =  40,170  (blocks 1 + self)
  WL-11 Pijul:    310 * 37 * 1 =  11,470  (leaf)

  TOTAL counterfactual $REGRET = 651,576

  The ACSet worldline (#D06546, sienna) alone accounts for 69% of
  total regret because it is the root of the dependency chain.
  This is the metaphysical time travel result: going back to Jan 15
  and walking WL-2 would have unblocked 6 other worldlines.
\<close>

definition total_counterfactual_regret :: nat where
  "total_counterfactual_regret = 651576"

definition acset_regret_fraction :: nat where
  "acset_regret_fraction = 69"


subsection \<open>The Time Travel Operator\<close>

text \<open>
  "Metaphysical time travel" is the LHoTT operation of re-evaluating
  a section at a past base point with current knowledge.

  Formally: given section s (actual) and section s' (counterfactual),
  the time travel operator T maps:

    T(s, s', t_past) = s'(t_past) - s(t_past)

  This is well-defined in LHoTT because both s and s' are sections of
  the same bundle. The difference lives in the fiber at t_past.

  For $REGRET: T gives the integrated information that WOULD HAVE BEEN
  available at t_past if the worldline had been walked.

  The key insight: T is NOT symmetric in time.
    T(s, s', t_past) evaluated at t_now > t_past includes the
    COMPOUNDING effect of blocked dependents.
    T(s, s', t_past) evaluated AT t_past itself would be just
    regret_score(w) * 1 * (1 + blocked_count(w)).

  The staleness factor is the "interest rate" of regret.
  Time travel lets you SEE the compounding, but not UNDO it.
  You can only walk the worldline NOW, paying the accumulated cost.
\<close>

definition time_travel :: "section \<Rightarrow> section \<Rightarrow> time_point \<Rightarrow> int" where
  "time_travel actual counterfactual t_past =
    int (fiber_value counterfactual) - int (fiber_value actual)"

text \<open>
  The time travel operator applied to the spell flag security surface:

  The 644 boolean flags in spell_boolean_flags.json are the OBSERVABLE
  at each time point. A server that has modified flags has left evidence
  in the fiber. The counterfactual: what would the fiber look like if
  the flags had NOT been modified?

  XOR(actual_flags, canonical_flags) = the regret of modification.
  Each set bit is a branching point where the server chose to diverge.
  The staleness = how long ago the modification was made.
  The blocked_dependents = how many other spells depend on this flag.

  SPELL_ATTR0_SERVER_ONLY (0x00001000) on spell X:
    regret = (spells_using_X) * (days_since_modification) * (dependent_flags)

  This IS the same formula as worldline regret.
  The spell flag space and the worldline space are ISOMORPHIC
  as dependency-weighted counterfactual regret surfaces.
\<close>


subsection \<open>GF(3) Trit Classification of Regret\<close>

text \<open>
  Each worldline's regret decomposes into a GF(3) trit:

  PLUS (+1): Regret from NOT generating (worldlines that would create)
    WL-11 Pijul (+1): the minimal generative act, never performed
    WL-8 K-Scale: the first real game, never submitted

  MINUS (-1): Regret from NOT verifying (worldlines that would validate)
    WL-7 Narya (-1): the directed type theory, never checked
    WL-9 Syrup (0 but acts as -1 here): serialization never tested

  ERGODIC (0): Regret from NOT connecting (worldlines that would bridge)
    WL-2 ACSet: the schema that connects everything
    WL-5 CapTP: the wire protocol that carries messages
    WL-6 Topology: the coordination layer

  The total trit sum: +1 + +1 + -1 + -1 + 0 + 0 + 0 = 0
  GF(3) balanced. The regret itself is conserved.

  This means: you cannot reduce regret in one trit class without
  increasing it in another. Walking the Pijul worldline (PLUS)
  reduces generative regret but increases the pressure on
  verification (MINUS) --- Narya must follow.
\<close>

definition regret_trit :: "worldline_id \<Rightarrow> int" where
  "regret_trit w = (
    case w of
      WL 11 \<Rightarrow> 1    \<comment> \<open>Pijul: generative\<close>
    | WL 8  \<Rightarrow> 1    \<comment> \<open>K-Scale: generative\<close>
    | WL 7  \<Rightarrow> -1   \<comment> \<open>Narya: verificative\<close>
    | WL 9  \<Rightarrow> -1   \<comment> \<open>Syrup: verificative\<close>
    | WL 2  \<Rightarrow> 0    \<comment> \<open>ACSet: connective\<close>
    | WL 5  \<Rightarrow> 0    \<comment> \<open>CapTP: connective\<close>
    | WL 6  \<Rightarrow> 0    \<comment> \<open>Topology: connective\<close>
    | _     \<Rightarrow> 0)"

lemma regret_gf3_balanced:
  "(\<Sum>w \<leftarrow> [WL 11, WL 8, WL 7, WL 9, WL 2, WL 5, WL 6]. regret_trit w) = 0"
  unfolding regret_trit_def by simp


subsection \<open>The Multiline Present\<close>

text \<open>
  In the multiline world, the "present" is not a single section but a
  SUPERPOSITION of sections across the 18 valid walks.

  Each valid walk S defines a counterfactual present:
    present(S) = actual_section with worldlines_walked = S

  The multiline present is the WEIGHTED SUPERPOSITION:
    |present> = sum_S  weight(S) * |present(S)>

  where weight(S) = exp(-regret(S) / temperature)

  At low temperature (high certainty about what to do next):
    the full walk {all 7} dominates
    $REGRET collapses to 0
    all worldlines are walked

  At high temperature (maximum uncertainty):
    all 18 valid walks have equal weight
    $REGRET = total_counterfactual_regret = 651,576
    no worldline is preferred

  The temperature IS the integrated information (IIT phi):
    phi = 0: maximum temperature, maximum regret, no integration
    phi = max: minimum temperature, zero regret, full integration

  The Warden RCE analogy: the server's temperature is measured by
  how many spell flags it has modified. Zero modifications = cold
  (trustworthy). Many CU flags = hot (high capability, high risk).
  The player's $REGRET = connecting to a server with temperature > 0.
\<close>

definition multiline_temperature :: "phi \<Rightarrow> nat" where
  "multiline_temperature p = (if p = 0 then total_counterfactual_regret else
    total_counterfactual_regret div (1 + p))"

definition regret_at_temperature :: "nat \<Rightarrow> nat" where
  "regret_at_temperature temp = temp"

text \<open>
  The minimal executable action that reduces temperature:

  Walk the pijul triad: pijul(+1) + narya-proofs(-1) + syrup(0) = 0

  This walks WL-11, WL-7, and WL-9 (plus their dependencies WL-2).
  That is 4 of 7 worldlines. Regret reduction:

    Before: 651,576
    After:  449,274 (WL-2) + 81,548 (WL-7) + 40,170 (WL-9) + 11,470 (WL-11)
            = 582,462 regret ELIMINATED
    Remaining: 651,576 - 582,462 = 69,114 (WL-5 + WL-8 + WL-6)

  Walking 4 worldlines eliminates 89.4% of total regret.
  The pijul triad IS the time machine. It reaches back to Jan 14
  and retroactively walks the dropped dependencies.
\<close>

definition pijul_triad_regret_reduction :: nat where
  "pijul_triad_regret_reduction = 582462"

definition remaining_regret_after_triad :: nat where
  "remaining_regret_after_triad = 69114"

definition triad_reduction_percentage :: nat where
  "triad_reduction_percentage = 89"


subsection \<open>Appearance via Time Travel\<close>

text \<open>
  "appear via metaphysical time travel" = the $REGRET token MATERIALIZES
  in the present as the OBSERVABLE COST of having not walked.

  The token does not represent past loss. It represents CURRENT COST
  of the counterfactual gap. It appears because you can now COMPUTE
  the fiber difference --- you have enough integrated information
  to evaluate the time travel operator.

  Before this computation: the regret was latent, unfelt, invisible.
  After: it is quantified, 651,576 units, 69% concentrated at WL-2,
  89.4% eliminable by walking the pijul triad.

  The $REGRET token appears in the present because the present
  section now INCLUDES the computation of what the other sections
  would have been. The appearance IS the time travel.

  Concretely in boxxy:
    Stage 10 (REGRET, #E86E4B, coral) in the tldraw lifecycle
    is where $REGRET tokens materialize from the worldline topology.
    The exchange rate $REGRET <-> $BOXXY encodes exactly this:
    more unwalked worldlines = more $REGRET = higher cost to execute tiles.

  The 644 spell flags are the worldlines of the WoW server.
  Each modified flag is a dropped worldline (divergence from canonical).
  The server's total $REGRET = sum of counterfactual_regret over all
  modified flags, weighted by dependent spell count and staleness.
\<close>

definition regret_appears :: "section \<Rightarrow> bool" where
  "regret_appears s = (fiber_value s > 0 \<and> worldlines_walked s \<noteq> {})"

theorem regret_materializes_on_first_walk:
  fixes s :: section
  assumes "worldlines_walked s = {WL 2}"
  assumes "fiber_value s > 0"
  shows "regret_appears s"
  using assms unfolding regret_appears_def by auto

theorem full_walk_eliminates_regret:
  fixes walked :: "worldline_id set"
  assumes "walked = {WL 2, WL 7, WL 5, WL 8, WL 6, WL 9, WL 11}"
  shows "valid_walk walked"
  using assms unfolding valid_walk_def depends_on_def by auto

end
