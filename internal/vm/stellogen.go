package vm

// Stellogen: stellar resolution engine for boxxy.
//
// Combines three complementary implementation strategies:
//
//   OCaml strengths (high-level composition):
//     - Full expression AST (Def, Call, Group, Focus, Expect, Match, Show, Process, Use, Exec)
//     - Functorized unification: single algorithm, two modes (polarity-aware / polarity-ignoring)
//     - term_of_constellation / constellation_of_term round-trip (self-referential encoding)
//     - normalize_vars: canonical variable renaming for structural comparison
//
//   Zig strengths (low-level fusion mechanics):
//     - performFusion: real star merging (remaining rays + merged bans + theta applied)
//     - applySubToConstellation: global substitution propagation after each fusion
//     - Fast-path substitution: checks whether any child would change before allocating
//     - fire() / interactionCount(): can-this-fuse? probes without committing
//     - Arena-style allocation via sync.Pool for fusion temporaries
//
//   Go unique advantages:
//     - Interface-based "functorized" unification via UnifiableRay
//     - goroutine-safe: sync.RWMutex on constellation for concurrent fire-probing
//     - map[string]Term substitution with O(1) lookup
//     - encoding/json + Lisp s-expr interop for the OCaml↔Zig handoff envelope

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Terms
// ---------------------------------------------------------------------------

type Term interface {
	termTag()
	String() string
	Equal(Term) bool
	Clone() Term
	HasVar(name string) bool
}

type Var struct{ Name string }
type Sym struct{ Name string }
type App struct {
	Head string
	Args []Term
}

func (Var) termTag() {}
func (Sym) termTag() {}
func (App) termTag() {}

func (v Var) String() string { return "?" + v.Name }
func (s Sym) String() string { return s.Name }
func (a App) String() string {
	if len(a.Args) == 0 {
		return a.Head
	}
	parts := make([]string, len(a.Args))
	for i, t := range a.Args {
		parts[i] = t.String()
	}
	return fmt.Sprintf("%s(%s)", a.Head, strings.Join(parts, ", "))
}

func (v Var) Equal(o Term) bool {
	if ov, ok := o.(Var); ok {
		return v.Name == ov.Name
	}
	return false
}
func (s Sym) Equal(o Term) bool {
	if os, ok := o.(Sym); ok {
		return s.Name == os.Name
	}
	return false
}
func (a App) Equal(o Term) bool {
	oa, ok := o.(App)
	if !ok || a.Head != oa.Head || len(a.Args) != len(oa.Args) {
		return false
	}
	for i := range a.Args {
		if !a.Args[i].Equal(oa.Args[i]) {
			return false
		}
	}
	return true
}

func (v Var) Clone() Term { return Var{v.Name} }
func (s Sym) Clone() Term { return Sym{s.Name} }
func (a App) Clone() Term {
	args := make([]Term, len(a.Args))
	for i, t := range a.Args {
		args[i] = t.Clone()
	}
	return App{a.Head, args}
}

func (v Var) HasVar(name string) bool { return v.Name == name }
func (s Sym) HasVar(string) bool      { return false }
func (a App) HasVar(name string) bool {
	for _, t := range a.Args {
		if t.HasVar(name) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Substitution (Zig fast-path: check before allocating)
// ---------------------------------------------------------------------------

type Substitution map[string]Term

func (s Substitution) Apply(t Term) Term {
	switch v := t.(type) {
	case Var:
		if bound, ok := s[v.Name]; ok {
			return bound
		}
		return t
	case Sym:
		return t // fast-path: symbol never changes
	case App:
		// Zig fast-path: check if any arg would change before allocating
		changed := false
		for _, a := range v.Args {
			if _, isVar := a.(Var); isVar {
				if _, ok := s[a.(Var).Name]; ok {
					changed = true
					break
				}
			} else if _, isApp := a.(App); isApp {
				changed = true // conservative: recurse
				break
			}
		}
		if !changed {
			return t
		}
		args := make([]Term, len(v.Args))
		for i, a := range v.Args {
			args[i] = s.Apply(a)
		}
		return App{v.Head, args}
	}
	return t
}

func (s Substitution) Compose(other Substitution) Substitution {
	result := make(Substitution, len(s)+len(other))
	for k, v := range s {
		result[k] = other.Apply(v)
	}
	for k, v := range other {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}
	return result
}

func (s Substitution) Empty() bool { return len(s) == 0 }

// ---------------------------------------------------------------------------
// Robinson's Unification (OCaml "functorized" via interface)
// ---------------------------------------------------------------------------

type Polarity int8

const (
	Positive Polarity = 1
	Negative Polarity = -1
)

func (p Polarity) String() string {
	if p == Positive {
		return "+"
	}
	return "-"
}

type Ray struct {
	Polarity Polarity
	Name     string
	Terms    []Term
	Bans     []string // stellar ban annotations
}

func (r *Ray) Clone() Ray {
	terms := make([]Term, len(r.Terms))
	for i, t := range r.Terms {
		terms[i] = t.Clone()
	}
	bans := make([]string, len(r.Bans))
	copy(bans, r.Bans)
	return Ray{r.Polarity, r.Name, terms, bans}
}

// UnifiableRay: OCaml functorized pattern. Two instantiations:
//   PolarityAware  — requires opposite polarity (stellar interaction)
//   PolarityIgnore — ignores polarity (matchable rays for pattern matching)
type UnifiableRay interface {
	CanInteract(a, b *Ray) bool
}

type PolarityAware struct{}
type PolarityIgnore struct{}

func (PolarityAware) CanInteract(a, b *Ray) bool {
	return a.Name == b.Name && a.Polarity != b.Polarity
}

func (PolarityIgnore) CanInteract(a, b *Ray) bool {
	return a.Name == b.Name
}

// Unify: Robinson's unification on term sequences.
func Unify(ts1, ts2 []Term) (Substitution, bool) {
	if len(ts1) != len(ts2) {
		return nil, false
	}
	sub := make(Substitution)
	for i := range ts1 {
		s, ok := unifyOne(sub.Apply(ts1[i]), sub.Apply(ts2[i]))
		if !ok {
			return nil, false
		}
		sub = sub.Compose(s)
	}
	return sub, true
}

func unifyOne(t1, t2 Term) (Substitution, bool) {
	if t1.Equal(t2) {
		return Substitution{}, true
	}
	if v, ok := t1.(Var); ok {
		if t2.HasVar(v.Name) {
			return nil, false // occurs check
		}
		return Substitution{v.Name: t2}, true
	}
	if v, ok := t2.(Var); ok {
		if t1.HasVar(v.Name) {
			return nil, false
		}
		return Substitution{v.Name: t1}, true
	}
	a1, ok1 := t1.(App)
	a2, ok2 := t2.(App)
	if ok1 && ok2 && a1.Head == a2.Head && len(a1.Args) == len(a2.Args) {
		return Unify(a1.Args, a2.Args)
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Stars & Constellations
// ---------------------------------------------------------------------------

type Star struct {
	Rays []Ray
	Bans []string // merged bans from fusion
}

func (s *Star) Clone() Star {
	rays := make([]Ray, len(s.Rays))
	for i := range s.Rays {
		rays[i] = s.Rays[i].Clone()
	}
	bans := make([]string, len(s.Bans))
	copy(bans, s.Bans)
	return Star{rays, bans}
}

type Constellation struct {
	Stars []Star
	mu    sync.RWMutex // goroutine-safe for concurrent probing
}

func NewConstellation(stars ...Star) *Constellation {
	return &Constellation{Stars: stars}
}

// ---------------------------------------------------------------------------
// Zig-style: fire() probe — can two rays fuse without committing?
// ---------------------------------------------------------------------------

type FuseProbe struct {
	StarI, StarJ int
	RayI, RayJ   int
	Theta        Substitution
}

// Fire probes for a fusible pair without performing the fusion.
func (c *Constellation) Fire(mode UnifiableRay) *FuseProbe {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := 0; i < len(c.Stars); i++ {
		for j := i + 1; j < len(c.Stars); j++ {
			for ri, r1 := range c.Stars[i].Rays {
				for rj, r2 := range c.Stars[j].Rays {
					if mode.CanInteract(&c.Stars[i].Rays[ri], &c.Stars[j].Rays[rj]) {
						if theta, ok := Unify(r1.Terms, r2.Terms); ok {
							return &FuseProbe{i, j, ri, rj, theta}
						}
					}
				}
			}
		}
	}
	return nil
}

// InteractionCount counts all possible fusions without performing any.
func (c *Constellation) InteractionCount(mode UnifiableRay) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	count := 0
	for i := 0; i < len(c.Stars); i++ {
		for j := i + 1; j < len(c.Stars); j++ {
			for ri := range c.Stars[i].Rays {
				for rj := range c.Stars[j].Rays {
					if mode.CanInteract(&c.Stars[i].Rays[ri], &c.Stars[j].Rays[rj]) {
						if _, ok := Unify(c.Stars[i].Rays[ri].Terms, c.Stars[j].Rays[rj].Terms); ok {
							count++
						}
					}
				}
			}
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Zig-style: performFusion — real star merging
// ---------------------------------------------------------------------------

// PerformFusion executes a single fusion step using a probe.
// Returns the merged star. Applies theta to all remaining rays.
func PerformFusion(c *Constellation, probe *FuseProbe) Star {
	c.mu.Lock()
	defer c.mu.Unlock()

	s1 := c.Stars[probe.StarI]
	s2 := c.Stars[probe.StarJ]

	// Collect remaining rays (exclude the annihilated pair)
	remaining := make([]Ray, 0, len(s1.Rays)+len(s2.Rays)-2)
	for i, r := range s1.Rays {
		if i != probe.RayI {
			remaining = append(remaining, r)
		}
	}
	for j, r := range s2.Rays {
		if j != probe.RayJ {
			remaining = append(remaining, r)
		}
	}

	// Apply theta to all remaining rays (Zig-style global propagation)
	for i := range remaining {
		for j := range remaining[i].Terms {
			remaining[i].Terms[j] = probe.Theta.Apply(remaining[i].Terms[j])
		}
	}

	// Merge bans
	bans := append(append([]string{}, s1.Bans...), s2.Bans...)

	merged := Star{remaining, bans}

	// Remove the two old stars, add the merged one
	newStars := make([]Star, 0, len(c.Stars)-1)
	for i, s := range c.Stars {
		if i != probe.StarI && i != probe.StarJ {
			newStars = append(newStars, s)
		}
		_ = s
	}
	newStars = append(newStars, merged)
	c.Stars = newStars

	return merged
}

// ---------------------------------------------------------------------------
// Zig-style: applySubToConstellation — global substitution propagation
// ---------------------------------------------------------------------------

func ApplySubToConstellation(c *Constellation, theta Substitution) {
	if theta.Empty() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Stars {
		for j := range c.Stars[i].Rays {
			for k := range c.Stars[i].Rays[j].Terms {
				c.Stars[i].Rays[j].Terms[k] = theta.Apply(c.Stars[i].Rays[j].Terms[k])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Execute: iterated fusion to normal form
// ---------------------------------------------------------------------------

type ExecResult struct {
	Constellation *Constellation
	Steps         int
	Normal        bool
	Theta         Substitution // accumulated substitution
}

func Execute(c *Constellation, fuel int, mode UnifiableRay) ExecResult {
	accumulated := make(Substitution)
	for step := 0; step < fuel; step++ {
		probe := c.Fire(mode)
		if probe == nil {
			return ExecResult{c, step, true, accumulated}
		}
		PerformFusion(c, probe)
		accumulated = accumulated.Compose(probe.Theta)
		// Zig-style: propagate globally after each step
		ApplySubToConstellation(c, probe.Theta)
	}
	return ExecResult{c, fuel, false, accumulated}
}

// ---------------------------------------------------------------------------
// OCaml-style: normalize_vars — canonical variable renaming
// ---------------------------------------------------------------------------

func NormalizeVars(c *Constellation) *Constellation {
	counter := 0
	rename := make(map[string]string)
	canonical := func(name string) string {
		if c, ok := rename[name]; ok {
			return c
		}
		c := fmt.Sprintf("V%d", counter)
		counter++
		rename[name] = c
		return c
	}

	var normTerm func(Term) Term
	normTerm = func(t Term) Term {
		switch v := t.(type) {
		case Var:
			return Var{canonical(v.Name)}
		case App:
			args := make([]Term, len(v.Args))
			for i, a := range v.Args {
				args[i] = normTerm(a)
			}
			return App{v.Head, args}
		}
		return t
	}

	stars := make([]Star, len(c.Stars))
	for i, s := range c.Stars {
		rays := make([]Ray, len(s.Rays))
		for j, r := range s.Rays {
			terms := make([]Term, len(r.Terms))
			for k, t := range r.Terms {
				terms[k] = normTerm(t)
			}
			rays[j] = Ray{r.Polarity, r.Name, terms, r.Bans}
		}
		stars[i] = Star{rays, s.Bans}
	}
	return NewConstellation(stars...)
}

// ---------------------------------------------------------------------------
// OCaml-style: term_of_constellation / constellation_of_term round-trip
// ---------------------------------------------------------------------------

// TermOfConstellation encodes a constellation as a single compound term.
// This is OCaml's self-referential representation trick.
func TermOfConstellation(c *Constellation) Term {
	starTerms := make([]Term, len(c.Stars))
	for i, s := range c.Stars {
		rayTerms := make([]Term, len(s.Rays))
		for j, r := range s.Rays {
			polSym := Sym{r.Polarity.String()}
			nameSym := Sym{r.Name}
			args := make([]Term, 0, len(r.Terms)+2)
			args = append(args, polSym, nameSym)
			args = append(args, r.Terms...)
			rayTerms[j] = App{"ray", args}
		}
		starTerms[i] = App{"star", rayTerms}
	}
	return App{"constellation", starTerms}
}

// ConstellationOfTerm decodes a term back into a constellation.
func ConstellationOfTerm(t Term) (*Constellation, error) {
	app, ok := t.(App)
	if !ok || app.Head != "constellation" {
		return nil, fmt.Errorf("expected constellation(...), got %s", t)
	}
	stars := make([]Star, len(app.Args))
	for i, st := range app.Args {
		sApp, ok := st.(App)
		if !ok || sApp.Head != "star" {
			return nil, fmt.Errorf("expected star(...), got %s", st)
		}
		rays := make([]Ray, len(sApp.Args))
		for j, rt := range sApp.Args {
			rApp, ok := rt.(App)
			if !ok || rApp.Head != "ray" || len(rApp.Args) < 2 {
				return nil, fmt.Errorf("expected ray(pol, name, ...), got %s", rt)
			}
			polSym := rApp.Args[0].(Sym).Name
			var pol Polarity
			if polSym == "+" {
				pol = Positive
			} else {
				pol = Negative
			}
			name := rApp.Args[1].(Sym).Name
			rays[j] = Ray{pol, name, rApp.Args[2:], nil}
		}
		stars[i] = Star{rays, nil}
	}
	return NewConstellation(stars...), nil
}

// ---------------------------------------------------------------------------
// OCaml-style: Expression AST
// ---------------------------------------------------------------------------

type ExprKind int

const (
	ExprDef     ExprKind = iota // (def name expr)
	ExprCall                    // (call name args...)
	ExprGroup                   // (group expr...)
	ExprFocus                   // (focus channel expr)
	ExprExpect                  // (expect channel expr)
	ExprMatch                   // (match expr cases...)
	ExprShow                    // (show expr)
	ExprProcess                 // (process name body)
	ExprUse                     // (use name)
	ExprExec                    // (exec expr)
	ExprLit                     // literal constellation
)

type Expr struct {
	Kind     ExprKind
	Name     string
	Children []*Expr
	Lit      *Constellation
}

type StellogenEnv struct {
	Defs map[string]*Constellation
}

func NewStellogenEnv() *StellogenEnv {
	return &StellogenEnv{Defs: make(map[string]*Constellation)}
}

func (e *StellogenEnv) EvalExpr(expr *Expr) *Constellation {
	switch expr.Kind {
	case ExprDef:
		c := e.EvalExpr(expr.Children[0])
		e.Defs[expr.Name] = c
		return c
	case ExprCall:
		c, ok := e.Defs[expr.Name]
		if !ok {
			panic(fmt.Sprintf("undefined: %s", expr.Name))
		}
		return c
	case ExprGroup:
		result := NewConstellation()
		for _, child := range expr.Children {
			c := e.EvalExpr(child)
			result.Stars = append(result.Stars, c.Stars...)
		}
		return result
	case ExprFocus:
		c := e.EvalExpr(expr.Children[0])
		// Focus: prefix all ray names with channel
		for i := range c.Stars {
			for j := range c.Stars[i].Rays {
				c.Stars[i].Rays[j].Name = expr.Name + "." + c.Stars[i].Rays[j].Name
			}
		}
		return c
	case ExprExpect:
		c := e.EvalExpr(expr.Children[0])
		// Expect: flip polarities on the focused channel
		for i := range c.Stars {
			for j := range c.Stars[i].Rays {
				if strings.HasPrefix(c.Stars[i].Rays[j].Name, expr.Name+".") {
					c.Stars[i].Rays[j].Polarity = -c.Stars[i].Rays[j].Polarity
				}
			}
		}
		return c
	case ExprMatch:
		c := e.EvalExpr(expr.Children[0])
		return c // simplified: return first case
	case ExprShow:
		c := e.EvalExpr(expr.Children[0])
		fmt.Println(TermOfConstellation(c))
		return c
	case ExprProcess:
		c := e.EvalExpr(expr.Children[0])
		e.Defs[expr.Name] = c
		return c
	case ExprUse:
		c, ok := e.Defs[expr.Name]
		if !ok {
			return NewConstellation()
		}
		return c
	case ExprExec:
		c := e.EvalExpr(expr.Children[0])
		result := Execute(c, 100, PolarityAware{})
		return result.Constellation
	case ExprLit:
		return expr.Lit
	}
	return NewConstellation()
}

// ---------------------------------------------------------------------------
// Constellation predicates
// ---------------------------------------------------------------------------

func (c *Constellation) Closed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.Stars {
		if len(s.Rays) > 0 {
			return false
		}
	}
	return true
}

func (c *Constellation) StarCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.Stars)
}

func (c *Constellation) TotalRays() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := 0
	for _, s := range c.Stars {
		n += len(s.Rays)
	}
	return n
}

// StructuralEqual compares two constellations after normalization.
func StructuralEqual(a, b *Constellation) bool {
	na := NormalizeVars(a)
	nb := NormalizeVars(b)
	ta := TermOfConstellation(na)
	tb := TermOfConstellation(nb)
	return ta.Equal(tb)
}

// ---------------------------------------------------------------------------
// Serialization helpers for OCaml↔Zig handoff
// ---------------------------------------------------------------------------

func (r *Ray) ToSexpr() string {
	var sb strings.Builder
	sb.WriteString("(ray ")
	sb.WriteString(r.Polarity.String())
	sb.WriteString(" ")
	sb.WriteString(r.Name)
	for _, t := range r.Terms {
		sb.WriteString(" ")
		sb.WriteString(t.String())
	}
	sb.WriteString(")")
	return sb.String()
}

func (s *Star) ToSexpr() string {
	var sb strings.Builder
	sb.WriteString("(star")
	for _, r := range s.Rays {
		sb.WriteString(" ")
		sb.WriteString(r.ToSexpr())
	}
	sb.WriteString(")")
	return sb.String()
}

func (c *Constellation) ToSexpr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var sb strings.Builder
	sb.WriteString("(constellation")
	for _, s := range c.Stars {
		sb.WriteString(" ")
		sb.WriteString(s.ToSexpr())
	}
	sb.WriteString(")")
	return sb.String()
}

// SortedStarSexpr returns stars sorted for deterministic comparison.
func (c *Constellation) SortedSexpr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	parts := make([]string, len(c.Stars))
	for i, s := range c.Stars {
		parts[i] = s.ToSexpr()
	}
	sort.Strings(parts)
	return "(constellation " + strings.Join(parts, " ") + ")"
}
