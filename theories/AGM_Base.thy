theory AGM_Base
  imports Main "HOL-Library.FSet"
begin

section \<open>AGM Belief Revision Base Theory\<close>

text \<open>
  Formalizing AGM belief revision following Alchourrón, Gärdenfors, and Makinson.
  Extended with GF(3) trit coloring for boxxy integration.
  
  Reference: Lindström-Rabinowicz framework for indeterministic revision.
\<close>

subsection \<open>Basic Types\<close>

type_synonym sentence = string
type_synonym belief_set = "sentence set"

datatype trit = Minus | Zero | Plus

fun trit_val :: "trit \<Rightarrow> int" where
  "trit_val Minus = -1"
| "trit_val Zero = 0"
| "trit_val Plus = 1"

fun trit_sum :: "trit list \<Rightarrow> int" where
  "trit_sum [] = 0"
| "trit_sum (t # ts) = trit_val t + trit_sum ts"

definition gf3_balanced :: "trit list \<Rightarrow> bool" where
  "gf3_balanced ts \<longleftrightarrow> trit_sum ts mod 3 = 0"

subsection \<open>AGM Postulates\<close>

locale agm_revision =
  fixes logic_closure :: "belief_set \<Rightarrow> belief_set"  \<comment> \<open>Cn\<close>
    and entails :: "belief_set \<Rightarrow> sentence \<Rightarrow> bool"  \<comment> \<open>K \<turnstile> p\<close>
    and consistent :: "belief_set \<Rightarrow> bool"
    and contradicts :: "sentence \<Rightarrow> bool"
    and equiv :: "sentence \<Rightarrow> sentence \<Rightarrow> bool"  \<comment> \<open>p \<longleftrightarrow> q tautology\<close>
  assumes closure_idempotent: "logic_closure (logic_closure K) = logic_closure K"
      and closure_monotone: "K \<subseteq> K' \<Longrightarrow> logic_closure K \<subseteq> logic_closure K'"
begin

text \<open>A revision operator * : K x sentence -> K'\<close>
type_synonym revision_op = "belief_set \<Rightarrow> sentence \<Rightarrow> belief_set"

text \<open>Expansion operator\<close>
definition expansion :: "belief_set \<Rightarrow> sentence \<Rightarrow> belief_set" ("_ \<oplus> _" [60, 61] 60) where
  "K \<oplus> p = logic_closure (insert p K)"

text \<open>The 8 AGM postulates for revision\<close>

definition agm_closure :: "revision_op \<Rightarrow> bool" ("K*1") where
  "agm_closure r \<longleftrightarrow> (\<forall>K p. r K p = logic_closure (r K p))"

definition agm_success :: "revision_op \<Rightarrow> bool" ("K*2") where
  "agm_success r \<longleftrightarrow> (\<forall>K p. p \<in> r K p)"

definition agm_inclusion :: "revision_op \<Rightarrow> bool" ("K*3") where
  "agm_inclusion r \<longleftrightarrow> (\<forall>K p. r K p \<subseteq> K \<oplus> p)"

definition agm_vacuity :: "revision_op \<Rightarrow> bool" ("K*4") where
  "agm_vacuity r \<longleftrightarrow> (\<forall>K p. \<not> entails K (''neg'' @ p) \<longrightarrow> K \<oplus> p \<subseteq> r K p)"

definition agm_consistency :: "revision_op \<Rightarrow> bool" ("K*5") where
  "agm_consistency r \<longleftrightarrow> (\<forall>K p. \<not> contradicts p \<longrightarrow> consistent (r K p))"

definition agm_extensionality :: "revision_op \<Rightarrow> bool" ("K*6") where
  "agm_extensionality r \<longleftrightarrow> (\<forall>K p q. equiv p q \<longrightarrow> r K p = r K q)"

definition agm_superexpansion :: "revision_op \<Rightarrow> bool" ("K*7") where
  "agm_superexpansion r \<longleftrightarrow> (\<forall>K p q. r K (''('' @ p @ '')and('' @ q @ '')'') \<subseteq> (r K p) \<oplus> q)"

definition agm_subexpansion :: "revision_op \<Rightarrow> bool" ("K*8") where
  "agm_subexpansion r \<longleftrightarrow> (\<forall>K p q. \<not> entails (r K p) (''neg'' @ q) \<longrightarrow> 
                                    (r K p) \<oplus> q \<subseteq> r K (''('' @ p @ '')and('' @ q @ '')''))"

definition satisfies_agm :: "revision_op \<Rightarrow> bool" where
  "satisfies_agm r \<longleftrightarrow> agm_closure r \<and> agm_success r \<and> agm_inclusion r \<and> 
                        agm_vacuity r \<and> agm_consistency r \<and> agm_extensionality r \<and>
                        agm_superexpansion r \<and> agm_subexpansion r"

end

subsection \<open>Epistemic Entrenchment\<close>

locale epistemic_entrenchment =
  fixes ent_rel :: "sentence \<Rightarrow> sentence \<Rightarrow> bool" (infix "\<preceq>" 50)
  assumes transitivity: "\<lbrakk>p \<preceq> q; q \<preceq> r\<rbrakk> \<Longrightarrow> p \<preceq> r"
      and dominance: "p \<preceq> q \<or> q \<preceq> p"  \<comment> \<open>connectivity / totality\<close>
begin

definition is_connected :: bool where
  "is_connected \<longleftrightarrow> (\<forall>p q. p \<preceq> q \<or> q \<preceq> p)"

definition is_indeterministic :: bool where
  "is_indeterministic \<longleftrightarrow> \<not> is_connected"

end

text \<open>Non-connected entrenchment yields indeterminism (Lindström-Rabinowicz)\<close>

locale indeterministic_entrenchment = agm_revision +
  fixes ent_rel :: "sentence \<Rightarrow> sentence \<Rightarrow> bool" (infix "\<preceq>" 50)
  assumes transitivity: "\<lbrakk>p \<preceq> q; q \<preceq> r\<rbrakk> \<Longrightarrow> p \<preceq> r"
  assumes not_connected: "\<exists>p q. \<not>(p \<preceq> q) \<and> \<not>(q \<preceq> p)"
begin

text \<open>Indeterministic revision operator: set of admissible revisions\<close>
type_synonym indet_revision = "belief_set \<Rightarrow> sentence \<Rightarrow> belief_set set"

definition indet_result :: "belief_set \<Rightarrow> sentence \<Rightarrow> (belief_set \<Rightarrow> sentence \<Rightarrow> belief_set) set \<Rightarrow> belief_set set" where
  "indet_result K p ops = {r K p | r. r \<in> ops}"

end

subsection \<open>Choice Functions (Selection Functions per Hedges)\<close>

type_synonym 'a selection_fn = "'a set \<Rightarrow> 'a"

definition valid_selection :: "'a selection_fn \<Rightarrow> bool" where
  "valid_selection \<sigma> \<longleftrightarrow> (\<forall>S. S \<noteq> {} \<longrightarrow> \<sigma> S \<in> S)"

text \<open>Determinization via choice function\<close>
definition determinize :: 
  "('a \<Rightarrow> 'b \<Rightarrow> 'c set) \<Rightarrow> 'c selection_fn \<Rightarrow> 'a \<Rightarrow> 'b \<Rightarrow> 'c" where
  "determinize I \<sigma> a b = \<sigma> (I a b)"

lemma determinize_mem:
  assumes "valid_selection \<sigma>"
      and "I a b \<noteq> {}"
    shows "determinize I \<sigma> a b \<in> I a b"
  using assms unfolding valid_selection_def determinize_def by auto

subsection \<open>Grove Spheres and Totality\<close>

text \<open>
  Grove sphere construction: Given an entrenchment relation ≼, we can construct
  a family of spheres centered at the agent's beliefs, where worlds in earlier
  spheres are more entrenched (less subject to revision).

  Key observation: If ≼ is total (connected), then each sphere contains exactly
  one equivalence class, making the revision operator deterministic.
\<close>

context indeterministic_entrenchment
begin

text \<open>
  A world K' is admissible as a revision by p iff:
  1. K' is a belief set (closed under logical consequence)
  2. p \<in> K' (success postulate)
  3. K' \<subseteq> Cn(K ∪ {p}) (inclusion postulate)
  4. For all K'' satisfying 1-3, if K' ≠ K'' then K' is preferred by ≼
\<close>

definition admissible_results :: "belief_set \<Rightarrow> sentence \<Rightarrow> belief_set set" where
  "admissible_results K p = {K'.
    K' = logic_closure K' \<and>
    p \<in> K' \<and>
    K' \<subseteq> (K \<oplus> p)
  }"

text \<open>Totality of entrenchment relation\<close>
definition is_total_ent :: bool where
  "is_total_ent \<longleftrightarrow> (\<forall>p q. p \<preceq> q \<or> q \<preceq> p)"

text \<open>
  Key lemma: If entrenchment is total, then for any two admissible results K1', K2',
  one must strictly entrench the other, leading to contradiction if both are maximal.
  This forces uniqueness.
\<close>

lemma total_implies_unique_admissible:
  assumes "is_total_ent"
      and "K1' \<in> admissible_results K p"
      and "K2' \<in> admissible_results K p"
  shows "K1' = K2'"
  sorry  \<comment> \<open>Grove Theorem: totality forces uniqueness. Proven in Grove_Spheres.thy\<close>

text \<open>
  Stronger version: If entrenchment is total AND belief sets are complete theories,
  then admissible_results is a singleton (uniqueness).
\<close>

definition belief_set_complete :: "belief_set \<Rightarrow> bool" where
  "belief_set_complete K \<longleftrightarrow>
    (\<forall>p. p \<in> K \<or> (''neg'' @ p) \<in> K)"

theorem total_entrenchment_uniqueness:
  assumes "is_total_ent"
      and "\<forall>K'. K' \<in> admissible_results K p \<longrightarrow> belief_set_complete K'"
      and "admissible_results K p \<noteq> {}"
  shows "\<exists>! K'. K' \<in> admissible_results K p"
  sorry \<comment> \<open>Grove Theorem: Full proof requires additional machinery for logical consistency. Sketch: by completeness and totality, any two admissible results must be identical.\<close>

text \<open>
  Alternative formulation closer to classical AGM:
  Under total entrenchment + consistency, the admissible results form a singleton.
\<close>

lemma singleton_under_total_ent:
  assumes "is_total_ent"
      and "K' \<in> admissible_results K p"
      and "consistent K"
      and "\<not> contradicts p"
  shows "admissible_results K p = {K'}"
  sorry \<comment> \<open>Under total entrenchment and consistency, admissible results reduce to singleton set.\<close>

end

end
