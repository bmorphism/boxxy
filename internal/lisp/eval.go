//go:build darwin

package lisp

import (
	"fmt"
	"os"
	"strings"
)

// Env is an evaluation environment
type Env struct {
	parent  *Env
	bindings map[Symbol]Value
}

// NewEnv creates a new environment
func NewEnv(parent *Env) *Env {
	return &Env{
		parent:   parent,
		bindings: make(map[Symbol]Value),
	}
}

// Get looks up a symbol
func (e *Env) Get(s Symbol) (Value, bool) {
	if v, ok := e.bindings[s]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Get(s)
	}
	return nil, false
}

// Set binds a symbol in this environment
func (e *Env) Set(s Symbol, v Value) {
	e.bindings[s] = v
}

// === Namespace System (SCI-style Clojure namespaces) ===

// Namespace represents a Clojure-style namespace with bindings and aliases.
type Namespace struct {
	Name     string
	Bindings map[Symbol]Value
	Aliases  map[string]string // alias → fully-qualified ns name
	Refers   map[Symbol]string // symbol → source ns name
}

// NSRegistry manages all namespaces.
type NSRegistry struct {
	namespaces map[string]*Namespace
	current    string
}

var globalNSRegistry = &NSRegistry{
	namespaces: map[string]*Namespace{},
	current:    "user",
}

// GetNSRegistry returns the global namespace registry.
func GetNSRegistry() *NSRegistry { return globalNSRegistry }

// ResetNSRegistry replaces the global registry with a fresh one (for testing).
func ResetNSRegistry() {
	globalNSRegistry = &NSRegistry{
		namespaces: map[string]*Namespace{},
		current:    "user",
	}
}

func (r *NSRegistry) FindOrCreate(name string) *Namespace {
	if ns, ok := r.namespaces[name]; ok {
		return ns
	}
	ns := &Namespace{
		Name:     name,
		Bindings: make(map[Symbol]Value),
		Aliases:  make(map[string]string),
		Refers:   make(map[Symbol]string),
	}
	r.namespaces[name] = ns
	return ns
}

func (r *NSRegistry) Current() *Namespace  { return r.FindOrCreate(r.current) }
func (r *NSRegistry) CurrentName() string   { return r.current }

func (r *NSRegistry) SetCurrent(name string) {
	r.current = name
	r.FindOrCreate(name)
}

func (r *NSRegistry) AllNames() []string {
	names := make([]string, 0, len(r.namespaces))
	for n := range r.namespaces {
		names = append(names, n)
	}
	return names
}

// Intern binds a symbol in the current namespace.
func (r *NSRegistry) Intern(sym Symbol, val Value) {
	r.Current().Bindings[sym] = val
}

// ResolveSymbol resolves a symbol through namespace machinery:
//  1. Qualified (alias/local): look up alias in current ns, then as full ns name
//  2. Unqualified: check refers, then current ns bindings
func (r *NSRegistry) ResolveSymbol(sym Symbol) (Value, bool) {
	name := string(sym)

	if idx := strings.Index(name, "/"); idx > 0 {
		alias := name[:idx]
		local := Symbol(name[idx+1:])
		cur := r.Current()
		if nsName, ok := cur.Aliases[alias]; ok {
			if ns, ok := r.namespaces[nsName]; ok {
				if v, ok := ns.Bindings[local]; ok {
					return v, true
				}
			}
		}
		if ns, ok := r.namespaces[alias]; ok {
			if v, ok := ns.Bindings[local]; ok {
				return v, true
			}
		}
		return nil, false
	}

	cur := r.Current()
	if srcNS, ok := cur.Refers[sym]; ok {
		if ns, ok := r.namespaces[srcNS]; ok {
			if v, ok := ns.Bindings[sym]; ok {
				return v, true
			}
		}
	}
	if v, ok := cur.Bindings[sym]; ok {
		return v, true
	}
	return nil, false
}

// InternInNS binds a value in a named namespace (for Go-side registration).
func InternInNS(nsName string, sym Symbol, val Value) {
	globalNSRegistry.FindOrCreate(nsName).Bindings[sym] = val
}

// SetupDefaultAliases configures the user namespace with standard aliases.
func SetupDefaultAliases() {
	user := globalNSRegistry.FindOrCreate("user")
	user.Aliases["vz"] = "boxxy.vz"
	user.Aliases["vm"] = "boxxy.vm"
	user.Aliases["color"] = "boxxy.color"
	user.Aliases["trace"] = "boxxy.trace"
}

// evalNSForm implements (ns name (:require ...))
func evalNSForm(form List, env *Env) Value {
	if len(form) < 2 {
		panic("ns requires a name")
	}
	name, ok := form[1].(Symbol)
	if !ok {
		panic("ns name must be a symbol")
	}
	nsName := string(name)
	globalNSRegistry.SetCurrent(nsName)

	for _, directive := range form[2:] {
		lst, ok := directive.(List)
		if !ok {
			continue
		}
		if len(lst) < 1 {
			continue
		}
		kw, ok := lst[0].(Keyword)
		if !ok {
			continue
		}
		if kw == "require" {
			processRequireSpecs(lst[1:])
		}
	}
	return Symbol(nsName)
}

// processRequireSpecs processes require specs like [ns :as alias] or [ns :refer [sym ...]]
func processRequireSpecs(specs []Value) {
	for _, spec := range specs {
		if lst, ok := spec.(List); ok && len(lst) == 2 {
			if sym, ok := lst[0].(Symbol); ok && sym == "quote" {
				spec = lst[1]
			}
		}
		switch s := spec.(type) {
		case Vector:
			processRequireVec(s)
		case Symbol:
			globalNSRegistry.FindOrCreate(string(s))
		}
	}
}

func processRequireVec(v Vector) {
	if len(v) == 0 {
		return
	}
	nsName, ok := v[0].(Symbol)
	if !ok {
		panic("require: namespace name must be a symbol")
	}
	targetNS := string(nsName)
	globalNSRegistry.FindOrCreate(targetNS)
	cur := globalNSRegistry.Current()

	for i := 1; i < len(v)-1; i += 2 {
		kw, ok := v[i].(Keyword)
		if !ok {
			continue
		}
		switch kw {
		case "as":
			alias, ok := v[i+1].(Symbol)
			if !ok {
				panic(":as value must be a symbol")
			}
			cur.Aliases[string(alias)] = targetNS
		case "refer":
			switch ref := v[i+1].(type) {
			case Vector:
				for _, sym := range ref {
					if name, ok := sym.(Symbol); ok {
						cur.Refers[name] = targetNS
					}
				}
			case Keyword:
				if ref == "all" {
					ns := globalNSRegistry.FindOrCreate(targetNS)
					for sym := range ns.Bindings {
						cur.Refers[sym] = targetNS
					}
				}
			default:
				panic(":refer value must be a vector or :all")
			}
		}
	}
}

// Eval evaluates a value in an environment
func Eval(val Value, env *Env) Value {
	switch v := val.(type) {
	case Nil, Bool, Int, Float, String, Keyword, *Fn, *ExternalValue:
		return v

	case Symbol:
		result, ok := env.Get(v)
		if !ok {
			result, ok = globalNSRegistry.ResolveSymbol(v)
		}
		if !ok {
			panic(fmt.Sprintf("undefined symbol: %s", v))
		}
		return result

	case Vector:
		result := make(Vector, len(v))
		for i, elem := range v {
			result[i] = Eval(elem, env)
		}
		return result

	case HashMap:
		result := make(HashMap)
		for key, val := range v {
			result[Eval(key, env)] = Eval(val, env)
		}
		return result

	case List:
		if len(v) == 0 {
			return v
		}

		// Check for special forms
		if sym, ok := v[0].(Symbol); ok {
			switch sym {
			case "quote":
				if len(v) != 2 {
					panic("quote requires exactly one argument")
				}
				return v[1]

			case "defn":
				if len(v) < 4 {
					panic("defn requires name, params, and body")
				}
				name, ok := v[1].(Symbol)
				if !ok {
					panic("defn first argument must be a symbol")
				}
				fnForm := make(List, 0, len(v)-1)
				fnForm = append(fnForm, Symbol("fn"))
				fnForm = append(fnForm, v[2:]...)
				value := Eval(fnForm, env)
				env.Set(name, value)
				globalNSRegistry.Intern(name, value)
				return value

			case "def":
				if len(v) != 3 {
					panic("def requires exactly two arguments")
				}
				name, ok := v[1].(Symbol)
				if !ok {
					panic("def first argument must be a symbol")
				}
				value := Eval(v[2], env)
				env.Set(name, value)
				globalNSRegistry.Intern(name, value)
				return value

			case "let":
				if len(v) < 2 {
					panic("let requires at least bindings and body")
				}
				bindings, ok := v[1].(Vector)
				if !ok {
					panic("let bindings must be a vector")
				}
				if len(bindings)%2 != 0 {
					panic("let bindings must have even number of elements")
				}
				letEnv := NewEnv(env)
				for i := 0; i < len(bindings); i += 2 {
					name, ok := bindings[i].(Symbol)
					if !ok {
						panic("let binding name must be a symbol")
					}
					letEnv.Set(name, Eval(bindings[i+1], letEnv))
				}
				var result Value = Nil{}
				for _, expr := range v[2:] {
					result = Eval(expr, letEnv)
				}
				return result

			case "fn":
				if len(v) < 3 {
					panic("fn requires parameters and body")
				}
				params, ok := v[1].(Vector)
				if !ok {
					panic("fn parameters must be a vector")
				}
				paramNames := make([]Symbol, len(params))
				for i, p := range params {
					name, ok := p.(Symbol)
					if !ok {
						panic("fn parameter must be a symbol")
					}
					paramNames[i] = name
				}
				body := v[2:]
				closure := env
				return &Fn{
					Name: "lambda",
					Func: func(args []Value) Value {
						if len(args) != len(paramNames) {
							panic(fmt.Sprintf("wrong number of arguments: expected %d, got %d",
								len(paramNames), len(args)))
						}
						fnEnv := NewEnv(closure)
						for i, name := range paramNames {
							fnEnv.Set(name, args[i])
						}
						var result Value = Nil{}
						for _, expr := range body {
							result = Eval(expr, fnEnv)
						}
						return result
					},
				}

			case "if":
				if len(v) < 3 || len(v) > 4 {
					panic("if requires 2 or 3 arguments")
				}
				cond := Eval(v[1], env)
				if isTruthy(cond) {
					return Eval(v[2], env)
				}
				if len(v) == 4 {
					return Eval(v[3], env)
				}
				return Nil{}

			case "cond":
				for i := 1; i < len(v)-1; i += 2 {
					if kw, ok := v[i].(Keyword); ok && string(kw) == "else" {
						return Eval(v[i+1], env)
					}
					if isTruthy(Eval(v[i], env)) {
						return Eval(v[i+1], env)
					}
				}
				return Nil{}

			case "do":
				var result Value = Nil{}
				for _, expr := range v[1:] {
					result = Eval(expr, env)
				}
				return result

			case "loop":
				// (loop [x 0 y 1] body...)
				// recur in tail position restarts the loop with new bindings.
				if len(v) < 2 {
					panic("loop requires bindings and body")
				}
				bindings, ok := v[1].(Vector)
				if !ok {
					panic("loop bindings must be a vector")
				}
				if len(bindings)%2 != 0 {
					panic("loop bindings must have even number of elements")
				}
				nBindings := len(bindings) / 2
				names := make([]Symbol, nBindings)
				for i := 0; i < nBindings; i++ {
					n, ok := bindings[i*2].(Symbol)
					if !ok {
						panic("loop binding name must be a symbol")
					}
					names[i] = n
				}
				body := v[2:]

				// Initialize bindings
				loopEnv := NewEnv(env)
				for i := 0; i < nBindings; i++ {
					loopEnv.Set(names[i], Eval(bindings[i*2+1], loopEnv))
				}

				// TCO loop: iterate until result is not a Recur
				for {
					var result Value = Nil{}
					for _, expr := range body {
						result = Eval(expr, loopEnv)
					}
					rec, isRecur := result.(Recur)
					if !isRecur {
						return result
					}
					if len(rec.Args) != nBindings {
						panic(fmt.Sprintf("recur arity mismatch: expected %d, got %d",
							nBindings, len(rec.Args)))
					}
					// Rebind without growing the stack
					for i, name := range names {
						loopEnv.Set(name, rec.Args[i])
					}
				}

			case "recur":
				// (recur new-x new-y)
				// Returns a Recur sentinel; only valid inside loop.
				args := make([]Value, len(v)-1)
				for i, arg := range v[1:] {
					args[i] = Eval(arg, env)
				}
				return Recur{Args: args}

			case "require":
				processRequireSpecs(v[1:])
				return Nil{}

			case "ns":
				return evalNSForm(v, env)

			case "in-ns":
				if len(v) != 2 {
					panic("in-ns requires exactly one argument")
				}
				nsVal := Eval(v[1], env)
				var nsName string
				switch n := nsVal.(type) {
				case Symbol:
					nsName = string(n)
				case String:
					nsName = string(n)
				default:
					panic("in-ns requires a symbol or string")
				}
				globalNSRegistry.SetCurrent(nsName)
				return Symbol(nsName)
			}
		}

		// Function application
		fn := Eval(v[0], env)
		args := make([]Value, len(v)-1)
		for i, arg := range v[1:] {
			args[i] = Eval(arg, env)
		}

		switch f := fn.(type) {
		case *Fn:
			return f.Func(args)
		default:
			panic(fmt.Sprintf("cannot call %T", fn))
		}

	default:
		panic(fmt.Sprintf("cannot evaluate %T", val))
	}
}

func isTruthy(v Value) bool {
	switch val := v.(type) {
	case Nil:
		return false
	case Bool:
		return bool(val)
	default:
		return true
	}
}

// CreateStandardEnv creates an environment with standard functions
func CreateStandardEnv() *Env {
	env := NewEnv(nil)

	// Arithmetic
	env.Set("+", &Fn{"+", func(args []Value) Value {
		var sum int64
		for _, a := range args {
			sum += int64(a.(Int))
		}
		return Int(sum)
	}})

	env.Set("-", &Fn{"-", func(args []Value) Value {
		if len(args) == 0 {
			return Int(0)
		}
		if len(args) == 1 {
			return Int(-int64(args[0].(Int)))
		}
		result := int64(args[0].(Int))
		for _, a := range args[1:] {
			result -= int64(a.(Int))
		}
		return Int(result)
	}})

	env.Set("*", &Fn{"*", func(args []Value) Value {
		var product int64 = 1
		for _, a := range args {
			product *= int64(a.(Int))
		}
		return Int(product)
	}})

	env.Set("/", &Fn{"/", func(args []Value) Value {
		if len(args) < 2 {
			panic("/ requires at least 2 arguments")
		}
		result := int64(args[0].(Int))
		for _, a := range args[1:] {
			result /= int64(a.(Int))
		}
		return Int(result)
	}})

	// Comparison
	env.Set("=", &Fn{"=", func(args []Value) Value {
		if len(args) < 2 {
			return Bool(true)
		}
		first := args[0]
		for _, a := range args[1:] {
			if fmt.Sprintf("%v", first) != fmt.Sprintf("%v", a) {
				return Bool(false)
			}
		}
		return Bool(true)
	}})

	env.Set("<", &Fn{"<", func(args []Value) Value {
		if len(args) < 2 {
			return Bool(true)
		}
		for i := 0; i < len(args)-1; i++ {
			if int64(args[i].(Int)) >= int64(args[i+1].(Int)) {
				return Bool(false)
			}
		}
		return Bool(true)
	}})

	env.Set(">", &Fn{">", func(args []Value) Value {
		if len(args) < 2 {
			return Bool(true)
		}
		for i := 0; i < len(args)-1; i++ {
			if int64(args[i].(Int)) <= int64(args[i+1].(Int)) {
				return Bool(false)
			}
		}
		return Bool(true)
	}})

	// Predicates
	env.Set("nil?", &Fn{"nil?", func(args []Value) Value {
		if len(args) != 1 {
			panic("nil? requires exactly 1 argument")
		}
		_, ok := args[0].(Nil)
		return Bool(ok)
	}})

	// I/O
	env.Set("println", &Fn{"println", func(args []Value) Value {
		parts := make([]string, len(args))
		for i, a := range args {
			switch v := a.(type) {
			case String:
				parts[i] = string(v)
			default:
				parts[i] = v.String()
			}
		}
		fmt.Println(strings.Join(parts, " "))
		return Nil{}
	}})

	env.Set("print", &Fn{"print", func(args []Value) Value {
		parts := make([]string, len(args))
		for i, a := range args {
			switch v := a.(type) {
			case String:
				parts[i] = string(v)
			default:
				parts[i] = v.String()
			}
		}
		fmt.Print(strings.Join(parts, " "))
		return Nil{}
	}})

	// String operations
	env.Set("str", &Fn{"str", func(args []Value) Value {
		var sb strings.Builder
		for _, a := range args {
			switch v := a.(type) {
			case String:
				sb.WriteString(string(v))
			default:
				sb.WriteString(v.String())
			}
		}
		return String(sb.String())
	}})

	// Collections
	env.Set("vector", &Fn{"vector", func(args []Value) Value {
		return Vector(args)
	}})

	env.Set("count", &Fn{"count", func(args []Value) Value {
		if len(args) != 1 {
			panic("count requires exactly 1 argument")
		}
		switch v := args[0].(type) {
		case Vector:
			return Int(len(v))
		case List:
			return Int(len(v))
		case String:
			return Int(len(v))
		case Nil:
			return Int(0)
		default:
			panic(fmt.Sprintf("count not supported for %T", v))
		}
	}})

	env.Set("first", &Fn{"first", func(args []Value) Value {
		if len(args) != 1 {
			panic("first requires exactly 1 argument")
		}
		switch v := args[0].(type) {
		case Vector:
			if len(v) == 0 {
				return Nil{}
			}
			return v[0]
		case List:
			if len(v) == 0 {
				return Nil{}
			}
			return v[0]
		case Nil:
			return Nil{}
		default:
			panic(fmt.Sprintf("first not supported for %T", v))
		}
	}})

	env.Set("rest", &Fn{"rest", func(args []Value) Value {
		if len(args) != 1 {
			panic("rest requires exactly 1 argument")
		}
		switch v := args[0].(type) {
		case Vector:
			if len(v) <= 1 {
				return List{}
			}
			return List(v[1:])
		case List:
			if len(v) <= 1 {
				return List{}
			}
			return v[1:]
		case Nil:
			return List{}
		default:
			panic(fmt.Sprintf("rest not supported for %T", v))
		}
	}})

	env.Set("nth", &Fn{"nth", func(args []Value) Value {
		if len(args) < 2 {
			panic("nth requires at least 2 arguments")
		}
		idx := int(args[1].(Int))
		switch v := args[0].(type) {
		case Vector:
			if idx < 0 || idx >= len(v) {
				if len(args) > 2 {
					return args[2]
				}
				panic("index out of bounds")
			}
			return v[idx]
		case List:
			if idx < 0 || idx >= len(v) {
				if len(args) > 2 {
					return args[2]
				}
				panic("index out of bounds")
			}
			return v[idx]
		default:
			panic(fmt.Sprintf("nth not supported for %T", v))
		}
	}})

	env.Set("conj", &Fn{"conj", func(args []Value) Value {
		if len(args) < 2 {
			panic("conj requires at least 2 arguments")
		}
		switch coll := args[0].(type) {
		case Vector:
			result := make(Vector, len(coll))
			copy(result, coll)
			return append(result, args[1:]...)
		case List:
			result := make(List, 0, len(coll)+len(args)-1)
			for i := len(args) - 1; i >= 1; i-- {
				result = append(result, args[i])
			}
			return append(result, coll...)
		case Nil:
			result := make(List, len(args)-1)
			for i := len(args) - 1; i >= 1; i-- {
				result[len(args)-1-i] = args[i]
			}
			return result
		default:
			panic(fmt.Sprintf("conj not supported for %T", coll))
		}
	}})

	// HashMap operations
	env.Set("get", &Fn{"get", func(args []Value) Value {
		if len(args) < 2 {
			panic("get requires at least 2 arguments")
		}
		switch m := args[0].(type) {
		case HashMap:
			if v, ok := m[args[1]]; ok {
				return v
			}
			if len(args) > 2 {
				return args[2]
			}
			return Nil{}
		case Nil:
			if len(args) > 2 {
				return args[2]
			}
			return Nil{}
		default:
			panic(fmt.Sprintf("get not supported for %T", m))
		}
	}})

	env.Set("assoc", &Fn{"assoc", func(args []Value) Value {
		if len(args) < 3 || (len(args)-1)%2 != 0 {
			panic("assoc requires map and key-value pairs")
		}
		var result HashMap
		switch m := args[0].(type) {
		case HashMap:
			result = make(HashMap, len(m))
			for k, v := range m {
				result[k] = v
			}
		case Nil:
			result = make(HashMap)
		default:
			panic(fmt.Sprintf("assoc not supported for %T", m))
		}
		for i := 1; i < len(args); i += 2 {
			result[args[i]] = args[i+1]
		}
		return result
	}})

	// Comparison: >=, <=, mod
	env.Set(">=", &Fn{">=", func(args []Value) Value {
		if len(args) < 2 {
			return Bool(true)
		}
		for i := 0; i < len(args)-1; i++ {
			if int64(args[i].(Int)) < int64(args[i+1].(Int)) {
				return Bool(false)
			}
		}
		return Bool(true)
	}})

	env.Set("<=", &Fn{"<=", func(args []Value) Value {
		if len(args) < 2 {
			return Bool(true)
		}
		for i := 0; i < len(args)-1; i++ {
			if int64(args[i].(Int)) > int64(args[i+1].(Int)) {
				return Bool(false)
			}
		}
		return Bool(true)
	}})

	env.Set("mod", &Fn{"mod", func(args []Value) Value {
		if len(args) != 2 {
			panic("mod requires exactly 2 arguments")
		}
		a := int64(args[0].(Int))
		b := int64(args[1].(Int))
		r := a % b
		if r < 0 && b > 0 {
			r += b
		}
		return Int(r)
	}})

	env.Set("inc", &Fn{"inc", func(args []Value) Value {
		return Int(int64(args[0].(Int)) + 1)
	}})

	env.Set("dec", &Fn{"dec", func(args []Value) Value {
		return Int(int64(args[0].(Int)) - 1)
	}})

	env.Set("abs", &Fn{"abs", func(args []Value) Value {
		n := int64(args[0].(Int))
		if n < 0 {
			return Int(-n)
		}
		return Int(n)
	}})

	env.Set("max", &Fn{"max", func(args []Value) Value {
		best := int64(args[0].(Int))
		for _, a := range args[1:] {
			if v := int64(a.(Int)); v > best {
				best = v
			}
		}
		return Int(best)
	}})

	env.Set("min", &Fn{"min", func(args []Value) Value {
		best := int64(args[0].(Int))
		for _, a := range args[1:] {
			if v := int64(a.(Int)); v < best {
				best = v
			}
		}
		return Int(best)
	}})

	// Higher-order: map, reduce, filter, apply, some, every?
	env.Set("map", &Fn{"map", func(args []Value) Value {
		if len(args) != 2 {
			panic("map requires function and collection")
		}
		fn := args[0].(*Fn)
		var elems []Value
		switch c := args[1].(type) {
		case Vector:
			elems = []Value(c)
		case List:
			elems = []Value(c)
		default:
			panic(fmt.Sprintf("map not supported for %T", c))
		}
		result := make(Vector, len(elems))
		for i, e := range elems {
			result[i] = fn.Func([]Value{e})
		}
		return result
	}})

	env.Set("reduce", &Fn{"reduce", func(args []Value) Value {
		if len(args) < 2 || len(args) > 3 {
			panic("reduce requires 2 or 3 arguments")
		}
		fn := args[0].(*Fn)
		var init Value
		var coll []Value
		if len(args) == 3 {
			init = args[1]
			switch c := args[2].(type) {
			case Vector:
				coll = []Value(c)
			case List:
				coll = []Value(c)
			case Nil:
				return init
			default:
				panic(fmt.Sprintf("reduce not supported for %T", c))
			}
		} else {
			switch c := args[1].(type) {
			case Vector:
				coll = []Value(c)
			case List:
				coll = []Value(c)
			default:
				panic(fmt.Sprintf("reduce not supported for %T", c))
			}
			if len(coll) == 0 {
				return fn.Func(nil)
			}
			init = coll[0]
			coll = coll[1:]
		}
		acc := init
		for _, e := range coll {
			acc = fn.Func([]Value{acc, e})
		}
		return acc
	}})

	env.Set("filter", &Fn{"filter", func(args []Value) Value {
		if len(args) != 2 {
			panic("filter requires predicate and collection")
		}
		fn := args[0].(*Fn)
		var elems []Value
		switch c := args[1].(type) {
		case Vector:
			elems = []Value(c)
		case List:
			elems = []Value(c)
		default:
			panic(fmt.Sprintf("filter not supported for %T", c))
		}
		result := make(Vector, 0)
		for _, e := range elems {
			if isTruthy(fn.Func([]Value{e})) {
				result = append(result, e)
			}
		}
		return result
	}})

	env.Set("apply", &Fn{"apply", func(args []Value) Value {
		if len(args) < 2 {
			panic("apply requires function and args")
		}
		fn := args[0].(*Fn)
		last := args[len(args)-1]
		var fnArgs []Value
		fnArgs = append(fnArgs, args[1:len(args)-1]...)
		switch c := last.(type) {
		case Vector:
			fnArgs = append(fnArgs, []Value(c)...)
		case List:
			fnArgs = append(fnArgs, []Value(c)...)
		default:
			fnArgs = append(fnArgs, last)
		}
		return fn.Func(fnArgs)
	}})

	env.Set("some", &Fn{"some", func(args []Value) Value {
		fn := args[0].(*Fn)
		var elems []Value
		switch c := args[1].(type) {
		case Vector:
			elems = []Value(c)
		case List:
			elems = []Value(c)
		}
		for _, e := range elems {
			if v := fn.Func([]Value{e}); isTruthy(v) {
				return v
			}
		}
		return Nil{}
	}})

	env.Set("every?", &Fn{"every?", func(args []Value) Value {
		fn := args[0].(*Fn)
		var elems []Value
		switch c := args[1].(type) {
		case Vector:
			elems = []Value(c)
		case List:
			elems = []Value(c)
		}
		for _, e := range elems {
			if !isTruthy(fn.Func([]Value{e})) {
				return Bool(false)
			}
		}
		return Bool(true)
	}})

	// Collection: concat, into, keys, vals, dissoc, merge, hash-map, empty?, contains?, range
	env.Set("concat", &Fn{"concat", func(args []Value) Value {
		result := make(Vector, 0)
		for _, a := range args {
			switch c := a.(type) {
			case Vector:
				result = append(result, []Value(c)...)
			case List:
				result = append(result, []Value(c)...)
			case Nil:
				// skip
			default:
				result = append(result, c)
			}
		}
		return result
	}})

	env.Set("into", &Fn{"into", func(args []Value) Value {
		if len(args) != 2 {
			panic("into requires 2 arguments")
		}
		switch target := args[0].(type) {
		case Vector:
			result := make(Vector, len(target))
			copy(result, target)
			switch src := args[1].(type) {
			case Vector:
				return append(result, []Value(src)...)
			case List:
				return append(result, []Value(src)...)
			}
			return result
		case HashMap:
			result := make(HashMap, len(target))
			for k, v := range target {
				result[k] = v
			}
			switch src := args[1].(type) {
			case Vector:
				for _, e := range src {
					pair := e.(Vector)
					result[pair[0]] = pair[1]
				}
			case HashMap:
				for k, v := range src {
					result[k] = v
				}
			}
			return result
		case Nil:
			return args[1]
		default:
			panic(fmt.Sprintf("into not supported for %T", target))
		}
	}})

	env.Set("keys", &Fn{"keys", func(args []Value) Value {
		m := args[0].(HashMap)
		result := make(Vector, 0, len(m))
		for k := range m {
			result = append(result, k)
		}
		return result
	}})

	env.Set("vals", &Fn{"vals", func(args []Value) Value {
		m := args[0].(HashMap)
		result := make(Vector, 0, len(m))
		for _, v := range m {
			result = append(result, v)
		}
		return result
	}})

	env.Set("dissoc", &Fn{"dissoc", func(args []Value) Value {
		m := args[0].(HashMap)
		result := make(HashMap, len(m))
		for k, v := range m {
			result[k] = v
		}
		for _, k := range args[1:] {
			delete(result, k)
		}
		return result
	}})

	env.Set("merge", &Fn{"merge", func(args []Value) Value {
		result := make(HashMap)
		for _, a := range args {
			switch m := a.(type) {
			case HashMap:
				for k, v := range m {
					result[k] = v
				}
			case Nil:
				// skip
			}
		}
		return result
	}})

	env.Set("hash-map", &Fn{"hash-map", func(args []Value) Value {
		if len(args)%2 != 0 {
			panic("hash-map requires even number of arguments")
		}
		result := make(HashMap, len(args)/2)
		for i := 0; i < len(args); i += 2 {
			result[args[i]] = args[i+1]
		}
		return result
	}})

	env.Set("empty?", &Fn{"empty?", func(args []Value) Value {
		switch c := args[0].(type) {
		case Vector:
			return Bool(len(c) == 0)
		case List:
			return Bool(len(c) == 0)
		case HashMap:
			return Bool(len(c) == 0)
		case String:
			return Bool(len(c) == 0)
		case Nil:
			return Bool(true)
		}
		return Bool(false)
	}})

	env.Set("contains?", &Fn{"contains?", func(args []Value) Value {
		switch m := args[0].(type) {
		case HashMap:
			_, ok := m[args[1]]
			return Bool(ok)
		case Vector:
			idx := int(args[1].(Int))
			return Bool(idx >= 0 && idx < len(m))
		}
		return Bool(false)
	}})

	env.Set("range", &Fn{"range", func(args []Value) Value {
		var start, end, step int64
		switch len(args) {
		case 1:
			end = int64(args[0].(Int))
		case 2:
			start = int64(args[0].(Int))
			end = int64(args[1].(Int))
		case 3:
			start = int64(args[0].(Int))
			end = int64(args[1].(Int))
			step = int64(args[2].(Int))
		default:
			panic("range requires 1-3 arguments")
		}
		if step == 0 {
			if start < end {
				step = 1
			} else {
				step = -1
			}
		}
		result := make(Vector, 0)
		if step > 0 {
			for i := start; i < end; i += step {
				result = append(result, Int(i))
			}
		} else {
			for i := start; i > end; i += step {
				result = append(result, Int(i))
			}
		}
		return result
	}})

	env.Set("update", &Fn{"update", func(args []Value) Value {
		if len(args) < 3 {
			panic("update requires map, key, and function")
		}
		m := args[0].(HashMap)
		key := args[1]
		fn := args[2].(*Fn)
		result := make(HashMap, len(m))
		for k, v := range m {
			result[k] = v
		}
		old := result[key]
		if old == nil {
			old = Nil{}
		}
		fnArgs := []Value{old}
		fnArgs = append(fnArgs, args[3:]...)
		result[key] = fn.Func(fnArgs)
		return result
	}})

	env.Set("keyword", &Fn{"keyword", func(args []Value) Value {
		switch v := args[0].(type) {
		case String:
			return Keyword(v)
		case Keyword:
			return v
		case Symbol:
			return Keyword(v)
		default:
			return Keyword(fmt.Sprint(v))
		}
	}})

	env.Set("symbol", &Fn{"symbol", func(args []Value) Value {
		switch v := args[0].(type) {
		case String:
			return Symbol(v)
		case Symbol:
			return v
		case Keyword:
			return Symbol(v)
		default:
			return Symbol(fmt.Sprint(v))
		}
	}})

	env.Set("name", &Fn{"name", func(args []Value) Value {
		switch v := args[0].(type) {
		case Keyword:
			return String(v)
		case Symbol:
			return String(v)
		case String:
			return v
		default:
			return String(fmt.Sprint(v))
		}
	}})

	env.Set("map?", &Fn{"map?", func(args []Value) Value {
		_, ok := args[0].(HashMap)
		return Bool(ok)
	}})

	env.Set("keyword?", &Fn{"keyword?", func(args []Value) Value {
		_, ok := args[0].(Keyword)
		return Bool(ok)
	}})

	env.Set("list?", &Fn{"list?", func(args []Value) Value {
		_, ok := args[0].(List)
		return Bool(ok)
	}})

	env.Set("cons", &Fn{"cons", func(args []Value) Value {
		if len(args) != 2 {
			panic("cons requires exactly 2 arguments")
		}
		switch c := args[1].(type) {
		case Vector:
			result := make(List, 0, len(c)+1)
			result = append(result, args[0])
			result = append(result, []Value(c)...)
			return result
		case List:
			result := make(List, 0, len(c)+1)
			result = append(result, args[0])
			result = append(result, []Value(c)...)
			return result
		case Nil:
			return List{args[0]}
		default:
			return List{args[0], args[1]}
		}
	}})

	env.Set("mapv", &Fn{"mapv", func(args []Value) Value {
		if len(args) != 2 {
			panic("mapv requires function and collection")
		}
		fn := args[0].(*Fn)
		var elems []Value
		switch c := args[1].(type) {
		case Vector:
			elems = []Value(c)
		case List:
			elems = []Value(c)
		}
		result := make(Vector, len(elems))
		for i, e := range elems {
			result[i] = fn.Func([]Value{e})
		}
		return result
	}})

	// Type predicates
	env.Set("string?", &Fn{"string?", func(args []Value) Value {
		_, ok := args[0].(String)
		return Bool(ok)
	}})

	env.Set("number?", &Fn{"number?", func(args []Value) Value {
		switch args[0].(type) {
		case Int, Float:
			return Bool(true)
		}
		return Bool(false)
	}})

	env.Set("vector?", &Fn{"vector?", func(args []Value) Value {
		_, ok := args[0].(Vector)
		return Bool(ok)
	}})

	// Type coercion
	env.Set("type", &Fn{"type", func(args []Value) Value {
		if len(args) != 1 {
			panic("type requires exactly 1 argument")
		}
		switch args[0].(type) {
		case Nil:
			return Keyword("nil")
		case Bool:
			return Keyword("boolean")
		case Int:
			return Keyword("int")
		case Float:
			return Keyword("float")
		case String:
			return Keyword("string")
		case Symbol:
			return Keyword("symbol")
		case Keyword:
			return Keyword("keyword")
		case List:
			return Keyword("list")
		case Vector:
			return Keyword("vector")
		case HashMap:
			return Keyword("hashmap")
		case *Fn:
			return Keyword("function")
		case *ExternalValue:
			return Keyword("external")
		default:
			return Keyword("unknown")
		}
	}})

	// OS functions
	env.Set("getenv", &Fn{"getenv", func(args []Value) Value {
		if len(args) < 1 {
			panic("getenv requires at least 1 argument")
		}
		name := string(args[0].(String))
		val := os.Getenv(name)
		if val == "" && len(args) > 1 {
			return args[1]
		}
		return String(val)
	}})

	// Or/And
	env.Set("or", &Fn{"or", func(args []Value) Value {
		for _, a := range args {
			if isTruthy(a) {
				return a
			}
		}
		return Nil{}
	}})

	env.Set("and", &Fn{"and", func(args []Value) Value {
		var result Value = Bool(true)
		for _, a := range args {
			if !isTruthy(a) {
				return a
			}
			result = a
		}
		return result
	}})

	env.Set("not", &Fn{"not", func(args []Value) Value {
		if len(args) != 1 {
			panic("not requires exactly 1 argument")
		}
		return Bool(!isTruthy(args[0]))
	}})

	// Namespace introspection
	env.Set("ns-name", &Fn{"ns-name", func(args []Value) Value {
		return Symbol(globalNSRegistry.CurrentName())
	}})

	env.Set("all-ns", &Fn{"all-ns", func(args []Value) Value {
		names := globalNSRegistry.AllNames()
		result := make(Vector, len(names))
		for i, n := range names {
			result[i] = Symbol(n)
		}
		return result
	}})

	env.Set("ns-aliases", &Fn{"ns-aliases", func(args []Value) Value {
		var ns *Namespace
		if len(args) > 0 {
			ns = globalNSRegistry.FindOrCreate(string(args[0].(Symbol)))
		} else {
			ns = globalNSRegistry.Current()
		}
		result := make(HashMap)
		for alias, target := range ns.Aliases {
			result[Symbol(alias)] = Symbol(target)
		}
		return result
	}})

	env.Set("ns-map", &Fn{"ns-map", func(args []Value) Value {
		var ns *Namespace
		if len(args) > 0 {
			ns = globalNSRegistry.FindOrCreate(string(args[0].(Symbol)))
		} else {
			ns = globalNSRegistry.Current()
		}
		result := make(HashMap)
		for sym, val := range ns.Bindings {
			result[sym] = val
		}
		return result
	}})

	env.Set("ns-resolve", &Fn{"ns-resolve", func(args []Value) Value {
		if len(args) < 1 {
			panic("ns-resolve requires a symbol")
		}
		sym, ok := args[0].(Symbol)
		if !ok {
			panic("ns-resolve requires a symbol")
		}
		val, found := globalNSRegistry.ResolveSymbol(sym)
		if !found {
			return Nil{}
		}
		return val
	}})

	// trampoline — bounces thunks until a non-thunk value.
	env.Set("trampoline", &Fn{"trampoline", func(args []Value) Value {
		if len(args) < 1 {
			panic("trampoline requires at least a function")
		}
		fn := args[0].(*Fn)
		result := fn.Func(args[1:])
		for {
			thunk, ok := result.(Thunk)
			if !ok {
				return result
			}
			result = thunk.Func()
		}
	}})

	// thunk — wrap an expression as a zero-arg thunk for trampoline.
	env.Set("thunk", &Fn{"thunk", func(args []Value) Value {
		if len(args) < 1 {
			panic("thunk requires at least a function")
		}
		fn := args[0].(*Fn)
		captured := make([]Value, len(args)-1)
		copy(captured, args[1:])
		return Thunk{Func: func() Value {
			return fn.Func(captured)
		}}
	}})

	// memoize — returns a memoized version of a function.
	env.Set("memoize", &Fn{"memoize", func(args []Value) Value {
		if len(args) != 1 {
			panic("memoize requires exactly 1 function")
		}
		fn := args[0].(*Fn)
		cache := make(map[string]Value)
		return &Fn{
			Name: fn.Name + "/memo",
			Func: func(innerArgs []Value) Value {
				key := fmt.Sprintf("%v", innerArgs)
				if v, ok := cache[key]; ok {
					return v
				}
				result := fn.Func(innerArgs)
				cache[key] = result
				return result
			},
		}
	}})

	env.Set("memo-stats", &Fn{"memo-stats", func(args []Value) Value {
		if len(args) != 1 {
			panic("memo-stats requires 1 function")
		}
		fn := args[0].(*Fn)
		return String(fn.Name)
	}})

	// mod — integer modular arithmetic (needed for GF(3) in Lisp)
	env.Set("mod", &Fn{"mod", func(args []Value) Value {
		if len(args) != 2 {
			panic("mod requires exactly 2 arguments")
		}
		a := int64(args[0].(Int))
		b := int64(args[1].(Int))
		r := a % b
		if r < 0 {
			r += b
		}
		return Int(r)
	}})

	// map — (map f coll) → vector
	env.Set("map", &Fn{"map", func(args []Value) Value {
		if len(args) != 2 {
			panic("map requires function and collection")
		}
		fn := args[0].(*Fn)
		var items []Value
		switch coll := args[1].(type) {
		case Vector:
			items = []Value(coll)
		case List:
			items = []Value(coll)
		default:
			panic(fmt.Sprintf("map not supported for %T", coll))
		}
		result := make(Vector, len(items))
		for i, item := range items {
			result[i] = fn.Func([]Value{item})
		}
		return result
	}})

	// reduce — (reduce f init coll) → value
	env.Set("reduce", &Fn{"reduce", func(args []Value) Value {
		if len(args) < 2 || len(args) > 3 {
			panic("reduce requires (fn init? coll)")
		}
		fn := args[0].(*Fn)
		var acc Value
		var coll []Value
		if len(args) == 3 {
			acc = args[1]
			switch c := args[2].(type) {
			case Vector:
				coll = []Value(c)
			case List:
				coll = []Value(c)
			default:
				panic(fmt.Sprintf("reduce not supported for %T", c))
			}
		} else {
			switch c := args[1].(type) {
			case Vector:
				if len(c) == 0 {
					return fn.Func(nil)
				}
				acc = c[0]
				coll = []Value(c[1:])
			case List:
				if len(c) == 0 {
					return fn.Func(nil)
				}
				acc = c[0]
				coll = []Value(c[1:])
			default:
				panic(fmt.Sprintf("reduce not supported for %T", c))
			}
		}
		for _, item := range coll {
			acc = fn.Func([]Value{acc, item})
		}
		return acc
	}})

	return env
}
