//go:build darwin

package vm

import (
	"strings"
	"testing"

	"github.com/bmorphism/boxxy/internal/lisp"
)

// helper: fresh env + namespace registry for each test
func nsTestEnv(t *testing.T) *lisp.Env {
	t.Helper()
	lisp.ResetNSRegistry()
	env := lisp.CreateStandardEnv()
	RegisterNamespace(env)
	return env
}

func evalStr(t *testing.T, env *lisp.Env, code string) lisp.Value {
	t.Helper()
	reader := lisp.NewReader(strings.NewReader(code))
	var result lisp.Value
	for {
		val, err := reader.Read()
		if err != nil {
			break
		}
		result = lisp.Eval(val, env)
	}
	return result
}

// --- Basic ns form ---

func TestNSCreatesNamespace(t *testing.T) {
	env := nsTestEnv(t)
	result := evalStr(t, env, `(ns boxxy.test-ns)`)
	if result.String() != "boxxy.test-ns" {
		t.Fatalf("expected boxxy.test-ns, got %s", result)
	}
	// ns-name should reflect the switch
	name := evalStr(t, env, `(ns-name)`)
	if name.String() != "boxxy.test-ns" {
		t.Fatalf("ns-name: expected boxxy.test-ns, got %s", name)
	}
}

func TestNSSwitchesBack(t *testing.T) {
	env := nsTestEnv(t)
	evalStr(t, env, `(ns boxxy.alpha)`)
	evalStr(t, env, `(ns boxxy.beta)`)
	name := evalStr(t, env, `(ns-name)`)
	if name.String() != "boxxy.beta" {
		t.Fatalf("expected boxxy.beta, got %s", name)
	}
}

func TestInNS(t *testing.T) {
	env := nsTestEnv(t)
	evalStr(t, env, `(ns boxxy.first)`)
	evalStr(t, env, `(in-ns 'boxxy.second)`)
	name := evalStr(t, env, `(ns-name)`)
	if name.String() != "boxxy.second" {
		t.Fatalf("expected boxxy.second, got %s", name)
	}
}

// --- def interns in namespace ---

func TestDefInternsInCurrentNS(t *testing.T) {
	env := nsTestEnv(t)
	evalStr(t, env, `(ns boxxy.defs)`)
	evalStr(t, env, `(def x 42)`)

	// x should be resolvable in boxxy.defs
	val := evalStr(t, env, `x`)
	if val.String() != "42" {
		t.Fatalf("expected 42, got %s", val)
	}

	// Switch to another ns; x should NOT be in namespace resolution
	// (it IS in the env though, for backward compat)
	evalStr(t, env, `(ns boxxy.other)`)
	reg := lisp.GetNSRegistry()
	_, found := reg.ResolveSymbol(lisp.Symbol("x"))
	if found {
		t.Fatal("x should not resolve in boxxy.other namespace")
	}
}

// --- require with :as ---

func TestRequireAs(t *testing.T) {
	env := nsTestEnv(t)

	// Populate a namespace
	lisp.InternInNS("boxxy.math", lisp.Symbol("add"), &lisp.Fn{
		Name: "add",
		Func: func(args []lisp.Value) lisp.Value {
			return lisp.Int(int64(args[0].(lisp.Int)) + int64(args[1].(lisp.Int)))
		},
	})

	evalStr(t, env, `(ns boxxy.consumer (:require [boxxy.math :as m]))`)
	result := evalStr(t, env, `(m/add 3 4)`)
	if result.String() != "7" {
		t.Fatalf("expected 7, got %s", result)
	}
}

// --- require with :refer ---

func TestRequireRefer(t *testing.T) {
	env := nsTestEnv(t)

	lisp.InternInNS("boxxy.util", lisp.Symbol("double"), &lisp.Fn{
		Name: "double",
		Func: func(args []lisp.Value) lisp.Value {
			return lisp.Int(int64(args[0].(lisp.Int)) * 2)
		},
	})

	evalStr(t, env, `(ns boxxy.consumer2 (:require [boxxy.util :refer [double]]))`)
	result := evalStr(t, env, `(double 21)`)
	if result.String() != "42" {
		t.Fatalf("expected 42, got %s", result)
	}
}

// --- require with quoted vector ---

func TestRequireQuoted(t *testing.T) {
	env := nsTestEnv(t)

	lisp.InternInNS("boxxy.q", lisp.Symbol("ping"), &lisp.Fn{
		Name: "ping",
		Func: func(args []lisp.Value) lisp.Value { return lisp.String("pong") },
	})

	evalStr(t, env, `(ns boxxy.qtest)`)
	evalStr(t, env, `(require '[boxxy.q :as q])`)
	result := evalStr(t, env, `(q/ping)`)
	if result.String() != `"pong"` {
		t.Fatalf("expected pong, got %s", result)
	}
}

// --- all-ns introspection ---

func TestAllNS(t *testing.T) {
	env := nsTestEnv(t)
	evalStr(t, env, `(ns boxxy.a)`)
	evalStr(t, env, `(ns boxxy.b)`)
	result := evalStr(t, env, `(all-ns)`)

	vec, ok := result.(lisp.Vector)
	if !ok {
		t.Fatalf("all-ns should return vector, got %T", result)
	}
	names := map[string]bool{}
	for _, v := range vec {
		names[v.String()] = true
	}
	for _, expected := range []string{"boxxy.a", "boxxy.b"} {
		if !names[expected] {
			t.Fatalf("all-ns missing %s, got %v", expected, names)
		}
	}
}

// --- ns-aliases introspection ---

func TestNSAliases(t *testing.T) {
	env := nsTestEnv(t)
	evalStr(t, env, `(ns boxxy.aliased (:require [boxxy.vz :as vz]))`)
	result := evalStr(t, env, `(ns-aliases)`)

	hm, ok := result.(lisp.HashMap)
	if !ok {
		t.Fatalf("ns-aliases should return hashmap, got %T", result)
	}
	target, ok := hm[lisp.Symbol("vz")]
	if !ok {
		t.Fatal("ns-aliases missing 'vz' alias")
	}
	if target.String() != "boxxy.vz" {
		t.Fatalf("vz alias should point to boxxy.vz, got %s", target)
	}
}

// --- Backward compatibility: flat vz/ symbols still work ---

func TestBackwardCompatFlatSymbols(t *testing.T) {
	env := nsTestEnv(t)

	// RegisterNamespace puts flat "vz/nested-virt-supported?" in env.
	// This should still resolve directly.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("flat vz/ symbol lookup panicked: %v", r)
		}
	}()
	evalStr(t, env, `(vz/nested-virt-supported?)`)
}

// --- Namespace-qualified resolution via boxxy.vz ---

func TestNamespaceQualifiedResolution(t *testing.T) {
	env := nsTestEnv(t)

	// RegisterNamespace also populates boxxy.vz namespace.
	// Create a consumer that requires it.
	evalStr(t, env, `(ns boxxy.vm-user (:require [boxxy.vz :as virt]))`)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("namespace-qualified resolution panicked: %v", r)
		}
	}()
	evalStr(t, env, `(virt/nested-virt-supported?)`)
}

// --- Full ns name resolution without alias ---

func TestFullNSNameResolution(t *testing.T) {
	env := nsTestEnv(t)

	lisp.InternInNS("boxxy.direct", lisp.Symbol("value"), lisp.Int(99))

	evalStr(t, env, `(ns boxxy.caller)`)
	result := evalStr(t, env, `boxxy.direct/value`)
	if result.String() != "99" {
		t.Fatalf("expected 99, got %s", result)
	}
}

// --- ns-map ---

func TestNSMap(t *testing.T) {
	env := nsTestEnv(t)

	lisp.InternInNS("boxxy.mapped", lisp.Symbol("a"), lisp.Int(1))
	lisp.InternInNS("boxxy.mapped", lisp.Symbol("b"), lisp.Int(2))

	result := evalStr(t, env, `(ns-map 'boxxy.mapped)`)
	hm, ok := result.(lisp.HashMap)
	if !ok {
		t.Fatalf("ns-map should return hashmap, got %T", result)
	}
	if len(hm) < 2 {
		t.Fatalf("ns-map should have at least 2 entries, got %d", len(hm))
	}
}

// --- ns-resolve ---

func TestNSResolve(t *testing.T) {
	env := nsTestEnv(t)

	lisp.InternInNS("boxxy.res", lisp.Symbol("secret"), lisp.String("found"))

	evalStr(t, env, `(ns boxxy.resolver (:require [boxxy.res :as r]))`)
	result := evalStr(t, env, `(ns-resolve 'r/secret)`)
	if result.String() != `"found"` {
		t.Fatalf("ns-resolve: expected found, got %s", result)
	}
}
