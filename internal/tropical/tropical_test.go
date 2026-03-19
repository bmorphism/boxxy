//go:build darwin

package tropical

import (
	"math"
	"testing"
)

func TestAdd(t *testing.T) {
	a := TropVal{Val: 3}
	b := TropVal{Val: 5}
	r := Add(a, b)
	if r.Val != 5 {
		t.Errorf("Add(3,5) = %v, want 5", r.Val)
	}

	inf := TropVal{IsInf: true}
	r2 := Add(inf, b)
	if r2.Val != 5 {
		t.Errorf("Add(-∞,5) = %v, want 5", r2.Val)
	}

	r3 := Add(a, inf)
	if r3.Val != 3 {
		t.Errorf("Add(3,-∞) = %v, want 3", r3.Val)
	}
}

func TestMul(t *testing.T) {
	a := TropVal{Val: 3}
	b := TropVal{Val: 5}
	r := Mul(a, b)
	if r.Val != 8 {
		t.Errorf("Mul(3,5) = %v, want 8", r.Val)
	}

	inf := TropVal{IsInf: true}
	r2 := Mul(inf, b)
	if !r2.IsInf {
		t.Error("Mul(-∞,5) should be -∞")
	}
}

func TestBellman(t *testing.T) {
	a := DPVal{Addr: "a", Val: TropVal{Val: 3}}
	b := DPVal{Addr: "b", Val: TropVal{Val: 5}}
	r := Bellman(a, b, 1)
	// max(3+1, 5+1) = max(4, 6) = 6
	if r.Val.Val != 6 {
		t.Errorf("Bellman(3,5,cost=1) = %v, want 6", r.Val.Val)
	}
	if r.Addr != "(a+b)" {
		t.Errorf("Bellman addr = %v, want (a+b)", r.Addr)
	}
}

func TestGF3Add(t *testing.T) {
	cases := [][3]int{
		{1, -1, 0}, {1, 0, 1}, {1, 1, -1},
		{0, -1, -1}, {0, 0, 0}, {0, 1, 1},
		{-1, -1, 1}, {-1, 0, -1}, {-1, 1, 0},
	}
	for _, c := range cases {
		r := GF3Add(c[0], c[1])
		if r != c[2] {
			t.Errorf("GF3Add(%d,%d) = %d, want %d", c[0], c[1], r, c[2])
		}
	}
}

func TestDerange(t *testing.T) {
	skills := []string{"alpha", "beta", "gamma"}
	validators := []string{"alpha", "beta", "gamma"}
	r := Derange(skills, validators)
	if r == nil {
		t.Fatal("Derange returned nil")
	}
	for i, pair := range r {
		if pair[0] == pair[1] {
			t.Errorf("Derange: skill[%d]=%s maps to itself", i, pair[0])
		}
	}
}

func TestAddIdentity(t *testing.T) {
	inf := TropVal{IsInf: true}
	r := Add(inf, inf)
	if !r.IsInf {
		t.Error("Add(-∞,-∞) should be -∞")
	}
}

func TestMulIdentity(t *testing.T) {
	a := TropVal{Val: 7}
	zero := TropVal{Val: 0} // multiplicative identity
	r := Mul(a, zero)
	if math.Abs(r.Val-7) > 1e-10 {
		t.Errorf("Mul(7,0) = %v, want 7", r.Val)
	}
}
