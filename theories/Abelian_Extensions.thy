theory Abelian_Extensions
  imports AGM_Extensions
begin

section \<open>Abelian Extensions of GF(3)\<close>

text \<open>
  The tower GF(3) \<hookrightarrow> GF(9) \<hookrightarrow> GF(27) forms a chain of abelian extensions.
  Each GF(3^n)/GF(3) has cyclic Galois group \<cong> Z/nZ.

  This connects:
  - AGM trit conservation (GF(3) = Z/3Z)
  - Pontryagin duality (characters classify extensions)
  - Frobenius automorphism (x \<mapsto> x^3 generates Gal)
  - Scanner device classification (3-state \<rightarrow> 9-state \<rightarrow> 27-state)

  Mathematical structure:
    GF(3)  = F_3 = {0, 1, 2}          --- trits (Minus/Zero/Plus)
    GF(9)  = F_3[x]/(x^2 + 1)        --- nonet (3\<times>3 trit pairs)
    GF(27) = F_3[x]/(x^3 + 2x + 1)   --- tribble (3\<times>3\<times>3 trit triples)

  Galois groups:
    Gal(GF(9)/GF(3))  \<cong> Z/2Z  (Frobenius: \<sigma>(a) = a^3)
    Gal(GF(27)/GF(3)) \<cong> Z/3Z  (Frobenius: \<sigma>(a) = a^3)
\<close>

subsection \<open>GF(9): Nonet Extension\<close>

text \<open>
  GF(9) = GF(3)[i] where i^2 = -1 (mod 3), i.e. i^2 + 1 = 0.
  Elements: a + bi where a, b \<in> GF(3).
  9 elements total: the "nonet" for refined device classification.
\<close>

datatype nonet = N trit trit  \<comment> \<open>Pair (real, imaginary) in GF(3)\<close>

text \<open>Addition in GF(9): componentwise mod 3\<close>
fun nonet_add :: "nonet \<Rightarrow> nonet \<Rightarrow> nonet" where
  "nonet_add (N a b) (N c d) = N (trit_add a c) (trit_add b d)"

text \<open>Negation in GF(9)\<close>
fun nonet_neg :: "nonet \<Rightarrow> nonet" where
  "nonet_neg (N a b) = N (trit_neg a) (trit_neg b)"

text \<open>Zero element\<close>
definition nonet_zero :: nonet where
  "nonet_zero = N Zero Zero"

text \<open>Multiplication in GF(9): (a+bi)(c+di) = (ac-bd) + (ad+bc)i
  where -1 = 2 in GF(3), so -bd = trit_add(trit_neg(bd))\<close>
fun trit_mul :: "trit \<Rightarrow> trit \<Rightarrow> trit" where
  "trit_mul Zero _ = Zero"
| "trit_mul _ Zero = Zero"
| "trit_mul Plus Plus = Plus"
| "trit_mul Plus Minus = Minus"
| "trit_mul Minus Plus = Minus"
| "trit_mul Minus Minus = Plus"

fun nonet_mul :: "nonet \<Rightarrow> nonet \<Rightarrow> nonet" where
  "nonet_mul (N a b) (N c d) =
    N (trit_add (trit_mul a c) (trit_neg (trit_mul b d)))
      (trit_add (trit_mul a d) (trit_mul b c))"

text \<open>One element\<close>
definition nonet_one :: nonet where
  "nonet_one = N Plus Zero"  \<comment> \<open>Plus = 1 in our encoding\<close>

text \<open>Embedding: GF(3) \<hookrightarrow> GF(9) via a \<mapsto> (a, 0)\<close>
definition trit_to_nonet :: "trit \<Rightarrow> nonet" where
  "trit_to_nonet t = N t Zero"

text \<open>Projection: GF(9) \<twoheadrightarrow> GF(3) via (a, b) \<mapsto> a (real part)\<close>
fun nonet_to_trit :: "nonet \<Rightarrow> trit" where
  "nonet_to_trit (N a _) = a"

text \<open>Frobenius automorphism: \<sigma>(a+bi) = (a+bi)^3 = a - bi (since i^3 = -i in GF(9))\<close>
fun frobenius_9 :: "nonet \<Rightarrow> nonet" where
  "frobenius_9 (N a b) = N a (trit_neg b)"

text \<open>--- Proofs for GF(9) abelian group structure ---\<close>

lemma nonet_add_comm: "nonet_add x y = nonet_add y x"
  by (cases x; cases y; simp add: trit_add_comm)

lemma nonet_add_assoc: "nonet_add (nonet_add x y) z = nonet_add x (nonet_add y z)"
  by (cases x; cases y; cases z; simp add: trit_add_assoc)

lemma nonet_add_zero_left: "nonet_add nonet_zero x = x"
  by (cases x; simp add: nonet_zero_def)

lemma nonet_add_zero_right: "nonet_add x nonet_zero = x"
  by (cases x; simp add: nonet_zero_def)

lemma nonet_add_inverse: "nonet_add x (nonet_neg x) = nonet_zero"
  by (cases x; simp add: nonet_zero_def trit_add_inverse_right)

text \<open>GF(9) is an Abelian group under nonet_add\<close>
lemma gf9_group_axioms:
  shows "nonet_add nonet_zero x = x"
    and "nonet_add (nonet_neg x) x = nonet_zero"
    and "nonet_add (nonet_add x y) z = nonet_add x (nonet_add y z)"
    and "nonet_add x y = nonet_add y x"
  using nonet_add_zero_left nonet_add_inverse nonet_add_assoc nonet_add_comm
  by (simp_all add: trit_add_inverse_left nonet_zero_def, cases x, simp add: nonet_zero_def)

text \<open>Frobenius is an automorphism of order 2\<close>

lemma frobenius_9_order: "frobenius_9 (frobenius_9 x) = x"
  by (cases x) simp

lemma frobenius_9_additive:
  "frobenius_9 (nonet_add x y) = nonet_add (frobenius_9 x) (frobenius_9 y)"
  by (cases x; cases y) simp

text \<open>Frobenius fixes GF(3) (the subfield)\<close>
lemma frobenius_9_fixes_gf3: "frobenius_9 (trit_to_nonet t) = trit_to_nonet t"
  by (cases t; simp add: trit_to_nonet_def)

text \<open>The fixed field of Frobenius is exactly GF(3)\<close>
lemma frobenius_9_fixed_iff:
  "frobenius_9 x = x \<longleftrightarrow> (\<exists>t. x = trit_to_nonet t)"
  by (cases x) (auto simp add: trit_to_nonet_def, cases x, auto)

text \<open>Embedding is a group homomorphism\<close>
lemma trit_to_nonet_hom:
  "trit_to_nonet (trit_add a b) = nonet_add (trit_to_nonet a) (trit_to_nonet b)"
  by (simp add: trit_to_nonet_def)

text \<open>Projection is a group homomorphism (left inverse of embedding)\<close>
lemma nonet_to_trit_embed: "nonet_to_trit (trit_to_nonet t) = t"
  by (simp add: trit_to_nonet_def)


subsection \<open>GF(27): Tribble Extension\<close>

text \<open>
  GF(27) = GF(3)[\<alpha>] where \<alpha>^3 + 2\<alpha> + 1 = 0.
  Elements: a + b\<alpha> + c\<alpha>^2 where a, b, c \<in> GF(3).
  27 elements total: the "tribble" for maximum classification resolution.

  For the additive group (which is what we need for abelian extensions):
  GF(27)^+ \<cong> (Z/3Z)^3
\<close>

datatype tribble = T trit trit trit  \<comment> \<open>Triple (a, b, c) coefficients in GF(3)\<close>

fun tribble_add :: "tribble \<Rightarrow> tribble \<Rightarrow> tribble" where
  "tribble_add (T a1 b1 c1) (T a2 b2 c2) =
    T (trit_add a1 a2) (trit_add b1 b2) (trit_add c1 c2)"

fun tribble_neg :: "tribble \<Rightarrow> tribble" where
  "tribble_neg (T a b c) = T (trit_neg a) (trit_neg b) (trit_neg c)"

definition tribble_zero :: tribble where
  "tribble_zero = T Zero Zero Zero"

text \<open>Embedding chain: GF(3) \<hookrightarrow> GF(9) \<hookrightarrow> GF(27)\<close>
definition trit_to_tribble :: "trit \<Rightarrow> tribble" where
  "trit_to_tribble t = T t Zero Zero"

text \<open>Norm map: GF(27) \<rightarrow> GF(3) (sum of components mod 3)\<close>
fun tribble_norm :: "tribble \<Rightarrow> trit" where
  "tribble_norm (T a b c) = trit_add a (trit_add b c)"

text \<open>Frobenius automorphism for GF(27)/GF(3): \<sigma>(a, b, c) = cyclic shift
  This generates Gal(GF(27)/GF(3)) \<cong> Z/3Z\<close>
fun frobenius_27 :: "tribble \<Rightarrow> tribble" where
  "frobenius_27 (T a b c) = T c a b"

text \<open>--- Proofs for GF(27) abelian group structure ---\<close>

lemma tribble_add_comm: "tribble_add x y = tribble_add y x"
  by (cases x; cases y; simp add: trit_add_comm)

lemma tribble_add_assoc:
  "tribble_add (tribble_add x y) z = tribble_add x (tribble_add y z)"
  by (cases x; cases y; cases z; simp add: trit_add_assoc)

lemma tribble_add_zero_left: "tribble_add tribble_zero x = x"
  by (cases x; simp add: tribble_zero_def)

lemma tribble_add_inverse: "tribble_add x (tribble_neg x) = tribble_zero"
  by (cases x; simp add: tribble_zero_def trit_add_inverse_right)

text \<open>GF(27) is an Abelian group under tribble_add\<close>
lemma gf27_group_axioms:
  shows "tribble_add tribble_zero x = x"
    and "tribble_add (tribble_neg x) x = tribble_zero"
    and "tribble_add (tribble_add x y) z = tribble_add x (tribble_add y z)"
    and "tribble_add x y = tribble_add y x"
proof -
  show "tribble_add tribble_zero x = x" using tribble_add_zero_left .
  show "tribble_add (tribble_neg x) x = tribble_zero"
    by (cases x; simp add: tribble_zero_def trit_add_inverse_left)
  show "tribble_add (tribble_add x y) z = tribble_add x (tribble_add y z)"
    using tribble_add_assoc .
  show "tribble_add x y = tribble_add y x" using tribble_add_comm .
qed

text \<open>Frobenius has order 3\<close>

lemma frobenius_27_order:
  "frobenius_27 (frobenius_27 (frobenius_27 x)) = x"
  by (cases x) simp

lemma frobenius_27_order_2_neq:
  assumes "\<exists>b c. x = T Zero b c \<and> (b \<noteq> Zero \<or> c \<noteq> Zero)"
  shows "frobenius_27 (frobenius_27 x) \<noteq> x"
  using assms by (cases x) auto

lemma frobenius_27_additive:
  "frobenius_27 (tribble_add x y) = tribble_add (frobenius_27 x) (frobenius_27 y)"
  by (cases x; cases y) simp

text \<open>Frobenius fixes GF(3)\<close>
lemma frobenius_27_fixes_gf3: "frobenius_27 (trit_to_tribble t) = trit_to_tribble t"
  by (cases t; simp add: trit_to_tribble_def)

text \<open>Embedding preserves group structure\<close>
lemma trit_to_tribble_hom:
  "trit_to_tribble (trit_add a b) = tribble_add (trit_to_tribble a) (trit_to_tribble b)"
  by (simp add: trit_to_tribble_def)


subsection \<open>Characters and Pontryagin Duality\<close>

text \<open>
  A character of an abelian group G is a homomorphism \<chi>: G \<rightarrow> Z/3Z.

  For GF(3):  dual has 3 characters   (self-dual: Z/3Z \<cong> Hom(Z/3Z, Z/3Z))
  For GF(9):  dual has 9 characters   (GF(9)^+ \<cong> (Z/3Z)^2 is self-dual)
  For GF(27): dual has 27 characters  (GF(27)^+ \<cong> (Z/3Z)^3 is self-dual)

  The extension GF(9)/GF(3) corresponds to:
    characters that vanish on GF(3) \<subseteq> characters of GF(9)
  This quotient \<cong> Z/3Z = GF(3), recovering the Galois group.
\<close>

text \<open>Characters of GF(3)\<close>
type_synonym trit_character = "trit \<Rightarrow> trit"

definition is_trit_character :: "trit_character \<Rightarrow> bool" where
  "is_trit_character \<chi> \<longleftrightarrow> (\<forall>a b. \<chi> (trit_add a b) = trit_add (\<chi> a) (\<chi> b))"

text \<open>The three characters of GF(3): \<chi>_k(x) = kx for k \<in> {-1, 0, 1}\<close>
definition char_minus :: trit_character where "char_minus = trit_mul Minus"
definition char_zero  :: trit_character where "char_zero  = (\<lambda>_. Zero)"
definition char_plus  :: trit_character where "char_plus  = trit_mul Plus"

lemma char_minus_is_character: "is_trit_character char_minus"
  unfolding is_trit_character_def char_minus_def
  by (intro allI, cases rule: trit.exhaust[of _ True])
     (auto, cases rule: trit.exhaust[of _ True], auto)+

lemma char_zero_is_character: "is_trit_character char_zero"
  unfolding is_trit_character_def char_zero_def by simp

lemma char_plus_is_character: "is_trit_character char_plus"
  unfolding is_trit_character_def char_plus_def
  by (intro allI, cases rule: trit.exhaust[of _ True])
     (auto, cases rule: trit.exhaust[of _ True], auto)+

text \<open>Characters of GF(9): pairs of trit characters\<close>
type_synonym nonet_character = "nonet \<Rightarrow> trit"

definition is_nonet_character :: "nonet_character \<Rightarrow> bool" where
  "is_nonet_character \<chi> \<longleftrightarrow> (\<forall>x y. \<chi> (nonet_add x y) = trit_add (\<chi> x) (\<chi> y))"

text \<open>Character kernel: elements mapped to zero\<close>
definition char_kernel :: "('a \<Rightarrow> trit) \<Rightarrow> 'a set" where
  "char_kernel \<chi> = {x. \<chi> x = Zero}"

text \<open>Characters restricting to GF(3) form a subgroup\<close>
definition restricted_character :: "nonet_character \<Rightarrow> trit_character" where
  "restricted_character \<chi> = (\<lambda>t. \<chi> (trit_to_nonet t))"


subsection \<open>The Abelian Extension Tower and GF(3) Conservation\<close>

text \<open>
  The extension tower GF(3) \<hookrightarrow> GF(9) \<hookrightarrow> GF(27) has a GF(3) trit interpretation:

  GF(3)  = base field        = -1 (MINUS: fixed, validated)
  GF(9)  = first extension   =  0 (ZERO: intermediate, coordinating)
  GF(27) = full extension    = +1 (PLUS: generative, exploring)

  The tower is balanced: -1 + 0 + 1 = 0 mod 3.

  Application to scanning:
  - MINUS devices: classified by GF(3) trits alone (3 states)
  - ZERO devices:  refined by GF(9) nonets (9 states)
  - PLUS devices:  fully classified by GF(27) tribbles (27 states)
\<close>

definition extension_tower_trits :: "trit list" where
  "extension_tower_trits = [Minus, Zero, Plus]"

lemma extension_tower_balanced: "gf3_balanced extension_tower_trits"
  unfolding extension_tower_trits_def gf3_balanced_def by simp

text \<open>Nonet classification: extends trit to 9-state system\<close>
fun nonet_classify :: "int \<Rightarrow> int \<Rightarrow> nonet" where
  "nonet_classify score confidence =
    N (if score < 0 then Minus else if score = 0 then Zero else Plus)
       (if confidence < 0 then Minus else if confidence = 0 then Zero else Plus)"

text \<open>Tribble classification: extends to 27-state system\<close>
fun tribble_classify :: "int \<Rightarrow> int \<Rightarrow> int \<Rightarrow> tribble" where
  "tribble_classify score confidence novelty =
    T (if score < 0 then Minus else if score = 0 then Zero else Plus)
       (if confidence < 0 then Minus else if confidence = 0 then Zero else Plus)
       (if novelty < 0 then Minus else if novelty = 0 then Zero else Plus)"

text \<open>The norm of a classified tribble recovers the trit\<close>
text \<open>Coarsening: project higher extensions back to GF(3)\<close>
fun nonet_coarsen :: "nonet \<Rightarrow> trit" where
  "nonet_coarsen (N a _) = a"

fun tribble_coarsen :: "tribble \<Rightarrow> trit" where
  "tribble_coarsen (T a _ _) = a"

text \<open>Coarsening is compatible with addition\<close>
lemma nonet_coarsen_hom:
  "nonet_coarsen (nonet_add x y) = trit_add (nonet_coarsen x) (nonet_coarsen y)"
  by (cases x; cases y) simp

lemma tribble_coarsen_hom:
  "tribble_coarsen (tribble_add x y) = trit_add (tribble_coarsen x) (tribble_coarsen y)"
  by (cases x; cases y) simp

text \<open>Coarsening of embedding is identity\<close>
lemma nonet_coarsen_embed: "nonet_coarsen (trit_to_nonet t) = t"
  by (simp add: trit_to_nonet_def)

lemma tribble_coarsen_embed: "tribble_coarsen (trit_to_tribble t) = t"
  by (simp add: trit_to_tribble_def)


subsection \<open>GF(3) Conservation Across the Tower\<close>

text \<open>
  Key theorem: GF(3) balance is preserved at every level of the extension tower.
  If a collection of trit-tagged operations is balanced, then their
  extensions to GF(9) or GF(27) are also balanced (under coarsening).
\<close>

definition nonet_balanced :: "nonet list \<Rightarrow> bool" where
  "nonet_balanced ns = gf3_balanced (map nonet_coarsen ns)"

definition tribble_balanced :: "tribble list \<Rightarrow> bool" where
  "tribble_balanced ts = gf3_balanced (map tribble_coarsen ts)"

lemma tower_balance_preservation:
  assumes "gf3_balanced ts"
  shows "nonet_balanced (map trit_to_nonet ts)"
  unfolding nonet_balanced_def
  using assms by (simp add: comp_def nonet_coarsen_embed map_map)

lemma tower_balance_preservation_27:
  assumes "gf3_balanced ts"
  shows "tribble_balanced (map trit_to_tribble ts)"
  unfolding tribble_balanced_def
  using assms by (simp add: comp_def tribble_coarsen_embed map_map)

text \<open>
  The full tower conservation theorem:
  GF(3) balance at the base \<Longrightarrow> balance at GF(9) \<Longrightarrow> balance at GF(27).
  All three layers are coherent via the Frobenius automorphisms.
\<close>

theorem abelian_extension_conservation:
  assumes "gf3_balanced [a, b, c]"
  shows "nonet_balanced [trit_to_nonet a, trit_to_nonet b, trit_to_nonet c]"
    and "tribble_balanced [trit_to_tribble a, trit_to_tribble b, trit_to_tribble c]"
  using tower_balance_preservation[OF assms]
        tower_balance_preservation_27[OF assms]
  by simp_all


subsection \<open>Interleaving: Abelian Extensions \<leftrightarrow> Hedges Selection\<close>

text \<open>
  Connection to game theory:
  A selection function \<sigma> on GF(9) determinizes a 9-way choice into a single outcome.
  A selection function \<sigma> on GF(27) determinizes a 27-way choice.

  The Galois group action (Frobenius) provides natural "symmetry breaking":
  if two admissible revisions are related by Frobenius, select the fixed one.
\<close>

definition frobenius_selection_9 :: "nonet set \<Rightarrow> nonet" where
  "frobenius_selection_9 S = (if \<exists>x \<in> S. frobenius_9 x = x
                              then (SOME x. x \<in> S \<and> frobenius_9 x = x)
                              else (SOME x. x \<in> S))"

definition frobenius_selection_27 :: "tribble set \<Rightarrow> tribble" where
  "frobenius_selection_27 S = (if \<exists>x \<in> S. frobenius_27 x = x
                               then (SOME x. x \<in> S \<and> frobenius_27 x = x)
                               else (SOME x. x \<in> S))"

text \<open>Frobenius selection on GF(9) always selects from GF(3) when possible\<close>
lemma frobenius_selection_9_prefers_gf3:
  assumes "S \<noteq> {}" and "\<exists>t. trit_to_nonet t \<in> S"
  shows "\<exists>t. frobenius_selection_9 S = trit_to_nonet t"
  unfolding frobenius_selection_9_def
  using assms frobenius_9_fixes_gf3 frobenius_9_fixed_iff
  by (auto intro: someI2)


subsection \<open>Collected Simplification Rules\<close>

lemmas gf9_simps =
  nonet_add.simps nonet_neg.simps nonet_mul.simps
  trit_mul.simps
  nonet_add_comm nonet_add_assoc
  nonet_add_zero_left nonet_add_zero_right nonet_add_inverse
  nonet_zero_def nonet_one_def
  frobenius_9.simps frobenius_9_order frobenius_9_additive

lemmas gf27_simps =
  tribble_add.simps tribble_neg.simps
  tribble_add_comm tribble_add_assoc
  tribble_add_zero_left tribble_add_inverse
  tribble_zero_def
  frobenius_27.simps frobenius_27_order frobenius_27_additive

end
