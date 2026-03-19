//go:build darwin

package tropical

import (
	"math"

	"github.com/bmorphism/boxxy/internal/lisp"
)

const NegInf = -math.MaxFloat64

// TropVal is a value in the max-plus semiring (R ∪ {-∞}, max, +)
type TropVal struct {
	Val   float64
	IsInf bool // true means -∞
}

func (t TropVal) String() string {
	if t.IsInf {
		return "-∞"
	}
	return lisp.Float(t.Val).String()
}

// DPVal is a labeled tropical value at an address
type DPVal struct {
	Addr string
	Val  TropVal
}

// PantsLeg is one leg of a cobordant pants diagram
type PantsLeg struct {
	ID   string
	Trit int // -1, 0, +1
}

// Add returns max(a, b) in the max-plus semiring
func Add(a, b TropVal) TropVal {
	if a.IsInf {
		return b
	}
	if b.IsInf {
		return a
	}
	return TropVal{Val: math.Max(a.Val, b.Val)}
}

// Mul returns a + b in the max-plus semiring
func Mul(a, b TropVal) TropVal {
	if a.IsInf || b.IsInf {
		return TropVal{IsInf: true}
	}
	return TropVal{Val: a.Val + b.Val}
}

// Bellman computes dp[parent] = max(dp[a]+cost, dp[b]+cost)
func Bellman(a, b DPVal, cost float64) DPVal {
	c := TropVal{Val: cost}
	left := Mul(a.Val, c)
	right := Mul(b.Val, c)
	merged := Add(left, right)
	return DPVal{Addr: "(" + a.Addr + "+" + b.Addr + ")", Val: merged}
}

// GF3Add returns (a + b) mod 3 in balanced ternary {-1, 0, +1}
func GF3Add(a, b int) int {
	s := a + b
	switch {
	case s > 1:
		return s - 3
	case s < -1:
		return s + 3
	default:
		return s
	}
}

// Derange checks that no skill maps to itself
func Derange(skills, validators []string) [][2]string {
	if len(skills) != len(validators) {
		return nil
	}
	n := len(skills)
	if n == 0 {
		return nil
	}
	// simple rotation derangement: validator[i] = skills[(i+1) % n]
	result := make([][2]string, n)
	for i := 0; i < n; i++ {
		vi := (i + 1) % n
		if skills[i] == validators[vi] {
			return nil // can't derange with rotation, would need backtracking
		}
		result[i] = [2]string{skills[i], validators[vi]}
	}
	return result
}

func tropValFromLisp(v lisp.Value) TropVal {
	switch x := v.(type) {
	case *lisp.ExternalValue:
		if x.Type == "TropNeginf" {
			return TropVal{IsInf: true}
		}
		if x.Type == "TropVal" {
			return TropVal{Val: x.Value.(float64)}
		}
	case lisp.Float:
		return TropVal{Val: float64(x)}
	case lisp.Int:
		return TropVal{Val: float64(x)}
	}
	panic("expected tropical value")
}

func tropValToLisp(t TropVal) lisp.Value {
	if t.IsInf {
		return &lisp.ExternalValue{Value: nil, Type: "TropNeginf"}
	}
	return &lisp.ExternalValue{Value: t.Val, Type: "TropVal"}
}

func dpValFromLisp(v lisp.Value) DPVal {
	ext := v.(*lisp.ExternalValue)
	m := ext.Value.(map[string]interface{})
	return DPVal{
		Addr: m["addr"].(string),
		Val:  m["val"].(TropVal),
	}
}

func dpValToLisp(d DPVal) lisp.Value {
	return &lisp.ExternalValue{
		Value: map[string]interface{}{"addr": d.Addr, "val": d.Val},
		Type:  "DPVal",
	}
}

// RegisterNamespace registers the tropical namespace in the Lisp environment
func RegisterNamespace(env *lisp.Env) {
	reg := func(name string, f func([]lisp.Value) lisp.Value) {
		env.Set(lisp.Symbol(name), &lisp.Fn{Name: name, Func: f})
	}

	// -- Tropical Values --

	reg("tropical/val", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tropical/val: requires (n)")
		}
		var f float64
		switch x := args[0].(type) {
		case lisp.Int:
			f = float64(x)
		case lisp.Float:
			f = float64(x)
		default:
			panic("tropical/val: requires numeric argument")
		}
		return &lisp.ExternalValue{Value: f, Type: "TropVal"}
	})

	reg("tropical/neginf", func(args []lisp.Value) lisp.Value {
		return &lisp.ExternalValue{Value: nil, Type: "TropNeginf"}
	})

	// -- Tropical Arithmetic --

	reg("tropical/add", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tropical/add: requires (a b)")
		}
		a := tropValFromLisp(args[0])
		b := tropValFromLisp(args[1])
		return tropValToLisp(Add(a, b))
	})

	reg("tropical/mul", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tropical/mul: requires (a b)")
		}
		a := tropValFromLisp(args[0])
		b := tropValFromLisp(args[1])
		return tropValToLisp(Mul(a, b))
	})

	// -- Tropicalization Functor --

	reg("tropical/tropicalize", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tropical/tropicalize: requires (op a b)")
		}
		op := string(args[0].(lisp.Keyword))
		var af, bf float64
		switch x := args[1].(type) {
		case lisp.Int:
			af = float64(x)
		case lisp.Float:
			af = float64(x)
		}
		switch x := args[2].(type) {
		case lisp.Int:
			bf = float64(x)
		case lisp.Float:
			bf = float64(x)
		}
		switch op {
		case "product":
			return tropValToLisp(TropVal{Val: af + bf})
		case "sum":
			return tropValToLisp(TropVal{Val: math.Max(af, bf)})
		case "zero":
			return tropValToLisp(TropVal{IsInf: true})
		case "one":
			return tropValToLisp(TropVal{Val: 0})
		default:
			panic("tropical/tropicalize: op must be :product, :sum, :zero, or :one")
		}
	})

	// -- DP Leaf --

	reg("tropical/dp-leaf", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tropical/dp-leaf: requires (addr cost)")
		}
		addr := string(args[0].(lisp.String))
		var cost float64
		switch x := args[1].(type) {
		case lisp.Int:
			cost = float64(x)
		case lisp.Float:
			cost = float64(x)
		}
		d := DPVal{Addr: addr, Val: TropVal{Val: cost}}
		return dpValToLisp(d)
	})

	// -- Bellman Merge --

	reg("tropical/bellman", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tropical/bellman: requires (dp-a dp-b cost)")
		}
		a := dpValFromLisp(args[0])
		b := dpValFromLisp(args[1])
		var cost float64
		switch x := args[2].(type) {
		case lisp.Int:
			cost = float64(x)
		case lisp.Float:
			cost = float64(x)
		}
		return dpValToLisp(Bellman(a, b, cost))
	})

	// -- Pants Merge / Copy --

	reg("tropical/pants-merge", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tropical/pants-merge: requires (leg-a leg-b cost)")
		}
		ea := args[0].(*lisp.ExternalValue)
		ma := ea.Value.(map[string]interface{})
		eb := args[1].(*lisp.ExternalValue)
		mb := eb.Value.(map[string]interface{})
		idA := ma["id"].(string)
		idB := mb["id"].(string)
		if idA == idB {
			panic("tropical/pants-merge: legs must differ (Frobenius condition)")
		}
		tritA := ma["trit"].(int)
		tritB := mb["trit"].(int)
		var cost float64
		switch x := args[2].(type) {
		case lisp.Int:
			cost = float64(x)
		case lisp.Float:
			cost = float64(x)
		}
		return &lisp.ExternalValue{
			Value: map[string]interface{}{
				"merged":   [2]string{idA, idB},
				"trit-sum": GF3Add(tritA, tritB),
				"cost":     cost,
			},
			Type: "PantsWaist",
		}
	})

	reg("tropical/pants-copy", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tropical/pants-copy: requires (waist)")
		}
		ew := args[0].(*lisp.ExternalValue)
		mw := ew.Value.(map[string]interface{})
		leg := &lisp.ExternalValue{
			Value: map[string]interface{}{"id": mw["id"], "trit": mw["trit"]},
			Type:  "PantsLeg",
		}
		return lisp.Vector{leg, leg}
	})

	// -- Derangement --

	reg("tropical/derange", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tropical/derange: requires (skills validators)")
		}
		sv := args[0].(lisp.Vector)
		vv := args[1].(lisp.Vector)
		skills := make([]string, len(sv))
		validators := make([]string, len(vv))
		for i, s := range sv {
			skills[i] = string(s.(lisp.String))
		}
		for i, v := range vv {
			validators[i] = string(v.(lisp.String))
		}
		result := Derange(skills, validators)
		if result == nil {
			return lisp.Nil{}
		}
		pairs := make(lisp.Vector, len(result))
		for i, p := range result {
			pairs[i] = lisp.Vector{lisp.String(p[0]), lisp.String(p[1])}
		}
		return pairs
	})

	// -- Obstruction Check --

	reg("tropical/check-obstruct", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tropical/check-obstruct: requires (dp-a dp-b constraints)")
		}
		a := dpValFromLisp(args[0])
		b := dpValFromLisp(args[1])
		cm := args[2].(lisp.HashMap)
		var ca, cb lisp.Value
		for k, v := range cm {
			ks := string(k.(lisp.String))
			if ks == a.Addr {
				ca = v
			}
			if ks == b.Addr {
				cb = v
			}
		}
		if ca != nil && cb != nil && ca.String() != cb.String() {
			return &lisp.ExternalValue{
				Value: map[string]interface{}{
					"witness":  [2]string{a.Addr, b.Addr},
					"conflict": [2]string{ca.String(), cb.String()},
				},
				Type: "H1Obstruction",
			}
		}
		return &lisp.ExternalValue{
			Value: map[string]interface{}{"parent": "(" + a.Addr + "+" + b.Addr + ")"},
			Type:  "CleanMerge",
		}
	})

	// -- Plugin Sheaf --

	reg("tropical/plugin-attach", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tropical/plugin-attach: requires (node plugin data)")
		}
		node := string(args[0].(lisp.String))
		plugin := string(args[1].(lisp.String))
		return &lisp.ExternalValue{
			Value: map[string]interface{}{"node": node, "plugin": plugin, "data": args[2]},
			Type:  "SheafSection",
		}
	})

	reg("tropical/plugin-restrict", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tropical/plugin-restrict: requires (section child)")
		}
		es := args[0].(*lisp.ExternalValue)
		ms := es.Value.(map[string]interface{})
		child := string(args[1].(lisp.String))
		return &lisp.ExternalValue{
			Value: map[string]interface{}{
				"node":   child,
				"plugin": ms["plugin"],
				"data":   ms["data"],
			},
			Type: "SheafSection",
		}
	})
}
