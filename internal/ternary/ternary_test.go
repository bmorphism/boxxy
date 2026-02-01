package ternary_test

import (
	"testing"

	cristal "github.com/cristalhq/ternary"
	"github.com/goloop/trit"
)

// GF(3) field: {-1, 0, +1} with addition mod 3
// This is the foundation of balanced ternary computation
// and maps directly to boxxy's provider trit assignments.

func TestTritGF3Values(t *testing.T) {
	// goloop/trit uses int8: False=-1, Unknown=0, True=1
	if trit.False.Int() != -1 {
		t.Errorf("False should be -1, got %d", trit.False.Int())
	}
	if trit.Unknown.Int() != 0 {
		t.Errorf("Unknown should be 0, got %d", trit.Unknown.Int())
	}
	if trit.True.Int() != 1 {
		t.Errorf("True should be 1, got %d", trit.True.Int())
	}
}

func TestTritLogicGates(t *testing.T) {
	// NOT is negation in GF(3): -(-1)=1, -(0)=0, -(1)=-1
	if trit.False.Not() != trit.True {
		t.Error("NOT False should be True")
	}
	if trit.Unknown.Not() != trit.Unknown {
		t.Error("NOT Unknown should be Unknown")
	}
	if trit.True.Not() != trit.False {
		t.Error("NOT True should be False")
	}

	// AND = min, OR = max in Kleene logic
	if trit.True.And(trit.False) != trit.False {
		t.Error("True AND False should be False")
	}
	if trit.True.Or(trit.False) != trit.True {
		t.Error("True OR False should be True")
	}

	// XOR with Unknown propagates uncertainty
	if trit.True.Xor(trit.Unknown) != trit.Unknown {
		t.Error("True XOR Unknown should be Unknown")
	}
}

func TestTritBalancedTernaryQuad(t *testing.T) {
	// A GF(3)-balanced quad sums to 0 mod 3.
	// This is how skill triads + balancer work in Gay MCP.
	//
	// Example quad: (+1, +1, +1, 0) -> sum = 3 = 0 (mod 3)
	// Example quad: (+1, -1, 0, 0) -> sum = 0 = 0 (mod 3)
	// Example quad: (+1, +1, -1, -1) -> sum = 0

	quads := [][4]trit.Trit{
		{trit.True, trit.True, trit.True, trit.Unknown},
		{trit.True, trit.False, trit.Unknown, trit.Unknown},
		{trit.True, trit.True, trit.False, trit.False},
	}

	for i, q := range quads {
		sum := int(q[0].Int()) + int(q[1].Int()) + int(q[2].Int()) + int(q[3].Int())
		// In GF(3), balanced means sum = 0 (mod 3)
		mod := ((sum % 3) + 3) % 3
		if mod != 0 {
			t.Errorf("quad %d: sum=%d, mod3=%d, expected 0", i, sum, mod)
		}
	}
}

func TestCristalKleeneLogic(t *testing.T) {
	// cristalhq/ternary provides Kleene (K3) logic
	a := cristal.True
	b := cristal.False
	u := cristal.Unknown

	// NOT
	if cristal.Not(a) != cristal.False {
		t.Error("K3: NOT True should be False")
	}
	if cristal.Not(u) != cristal.Unknown {
		t.Error("K3: NOT Unknown should be Unknown")
	}

	// Implication: a -> b = not(a) or b
	imp := cristal.Imp(a, b)
	if imp != cristal.False {
		t.Errorf("K3: True -> False should be False, got %v", imp)
	}

	// Modus Ponens Absorption
	ma := cristal.MA(u)
	if ma != cristal.True {
		t.Errorf("K3: MA(Unknown) should be True, got %v", ma)
	}
}

func TestCristalBochvarLogic(t *testing.T) {
	// Bochvar (B3) logic: Unknown is "meaningless" and absorbs everything
	// This is stricter than Kleene -- any Unknown input -> Unknown output
	a := cristal.True
	u := cristal.Unknown

	// In Bochvar, True AND Unknown = Unknown (absorbed)
	result := cristal.AndB(a, u)
	if result != cristal.Unknown {
		t.Errorf("B3: True AND Unknown should be Unknown (absorbed), got %v", result)
	}

	// Bochvar OR: Unknown absorbs
	result = cristal.OrB(a, u)
	if result != cristal.Unknown {
		t.Errorf("B3: True OR Unknown should be Unknown (absorbed), got %v", result)
	}
}

func TestTritJSONMarshal(t *testing.T) {
	// goloop/trit marshals Unknown as JSON null -- useful for provider EDN/JSON interop
	data, err := trit.Unknown.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("Unknown should marshal to null, got %s", string(data))
	}

	data, err = trit.True.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if string(data) != "true" {
		t.Errorf("True should marshal to true, got %s", string(data))
	}

	data, err = trit.False.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if string(data) != "false" {
		t.Errorf("False should marshal to false, got %s", string(data))
	}
}
