//go:build darwin

package gf3

import "testing"

// Every test below corresponds to a lemma in verified/GF3.dfy.
// Since GF(3) has only 3 elements, we exhaustively check all cases
// (3^1 = 3 for unary, 3^2 = 9 for binary, 3^3 = 27 for ternary).
// This is equivalent to Dafny's verification for finite domains.

var elems = [3]Elem{Zero, One, Two}

// --- Field Axiom Tests (exhaustive) ---

func TestAdditiveIdentity(t *testing.T) {
	// Dafny lemma: AdditiveIdentity — ∀ a: Add(a, 0) == a
	for _, a := range elems {
		if Add(a, Zero) != a {
			t.Errorf("Add(%d, 0) = %d, want %d", a, Add(a, Zero), a)
		}
	}
}

func TestAdditiveCommutativity(t *testing.T) {
	// Dafny lemma: AdditiveCommutativity — ∀ a,b: Add(a,b) == Add(b,a)
	for _, a := range elems {
		for _, b := range elems {
			if Add(a, b) != Add(b, a) {
				t.Errorf("Add(%d,%d)=%d != Add(%d,%d)=%d", a, b, Add(a, b), b, a, Add(b, a))
			}
		}
	}
}

func TestAdditiveAssociativity(t *testing.T) {
	// Dafny lemma: AdditiveAssociativity — ∀ a,b,c: Add(Add(a,b),c) == Add(a,Add(b,c))
	for _, a := range elems {
		for _, b := range elems {
			for _, c := range elems {
				lhs := Add(Add(a, b), c)
				rhs := Add(a, Add(b, c))
				if lhs != rhs {
					t.Errorf("(%d+%d)+%d=%d != %d+(%d+%d)=%d", a, b, c, lhs, a, b, c, rhs)
				}
			}
		}
	}
}

func TestAdditiveInverse(t *testing.T) {
	// Dafny lemma: AdditiveInverse — ∀ a: Add(a, Neg(a)) == 0
	for _, a := range elems {
		if Add(a, Neg(a)) != Zero {
			t.Errorf("Add(%d, Neg(%d)) = %d, want 0", a, a, Add(a, Neg(a)))
		}
	}
}

func TestMultiplicativeIdentity(t *testing.T) {
	// Dafny lemma: MultiplicativeIdentity — ∀ a: Mul(a, 1) == a
	for _, a := range elems {
		if Mul(a, One) != a {
			t.Errorf("Mul(%d, 1) = %d, want %d", a, Mul(a, One), a)
		}
	}
}

func TestMultiplicativeCommutativity(t *testing.T) {
	// Dafny lemma: MultiplicativeCommutativity — ∀ a,b: Mul(a,b) == Mul(b,a)
	for _, a := range elems {
		for _, b := range elems {
			if Mul(a, b) != Mul(b, a) {
				t.Errorf("Mul(%d,%d)=%d != Mul(%d,%d)=%d", a, b, Mul(a, b), b, a, Mul(b, a))
			}
		}
	}
}

func TestMultiplicativeAssociativity(t *testing.T) {
	// Dafny lemma: MultiplicativeAssociativity — ∀ a,b,c: Mul(Mul(a,b),c) == Mul(a,Mul(b,c))
	for _, a := range elems {
		for _, b := range elems {
			for _, c := range elems {
				lhs := Mul(Mul(a, b), c)
				rhs := Mul(a, Mul(b, c))
				if lhs != rhs {
					t.Errorf("(%d*%d)*%d=%d != %d*(%d*%d)=%d", a, b, c, lhs, a, b, c, rhs)
				}
			}
		}
	}
}

func TestMultiplicativeInverse(t *testing.T) {
	// Dafny lemma: MultiplicativeInverse — ∀ a≠0: Mul(a, Inv(a)) == 1
	for _, a := range elems {
		if a == Zero {
			continue
		}
		if Mul(a, Inv(a)) != One {
			t.Errorf("Mul(%d, Inv(%d)) = %d, want 1", a, a, Mul(a, Inv(a)))
		}
	}
}

func TestDistributivity(t *testing.T) {
	// Dafny lemma: Distributivity — ∀ a,b,c: Mul(a, Add(b,c)) == Add(Mul(a,b), Mul(a,c))
	for _, a := range elems {
		for _, b := range elems {
			for _, c := range elems {
				lhs := Mul(a, Add(b, c))
				rhs := Add(Mul(a, b), Mul(a, c))
				if lhs != rhs {
					t.Errorf("%d*(%d+%d)=%d != %d*%d+%d*%d=%d", a, b, c, lhs, a, b, a, c, rhs)
				}
			}
		}
	}
}

func TestZeroAnnihilation(t *testing.T) {
	// Dafny lemma: ZeroAnnihilation — ∀ a: Mul(a, 0) == 0
	for _, a := range elems {
		if Mul(a, Zero) != Zero {
			t.Errorf("Mul(%d, 0) = %d, want 0", a, Mul(a, Zero))
		}
	}
}

// --- Balanced Ternary Roundtrip Tests ---

func TestBalancedRoundtrip(t *testing.T) {
	// Dafny lemma: BalancedRoundtrip — ∀ a: FromBalanced(ToBalanced(a)) == a
	for _, a := range elems {
		if FromBalanced(ToBalanced(a)) != a {
			t.Errorf("FromBalanced(ToBalanced(%d)) = %d", a, FromBalanced(ToBalanced(a)))
		}
	}
}

func TestBalancedRoundtripInverse(t *testing.T) {
	// Dafny lemma: BalancedRoundtripInverse — ∀ b: ToBalanced(FromBalanced(b)) == b
	for _, b := range [3]BalancedTrit{Minus, Ergodic, Plus} {
		if ToBalanced(FromBalanced(b)) != b {
			t.Errorf("ToBalanced(FromBalanced(%d)) = %d", b, ToBalanced(FromBalanced(b)))
		}
	}
}

func TestBalancedMapping(t *testing.T) {
	// Verify the exact mapping: 0→0, 1→+1, 2→-1
	if ToBalanced(Zero) != Ergodic {
		t.Error("ToBalanced(0) should be 0")
	}
	if ToBalanced(One) != Plus {
		t.Error("ToBalanced(1) should be +1")
	}
	if ToBalanced(Two) != Minus {
		t.Error("ToBalanced(2) should be -1")
	}
}

// --- Skill Role Tests ---

func TestRoleRoundtrip(t *testing.T) {
	// Dafny lemma: RoleRoundtrip — ∀ r: ElemToRole(RoleToElem(r)) == r
	for _, r := range [3]SkillRole{Coordinator, Generator, Verifier} {
		if ElemToRole(RoleToElem(r)) != r {
			t.Errorf("ElemToRole(RoleToElem(%v)) != %v", r, r)
		}
	}
}

// --- Conservation Law Tests ---

func TestExampleQuads(t *testing.T) {
	// Dafny lemma: ExampleQuads — three known balanced quads
	if !IsBalancedQuad(Generator, Generator, Generator, Coordinator) {
		t.Error("[Gen,Gen,Gen,Coord] should be balanced (1+1+1+0=3≡0)")
	}
	if !IsBalancedQuad(Generator, Verifier, Coordinator, Coordinator) {
		t.Error("[Gen,Ver,Coord,Coord] should be balanced (1+2+0+0=3≡0)")
	}
	if !IsBalancedQuad(Generator, Generator, Verifier, Verifier) {
		t.Error("[Gen,Gen,Ver,Ver] should be balanced (1+1+2+2=6≡0)")
	}
}

func TestFindBalancer(t *testing.T) {
	// Exhaustively verify FindBalancer for all 27 triads
	for _, a := range elems {
		for _, b := range elems {
			for _, c := range elems {
				d := FindBalancer(a, b, c)
				if !IsBalanced([]Elem{a, b, c, d}) {
					t.Errorf("FindBalancer(%d,%d,%d)=%d but quad not balanced", a, b, c, d)
				}
			}
		}
	}
}

func TestConservationLaw(t *testing.T) {
	// Exhaustively verify: all 81 quads (3^4), check which are balanced
	balanced := 0
	for _, a := range elems {
		for _, b := range elems {
			for _, c := range elems {
				for _, d := range elems {
					if IsBalanced([]Elem{a, b, c, d}) {
						balanced++
					}
				}
			}
		}
	}
	// Exactly 1/3 of all quads should be balanced (27 out of 81)
	if balanced != 27 {
		t.Errorf("expected 27 balanced quads out of 81, got %d", balanced)
	}
}

// --- Cayley Table Verification ---
// The definitive test: verify the complete addition and multiplication tables.

func TestCayleyAddition(t *testing.T) {
	// Complete GF(3) addition table
	expected := [3][3]Elem{
		//       +0  +1  +2
		/* 0 */ {0, 1, 2},
		/* 1 */ {1, 2, 0},
		/* 2 */ {2, 0, 1},
	}
	for i, a := range elems {
		for j, b := range elems {
			got := Add(a, b)
			if got != expected[i][j] {
				t.Errorf("Add(%d,%d) = %d, want %d", a, b, got, expected[i][j])
			}
		}
	}
}

func TestCayleyMultiplication(t *testing.T) {
	// Complete GF(3) multiplication table
	expected := [3][3]Elem{
		//       *0  *1  *2
		/* 0 */ {0, 0, 0},
		/* 1 */ {0, 1, 2},
		/* 2 */ {0, 2, 1},
	}
	for i, a := range elems {
		for j, b := range elems {
			got := Mul(a, b)
			if got != expected[i][j] {
				t.Errorf("Mul(%d,%d) = %d, want %d", a, b, got, expected[i][j])
			}
		}
	}
}

func TestNegationTable(t *testing.T) {
	// Complete negation table
	expected := [3]Elem{0, 2, 1} // neg(0)=0, neg(1)=2, neg(2)=1
	for i, a := range elems {
		got := Neg(a)
		if got != expected[i] {
			t.Errorf("Neg(%d) = %d, want %d", a, got, expected[i])
		}
	}
}

// --- Inv panic test ---

func TestInvZeroPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Inv(0) should panic")
		}
	}()
	Inv(Zero)
}
