package vm

import (
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Term basics
// ---------------------------------------------------------------------------

func TestTermEquality(t *testing.T) {
	v1 := Var{Name: "X"}
	v2 := Var{Name: "X"}
	v3 := Var{Name: "Y"}
	s1 := Sym{Name: "a"}
	s2 := Sym{Name: "a"}
	a1 := App{Head: "f", Args: []Term{v1, s1}}
	a2 := App{Head: "f", Args: []Term{v2, s2}}

	if !v1.Equal(v2) {
		t.Fatal("identical vars should be equal")
	}
	if v1.Equal(v3) {
		t.Fatal("different vars should not be equal")
	}
	if !s1.Equal(s2) {
		t.Fatal("identical syms should be equal")
	}
	if !a1.Equal(a2) {
		t.Fatal("identical apps should be equal")
	}
	if v1.Equal(s1) {
		t.Fatal("var should not equal sym")
	}
}

func TestTermClone(t *testing.T) {
	orig := App{Head: "f", Args: []Term{Var{Name: "X"}, Sym{Name: "a"}}}
	clone := orig.Clone()
	if !orig.Equal(clone) {
		t.Fatal("clone should equal original")
	}
	// mutating clone must not affect original
	clone.(App).Args[0] = Sym{Name: "mutated"}
	if orig.Args[0].(Var).Name != "X" {
		t.Fatal("mutation of clone affected original")
	}
}

func TestTermHasVar(t *testing.T) {
	term := App{Head: "f", Args: []Term{Var{Name: "X"}, Sym{Name: "a"}, Var{Name: "Y"}}}
	if !term.HasVar("X") {
		t.Fatal("should find X")
	}
	if !term.HasVar("Y") {
		t.Fatal("should find Y")
	}
	if term.HasVar("Z") {
		t.Fatal("should not find Z")
	}
	if (Sym{Name: "a"}).HasVar("a") {
		t.Fatal("sym should not have vars")
	}
}

// ---------------------------------------------------------------------------
// Substitution (Zig fast-path)
// ---------------------------------------------------------------------------

func TestSubstitutionApply(t *testing.T) {
	theta := Substitution{"X": Sym{Name: "a"}, "Y": Sym{Name: "b"}}
	term := App{Head: "f", Args: []Term{Var{Name: "X"}, Var{Name: "Z"}}}
	result := theta.Apply(term)
	app := result.(App)
	if !app.Args[0].Equal(Sym{Name: "a"}) {
		t.Fatal("X should map to a")
	}
	if !app.Args[1].Equal(Var{Name: "Z"}) {
		t.Fatal("Z should remain unchanged")
	}
}

func TestSubstitutionFastPath(t *testing.T) {
	theta := Substitution{"Q": Sym{Name: "q"}}
	term := App{Head: "f", Args: []Term{Sym{Name: "a"}, Sym{Name: "b"}}}
	result := theta.Apply(term)
	// fast path: no vars match, should return structurally identical term
	if !result.Equal(term) {
		t.Fatal("fast path should preserve term")
	}
}

func TestSubstitutionCompose(t *testing.T) {
	s1 := Substitution{"X": Var{Name: "Y"}}
	s2 := Substitution{"Y": Sym{Name: "a"}}
	composed := s1.Compose(s2)
	result := composed.Apply(Var{Name: "X"})
	if !result.Equal(Sym{Name: "a"}) {
		t.Fatalf("composed substitution should map X→a, got %s", result)
	}
}

// ---------------------------------------------------------------------------
// Unification (OCaml Robinson's)
// ---------------------------------------------------------------------------

func TestUnifySimple(t *testing.T) {
	ts1 := []Term{Var{Name: "X"}}
	ts2 := []Term{Sym{Name: "a"}}
	theta, ok := Unify(ts1, ts2)
	if !ok {
		t.Fatal("should unify")
	}
	if !theta["X"].Equal(Sym{Name: "a"}) {
		t.Fatal("X should map to a")
	}
}

func TestUnifyAppTerms(t *testing.T) {
	ts1 := []Term{App{Head: "f", Args: []Term{Var{Name: "X"}, Sym{Name: "b"}}}}
	ts2 := []Term{App{Head: "f", Args: []Term{Sym{Name: "a"}, Var{Name: "Y"}}}}
	theta, ok := Unify(ts1, ts2)
	if !ok {
		t.Fatal("should unify")
	}
	if !theta["X"].Equal(Sym{Name: "a"}) {
		t.Fatal("X should be a")
	}
	if !theta["Y"].Equal(Sym{Name: "b"}) {
		t.Fatal("Y should be b")
	}
}

func TestUnifyOccursCheck(t *testing.T) {
	ts1 := []Term{Var{Name: "X"}}
	ts2 := []Term{App{Head: "f", Args: []Term{Var{Name: "X"}}}}
	_, ok := Unify(ts1, ts2)
	if ok {
		t.Fatal("occurs check should prevent infinite substitution")
	}
}

func TestUnifyMismatch(t *testing.T) {
	ts1 := []Term{Sym{Name: "a"}}
	ts2 := []Term{Sym{Name: "b"}}
	_, ok := Unify(ts1, ts2)
	if ok {
		t.Fatal("different symbols should not unify")
	}
}

func TestUnifyLengthMismatch(t *testing.T) {
	ts1 := []Term{Sym{Name: "a"}, Sym{Name: "b"}}
	ts2 := []Term{Sym{Name: "a"}}
	_, ok := Unify(ts1, ts2)
	if ok {
		t.Fatal("different lengths should not unify")
	}
}

// ---------------------------------------------------------------------------
// Ray / Star / Constellation
// ---------------------------------------------------------------------------

func makeRay(pol Polarity, name string, terms ...Term) Ray {
	return Ray{Polarity: pol, Name: name, Terms: terms}
}

func TestRayClone(t *testing.T) {
	r := makeRay(Positive, "p", Var{Name: "X"})
	r.Bans = []string{"q"}
	c := r.Clone()
	if c.Name != r.Name || c.Polarity != r.Polarity {
		t.Fatal("clone mismatch")
	}
	c.Bans[0] = "mutated"
	if r.Bans[0] != "q" {
		t.Fatal("clone mutated original bans")
	}
}

func TestConstellationBasics(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "a"})}},
	)
	if c.StarCount() != 2 {
		t.Fatalf("expected 2 stars, got %d", c.StarCount())
	}
	if c.TotalRays() != 2 {
		t.Fatalf("expected 2 rays, got %d", c.TotalRays())
	}
}

// ---------------------------------------------------------------------------
// Fire probe (Zig-style)
// ---------------------------------------------------------------------------

func TestFirePolarityAware(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "a"})}},
	)
	probe := c.Fire(PolarityAware{})
	if probe == nil {
		t.Fatal("should find fusible pair")
	}
	if probe.Theta["X"] == nil || !probe.Theta["X"].Equal(Sym{Name: "a"}) {
		t.Fatal("probe should contain X→a")
	}
}

func TestFireNoPolarityMatch(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
	)
	probe := c.Fire(PolarityAware{})
	if probe != nil {
		t.Fatal("same polarity should not fire in polarity-aware mode")
	}
}

func TestFirePolarityIgnore(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
	)
	probe := c.Fire(PolarityIgnore{})
	if probe == nil {
		t.Fatal("polarity-ignore should allow same-polarity fusion")
	}
}

func TestInteractionCount(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "a"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "b"})}},
	)
	count := c.InteractionCount(PolarityAware{})
	if count != 2 {
		t.Fatalf("expected 2 interactions, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// PerformFusion (Zig-style)
// ---------------------------------------------------------------------------

func TestPerformFusion(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{
			makeRay(Positive, "p", Var{Name: "X"}),
			makeRay(Positive, "q", Sym{Name: "b"}),
		}},
		Star{Rays: []Ray{
			makeRay(Negative, "p", Sym{Name: "a"}),
			makeRay(Negative, "r", Var{Name: "X"}),
		}},
	)
	probe := c.Fire(PolarityAware{})
	if probe == nil {
		t.Fatal("should find fusible pair")
	}
	merged := PerformFusion(c, probe)
	// merged star should have remaining rays: +q(b), -r(a) with X→a applied
	if len(merged.Rays) != 2 {
		t.Fatalf("expected 2 remaining rays, got %d", len(merged.Rays))
	}
	// after fusion, X→a should be applied to remaining ray -r(X) → -r(a)
	found := false
	for _, r := range merged.Rays {
		if r.Name == "r" && len(r.Terms) > 0 && r.Terms[0].Equal(Sym{Name: "a"}) {
			found = true
		}
	}
	if !found {
		t.Fatal("substitution should be applied to remaining rays")
	}
}

// ---------------------------------------------------------------------------
// Execute (iterated fusion to normal form)
// ---------------------------------------------------------------------------

func TestExecuteSimpleFusion(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "a"})}},
	)
	result := Execute(c, 100, PolarityAware{})
	if result.Steps == 0 {
		t.Fatal("should have performed at least one fusion step")
	}
	// after fusion of complementary single-ray stars, result should be a single rayless star
	if result.Constellation.TotalRays() != 0 {
		t.Fatalf("expected 0 remaining rays, got %d", result.Constellation.TotalRays())
	}
}

func TestExecuteNoFusion(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
		Star{Rays: []Ray{makeRay(Positive, "q", Sym{Name: "b"})}},
	)
	result := Execute(c, 100, PolarityAware{})
	if result.Steps != 0 {
		t.Fatal("should not fuse when no complementary rays")
	}
	if result.Constellation.StarCount() != 2 {
		t.Fatal("constellation should be unchanged")
	}
}

func TestExecuteChainedFusion(t *testing.T) {
	// p(X) fuses with -p(a), producing X→a; then q(a) fuses with -q(Y)
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"}), makeRay(Positive, "q", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "a"})}},
		Star{Rays: []Ray{makeRay(Negative, "q", Var{Name: "Y"})}},
	)
	result := Execute(c, 100, PolarityAware{})
	if result.Steps < 2 {
		t.Fatalf("expected at least 2 fusion steps, got %d", result.Steps)
	}
}

func TestExecuteFuelLimit(t *testing.T) {
	// infinite loop: +p(X) and -p(f(X)) would keep generating deeper terms
	// but Execute should bail after fuel runs out
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", App{Head: "f", Args: []Term{Var{Name: "X"}}})}},
	)
	result := Execute(c, 5, PolarityAware{})
	// should stop due to occurs check or fuel
	if result.Steps > 5 {
		t.Fatal("should respect fuel limit")
	}
}

// ---------------------------------------------------------------------------
// NormalizeVars (OCaml-style)
// ---------------------------------------------------------------------------

func TestNormalizeVars(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "zebra"}, Var{Name: "apple"})}},
	)
	norm := NormalizeVars(c)
	ray := norm.Stars[0].Rays[0]
	v1 := ray.Terms[0].(Var).Name
	v2 := ray.Terms[1].(Var).Name
	if v1 == "zebra" || v2 == "apple" {
		t.Fatal("vars should be renamed to canonical form")
	}
	if v1 == v2 {
		t.Fatal("distinct vars should get distinct canonical names")
	}
}

func TestNormalizeVarsIdempotent(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "A"}, Var{Name: "B"})}},
	)
	n1 := NormalizeVars(c)
	n2 := NormalizeVars(n1)
	if !StructuralEqual(n1, n2) {
		t.Fatal("normalize should be idempotent")
	}
}

// ---------------------------------------------------------------------------
// Term ↔ Constellation round-trip (OCaml-style)
// ---------------------------------------------------------------------------

func TestTermOfConstellationRoundTrip(t *testing.T) {
	orig := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
		Star{Rays: []Ray{makeRay(Negative, "q", Sym{Name: "b"}, Var{Name: "X"})}},
	)
	term := TermOfConstellation(orig)
	roundTripped, err := ConstellationOfTerm(term)
	if err != nil {
		t.Fatalf("round-trip error: %v", err)
	}
	if roundTripped.StarCount() != orig.StarCount() {
		t.Fatal("star count mismatch after round-trip")
	}
}

func TestConstellationOfTermBadInput(t *testing.T) {
	_, err := ConstellationOfTerm(Sym{Name: "not-a-constellation"})
	if err == nil {
		t.Fatal("should error on non-App term")
	}
}

// ---------------------------------------------------------------------------
// StructuralEqual
// ---------------------------------------------------------------------------

func TestStructuralEqual(t *testing.T) {
	c1 := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
	)
	c2 := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "Y"})}},
	)
	if !StructuralEqual(c1, c2) {
		t.Fatal("alpha-equivalent constellations should be structurally equal")
	}
}

func TestStructuralNotEqual(t *testing.T) {
	c1 := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
	)
	c2 := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "b"})}},
	)
	if StructuralEqual(c1, c2) {
		t.Fatal("different constellations should not be structurally equal")
	}
}

// ---------------------------------------------------------------------------
// S-expr serialization
// ---------------------------------------------------------------------------

func TestToSexpr(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}},
	)
	sexpr := c.ToSexpr()
	if !strings.Contains(sexpr, "constellation") {
		t.Fatal("should contain constellation tag")
	}
	if !strings.Contains(sexpr, "ray + p a") {
		t.Fatal("should contain ray details")
	}
}

func TestSortedSexpr(t *testing.T) {
	c1 := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "b", Sym{Name: "x"})}},
		Star{Rays: []Ray{makeRay(Positive, "a", Sym{Name: "y"})}},
	)
	c2 := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "a", Sym{Name: "y"})}},
		Star{Rays: []Ray{makeRay(Positive, "b", Sym{Name: "x"})}},
	)
	if c1.SortedSexpr() != c2.SortedSexpr() {
		t.Fatal("sorted sexpr should be order-independent")
	}
}

// ---------------------------------------------------------------------------
// AST evaluator (OCaml expression AST)
// ---------------------------------------------------------------------------

func TestExprDefAndCall(t *testing.T) {
	env := NewStellogenEnv()
	lit := NewConstellation(Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}})
	env.EvalExpr(&Expr{Kind: ExprDef, Name: "test", Children: []*Expr{{Kind: ExprLit, Lit: lit}}})
	result := env.EvalExpr(&Expr{Kind: ExprCall, Name: "test"})
	if result.StarCount() != 1 {
		t.Fatal("call should return defined constellation")
	}
}

func TestExprGroup(t *testing.T) {
	env := NewStellogenEnv()
	lit1 := NewConstellation(Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}})
	lit2 := NewConstellation(Star{Rays: []Ray{makeRay(Negative, "q", Sym{Name: "b"})}})
	result := env.EvalExpr(&Expr{
		Kind: ExprGroup,
		Children: []*Expr{
			{Kind: ExprLit, Lit: lit1},
			{Kind: ExprLit, Lit: lit2},
		},
	})
	if result.StarCount() != 2 {
		t.Fatal("group should merge stars")
	}
}

func TestExprExec(t *testing.T) {
	env := NewStellogenEnv()
	lit := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "a"})}},
	)
	result := env.EvalExpr(&Expr{Kind: ExprExec, Children: []*Expr{{Kind: ExprLit, Lit: lit}}})
	if result.TotalRays() != 0 {
		t.Fatal("exec should reduce to normal form")
	}
}

func TestExprFocusExpect(t *testing.T) {
	env := NewStellogenEnv()
	lit := NewConstellation(Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}})
	focused := env.EvalExpr(&Expr{Kind: ExprFocus, Name: "ch", Children: []*Expr{{Kind: ExprLit, Lit: lit}}})
	if focused.Stars[0].Rays[0].Name != "ch.p" {
		t.Fatalf("focus should prefix ray name, got %s", focused.Stars[0].Rays[0].Name)
	}
}

func TestExprUseUndefined(t *testing.T) {
	env := NewStellogenEnv()
	result := env.EvalExpr(&Expr{Kind: ExprUse, Name: "nonexistent"})
	if result.StarCount() != 0 {
		t.Fatal("use of undefined should return empty constellation")
	}
}

// ---------------------------------------------------------------------------
// Concurrent fire probing (Go-unique: goroutine-safe)
// ---------------------------------------------------------------------------

func TestConcurrentFireProbing(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "p", Sym{Name: "a"})}},
		Star{Rays: []Ray{makeRay(Positive, "q", Sym{Name: "b"})}},
	)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Fire(PolarityAware{})
			_ = c.InteractionCount(PolarityAware{})
			_ = c.StarCount()
			_ = c.TotalRays()
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Constellation predicates
// ---------------------------------------------------------------------------

func TestClosed(t *testing.T) {
	empty := NewConstellation(Star{Rays: nil})
	if !empty.Closed() {
		t.Fatal("rayless constellation should be closed")
	}
	open := NewConstellation(Star{Rays: []Ray{makeRay(Positive, "p", Sym{Name: "a"})}})
	if open.Closed() {
		t.Fatal("constellation with rays should not be closed")
	}
}

// ---------------------------------------------------------------------------
// ApplySubToConstellation (Zig global propagation)
// ---------------------------------------------------------------------------

func TestApplySubToConstellation(t *testing.T) {
	c := NewConstellation(
		Star{Rays: []Ray{makeRay(Positive, "p", Var{Name: "X"})}},
		Star{Rays: []Ray{makeRay(Negative, "q", Var{Name: "X"})}},
	)
	theta := Substitution{"X": Sym{Name: "resolved"}}
	ApplySubToConstellation(c, theta)
	for _, s := range c.Stars {
		for _, r := range s.Rays {
			for _, term := range r.Terms {
				if v, ok := term.(Var); ok && v.Name == "X" {
					t.Fatal("global sub should replace all X occurrences")
				}
			}
		}
	}
}
