//go:build darwin

// Package gf3 implements the Galois Field GF(3) with verified operations.
//
// This is a hand-translation of verified/GF3.dfy — every function here
// corresponds 1:1 to a Dafny function, and every Dafny lemma is covered
// by an exhaustive Go test (all 3^n cases for n-ary operations).
//
// GF(3) elements: {0, 1, 2} under arithmetic mod 3.
// Balanced ternary: {-1, 0, +1} maps to {2, 0, 1}.
//
// Conservation law: ∑ trits ≡ 0 (mod 3) across skill quads.
package gf3

// Elem is a GF(3) element: 0, 1, or 2.
type Elem uint8

const (
	Zero Elem = 0
	One  Elem = 1
	Two  Elem = 2
)

// Add returns (a + b) mod 3.
func Add(a, b Elem) Elem {
	return (a + b) % 3
}

// Mul returns (a * b) mod 3.
func Mul(a, b Elem) Elem {
	return (a * b) % 3
}

// Neg returns (3 - a) mod 3 — the additive inverse.
func Neg(a Elem) Elem {
	return (3 - a) % 3
}

// Sub returns a - b in GF(3), i.e. Add(a, Neg(b)).
func Sub(a, b Elem) Elem {
	return Add(a, Neg(b))
}

// Inv returns the multiplicative inverse. Panics on zero.
// In GF(3): inv(1) = 1, inv(2) = 2 (since 2*2 = 4 ≡ 1 mod 3).
func Inv(a Elem) Elem {
	if a == Zero {
		panic("gf3: multiplicative inverse of zero")
	}
	if a == One {
		return One
	}
	return Two
}

// --- Balanced Ternary ---

// BalancedTrit is a balanced ternary value: -1, 0, or +1.
type BalancedTrit int8

const (
	Minus   BalancedTrit = -1
	Ergodic BalancedTrit = 0
	Plus    BalancedTrit = 1
)

// ToBalanced converts a GF(3) element to balanced ternary.
// 0→0, 1→+1, 2→-1
func ToBalanced(a Elem) BalancedTrit {
	switch a {
	case Zero:
		return Ergodic
	case One:
		return Plus
	default:
		return Minus
	}
}

// FromBalanced converts a balanced trit to a GF(3) element.
// -1→2, 0→0, +1→1
func FromBalanced(b BalancedTrit) Elem {
	switch b {
	case Ergodic:
		return Zero
	case Plus:
		return One
	default:
		return Two
	}
}

// --- Skill Roles ---

// SkillRole encodes the GF(3) trit semantics for skill dispersal.
type SkillRole uint8

const (
	Coordinator SkillRole = iota // 0: balance, infrastructure
	Generator                    // 1: creation, synthesis
	Verifier                     // 2: validation, analysis
)

// RoleToElem maps a skill role to its GF(3) element.
func RoleToElem(r SkillRole) Elem {
	switch r {
	case Generator:
		return One
	case Coordinator:
		return Zero
	default:
		return Two
	}
}

// ElemToRole maps a GF(3) element to its skill role.
func ElemToRole(e Elem) SkillRole {
	switch e {
	case One:
		return Generator
	case Zero:
		return Coordinator
	default:
		return Verifier
	}
}

// --- Conservation Law ---

// SeqSum returns the integer sum of a sequence of GF(3) elements.
func SeqSum(s []Elem) int {
	sum := 0
	for _, e := range s {
		sum += int(e)
	}
	return sum
}

// IsBalanced checks the GF(3) conservation law: ∑ elements ≡ 0 (mod 3).
func IsBalanced(s []Elem) bool {
	return ((SeqSum(s) % 3) + 3) % 3 == 0
}

// IsBalancedQuad checks if four skill roles form a balanced quad.
func IsBalancedQuad(a, b, c, d SkillRole) bool {
	return IsBalanced([]Elem{
		RoleToElem(a), RoleToElem(b), RoleToElem(c), RoleToElem(d),
	})
}

// FindBalancer returns the GF(3) element needed to balance a triad.
// Given elements a, b, c, returns d such that a+b+c+d ≡ 0 (mod 3).
func FindBalancer(a, b, c Elem) Elem {
	partial := (int(a) + int(b) + int(c)) % 3
	return Elem((3 - partial) % 3)
}

// String returns the balanced ternary representation.
func (e Elem) String() string {
	switch e {
	case Zero:
		return "0"
	case One:
		return "+1"
	default:
		return "-1"
	}
}

func (r SkillRole) String() string {
	switch r {
	case Generator:
		return "Generator(+1)"
	case Coordinator:
		return "Coordinator(0)"
	default:
		return "Verifier(-1)"
	}
}
