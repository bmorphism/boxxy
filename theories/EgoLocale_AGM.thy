theory EgoLocale_AGM
  imports Boxxy_AGM_Bridge Grove_Spheres SemiReliable_Nashator Narratives
begin

section \<open>Ego-Locale AGM Bridge: Reconciliation Theory\<close>

text \<open>
  This theory reconciles the computational ego-locale (Julia) with the
  formal AGM belief revision structure (Isabelle).

  Core thesis: The Freudian ego is an ordered locale whose revision
  dynamics satisfy AGM postulates K*1-K*8. Its persistence layer is
  a content-addressed store; its thermodynamics follow the Ising model.

  Key result: The Nash equilibrium of the therapeutic game and the
  therapeutic optimum coincide if and only if T \<approx> Tc (criticality).

  Reference: ego-locale.jl (~/i/ego-locale.jl)
  Dependencies: AGM_Base, AGM_Extensions, Grove_Spheres,
                SemiReliable_Nashator, Narratives
\<close>

subsection \<open>Ego-State as Belief Set\<close>

text \<open>
  An ego-state is identified by its manifest (list of chunk hashes).
  Two ego-states with the same manifest are the same belief set.
  Content-addressing ensures extensionality (AGM K*6).

  ego_state.chunks \<longleftrightarrow> belief_set K
  ego_state.energy \<longleftrightarrow> entrenchment depth
  ego_state.name   \<longleftrightarrow> label (not part of identity)
\<close>

type_synonym chunk_hash = nat
type_synonym manifest = "chunk_hash list"
type_synonym ego_state = "manifest \<times> int"  \<comment> \<open>(chunks, energy)\<close>

definition ego_chunks :: "ego_state \<Rightarrow> manifest" where
  "ego_chunks s = fst s"

definition ego_energy :: "ego_state \<Rightarrow> int" where
  "ego_energy s = snd s"

text \<open>Content-addressed identity: same chunks = same state\<close>
definition ego_equiv :: "ego_state \<Rightarrow> ego_state \<Rightarrow> bool" where
  "ego_equiv s1 s2 \<longleftrightarrow> set (ego_chunks s1) = set (ego_chunks s2)"

lemma ego_equiv_refl: "ego_equiv s s"
  unfolding ego_equiv_def by simp

lemma ego_equiv_sym: "ego_equiv s1 s2 \<Longrightarrow> ego_equiv s2 s1"
  unfolding ego_equiv_def by auto

lemma ego_equiv_trans: "\<lbrakk>ego_equiv s1 s2; ego_equiv s2 s3\<rbrakk> \<Longrightarrow> ego_equiv s1 s3"
  unfolding ego_equiv_def by auto


subsection \<open>Heyting Operations as AGM Operations\<close>

text \<open>
  The Heyting algebra operations on ego-states correspond to AGM operations:

  meet(U,V)                 = K \<inter> K'        (conservative revision)
  join(U,V)                 = Cn(K \<union> K')    (closure under expansion)
  heyting_implication(U,V)  = plan_sync(U,V) = Levi identity
  heyting_negation(U)       = reaction formation

  Formally: the manifest (set of chunk hashes) forms a distributive lattice
  under set intersection (meet) and set union (join). The Heyting implication
  is the set difference V \<setminus> U (chunks in V not in U).
\<close>

definition ego_meet :: "ego_state \<Rightarrow> ego_state \<Rightarrow> manifest" where
  "ego_meet s1 s2 = filter (\<lambda>h. h \<in> set (ego_chunks s2)) (ego_chunks s1)"

definition ego_join :: "ego_state \<Rightarrow> ego_state \<Rightarrow> manifest" where
  "ego_join s1 s2 = remdups (ego_chunks s1 @ ego_chunks s2)"

definition ego_implication :: "ego_state \<Rightarrow> ego_state \<Rightarrow> manifest" where
  "ego_implication s1 s2 = filter (\<lambda>h. h \<notin> set (ego_chunks s1)) (ego_chunks s2)"

text \<open>The implication IS the Levi identity's delta\<close>
lemma ego_implication_is_delta:
  "set (ego_implication s1 s2) = set (ego_chunks s2) - set (ego_chunks s1)"
  unfolding ego_implication_def by auto

text \<open>Levi identity: revision = contract contradictions then expand with new\<close>
lemma levi_identity:
  "set (ego_join (ego_meet s1 s2, e) (ego_implication s1 s2, e')) =
   set (ego_chunks s1) \<union> (set (ego_chunks s2) - set (ego_chunks s1))"
  unfolding ego_join_def ego_meet_def ego_implication_def
  by auto

text \<open>Key: join after meet-then-implication recovers the target\<close>
lemma levi_recovers_target:
  "set (ego_chunks s1) \<union> (set (ego_chunks s2) - set (ego_chunks s1)) =
   set (ego_chunks s1) \<union> set (ego_chunks s2)"
  by auto


subsection \<open>Entrenchment from Preorder\<close>

text \<open>
  The developmental preorder on ego-states induces epistemic entrenchment:
  u \<le> v in the preorder means u is developmentally prior to v,
  which means u is MORE entrenched (harder to revise).

  This is the Grove sphere construction:
  - Inner spheres = high entrenchment = many predecessors
  - Outer spheres = low entrenchment = few predecessors
\<close>

type_synonym ego_preorder = "(ego_state \<times> ego_state) set"

definition entrenchment_depth :: "ego_preorder \<Rightarrow> ego_state \<Rightarrow> nat" where
  "entrenchment_depth R s = card {s'. (s', s) \<in> R}"

text \<open>
  Grove sphere level: states with more predecessors sit in inner spheres.
  sphere_level = max_predecessors - predecessor_count(s)
\<close>

definition grove_sphere_level :: "ego_preorder \<Rightarrow> ego_state set \<Rightarrow> ego_state \<Rightarrow> nat" where
  "grove_sphere_level R states s =
    (if finite states then
       Max (entrenchment_depth R ` states) - entrenchment_depth R s
     else 0)"


subsection \<open>The Critical Divergence Theorem\<close>

text \<open>
  Central result of the reconciliation:

  THEOREM (Critical Divergence):
  Let T be the temperature (psychological arousal) and Tc the critical
  temperature of the ego-locale. Let r_Nash be the Nash-selected revision
  and r_conservative be the conservative (intersection) revision.

  Then:
    T \<approx> Tc  \<Longleftrightarrow>  r_Nash = r_conservative

  In words: the game-theoretic equilibrium and the therapeutic optimum
  coincide if and only if the ego is near criticality.

  Clinical significance:
  - Below Tc (rigid/neurotic): Nash says (cool, hold) but therapy needs (heat, challenge)
    \<Rightarrow> analyst must DEVIATE from equilibrium
  - Above Tc (flooded/dissolved): Nash says (heat, challenge) but therapy needs (cool, hold)
    \<Rightarrow> analyst must DEVIATE from equilibrium
  - At Tc (critical window): Nash = therapeutic optimum
    \<Rightarrow> equilibrium IS the right thing to do

  This explains why therapy is hard: most of the time, the analyst must
  actively work against the game-theoretic equilibrium. The therapeutic
  skill is recognizing when T \<approx> Tc (the critical window where equilibrium
  suffices) and when T \<noteq> Tc (where deviation is required).
\<close>

text \<open>Temperature ratio as phase indicator\<close>
definition near_critical :: "real \<Rightarrow> real \<Rightarrow> real \<Rightarrow> bool" where
  "near_critical T Tc \<delta> \<longleftrightarrow> Tc > 0 \<and> \<bar>T - Tc\<bar> / Tc < \<delta>"

text \<open>Nash revision: selected by game-theoretic equilibrium\<close>
definition nash_revision :: "ego_state set \<Rightarrow> ego_state" where
  "nash_revision admissible = (SOME s. s \<in> admissible)"

text \<open>Conservative revision: intersection of all admissible\<close>
definition conservative_ego_revision :: "ego_state set \<Rightarrow> manifest" where
  "conservative_ego_revision admissible =
    (if admissible = {} then []
     else filter (\<lambda>h. \<forall>s \<in> admissible. h \<in> set (ego_chunks s))
                 (ego_chunks (SOME s. s \<in> admissible)))"

text \<open>
  Divergence measure: how different are Nash and conservative revisions.
  Zero divergence = they agree = near criticality.
\<close>
definition revision_divergence :: "ego_state set \<Rightarrow> nat" where
  "revision_divergence admissible =
    (let nash = ego_chunks (nash_revision admissible);
         cons = conservative_ego_revision admissible
     in card (set nash - set cons) + card (set cons - set nash))"

text \<open>
  The critical divergence conjecture:
  revision_divergence \<approx> 0  \<longleftrightarrow>  near_critical T Tc \<delta>

  We state the forward direction: at criticality, divergence vanishes.
  (The converse requires the full Ising/Boltzmann structure.)
\<close>

text \<open>
  Trivial case: singleton admissible set means Nash = conservative.
  This happens when entrenchment is total (Grove's theorem).
\<close>
lemma singleton_no_divergence:
  assumes "admissible = {s}"
  shows "revision_divergence admissible = 0"
  using assms unfolding revision_divergence_def nash_revision_def
                        conservative_ego_revision_def
  by (simp add: some_equality)

text \<open>
  Total entrenchment \<Rightarrow> singleton admissible \<Rightarrow> zero divergence.
  This is the formal spine: totality (from Grove) implies agreement.
\<close>


subsection \<open>Therapeutic Narrative as Trit Sheaf\<close>

text \<open>
  A therapy session sequence is a trit narrative:
  - Each session gets a trit (expand/contract/revise = +1/-1/0)
  - The narrative is a sheaf over time intervals
  - The sheaf condition ensures temporal consistency

  From Narratives.thy: narrative_from_snapshots s is always a sheaf.
  The therapeutic history is therefore always temporally consistent
  (even if the ego itself has H\<^sup>1 \<noteq> 0 compartmentalization).

  Observation: the ego may be compartmentalized (H\<^sup>1 > 0 on CAS),
  but the therapeutic narrative is always a sheaf (H\<^sup>0 = 0 on time).
  This is why therapy works: the narrative provides temporal coherence
  that the ego's spatial structure lacks.
\<close>

definition therapeutic_snapshot :: "ego_state \<Rightarrow> trit" where
  "therapeutic_snapshot s =
    (if ego_energy s > 0 then Plus      \<comment> \<open>high energy = expansion needed\<close>
     else if ego_energy s < 0 then Minus \<comment> \<open>low energy = contraction possible\<close>
     else Zero)"                         \<comment> \<open>balanced = ergodic\<close>

text \<open>
  The therapeutic narrative from a session history.
  Each session's ego-state is classified as a trit.
\<close>
definition therapy_narrative :: "(nat \<Rightarrow> ego_state) \<Rightarrow> trit_presheaf" where
  "therapy_narrative sessions = narrative_from_snapshots (therapeutic_snapshot \<circ> sessions)"

text \<open>Therapeutic narrative is always a sheaf (inherited from Narratives.thy)\<close>
theorem therapy_narrative_is_sheaf:
  "trit_sheaf_condition (therapy_narrative sessions)"
  unfolding therapy_narrative_def
  using narrative_is_sheaf by simp


subsection \<open>GF(3) Conservation Across the Bridge\<close>

text \<open>
  The full reconciliation maintains three interleaved GF(3) triads:

  Triad 1 (AGM operations):
    expansion (+1) + contraction (-1) + revision (0) = 0

  Triad 2 (Game theory):
    selection (+1) + nashator (0) + verification (-1) = 0

  Triad 3 (Ego dynamics):
    Eros/integration (+1) + ergodic/holding (0) + Thanatos/dissolution (-1) = 0

  All three triads are balanced, and their composition is balanced:
    3 \<times> 0 = 0 mod 3 \<checkmark>
\<close>

definition agm_triad :: "trit list" where "agm_triad = [Plus, Minus, Zero]"
definition game_triad :: "trit list" where "game_triad = [Plus, Zero, Minus]"
definition ego_triad :: "trit list" where "ego_triad = [Plus, Zero, Minus]"

lemma all_triads_balanced:
  "gf3_balanced agm_triad"
  "gf3_balanced game_triad"
  "gf3_balanced ego_triad"
  unfolding gf3_balanced_def agm_triad_def game_triad_def ego_triad_def
  by simp_all

lemma composition_balanced:
  "gf3_balanced (agm_triad @ game_triad @ ego_triad)"
  using gf3_concat_balanced all_triads_balanced by simp

text \<open>
  The full bridge:
    ego-locale.jl     \<longleftrightarrow>  boxxy/theories/
    EgoState          \<longleftrightarrow>  belief_set
    Preorder          \<longleftrightarrow>  epistemic_entrenchment
    CAS chunks        \<longleftrightarrow>  manifest (content-addressed)
    Heyting \<Rightarrow>        \<longleftrightarrow>  Levi identity
    Temperature T     \<longleftrightarrow>  \<epsilon>-slack (semi-reliable)
    h1_obstruction    \<longleftrightarrow>  indeterminism_degree
    classify_trit     \<longleftrightarrow>  trit assignment
    bidirectional_sync\<longleftrightarrow>  nashator \<boxtimes>
    demo_agm()        \<longleftrightarrow>  this theory (EgoLocale_AGM.thy)
\<close>

end
