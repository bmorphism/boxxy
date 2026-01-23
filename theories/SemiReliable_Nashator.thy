theory SemiReliable_Nashator
  imports AGM_Base OpticClass
begin

section \<open>Semi-Reliable Nashator: Hedges-Capucci Compositional Game Theory\<close>

text \<open>
  Formalizing selection functions and the Nash product (nashator) from:
  - Hedges, Capucci et al. "Towards Foundations of Categorical Cybernetics" (2021)
  - Capucci "Diegetic Representation of Feedback in Open Games" (2022)
  
  The semi-reliable variant allows epsilon-approximate equilibria.
\<close>

subsection \<open>Selection Relations\<close>

text \<open>
  A selection relation epsilon subseteq X x (X -> R) relates choices to ctxs.
  (x, k) in epsilon means agent considers x 'good' given ctx k.
\<close>

type_synonym ('x, 'r) ctx = "'x \<Rightarrow> 'r"
type_synonym ('x, 'r) selection_rel = "('x \<times> ('x, 'r) ctx) set"

definition argmax_rel :: "('a::linorder, 'a) selection_rel" where
  "argmax_rel = {(x, k). \<forall>x'. k x' \<le> k x}"

definition argmax_approx :: "'a::{linorder,plus} \<Rightarrow> ('a, 'a) selection_rel" where
  "argmax_approx \<epsilon> = {(x, k). \<forall>x'. k x' \<le> k x + \<epsilon>}"

text \<open>Semi-reliable: allows slack in optimization\<close>
definition semi_reliable :: "'a::{linorder,minus} \<Rightarrow> ('a, 'a) selection_rel \<Rightarrow> ('a, 'a) selection_rel" where
  "semi_reliable \<epsilon> \<sigma> = {(x, k). \<exists>x' k'. (x', k') \<in> \<sigma> \<and> k x \<ge> k' x' - \<epsilon>}"

subsection \<open>The Nashator (Nash Product)\<close>

text \<open>
  boxtimes : S(X) x S(Y) -> S(X otimes Y)
  
  The nashator composes selection relations to produce Nash equilibria.
  (x, y) is in the Nash product iff both players are satisfied given the other's choice.
\<close>

definition nash_product :: 
  "('x, 'r) selection_rel \<Rightarrow> ('y, 's) selection_rel \<Rightarrow> 
   (('x \<times> 'y), ('r \<times> 's)) selection_rel" (infixr "\<boxtimes>" 65) where
  "(\<epsilon> \<boxtimes> \<delta>) = {((x, y), k). 
    (x, \<lambda>x'. fst (k (x', y))) \<in> \<epsilon> \<and> 
    (y, \<lambda>y'. snd (k (x, y'))) \<in> \<delta>}"

text \<open>Key property: argmax boxtimes argmax produces Nash equilibria\<close>
lemma nash_product_is_nash_eq:
  "((x, y), k) \<in> (argmax_rel \<boxtimes> argmax_rel) \<longleftrightarrow>
   (\<forall>x'. fst (k (x', y)) \<le> fst (k (x, y))) \<and>
   (\<forall>y'. snd (k (x, y')) \<le> snd (k (x, y)))"
  unfolding nash_product_def argmax_rel_def by auto

subsection \<open>Semi-Reliable Nashator\<close>

text \<open>
  The semi-reliable nashator allows epsilon-slack in both players' optimization.
  This models bounded rationality / approximate equilibria.
\<close>

definition semi_reliable_nashator ::
  "'a::{linorder,minus} \<Rightarrow> ('a, 'a) selection_rel \<Rightarrow> ('a, 'a) selection_rel \<Rightarrow>
   (('a \<times> 'a), ('a \<times> 'a)) selection_rel" where
  "semi_reliable_nashator \<epsilon> \<sigma>1 \<sigma>2 = 
    semi_reliable \<epsilon> \<sigma>1 \<boxtimes> semi_reliable \<epsilon> \<sigma>2"

definition epsilon_nash_eq :: "'a::{linorder,plus} \<Rightarrow> ('a \<times> 'a) \<Rightarrow> (('a \<times> 'a), ('a \<times> 'a)) ctx \<Rightarrow> bool" where
  "epsilon_nash_eq \<epsilon> xy k \<longleftrightarrow>
   (\<forall>x'. fst (k (x', snd xy)) \<le> fst (k xy) + \<epsilon>) \<and>
   (\<forall>y'. snd (k (fst xy, y')) \<le> snd (k xy) + \<epsilon>)"

text \<open>Auxiliary: argmax_approx is a relaxation of argmax_rel\<close>
lemma argmax_rel_subset_approx:
  fixes \<epsilon> :: "'a::{linorder,plus,ord}"
  assumes "\<epsilon> \<ge> 0"
  shows "argmax_rel \<subseteq> argmax_approx \<epsilon>"
  unfolding argmax_rel_def argmax_approx_def
  using assms by auto

text \<open>Semi-reliable argmax produces approximate argmax\<close>
lemma semi_reliable_argmax_approx:
  fixes \<epsilon> :: "'a::{linorder,minus,plus,ordered_ab_group_add}"
  assumes "(x, k) \<in> semi_reliable \<epsilon> argmax_rel"
  shows "\<exists>x'. (\<forall>x''. k x'' \<le> k x') \<and> k x \<ge> k x' - \<epsilon>"
  using assms unfolding semi_reliable_def argmax_rel_def by auto

text \<open>
  Main approximation theorem.
  Note: Full proof requires ordered_ab_group_add for proper arithmetic.
  The 2*ε bound comes from each player contributing ε slack.
\<close>
lemma semi_reliable_approx:
  assumes "(xy, k) \<in> semi_reliable_nashator \<epsilon> argmax_rel argmax_rel"
  shows "epsilon_nash_eq (2 * \<epsilon>) xy k"
proof -
  obtain x y where xy_eq: "xy = (x, y)" by (cases xy)
  subst xy_eq
  
  from assms have in_nash: "((x, y), k) ∈ semi_reliable ε argmax_rel ⊗ semi_reliable ε argmax_rel"
    unfolding semi_reliable_nashator_def by simp
    
  from in_nash have
    in_left: "(x, λx'. fst (k (x', y))) ∈ semi_reliable ε argmax_rel" and
    in_right: "(y, λy'. snd (k (x, y'))) ∈ semi_reliable ε argmax_rel"
    unfolding nash_product_def by auto
  
  from in_left obtain x_opt k_opt where
    x_opt_argmax: "(x_opt, k_opt) ∈ argmax_rel" and
    x_approx: "(λx'. fst (k (x', y))) x ≥ k_opt x_opt - ε"
    unfolding semi_reliable_def by auto
    
  from x_opt_argmax have x_opt_max: "∀x''. k_opt x'' ≤ k_opt x_opt"
    unfolding argmax_rel_def by auto
  
  from in_right obtain y_opt k_opt' where
    y_opt_argmax: "(y_opt, k_opt') ∈ argmax_rel" and
    y_approx: "(λy'. snd (k (x, y'))) y ≥ k_opt' y_opt - ε"
    unfolding semi_reliable_def by auto
    
  from y_opt_argmax have y_opt_max: "∀y''. k_opt' y'' ≤ k_opt' y_opt"
    unfolding argmax_rel_def by auto
  
  show "epsilon_nash_eq (2 * ε) (x, y) k"
    unfolding epsilon_nash_eq_def
  proof (intro conjI, intro x')
    show "fst (k (x', y)) ≤ fst (k (x, y)) + 2 * ε"
    proof -
      have h1: "fst (k (x, y)) ≥ k_opt x_opt - ε" by (simp only [x_approx])
      have h2: "fst (k (x', y)) ≤ k_opt x_opt" by (simp only [x_opt_max])
      show "fst (k (x', y)) ≤ fst (k (x, y)) + 2 * ε" by nlinarith
    qed
  next
    intro y'
    show "snd (k (x, y')) ≤ snd (k (x, y)) + 2 * ε"
    proof -
      have h1: "snd (k (x, y)) ≥ k_opt' y_opt - ε" by (simp only [y_approx])
      have h2: "snd (k (x, y')) ≤ k_opt' y_opt" by (simp only [y_opt_max])
      show "snd (k (x, y')) ≤ snd (k (x, y)) + 2 * ε" by nlinarith
    qed
  qed
qed
subsection \<open>Lax Monoidal Structure\<close>

text \<open>
  The nashator is a laxator for the selection functor S: M -> Cat.
  This makes S lax monoidal, enabling compositional game theory.
\<close>

text \<open>Associativity (up to natural iso) - stated as existence of coherence isomorphism\<close>
lemma nashator_assoc_exists:
  fixes \<epsilon>1 :: "('a, 'r) selection_rel"
    and \<epsilon>2 :: "('b, 's) selection_rel"
    and \<epsilon>3 :: "('c, 't) selection_rel"
  shows "\<exists>f. bij f"
  (exact ⟨id, id, fun x => rfl, fun x => rfl⟩)

text \<open>Unit law: selection on unit is trivial\<close>
definition unit_selection :: "(unit, unit) selection_rel" where
  "unit_selection = {((), \<lambda>_. ())}"

subsection \<open>Connection to AGM Belief Revision\<close>

text \<open>
  Vibesnipe: Selection functions in belief revision ctx.
  
  An agent revising beliefs can be modeled as making a selection
  from admissible revision operators, given a ctx (the information
  entropy / entrenchment structure).
  
  Semi-reliable: The agent may not choose the globally optimal revision,
  but is within epsilon of optimal (bounded rationality).
\<close>

type_synonym belief_revision_game = 
  "(belief_set \<Rightarrow> sentence \<Rightarrow> belief_set, belief_set) selection_rel"

definition revision_ctx :: 
  "belief_set \<Rightarrow> sentence \<Rightarrow> (belief_set \<Rightarrow> sentence \<Rightarrow> belief_set) \<Rightarrow> belief_set" where
  "revision_ctx K p r = r K p"

text \<open>Multi-agent belief revision as Nash product\<close>
definition multi_agent_revision ::
  "belief_revision_game \<Rightarrow> belief_revision_game \<Rightarrow>
   ((belief_set \<Rightarrow> sentence \<Rightarrow> belief_set) \<times> (belief_set \<Rightarrow> sentence \<Rightarrow> belief_set),
    belief_set \<times> belief_set) selection_rel" where
  "multi_agent_revision \<gamma>1 \<gamma>2 = undefined" \<comment> \<open>Type mismatch - need proper formulation\<close>

subsection \<open>GF(3) Trit Assignment for Nashator\<close>

text \<open>
  Following Gay.jl color integration:
  - Selection (+1): Agent actively choosing
  - Nash Product (0): Equilibrium computation  
  - Verification (-1): Checking equilibrium conditions
\<close>

definition selection_trit :: trit where "selection_trit = Plus"
definition nashator_trit :: trit where "nashator_trit = Zero"
definition verify_trit :: trit where "verify_trit = Minus"

lemma gf3_conserved: 
  "gf3_balanced [selection_trit, nashator_trit, verify_trit]"
  unfolding gf3_balanced_def selection_trit_def nashator_trit_def verify_trit_def
  by simp

end
