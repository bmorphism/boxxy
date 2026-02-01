//go:build darwin

package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bmorphism/boxxy/internal/lisp"
)

// BeliefSet implements AGM belief revision (Alchourrón-Gärdenfors-Makinson).
// A belief set K is a deductively closed set of propositions.
// In boxxy, propositions are lisp symbols for simplicity.
//
// The three AGM operations:
//   Expansion:   K + p  — add p without consistency check
//   Revision:    K * p  — add p, maintaining consistency (may remove old beliefs)
//   Contraction: K - p  — remove p (and enough consequences)
//
// Connected to soft-serve's branch model via Grove's system of spheres:
//   - Each belief set K corresponds to a set of possible worlds [K]
//   - Branches are alternative histories (possible worlds)
//   - Merge = revision (incorporate new info, resolve conflicts)
//   - Revert = contraction (remove a belief)
//   - Commit = expansion (add new info)
type BeliefSet struct {
	beliefs     map[string]bool
	entrenchment map[string]int // epistemic entrenchment ordering
	revision    int             // revision counter (like commit count)
}

func newBeliefSet() *BeliefSet {
	return &BeliefSet{
		beliefs:      make(map[string]bool),
		entrenchment: make(map[string]int),
		revision:     0,
	}
}

// expand implements K + p: add belief without consistency check.
func (bs *BeliefSet) expand(prop string) {
	bs.beliefs[prop] = true
	bs.entrenchment[prop] = bs.revision
	bs.revision++
}

// contract implements K - p: remove belief and anything that depends on it.
// Uses epistemic entrenchment: least entrenched beliefs go first.
func (bs *BeliefSet) contract(prop string) {
	delete(bs.beliefs, prop)
	delete(bs.entrenchment, prop)
	// Also remove the negation if present
	if strings.HasPrefix(prop, "not-") {
		delete(bs.beliefs, strings.TrimPrefix(prop, "not-"))
		delete(bs.entrenchment, strings.TrimPrefix(prop, "not-"))
	} else {
		delete(bs.beliefs, "not-"+prop)
		delete(bs.entrenchment, "not-"+prop)
	}
	bs.revision++
}

// revise implements K * p: add belief while maintaining consistency.
// Uses the Levi identity: K * p = (K - ¬p) + p
func (bs *BeliefSet) revise(prop string) {
	// First contract by the negation (Levi identity)
	negation := negate(prop)
	bs.contract(negation)
	// Then expand
	bs.expand(prop)
}

// entails checks if K entails p (p is in the belief set).
func (bs *BeliefSet) entails(prop string) bool {
	return bs.beliefs[prop]
}

// consistent checks if K is consistent (no p and ¬p both present).
func (bs *BeliefSet) consistent() bool {
	for prop := range bs.beliefs {
		if strings.HasPrefix(prop, "not-") {
			base := strings.TrimPrefix(prop, "not-")
			if bs.beliefs[base] {
				return false
			}
		} else {
			if bs.beliefs["not-"+prop] {
				return false
			}
		}
	}
	return true
}

// listBeliefs returns sorted beliefs.
func (bs *BeliefSet) listBeliefs() []string {
	result := make([]string, 0, len(bs.beliefs))
	for b := range bs.beliefs {
		result = append(result, b)
	}
	sort.Strings(result)
	return result
}

// worlds returns the "possible worlds" — maximal consistent subsets.
// Simplified: returns the current belief set as one world, plus
// alternative worlds formed by toggling each belief.
func (bs *BeliefSet) worlds() [][]string {
	var worlds [][]string
	// The current belief set is the actual world
	worlds = append(worlds, bs.listBeliefs())
	// Each negated belief forms an alternative world (Grove sphere)
	for _, b := range bs.listBeliefs() {
		alt := make([]string, 0)
		for _, other := range bs.listBeliefs() {
			if other != b {
				alt = append(alt, other)
			}
		}
		alt = append(alt, negate(b))
		sort.Strings(alt)
		worlds = append(worlds, alt)
	}
	return worlds
}

func negate(prop string) string {
	if strings.HasPrefix(prop, "not-") {
		return strings.TrimPrefix(prop, "not-")
	}
	return "not-" + prop
}

// RegisterAGM adds AGM belief revision functions to the lisp environment.
func RegisterAGM(env *lisp.Env) {
	env.Set("agm/new-belief-set", &lisp.Fn{Name: "agm/new-belief-set", Func: func(args []lisp.Value) lisp.Value {
		bs := newBeliefSet()
		return &lisp.ExternalValue{Value: bs, Type: "belief-set"}
	}})

	env.Set("agm/expand", &lisp.Fn{Name: "agm/expand", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("agm/expand requires belief-set and proposition")
		}
		ext, ok := args[0].(*lisp.ExternalValue)
		if !ok || ext.Type != "belief-set" {
			panic("agm/expand: first argument must be a belief-set")
		}
		bs := ext.Value.(*BeliefSet)
		prop := symbolToString(args[1])
		bs.expand(prop)
		return args[0]
	}})

	env.Set("agm/revise", &lisp.Fn{Name: "agm/revise", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("agm/revise requires belief-set and proposition")
		}
		ext, ok := args[0].(*lisp.ExternalValue)
		if !ok || ext.Type != "belief-set" {
			panic("agm/revise: first argument must be a belief-set")
		}
		bs := ext.Value.(*BeliefSet)
		prop := symbolToString(args[1])
		bs.revise(prop)
		return args[0]
	}})

	env.Set("agm/contract", &lisp.Fn{Name: "agm/contract", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("agm/contract requires belief-set and proposition")
		}
		ext, ok := args[0].(*lisp.ExternalValue)
		if !ok || ext.Type != "belief-set" {
			panic("agm/contract: first argument must be a belief-set")
		}
		bs := ext.Value.(*BeliefSet)
		prop := symbolToString(args[1])
		bs.contract(prop)
		return args[0]
	}})

	env.Set("agm/beliefs", &lisp.Fn{Name: "agm/beliefs", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("agm/beliefs requires a belief-set")
		}
		ext, ok := args[0].(*lisp.ExternalValue)
		if !ok || ext.Type != "belief-set" {
			panic("agm/beliefs: argument must be a belief-set")
		}
		bs := ext.Value.(*BeliefSet)
		beliefs := bs.listBeliefs()
		result := make(lisp.Vector, len(beliefs))
		for i, b := range beliefs {
			result[i] = lisp.Symbol(b)
		}
		return result
	}})

	env.Set("agm/entails?", &lisp.Fn{Name: "agm/entails?", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("agm/entails? requires belief-set and proposition")
		}
		ext, ok := args[0].(*lisp.ExternalValue)
		if !ok || ext.Type != "belief-set" {
			panic("agm/entails?: first argument must be a belief-set")
		}
		bs := ext.Value.(*BeliefSet)
		prop := symbolToString(args[1])
		return lisp.Bool(bs.entails(prop))
	}})

	env.Set("agm/consistent?", &lisp.Fn{Name: "agm/consistent?", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("agm/consistent? requires a belief-set")
		}
		ext, ok := args[0].(*lisp.ExternalValue)
		if !ok || ext.Type != "belief-set" {
			panic("agm/consistent?: argument must be a belief-set")
		}
		bs := ext.Value.(*BeliefSet)
		return lisp.Bool(bs.consistent())
	}})

	env.Set("agm/worlds", &lisp.Fn{Name: "agm/worlds", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("agm/worlds requires a belief-set")
		}
		ext, ok := args[0].(*lisp.ExternalValue)
		if !ok || ext.Type != "belief-set" {
			panic("agm/worlds: argument must be a belief-set")
		}
		bs := ext.Value.(*BeliefSet)
		worlds := bs.worlds()
		result := make(lisp.Vector, len(worlds))
		for i, world := range worlds {
			beliefs := make(lisp.Vector, len(world))
			for j, b := range world {
				beliefs[j] = lisp.Symbol(b)
			}
			result[i] = beliefs
		}
		return result
	}})

	// Convenience: describe the AGM ↔ Git correspondence
	env.Set("agm/grove-spheres", &lisp.Fn{Name: "agm/grove-spheres", Func: func(args []lisp.Value) lisp.Value {
		fmt.Println(`
  Grove's System of Spheres ↔ Git/Soft-Serve:

    Possible world  =  commit (complete codebase snapshot)
    Belief set K    =  HEAD (what the codebase "believes" now)
    Expansion K+p   =  git commit (add without conflict)
    Revision  K*p   =  git merge  (add, resolve conflicts)
    Contraction K-p =  git revert (remove a belief/commit)
    Epistemic rank  =  branch protection / commit distance
    Selection fn    =  merge strategy (ours/theirs/recursive)
    Sphere shell    =  commit graph distance from HEAD`)
		return lisp.Nil{}
	}})
}

func symbolToString(v lisp.Value) string {
	switch s := v.(type) {
	case lisp.Symbol:
		return string(s)
	case lisp.String:
		return string(s)
	case lisp.Keyword:
		return string(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}
