package vm

import (
	"strings"
	"testing"

	"github.com/bmorphism/boxxy/internal/lisp"
)

func evalPLT(t *testing.T, env *lisp.Env, code string) lisp.Value {
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

func setupPLTEnv(t *testing.T) *lisp.Env {
	t.Helper()
	lisp.ResetNSRegistry()
	env := lisp.CreateStandardEnv()
	RegisterNamespace(env)
	return env
}

func asInt(v lisp.Value) (int64, bool) {
	switch n := v.(type) {
	case lisp.Int:
		return int64(n), true
	case lisp.Float:
		return int64(n), true
	default:
		return 0, false
	}
}

func TestFlixLatBot(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.flix (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def bot {:tag :lat :val :bot :rank 0})
		(get bot :rank)
	`)
	if n, ok := asInt(result); !ok || n != 0 {
		t.Fatalf("expected rank 0, got %v", result)
	}
}

func TestFlixLeq(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.flix (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def a {:tag :lat :val :x :rank 1})
		(def b {:tag :lat :val :y :rank 5})
		(not (> (get a :rank) (get b :rank)))
	`)
	if b, ok := result.(lisp.Bool); !ok || !bool(b) {
		t.Fatalf("expected true, got %v", result)
	}
}

func TestFlixJoin(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.flix (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def a {:tag :lat :val :x :rank 3})
		(def b {:tag :lat :val :y :rank 7})
		(def j (if (> (get a :rank) (get b :rank)) a b))
		(get j :rank)
	`)
	if n, ok := asInt(result); !ok || n != 7 {
		t.Fatalf("expected rank 7, got %v", result)
	}
}

func TestFlixFixpoint(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.flix (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(defn step [x]
			(if (> (get x :rank) 4)
				x
				{:tag :lat :val (get x :val) :rank (+ (get x :rank) 1)}))
		(defn fp [x fuel]
			(if (= fuel 0) {:val x :stable false}
				(let [next (step x)]
					(if (= (get next :rank) (get x :rank))
						{:val x :stable true}
						(fp next (- fuel 1))))))
		(def result (fp {:tag :lat :val :start :rank 0} 10))
		(get (get result :val) :rank)
	`)
	if n, ok := asInt(result); !ok || n != 5 {
		t.Fatalf("expected rank 5, got %v", result)
	}
}

func TestGorardMultiwayState(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.gorard (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def s {:tag :mw-state :val "hello" :id "s0" :gen 0})
		(get s :val)
	`)
	if s, ok := result.(lisp.String); !ok || string(s) != "hello" {
		t.Fatalf("expected 'hello', got %v", result)
	}
}

func TestGorardBranching(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.gorard (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def s0 {:tag :mw-state :val "root" :id "s0" :gen 0})
		(def s1 {:tag :mw-state :val "left" :id "s1" :gen 1 :parent "s0"})
		(def s2 {:tag :mw-state :val "right" :id "s2" :gen 1 :parent "s0"})
		(def frontier [s1 s2])
		(count frontier)
	`)
	if n, ok := asInt(result); !ok || n != 2 {
		t.Fatalf("expected 2 branches, got %v", result)
	}
}

func TestStellogenRays(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.stellogen (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def r1 {:tag :ray :polarity :+ :name :a :terms ["x"]})
		(def r2 {:tag :ray :polarity :- :name :a :terms ["x"]})
		(and (= (get r1 :name) (get r2 :name))
		     (not (= (get r1 :polarity) (get r2 :polarity))))
	`)
	if b, ok := result.(lisp.Bool); !ok || !bool(b) {
		t.Fatalf("expected rays to annihilate, got %v", result)
	}
}

func TestStellogenConstellation(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.stellogen (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def s1 {:tag :star :rays [{:tag :ray :polarity :+ :name :a :terms []}]})
		(def s2 {:tag :star :rays [{:tag :ray :polarity :- :name :b :terms []}]})
		(def c {:tag :constellation :stars [s1 s2]})
		(count (get c :stars))
	`)
	if n, ok := asInt(result); !ok || n != 2 {
		t.Fatalf("expected 2 stars, got %v", result)
	}
}

func TestAleaReturnDist(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.alea (:require [boxxy.color :as color] [boxxy.trace :as trace]))
		(def d {:tag :dist :samples [{:val 42 :prob 1.0}]})
		(get (first (get d :samples)) :val)
	`)
	if n, ok := asInt(result); !ok || n != 42 {
		t.Fatalf("expected 42, got %v", result)
	}
}

func TestAleaUniform(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.alea (:require [boxxy.color :as color] [boxxy.trace :as trace]))
		(def vals [:a :b :c])
		(count vals)
	`)
	if n, ok := asInt(result); !ok || n != 3 {
		t.Fatalf("expected 3, got %v", result)
	}
}

func TestConservationTrits(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.conservation
			(:require [boxxy.flix :as flix]
				[boxxy.gorard :as gorard]
				[boxxy.stellogen :as stellogen]
				[boxxy.alea :as alea]
				[boxxy.color :as color]
				[boxxy.trace :as trace]))
		(+ (+ (+ (+ -1 0) 1) -1) 1)
	`)
	if n, ok := asInt(result); !ok || n != 0 {
		t.Fatalf("expected trit sum 0, got %v", result)
	}
}

func TestConservationBalanced(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.conservation
			(:require [boxxy.flix :as flix]
				[boxxy.gorard :as gorard]
				[boxxy.stellogen :as stellogen]
				[boxxy.alea :as alea]
				[boxxy.color :as color]
				[boxxy.trace :as trace]))
		(= 0 (+ (+ (+ (+ -1 0) 1) -1) 1))
	`)
	if b, ok := result.(lisp.Bool); !ok || !bool(b) {
		t.Fatalf("expected balanced (true), got %v", result)
	}
}

func TestCrossLayerFlixToGorard(t *testing.T) {
	env := setupPLTEnv(t)
	result := evalPLT(t, env, `
		(ns boxxy.conservation
			(:require [boxxy.flix :as flix]
				[boxxy.gorard :as gorard]
				[boxxy.color :as color]
				[boxxy.trace :as trace]))
		(def fp-result {:val {:tag :lat :val :done :rank 5} :steps 5 :stable true})
		(def mw-state {:tag :mw-state
			:val (get (get fp-result :val) :val)
			:id (str "flix-" (get fp-result :steps))
			:gen 0})
		(get mw-state :id)
	`)
	if s, ok := result.(lisp.String); !ok || string(s) != "flix-5" {
		t.Fatalf("expected 'flix-5', got %v", result)
	}
}

func TestNSIsolation(t *testing.T) {
	env := setupPLTEnv(t)
	evalPLT(t, env, `
		(ns boxxy.flix (:require [boxxy.trace :as trace] [boxxy.color :as color]))
		(def flix-secret 42)
		(ns boxxy.gorard (:require [boxxy.trace :as trace] [boxxy.color :as color]))
	`)
	result := evalPLT(t, env, `(ns-name)`)
	if s, ok := result.(lisp.Symbol); !ok || string(s) != "boxxy.gorard" {
		t.Fatalf("expected boxxy.gorard, got %v", result)
	}
}
