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

// Eval evaluates a value in an environment
func Eval(val Value, env *Env) Value {
	switch v := val.(type) {
	case Nil, Bool, Int, Float, String, Keyword, *Fn, *ExternalValue:
		return v

	case Symbol:
		result, ok := env.Get(v)
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

			case "do":
				var result Value = Nil{}
				for _, expr := range v[1:] {
					result = Eval(expr, env)
				}
				return result

			case "require":
				// No-op for now - namespaces are pre-registered
				return Nil{}

			case "ns":
				// No-op - namespaces handled differently
				return Nil{}
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

	return env
}
