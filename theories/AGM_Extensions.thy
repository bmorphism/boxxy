theory AGM_Extensions
  imports Main
begin

section \<open>AGM Extensions: GF(3), Indeterminism, Selection Functions\<close>

text \<open>
  Extensions to AFP Belief_Revision that enable:
  1. GF(3) trit conservation (Gay.jl integration)
  2. Lindström-Rabinowicz indeterministic revision  
  3. Hedges-style selection functions for determinization
  
  These are designed to layer ON TOP of AFP locales via sublocale/interpretation.
\<close>

subsection \<open>GF(3) Trit Datatype\<close>

text \<open>Independent of AGM - can be used anywhere\<close>

datatype trit = Minus | Zero | Plus

fun trit_val :: "trit \<Rightarrow> int" where
  "trit_val Minus = -1"
| "trit_val Zero = 0"
| "trit_val Plus = 1"

fun trit_add :: "trit \<Rightarrow> trit \<Rightarrow> trit" where
  "trit_add Minus Minus = Plus"   \<comment> \<open>-1 + -1 = -2 \<equiv> 1 mod 3\<close>
| "trit_add Minus Zero = Minus"
| "trit_add Minus Plus = Zero"
| "trit_add Zero t = t"
| "trit_add Plus Minus = Zero"
| "trit_add Plus Zero = Plus"
| "trit_add Plus Plus = Minus"    \<comment> \<open>1 + 1 = 2 \<equiv> -1 mod 3\<close>

fun trit_sum :: "trit list \<Rightarrow> int" where
  "trit_sum [] = 0"
| "trit_sum (t # ts) = trit_val t + trit_sum ts"

definition gf3_balanced :: "trit list \<Rightarrow> bool" where
  "gf3_balanced ts \<longleftrightarrow> trit_sum ts mod 3 = 0"

lemma gf3_triple_balanced: "gf3_balanced [Minus, Zero, Plus]"
  unfolding gf3_balanced_def by simp

lemma gf3_empty_balanced: "gf3_balanced []"
  unfolding gf3_balanced_def by simp

lemma trit_add_comm: "trit_add a b = trit_add b a"
  by (cases a; cases b; simp)

lemma trit_add_assoc: "trit_add (trit_add a b) c = trit_add a (trit_add b c)"
  by (cases a; cases b; cases c; simp)

text \<open>Zero is the identity element\<close>
lemma trit_add_zero_left: "trit_add Zero a = a"
  by (cases a; simp)

lemma trit_add_zero_right: "trit_add a Zero = a"
  by (cases a; simp)

text \<open>Every element has an inverse\<close>
fun trit_neg :: "trit \<Rightarrow> trit" where
  "trit_neg Minus = Plus"
| "trit_neg Zero = Zero"
| "trit_neg Plus = Minus"

lemma trit_add_inverse_left: "trit_add (trit_neg a) a = Zero"
  by (cases a; simp)

lemma trit_add_inverse_right: "trit_add a (trit_neg a) = Zero"
  by (cases a; simp)

text \<open>GF(3) is indeed an Abelian group under trit_add\<close>

text \<open>Collected simp rules for one-shot trit proofs (Lean-style)\<close>
lemmas gf3_simps = 
  trit_add.simps trit_neg.simps trit_val.simps
  trit_add_comm trit_add_assoc 
  trit_add_zero_left trit_add_zero_right
  trit_add_inverse_left trit_add_inverse_right

text \<open>All group axioms in one statement\<close>
lemma gf3_group_axioms:
  shows "trit_add Zero a = a"                           \<comment> \<open>identity\<close>
    and "trit_add (trit_neg a) a = Zero"                \<comment> \<open>inverse\<close>
    and "trit_add (trit_add a b) c = trit_add a (trit_add b c)" \<comment> \<open>assoc\<close>
    and "trit_add a b = trit_add b a"                   \<comment> \<open>comm\<close>
  using trit_add_zero_left trit_add_inverse_left trit_add_assoc trit_add_comm by simp_all

text \<open>Helper: trit_sum is additive on concatenation\<close>
lemma trit_sum_append:
  "trit_sum (xs @ ys) = trit_sum xs + trit_sum ys"
  by (induction xs) auto

text \<open>Concatenation preserves balance\<close>
lemma gf3_concat_balanced:
  assumes "gf3_balanced xs" and "gf3_balanced ys"
  shows "gf3_balanced (xs @ ys)"
  unfolding gf3_balanced_def trit_sum_append
  using assms unfolding gf3_balanced_def
  by presburger

text \<open>Balance is preserved under cyclic permutation\<close>
lemma gf3_rotate_balanced:
  assumes "gf3_balanced (a # b # c # [])"
  shows "gf3_balanced (b # c # a # [])"
  using assms unfolding gf3_balanced_def
  by (simp only: trit_sum.simps, presburger)

text \<open>Any permutation of a balanced triple is balanced\<close>
lemma gf3_permute_balanced:
  assumes "gf3_balanced [a, b, c]"
  shows "gf3_balanced [a, c, b]" and "gf3_balanced [b, a, c]"
    and "gf3_balanced [b, c, a]" and "gf3_balanced [c, a, b]"
    and "gf3_balanced [c, b, a]"
  using assms unfolding gf3_balanced_def
  by (simp only: trit_sum.simps, presburger)+

subsection \<open>Selection Functions (Hedges)\<close>

text \<open>
  A selection function picks one element from a non-empty set.
  This is the bridge between indeterministic revision and game theory.
\<close>

type_synonym 'a selection_fn = "'a set \<Rightarrow> 'a"

definition valid_selection :: "'a selection_fn \<Rightarrow> bool" where
  "valid_selection \<sigma> \<longleftrightarrow> (\<forall>S. S \<noteq> {} \<longrightarrow> \<sigma> S \<in> S)"

definition determinize :: 
  "('a \<Rightarrow> 'b \<Rightarrow> 'c set) \<Rightarrow> 'c selection_fn \<Rightarrow> 'a \<Rightarrow> 'b \<Rightarrow> 'c" where
  "determinize I \<sigma> a b = \<sigma> (I a b)"

lemma determinize_mem:
  assumes "valid_selection \<sigma>"
      and "I a b \<noteq> {}"
    shows "determinize I \<sigma> a b \<in> I a b"
  using assms unfolding valid_selection_def determinize_def by auto

text \<open>Composition of selection functions\<close>
definition selection_compose ::
  "'a selection_fn \<Rightarrow> 'b selection_fn \<Rightarrow> ('a \<times> 'b) selection_fn" where
  "selection_compose \<sigma>1 \<sigma>2 = (\<lambda>S. (\<sigma>1 (fst ` S), \<sigma>2 (snd ` S)))"

subsection \<open>Indeterministic Entrenchment (Lindström-Rabinowicz)\<close>

text \<open>
  Standard AGM assumes total/connected entrenchment ordering.
  Lindström-Rabinowicz: what if some beliefs are incomparable?
  Result: revision becomes SET-valued (multiple admissible outcomes).
\<close>

locale partial_entrenchment =
  fixes ent_rel :: "'a \<Rightarrow> 'a \<Rightarrow> bool" (infix "\<preceq>" 50)
  assumes transitivity: "\<lbrakk>p \<preceq> q; q \<preceq> r\<rbrakk> \<Longrightarrow> p \<preceq> r"
  \<comment> \<open>Note: NO totality/connectivity assumption\<close>
begin

definition comparable :: "'a \<Rightarrow> 'a \<Rightarrow> bool" where
  "comparable p q \<longleftrightarrow> p \<preceq> q \<or> q \<preceq> p"

definition incomparable :: "'a \<Rightarrow> 'a \<Rightarrow> bool" where
  "incomparable p q \<longleftrightarrow> \<not> comparable p q"

definition incomparable_pairs :: "'a set \<Rightarrow> ('a \<times> 'a) set" where
  "incomparable_pairs S = {(p, q). p \<in> S \<and> q \<in> S \<and> incomparable p q}"

definition is_total :: bool where
  "is_total \<longleftrightarrow> (\<forall>p q. comparable p q)"

definition indeterminism_degree :: "'a set \<Rightarrow> nat" where
  "indeterminism_degree S = card (incomparable_pairs S)"

end

text \<open>Indeterministic revision: returns SET of admissible belief states\<close>

locale indet_revision = partial_entrenchment +
  fixes Cn :: "'a set \<Rightarrow> 'a set"
  assumes cn_mono: "A \<subseteq> B \<Longrightarrow> Cn A \<subseteq> Cn B"
      and cn_incl: "A \<subseteq> Cn A"
      and cn_idem: "Cn (Cn A) = Cn A"
begin

text \<open>Set of admissible revisions given partial entrenchment\<close>

definition admissible_revisions :: "'a set \<Rightarrow> 'a \<Rightarrow> 'a set set" where
  "admissible_revisions K p = {K'. p \<in> K' \<and> K' = Cn K'}"

text \<open>
  Determinization via selection over belief sets.
  Note: selection function must be over 'a set (belief sets), not 'a (propositions).
\<close>
definition determinized_revision ::
  "('a set) set \<Rightarrow> 'a set selection_fn \<Rightarrow> 'a set \<Rightarrow> 'a \<Rightarrow> 'a set" where
  "determinized_revision admissible \<sigma> K p = 
    (if admissible = {} then Cn {p} else \<sigma> admissible)"

text \<open>Convenient wrapper using admissible_revisions\<close>
definition determinize_revision ::
  "'a set selection_fn \<Rightarrow> 'a set \<Rightarrow> 'a \<Rightarrow> 'a set" where
  "determinize_revision \<sigma> K p = 
    determinized_revision (admissible_revisions K p) \<sigma> K p"

lemma determinize_revision_in_admissible:
  assumes "valid_selection \<sigma>"
      and "admissible_revisions K p \<noteq> {}"
    shows "determinize_revision \<sigma> K p \<in> admissible_revisions K p"
  using assms 
  unfolding determinize_revision_def determinized_revision_def valid_selection_def
  by auto

end

subsection \<open>Trit-Tagged Operations\<close>

text \<open>
  Every operation in the system gets a trit tag.
  Compositions must preserve GF(3) balance.
\<close>

record 'a tagged_op =
  op_fn :: 'a
  op_trit :: trit

definition compose_tagged ::
  "('a \<Rightarrow> 'b \<Rightarrow> 'c) tagged_op \<Rightarrow> 'a tagged_op \<Rightarrow> 'b tagged_op \<Rightarrow> 'c tagged_op" where
  "compose_tagged f x y = \<lparr>
    op_fn = op_fn f (op_fn x) (op_fn y),
    op_trit = trit_add (op_trit f) (trit_add (op_trit x) (op_trit y))
  \<rparr>"

definition tagged_balanced :: "'a tagged_op list \<Rightarrow> bool" where
  "tagged_balanced ops = gf3_balanced (map op_trit ops)"

subsection \<open>Totality Implies Uniqueness (Main Result)\<close>

text \<open>
  The central theorem of AGM belief revision under total entrenchment:

  THEOREM: If entrenchment is total (connected) and admissible revisions exist,
           then the set of admissible revisions is a singleton.

  This captures Grove's (1988) insight that totality of entrenchment forces
  a unique revision operator.

  NOTE: AGM-specific uniqueness proofs moved to Grove_Spheres.thy
\<close>

subsection \<open>GF(3) Conservation for Totality-to-Uniqueness Transition\<close>

text \<open>
  The transformation from indeterministic revision to deterministic revision
  maintains GF(3) balance:

  - Indeterminacy (admissible_revisions is large set) = +1 PLUS
  - Determinization (selection picks one) = 0 ZERO
  - Verification (checking uniqueness holds) = -1 MINUS
\<close>

definition indeterminacy_trit :: trit where "indeterminacy_trit = Plus"
definition determinization_trit :: trit where "determinization_trit = Zero"
definition verification_trit :: trit where "verification_trit = Minus"

lemma totality_determinization_conserved:
  "gf3_balanced [indeterminacy_trit, determinization_trit, verification_trit]"
  unfolding gf3_balanced_def indeterminacy_trit_def determinization_trit_def verification_trit_def
  by simp

subsection \<open>Interop Bridge to AFP Belief_Revision\<close>

text \<open>
  When AFP Belief_Revision is imported, we can:
  1. Interpret their AGM_Revision locale
  2. Extend with our indeterministic/GF(3) structure
  3. Use sublocale to inherit their proven theorems

  Example (requires AFP import):

  sublocale indet_revision \<subseteq> Tarskian_logic
    where Cn = Cn
    by (unfold_locales) (auto simp: cn_mono cn_incl cn_idem)
\<close>

end
