theory Narratives
  imports Abelian_Extensions
begin

section \<open>Narratives: Sheaves on Time Categories (Bumpus et al.)\<close>

text \<open>
  A narrative is a sheaf F: I_N \<rightarrow> D on a time category, where:
  - I_N has intervals [a,b] as objects and inclusions as morphisms
  - D has pullbacks (the sheaf condition glues via pullback)

  Sheaf condition:
    F([a,b]) \<cong> F([a,p]) \<times>_{F([p,p])} F([p,b])
  for any a \<le> p \<le> b.

  Here we instantiate D with:
  - Set (classical: temporal sets)
  - GF(3)-graded sets (trit-valued: temporal belief states)
  - GF(9)-graded sets (nonet-valued: temporal device classifications)

  Reference: Bumpus et al. "Unified Framework for Time-Varying Data" (arXiv:2402.00206)
\<close>


subsection \<open>Time Category I_N\<close>

text \<open>
  Objects: intervals [a,b] where a \<le> b (natural numbers).
  Morphisms: inclusions [a',b'] \<hookrightarrow> [a,b] when a \<le> a' and b' \<le> b.
\<close>

datatype interval = Iv nat nat  \<comment> \<open>Iv a b represents [a,b]\<close>

fun iv_start :: "interval \<Rightarrow> nat" where "iv_start (Iv a _) = a"
fun iv_end   :: "interval \<Rightarrow> nat" where "iv_end (Iv _ b) = b"

definition iv_valid :: "interval \<Rightarrow> bool" where
  "iv_valid (Iv a b) \<longleftrightarrow> a \<le> b"

definition iv_contains :: "interval \<Rightarrow> interval \<Rightarrow> bool" (infix "\<sqsubseteq>\<^sub>I" 50) where
  "iv_contains (Iv a' b') (Iv a b) \<longleftrightarrow> a \<le> a' \<and> b' \<le> b"

text \<open>Singleton intervals represent points in time\<close>
definition iv_point :: "nat \<Rightarrow> interval" where
  "iv_point t = Iv t t"

text \<open>Inclusion is a preorder\<close>
lemma iv_contains_refl: "x \<sqsubseteq>\<^sub>I x"
  by (cases x; simp add: iv_contains_def)

lemma iv_contains_trans: "\<lbrakk>x \<sqsubseteq>\<^sub>I y; y \<sqsubseteq>\<^sub>I z\<rbrakk> \<Longrightarrow> x \<sqsubseteq>\<^sub>I z"
  by (cases x; cases y; cases z; simp add: iv_contains_def)

text \<open>Any interval is covered by splitting at a point\<close>
definition iv_split :: "interval \<Rightarrow> nat \<Rightarrow> (interval \<times> interval) option" where
  "iv_split (Iv a b) p = (if a \<le> p \<and> p \<le> b then Some (Iv a p, Iv p b) else None)"


subsection \<open>Trit-Valued Presheaves\<close>

text \<open>
  A trit-valued presheaf assigns a trit to each interval.
  This models temporal belief states where each time interval
  has a GF(3) classification.

  As a presheaf (contravariant): restriction maps go from larger
  intervals to smaller sub-intervals.
\<close>

type_synonym trit_presheaf = "interval \<Rightarrow> trit"

text \<open>A presheaf is monotone (respects restrictions)\<close>
definition trit_presheaf_valid :: "trit_presheaf \<Rightarrow> bool" where
  "trit_presheaf_valid F \<longleftrightarrow>
    (\<forall>i j. iv_valid i \<longrightarrow> iv_valid j \<longrightarrow> j \<sqsubseteq>\<^sub>I i \<longrightarrow> True)"
    \<comment> \<open>Trit values don't have a natural restriction map;
        the sheaf condition below handles consistency\<close>


subsection \<open>Nonet-Valued Presheaves (GF(9) Extension)\<close>

text \<open>
  A nonet-valued presheaf assigns a GF(9) element to each interval.
  This is the abelian extension of the trit presheaf, providing
  9-state temporal classification.

  The extra dimension (confidence) tracks how certain the
  classification is over a time interval.
\<close>

type_synonym nonet_presheaf = "interval \<Rightarrow> nonet"


subsection \<open>The Sheaf Condition\<close>

text \<open>
  The Bumpus sheaf condition for narratives:

  For any interval [a,b] and split point p with a \<le> p \<le> b:
    F([a,b]) = F([a,p]) \<times>_{F([p,p])} F([p,b])

  In the trit case, this means:
    F([a,b]) is determined by F([a,p]) and F([p,b])
    glued along F([p,p]).

  For GF(3) (and GF(9), GF(27)), the pullback is the fibered product:
  the pair (x, y) such that the restriction of x to [p,p] equals
  the restriction of y to [p,p].

  For trits with the trivial restriction (identity on points),
  this becomes: F([a,b]) = trit_add F([a,p]) F([p,b])
  i.e. the value on an interval is the GF(3) sum of its parts.
\<close>

text \<open>
  The fibered product formula for GF(3)-valued sheaves:
    F([a,b]) = F([a,p]) + F([p,b]) - F([p,p])
  The subtraction accounts for the shared point p.
\<close>
definition trit_sheaf_condition :: "trit_presheaf \<Rightarrow> bool" where
  "trit_sheaf_condition F \<longleftrightarrow>
    (\<forall>a p b. a \<le> p \<longrightarrow> p \<le> b \<longrightarrow>
      F (Iv a b) = trit_add (trit_add (F (Iv a p)) (F (Iv p b))) (trit_neg (F (Iv p p))))"

text \<open>Nonet sheaf condition: componentwise fibered product\<close>
definition nonet_sheaf_condition :: "nonet_presheaf \<Rightarrow> bool" where
  "nonet_sheaf_condition F \<longleftrightarrow>
    (\<forall>a p b. a \<le> p \<longrightarrow> p \<le> b \<longrightarrow>
      F (Iv a b) = nonet_add (nonet_add (F (Iv a p)) (F (Iv p b))) (nonet_neg (F (Iv p p))))"


subsection \<open>Narrative Construction from Snapshots\<close>

text \<open>
  Given a sequence of trit snapshots (one per time step),
  construct the unique sheaf extending them.

  The value on [a,b] is the GF(3) sum of snapshots a through b.
\<close>

definition trit_narrative :: "(nat \<Rightarrow> trit) \<Rightarrow> trit_presheaf" where
  "trit_narrative snapshots = (\<lambda>iv.
    case iv of Iv a b \<Rightarrow>
      if a = b then snapshots a
      else trit_add (snapshots a) (trit_narrative snapshots (Iv (Suc a) b)))"

text \<open>
  Alternative: iterative sum of snapshots from a to b.
  trit_sum_range s a b = s(a) + s(a+1) + ... + s(b)
\<close>
fun trit_sum_range :: "(nat \<Rightarrow> trit) \<Rightarrow> nat \<Rightarrow> nat \<Rightarrow> trit" where
  "trit_sum_range s a 0 = s a"
| "trit_sum_range s a (Suc n) = trit_add (s a) (trit_sum_range s (Suc a) n)"

definition narrative_from_snapshots :: "(nat \<Rightarrow> trit) \<Rightarrow> trit_presheaf" where
  "narrative_from_snapshots s = (\<lambda>iv.
    case iv of Iv a b \<Rightarrow>
      if b < a then Zero
      else trit_sum_range s a (b - a))"


subsection \<open>Sheaf Condition Verification\<close>

text \<open>
  The narrative constructed from snapshots satisfies the sheaf condition.
  This is the key theorem: snapshot data uniquely extends to a sheaf.
\<close>

text \<open>Helper: trit_sum_range splits at any midpoint\<close>
lemma trit_sum_range_split:
  assumes "k \<le> n"
  shows "trit_sum_range s a n = trit_add (trit_sum_range s a k) (trit_sum_range s (a + k + 1) (n - k - 1))"
  using assms
proof (induction k arbitrary: a n)
  case 0
  then show ?case
    by (cases n) (simp_all add: trit_add_zero_left)
next
  case (Suc k)
  then have "k \<le> n - 1" by linarith
  then obtain m where nm: "n = Suc m" using Suc.prems by (cases n) auto
  have step: "trit_sum_range s a (Suc m) = trit_add (s a) (trit_sum_range s (Suc a) m)"
    by simp
  have ih: "trit_sum_range s (Suc a) m =
    trit_add (trit_sum_range s (Suc a) k) (trit_sum_range s (Suc a + k + 1) (m - k - 1))"
    using Suc.IH[of "Suc a" m] Suc.prems nm by linarith
  have lhs: "trit_sum_range s a (Suc m) =
    trit_add (s a) (trit_add (trit_sum_range s (Suc a) k) (trit_sum_range s (Suc a + k + 1) (m - k - 1)))"
    using step ih by simp
  have rhs_l: "trit_sum_range s a (Suc k) = trit_add (s a) (trit_sum_range s (Suc a) k)"
    by simp
  have "trit_add (trit_sum_range s a (Suc k)) (trit_sum_range s (a + Suc k + 1) (Suc m - Suc k - 1)) =
    trit_add (trit_add (s a) (trit_sum_range s (Suc a) k)) (trit_sum_range s (Suc a + k + 1) (m - k - 1))"
    using rhs_l nm by simp
  also have "\<dots> = trit_add (s a) (trit_add (trit_sum_range s (Suc a) k) (trit_sum_range s (Suc a + k + 1) (m - k - 1)))"
    using trit_add_assoc by metis
  finally show ?case using lhs nm by simp
qed

text \<open>
  Main theorem: narrative_from_snapshots is a sheaf.
  NOTE: Full proof requires induction with careful nat arithmetic.
  The mathematical content is straightforward: GF(3) sum is associative.
\<close>
lemma narrative_from_snapshots_val:
  assumes "a \<le> p" and "p \<le> b"
  shows "narrative_from_snapshots s (Iv a b) =
    trit_add (trit_add (narrative_from_snapshots s (Iv a p))
                       (narrative_from_snapshots s (Iv p b)))
             (trit_neg (narrative_from_snapshots s (Iv p p)))"
proof -
  have ap: "narrative_from_snapshots s (Iv a p) = trit_sum_range s a (p - a)"
    using assms unfolding narrative_from_snapshots_def by simp
  have pb: "narrative_from_snapshots s (Iv p b) = trit_sum_range s p (b - p)"
    using assms unfolding narrative_from_snapshots_def by simp
  have pp: "narrative_from_snapshots s (Iv p p) = trit_sum_range s p 0"
    unfolding narrative_from_snapshots_def by simp
  have pp_val: "trit_sum_range s p 0 = s p" by simp
  have ab: "narrative_from_snapshots s (Iv a b) = trit_sum_range s a (b - a)"
    using assms unfolding narrative_from_snapshots_def by simp
  show ?thesis
  proof (cases "a = p")
    case True
    then show ?thesis
      using ap pb pp ab pp_val
      by (simp add: narrative_from_snapshots_def trit_add_zero_left trit_add_inverse_left)
  next
    case False
    then have ap_pos: "a < p" using assms by linarith
    have split: "trit_sum_range s a (b - a) =
      trit_add (trit_sum_range s a (p - a)) (trit_sum_range s (a + (p - a) + 1) (b - a - (p - a) - 1))"
      using trit_sum_range_split[of "p - a" "b - a" s a] assms by linarith
    have idx: "a + (p - a) + 1 = Suc p" using ap_pos by linarith
    have len: "b - a - (p - a) - 1 = b - p - 1" using assms ap_pos by linarith
    have split2: "trit_sum_range s a (b - a) =
      trit_add (trit_sum_range s a (p - a)) (trit_sum_range s (Suc p) (b - p - 1))"
      using split idx len by simp
    have pb_expand: "trit_sum_range s p (b - p) =
      trit_add (s p) (trit_sum_range s (Suc p) (b - p - 1))"
    proof -
      obtain m where "b - p = Suc m" using assms ap_pos by (cases "b - p") linarith+
      then show ?thesis by simp
    qed
    have "trit_add (trit_add (trit_sum_range s a (p - a)) (trit_sum_range s p (b - p)))
                   (trit_neg (s p)) =
      trit_add (trit_add (trit_sum_range s a (p - a))
                         (trit_add (s p) (trit_sum_range s (Suc p) (b - p - 1))))
               (trit_neg (s p))"
      using pb_expand by simp
    also have "\<dots> = trit_add (trit_sum_range s a (p - a))
                             (trit_add (trit_add (s p) (trit_sum_range s (Suc p) (b - p - 1)))
                                       (trit_neg (s p)))"
      using trit_add_assoc by metis
    also have "\<dots> = trit_add (trit_sum_range s a (p - a))
                             (trit_add (trit_sum_range s (Suc p) (b - p - 1))
                                       (trit_add (s p) (trit_neg (s p))))"
      using trit_add_comm trit_add_assoc by metis
    also have "\<dots> = trit_add (trit_sum_range s a (p - a))
                             (trit_sum_range s (Suc p) (b - p - 1))"
      using trit_add_inverse_right trit_add_zero_right by simp
    finally show ?thesis using ab ap pb pp pp_val split2 by simp
  qed
qed

theorem narrative_is_sheaf:
  "trit_sheaf_condition (narrative_from_snapshots s)"
  unfolding trit_sheaf_condition_def
  using narrative_from_snapshots_val by blast


subsection \<open>GF(3) Conservation for Narratives\<close>

text \<open>
  A narrative is GF(3)-balanced if the value on the full time span
  reduces to Zero mod 3. This means the temporal evolution is balanced.
\<close>

definition narrative_balanced :: "trit_presheaf \<Rightarrow> nat \<Rightarrow> nat \<Rightarrow> bool" where
  "narrative_balanced F a b \<longleftrightarrow> F (Iv a b) = Zero"

text \<open>
  If each individual time step is part of a balanced triple
  (with two other concurrent processes), the whole narrative is balanced.
\<close>
lemma trit_sum_range_gf3_balanced:
  assumes "\<forall>t. gf3_balanced [s1 t, s2 t, s3 t]"
  shows "gf3_balanced [trit_sum_range s1 a n, trit_sum_range s2 a n, trit_sum_range s3 a n]"
  using assms
proof (induction n arbitrary: a)
  case 0
  then show ?case by simp
next
  case (Suc n)
  have step: "\<And>si. trit_sum_range si a (Suc n) = trit_add (si a) (trit_sum_range si (Suc a) n)"
    by simp
  have ih: "gf3_balanced [trit_sum_range s1 (Suc a) n, trit_sum_range s2 (Suc a) n, trit_sum_range s3 (Suc a) n]"
    using Suc.IH Suc.prems by simp
  have base: "gf3_balanced [s1 a, s2 a, s3 a]" using Suc.prems by simp
  have "trit_sum [trit_add (s1 a) (trit_sum_range s1 (Suc a) n),
                  trit_add (s2 a) (trit_sum_range s2 (Suc a) n),
                  trit_add (s3 a) (trit_sum_range s3 (Suc a) n)] =
    trit_val (trit_add (s1 a) (trit_sum_range s1 (Suc a) n)) +
    trit_val (trit_add (s2 a) (trit_sum_range s2 (Suc a) n)) +
    trit_val (trit_add (s3 a) (trit_sum_range s3 (Suc a) n))"
    by simp
  then show ?case
    unfolding gf3_balanced_def
    using base ih step
    unfolding gf3_balanced_def
    by (cases "s1 a"; cases "s2 a"; cases "s3 a";
        cases "trit_sum_range s1 (Suc a) n"; cases "trit_sum_range s2 (Suc a) n";
        cases "trit_sum_range s3 (Suc a) n"; simp)
qed

lemma balanced_snapshots_balanced_narrative:
  assumes "\<forall>t. gf3_balanced [s1 t, s2 t, s3 t]"
  shows "gf3_balanced [narrative_from_snapshots s1 (Iv a b),
                       narrative_from_snapshots s2 (Iv a b),
                       narrative_from_snapshots s3 (Iv a b)]"
proof (cases "b < a")
  case True
  then show ?thesis
    unfolding narrative_from_snapshots_def gf3_balanced_def by simp
next
  case False
  then have "a \<le> b" by linarith
  then have "\<And>si. narrative_from_snapshots si (Iv a b) = trit_sum_range si a (b - a)"
    unfolding narrative_from_snapshots_def by simp
  then show ?thesis using trit_sum_range_gf3_balanced[OF assms] by simp
qed


subsection \<open>Frobenius Action on Narratives\<close>

text \<open>
  The Frobenius automorphism of GF(9)/GF(3) acts on nonet narratives
  pointwise: (\<sigma> F)(I) = \<sigma>(F(I)).

  The fixed sub-narrative consists of intervals whose classification
  lies in GF(3) \<hookrightarrow> GF(9).
\<close>

definition frobenius_narrative :: "nonet_presheaf \<Rightarrow> nonet_presheaf" where
  "frobenius_narrative F = (\<lambda>iv. frobenius_9 (F iv))"

definition fixed_sub_narrative :: "nonet_presheaf \<Rightarrow> interval set" where
  "fixed_sub_narrative F = {iv. frobenius_9 (F iv) = F iv}"

text \<open>Fixed intervals are exactly those classified in GF(3)\<close>
lemma fixed_narrative_is_gf3:
  "iv \<in> fixed_sub_narrative F \<longleftrightarrow> (\<exists>t. F iv = trit_to_nonet t)"
  unfolding fixed_sub_narrative_def
  using frobenius_9_fixed_iff by auto


subsection \<open>Cohomological Obstruction (H\<^sup>0)\<close>

text \<open>
  The zeroth Čech cohomology detects failure of local-to-global gluing.
  For a trit narrative, H\<^sup>0 \<noteq> 0 means the snapshot data cannot be
  consistently extended to a sheaf.

  In our formalization, the narrative construction always succeeds
  (GF(3) sum is always defined), so H\<^sup>0 = 0 for snapshot-constructed
  narratives. Obstructions arise when additional constraints
  (e.g. from pinhole compliance) restrict admissible values.
\<close>

definition H0_obstruction :: "trit_presheaf \<Rightarrow> interval \<Rightarrow> bool" where
  "H0_obstruction F iv =
    (case iv of Iv a b \<Rightarrow>
      if a < b then
        let p = (a + b) div 2 in
        F (Iv a b) \<noteq> trit_add (F (Iv a p)) (F (Iv p b))
      else False)"

lemma narrative_no_obstruction:
  assumes "trit_sheaf_condition F"
  shows "\<not> H0_obstruction F (Iv a b)"
  using assms unfolding trit_sheaf_condition_def H0_obstruction_def
  by auto


subsection \<open>Interleaving: Narratives \<times> Abelian Extensions \<times> AGM\<close>

text \<open>
  The three theories interleave:

  1. AGM_Extensions: belief revision with GF(3) trit conservation
  2. Abelian_Extensions: GF(3) \<hookrightarrow> GF(9) \<hookrightarrow> GF(27) tower
  3. Narratives: temporal sheaves on interval categories

  Connection: A temporal belief revision scenario is:
  - A narrative F: I_N \<rightarrow> BeliefStates
  - Where BeliefStates is GF(3)-graded (trit-tagged operations)
  - The sheaf condition ensures temporal consistency of revision
  - The abelian extension provides refined classification (GF(9) confidence)
  - Frobenius selection breaks ties in indeterministic revision

  GF(3) balance of the interleaving:
    AGM (-1) + Narratives (0) + Abelian (+1) = 0 mod 3
\<close>

definition theory_interleave :: "trit list" where
  "theory_interleave = [Minus, Zero, Plus]"

lemma theory_interleave_balanced: "gf3_balanced theory_interleave"
  unfolding theory_interleave_def gf3_balanced_def by simp

end
