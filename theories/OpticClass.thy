theory OpticClass
  imports AGM_Base
begin

section \<open>Optic Class: Concrete Implementation from Open Games Engine\<close>

text \<open>
  Direct formalization of the Haskell Optic class from:
  github.com/Plurigrid/open-games-hs/src/OpenGames/Engine/OpticClass.hs
  
  The &&& operator IS the nashator - parallel composition producing product equilibria.
  
  Galois connection: coarsening (play) \<stileturn> refinement (coplay)
\<close>

subsection \<open>Optic Type Class\<close>

text \<open>
  Optic o where:
    lens :: (s -> a) -> (s -> b -> t) -> o s t a b
    (>>>>) :: o s t a b -> o a b p q -> o s t p q      -- sequential
    (&&&&) :: o s1 t1 a1 b1 -> o s2 t2 a2 b2 
           -> o (s1, s2) (t1, t2) (a1, a2) (b1, b2)    -- parallel = nashator
\<close>

record ('s, 't, 'a, 'b) lens =
  get :: "'s \<Rightarrow> 'a"
  put :: "'s \<Rightarrow> 'b \<Rightarrow> 't"

definition lens_id :: "('s, 't, 's, 't) lens" where
  "lens_id = \<lparr> get = id, put = \<lambda>_ t. t \<rparr>"

definition lens_compose :: 
  "('s, 't, 'a, 'b) lens \<Rightarrow> ('a, 'b, 'p, 'q) lens \<Rightarrow> ('s, 't, 'p, 'q) lens" (infixr "\<ggreater>" 55) where
  "l1 \<ggreater> l2 = \<lparr> 
    get = get l2 \<circ> get l1,
    put = \<lambda>s q. put l1 s (put l2 (get l1 s) q)
  \<rparr>"

text \<open>Parallel composition = Nashator\<close>
definition lens_parallel ::
  "('s1, 't1, 'a1, 'b1) lens \<Rightarrow> ('s2, 't2, 'a2, 'b2) lens \<Rightarrow> 
   ('s1 \<times> 's2, 't1 \<times> 't2, 'a1 \<times> 'a2, 'b1 \<times> 'b2) lens" (infixr "\<parallel>" 60) where
  "l1 \<parallel> l2 = \<lparr>
    get = \<lambda>(s1, s2). (get l1 s1, get l2 s2),
    put = \<lambda>(s1, s2) (b1, b2). (put l1 s1 b1, put l2 s2 b2)
  \<rparr>"

subsection \<open>Parametrized Lens (Arena)\<close>

text \<open>
  ParaLens p q x s y r where:
    get :: p -> x -> y           (forward pass / play)
    put :: p -> x -> r -> (s, q) (backward pass / coplay)
    
  Wires:
    x = game states observed before move
    p = strategies
    y = game states after move  
    r = utilities/payoffs received
    s = back-propagated utilities
    q = rewards (intrinsic utility)
\<close>

record ('p, 'q, 'x, 's, 'y, 'r) para_lens =
  para_get :: "'p \<Rightarrow> 'x \<Rightarrow> 'y"
  para_put :: "'p \<Rightarrow> 'x \<Rightarrow> 'r \<Rightarrow> 's \<times> 'q"

definition para_lens_id :: "(unit, unit, 'x, 's, 'x, 's) para_lens" where
  "para_lens_id = \<lparr> 
    para_get = \<lambda>_ x. x, 
    para_put = \<lambda>_ _ s. (s, ()) 
  \<rparr>"

text \<open>Sequential composition of parametrized lenses\<close>
definition para_compose ::
  "('p, 'q, 'x, 's, 'y, 'r) para_lens \<Rightarrow> 
   ('p', 'q', 'y, 'r, 'z, 't) para_lens \<Rightarrow>
   ('p \<times> 'p', 'q \<times> 'q', 'x, 's, 'z, 't) para_lens" (infixr "\<ggreater>\<ggreater>" 55) where
  "l1 \<ggreater>\<ggreater> l2 = \<lparr>
    para_get = \<lambda>(p, p') x. para_get l2 p' (para_get l1 p x),
    para_put = \<lambda>(p, p') x t.
      let (r, q') = para_put l2 p' (para_get l1 p x) t;
          (s, q)  = para_put l1 p x r
      in (s, (q, q'))
  \<rparr>"

text \<open>Parallel composition = Nashator for parametrized lenses\<close>
definition para_parallel ::
  "('p, 'q, 'x, 's, 'y, 'r) para_lens \<Rightarrow>
   ('p', 'q', 'x', 's', 'y', 'r') para_lens \<Rightarrow>
   ('p \<times> 'p', 'q \<times> 'q', 'x \<times> 'x', 's \<times> 's', 'y \<times> 'y', 'r \<times> 'r') para_lens" (infixr "\<parallel>\<parallel>" 60) where
  "l1 \<parallel>\<parallel> l2 = \<lparr>
    para_get = \<lambda>(p, p') (x, x'). (para_get l1 p x, para_get l2 p' x'),
    para_put = \<lambda>(p, p') (x, x') (r, r').
      let (s, q)   = para_put l1 p x r;
          (s', q') = para_put l2 p' x' r'
      in ((s, s'), (q, q'))
  \<rparr>"

subsection \<open>Corner Lens\<close>

text \<open>Bends parameter wires into right wires - key for bimatrix games\<close>
definition corner :: "('y, 'r, unit, unit, 'y, 'r) para_lens" where
  "corner = \<lparr>
    para_get = \<lambda>y _. y,
    para_put = \<lambda>_ _ r. ((), r)
  \<rparr>"

subsection \<open>Galois Connection: Coarsening \<stileturn> Refinement\<close>

text \<open>
  The adjunction structure:
  - Left adjoint (coarsening): play/get - forgets detail
  - Right adjoint (refinement): coplay/put - adds detail back
  
  Unit: id \<le> put \<circ> get (refinement after coarsening adds info)
  Counit: get \<circ> put \<le> id (coarsening after refinement loses info)
\<close>

definition galois_unit :: "('s, 't, 'a, 'b) lens \<Rightarrow> 's \<Rightarrow> 'b \<Rightarrow> bool" where
  "galois_unit l s b \<longleftrightarrow> True" \<comment> \<open>Placeholder for order structure\<close>

definition galois_counit :: "('s, 't, 'a, 'b) lens \<Rightarrow> 'a \<Rightarrow> 's \<Rightarrow> 'b \<Rightarrow> bool" where
  "galois_counit l a s b \<longleftrightarrow> get l s = a"

subsection \<open>GF(3) Trit Assignment\<close>

text \<open>
  Optic operations maintain GF(3) conservation:
  - get/play (+1): Forward information flow
  - put/coplay (-1): Backward utility flow  
  - compose (0): Neutral combination
\<close>

definition get_trit :: trit where "get_trit = Plus"
definition put_trit :: trit where "put_trit = Minus"
definition compose_trit :: trit where "compose_trit = Zero"

lemma optic_gf3_conserved:
  "gf3_balanced [get_trit, put_trit, compose_trit]"
  unfolding gf3_balanced_def get_trit_def put_trit_def compose_trit_def
  by simp

subsection \<open>Lens Laws\<close>

definition lens_law_get_put :: "('s, 's, 'a, 'a) lens \<Rightarrow> bool" where
  "lens_law_get_put l \<longleftrightarrow> (\<forall>s. put l s (get l s) = s)"

definition lens_law_put_get :: "('s, 's, 'a, 'a) lens \<Rightarrow> bool" where
  "lens_law_put_get l \<longleftrightarrow> (\<forall>s a. get l (put l s a) = a)"

definition lens_law_put_put :: "('s, 's, 'a, 'a) lens \<Rightarrow> bool" where
  "lens_law_put_put l \<longleftrightarrow> (\<forall>s a a'. put l (put l s a) a' = put l s a')"

definition lawful_lens :: "('s, 's, 'a, 'a) lens \<Rightarrow> bool" where
  "lawful_lens l \<longleftrightarrow> lens_law_get_put l \<and> lens_law_put_get l \<and> lens_law_put_put l"

lemma lens_id_lawful: "lawful_lens lens_id"
  unfolding lawful_lens_def lens_law_get_put_def lens_law_put_get_def lens_law_put_put_def
            lens_id_def
  by simp

end
