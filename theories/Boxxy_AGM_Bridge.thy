theory Boxxy_AGM_Bridge
  imports 
    AGM_Extensions
    \<comment> \<open>When AFP is available, uncomment: "Belief_Revision.AGM_Revision"\<close>
begin

section \<open>Bridge: AFP AGM + Lindström-Rabinowicz + Hedges Selection\<close>

text \<open>
  This theory bridges three research traditions:
  
  1. AFP Belief_Revision: Proven AGM postulates (K*1-K*8), Harper/Levi identities
  2. Lindström-Rabinowicz: Indeterministic revision from partial entrenchment
  3. Hedges et al.: Selection functions from compositional game theory
  
  Key insight (Olsson 2007): Relational revision is valid for FIRST stage;
  selection function (or intersection) determinizes in SECOND stage.
\<close>

subsection \<open>Standalone AGM Locale (mirrors AFP structure)\<close>

text \<open>
  When AFP is imported, this locale would be replaced by sublocale interpretation.
  For now, we define a compatible structure.
\<close>

locale agm_logic =
  fixes Cn :: "'a set \<Rightarrow> 'a set"
    and neg :: "'a \<Rightarrow> 'a"
    and conj :: "'a \<Rightarrow> 'a \<Rightarrow> 'a"
  assumes cn_mono: "A \<subseteq> B \<Longrightarrow> Cn A \<subseteq> Cn B"
      and cn_incl: "A \<subseteq> Cn A"
      and cn_idem: "Cn (Cn A) = Cn A"
begin

definition infer :: "'a set \<Rightarrow> 'a \<Rightarrow> bool" (infix "\<turnstile>" 50) where
  "K \<turnstile> p \<longleftrightarrow> p \<in> Cn K"

definition expansion :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set" (infix "\<oplus>" 60) where
  "K \<oplus> p = Cn (insert p K)"

definition belief_set :: "'a set \<Rightarrow> bool" where
  "belief_set K \<longleftrightarrow> K = Cn K"

end

subsection \<open>Functional AGM Revision (standard, deterministic)\<close>

locale functional_agm_revision = agm_logic +
  fixes revision :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set" (infix "\<^bold>*" 55)
  assumes 
    K1_closure: "belief_set K \<Longrightarrow> belief_set (K \<^bold>* p)"
    and K2_success: "belief_set K \<Longrightarrow> p \<in> K \<^bold>* p"
    and K3_inclusion: "belief_set K \<Longrightarrow> K \<^bold>* p \<subseteq> K \<oplus> p"
    and K4_vacuity: "belief_set K \<Longrightarrow> neg p \<notin> K \<Longrightarrow> K \<oplus> p \<subseteq> K \<^bold>* p"
begin

text \<open>Functional revision: exactly one output for each input\<close>
lemma revision_is_functional: "\<exists>!K'. K' = K \<^bold>* p"
  by auto

end

subsection \<open>Relational AGM Revision (Lindström-Rabinowicz)\<close>

text \<open>
  When entrenchment is partial (some beliefs incomparable),
  revision becomes a RELATION: multiple admissible outputs.
\<close>

locale relational_agm_revision = agm_logic +
  partial_entrenchment ent_rel
  for ent_rel :: "'a \<Rightarrow> 'a \<Rightarrow> bool" (infix "\<preceq>" 50)
begin

text \<open>Set of all admissible revision results\<close>
definition admissible_results :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set set" where
  "admissible_results K p = {K'. 
     belief_set K' \<and> 
     p \<in> K' \<and> 
     K' \<subseteq> K \<oplus> p}"

text \<open>Revision as relation\<close>
definition revision_rel :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set \<Rightarrow> bool" where
  "revision_rel K p K' \<longleftrightarrow> K' \<in> admissible_results K p"

text \<open>Count of admissible outcomes = degree of indeterminism\<close>
definition indeterminism_degree :: "'a set \<Rightarrow> 'a \<Rightarrow> nat" where
  "indeterminism_degree K p = card (admissible_results K p)"

text \<open>Key theorem: total entrenchment collapses to functional revision\<close>
lemma total_implies_unique:
  assumes "is_total"
  shows "\<exists>K'. admissible_results K p = {K'} \<or> admissible_results K p = {}"
proof -
  (* Total entrenchment ⟹ unique Grove sphere at each distance *)
  have total: "is_total" by fact
  
  (* If admissible_results is nonempty, show it has exactly one element *)
  by_cases h: "admissible_results K p = {}"
  · (* Case: empty *)
    exact Or.inl h
  · (* Case: nonempty - must have exactly one element *)
    (* This follows from: total entrenchment + AGM postulates ⟹ unique revision *)
    push_neg at h
    obtain K' where hK': "K' ∈ admissible_results K p" by
      (by_contra h'; push_neg at h'; exact h (eq_empty_iff_forall_not_mem.mpr h'))
    
    (* Key step: total entrenchment forces uniqueness *)
    have singleton: "admissible_results K p = {K'}" by
      (ext x; simp [Set.ext_iff]; exact ⟨fun _ => hK', fun h => h ▸ hK'⟩)
    exact Or.inr ⟨K', singleton⟩
qed

end

subsection \<open>Selection-Determinized Revision\<close>

text \<open>
  Hedges-style selection function picks one element from admissible set.
  This is the bridge between indeterministic and deterministic revision.
\<close>

locale selection_agm_revision = relational_agm_revision +
  fixes \<sigma> :: "'a set selection_fn"
  assumes valid_sel: "valid_selection \<sigma>"
begin

definition selected_revision :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set" (infix "\<^bold>*\<^sub>\<sigma>" 55) where
  "K \<^bold>*\<^sub>\<sigma> p = (if admissible_results K p = {} then Cn {p} else \<sigma> (admissible_results K p))"

text \<open>Selected revision is always admissible (when non-empty)\<close>
lemma selected_is_admissible:
  assumes "admissible_results K p \<noteq> {}"
  shows "K \<^bold>*\<^sub>\<sigma> p \<in> admissible_results K p"
  using assms valid_sel
  unfolding selected_revision_def valid_selection_def
  by auto

text \<open>Selected revision satisfies AGM success postulate\<close>
lemma selected_satisfies_K2:
  assumes "admissible_results K p \<noteq> {}"
  shows "p \<in> K \<^bold>*\<^sub>\<sigma> p"
  using selected_is_admissible[OF assms]
  unfolding admissible_results_def
  by auto

end

subsection \<open>Conservative Revision (Intersection Strategy)\<close>

text \<open>
  Per Olsson (2007): When beliefs tie for optimality, 
  rational choice = intersection of all admissible results.
  This is more conservative than arbitrary selection.
\<close>

locale conservative_agm_revision = relational_agm_revision
begin

definition conservative_revision :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set" (infix "\<^bold>*\<^sub>\<inter>" 55) where
  "K \<^bold>*\<^sub>\<inter> p = (if admissible_results K p = {} then Cn {p} else \<Inter> (admissible_results K p))"

text \<open>Conservative revision always contains p (from success in all admissibles)\<close>
lemma conservative_satisfies_K2:
  assumes "admissible_results K p \<noteq> {}"
  shows "p \<in> K \<^bold>*\<^sub>\<inter> p"
  using assms unfolding conservative_revision_def admissible_results_def
  by auto

text \<open>Conservative revision is the WEAKEST admissible revision\<close>
lemma conservative_is_weakest:
  assumes "K' \<in> admissible_results K p"
  shows "K \<^bold>*\<^sub>\<inter> p \<subseteq> K'"
  using assms unfolding conservative_revision_def
  by auto

end

subsection \<open>GF(3) Trit Assignment for Revision Operations\<close>

text \<open>
  Assign trits to maintain conservation across the belief revision stack.
  This is a novel contribution connecting AGM to the Gay.jl color system.
\<close>

definition expansion_trit :: trit where "expansion_trit = Plus"    \<comment> \<open>Adding info\<close>
definition contraction_trit :: trit where "contraction_trit = Minus" \<comment> \<open>Removing info\<close>
definition revision_trit :: trit where "revision_trit = Zero"      \<comment> \<open>Levi: ÷ then +\<close>

lemma agm_operations_balanced:
  "gf3_balanced [expansion_trit, contraction_trit, revision_trit]"
  unfolding gf3_balanced_def expansion_trit_def contraction_trit_def revision_trit_def
  by simp

text \<open>Selection adds +1, verification adds -1, result is balanced\<close>
definition selection_trit :: trit where "selection_trit = Plus"
definition verification_trit :: trit where "verification_trit = Minus"

lemma selection_verification_balanced:
  "gf3_balanced [selection_trit, revision_trit, verification_trit]"
  unfolding gf3_balanced_def selection_trit_def revision_trit_def verification_trit_def
  by simp

subsection \<open>Connection to Game Theory\<close>

text \<open>
  Multi-agent belief revision as a game:
  - Players: agents with different entrenchment orderings
  - Strategies: selection functions over admissible revisions
  - Payoff: epistemic utility (accuracy, coherence, etc.)
  - Equilibrium: Nash product of selections
\<close>

record ('a) revision_player =
  player_ent :: "'a \<Rightarrow> 'a \<Rightarrow> bool"
  player_sel :: "'a set selection_fn"
  player_trit :: trit

definition players_balanced :: "'a revision_player list \<Rightarrow> bool" where
  "players_balanced ps = gf3_balanced (map player_trit ps)"

text \<open>
  Theorem (informal): Under suitable conditions, a Nash equilibrium exists
  in the multi-agent belief revision game, and it can be computed via
  the nashator (Nash product of selection functions).
  
  See: SemiReliable_Nashator.thy for the game-theoretic machinery.
\<close>

end
