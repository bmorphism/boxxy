//go:build darwin

package tile

import (
	"testing"
)

func TestTileVerifierInitFromSeed(t *testing.T) {
	v := NewTileVerifier(1069)
	c := ColorFromSeed(1069)
	if v.IdentityColor != c {
		t.Fatalf("identity color mismatch: got %s want %s", v.IdentityColor.Hex(), c.Hex())
	}
	if !v.IsLifecycleBalanced() {
		t.Fatal("fresh verifier should be balanced")
	}
	if !v.IsVerified() {
		t.Fatal("fresh verifier should pass verification")
	}
}

func TestTileVerifierBalancedLifecycle(t *testing.T) {
	v := NewTileVerifier(42)
	c1 := ColorFromSeed(100)
	c2 := ColorFromSeed(200)
	v.RecordTransition(OpMoveTo, c1)
	v.RecordTransition(OpBorrowGlobal, c2)
	v.RecordTransition(OpMoveFrom, c1)
	if !v.IsLifecycleBalanced() {
		t.Fatal("balanced lifecycle should pass")
	}
	if v.Transitions != 3 {
		t.Fatalf("expected 3 transitions, got %d", v.Transitions)
	}
}

func TestTileVerifierUnbalanced(t *testing.T) {
	v := NewTileVerifier(7)
	v.RecordOp(OpMint)
	v.RecordOp(OpTransfer)
	// missing burn
	if v.IsLifecycleBalanced() {
		t.Fatal("unbalanced lifecycle should fail")
	}
	if v.IsVerified() {
		t.Fatal("unbalanced verifier should not pass")
	}
}

func TestTileVerifierDeltaE(t *testing.T) {
	v := NewTileVerifier(1069)
	v.RecordTransition(OpMoveTo, ColorFromSeed(999))
	de := v.MeanDeltaE()
	if de <= 0 {
		t.Fatal("delta-E between different seeds should be > 0")
	}
	t.Logf("mean delta-E: %.2f", de)
}

func TestTileVerifierReport(t *testing.T) {
	v := NewTileVerifier(1069)
	v.RecordOp(OpMoveTo)
	v.RecordOp(OpMoveFrom)
	r := v.Report()
	if r == "" {
		t.Fatal("report should not be empty")
	}
	t.Logf("report: %s", r)
}

func TestTileVerifierWinding(t *testing.T) {
	v := NewTileVerifier(42)
	for i := 0; i < 3; i++ {
		v.RecordOp(OpMint)
	}
	for i := 0; i < 3; i++ {
		v.RecordOp(OpBurn)
	}
	if v.Winding() != 0 {
		t.Fatalf("expected winding 0, got %d", v.Winding())
	}
}

func TestMultiTileVerifierConservation(t *testing.T) {
	mtv := NewMultiTileVerifier()
	t1 := mtv.AddTile(1069)
	t1.RecordOp(OpMoveTo)
	t1.RecordOp(OpMoveFrom)

	t2 := mtv.AddTile(42)
	t2.RecordOp(OpMint)
	t2.RecordOp(OpBurn)

	if !mtv.IsGloballyConserved() {
		t.Fatal("balanced multi-tile should be globally conserved")
	}
}

func TestMultiTileVerifierViolation(t *testing.T) {
	mtv := NewMultiTileVerifier()
	t1 := mtv.AddTile(1069)
	t1.RecordOp(OpMoveTo) // unmatched plus

	if mtv.IsGloballyConserved() {
		t.Fatal("unbalanced multi-tile should not be conserved")
	}
	if mtv.AllVerified() {
		t.Fatal("unbalanced multi-tile should not all-verify")
	}
}

func TestMultiTileVerifierAddExisting(t *testing.T) {
	mtv := NewMultiTileVerifier()
	t1 := mtv.AddTile(1069)
	t1.RecordOp(OpMint)
	t2 := mtv.AddTile(1069) // same seed, should return same pointer
	if t1 != t2 {
		t.Fatal("AddTile with same seed should return existing verifier")
	}
	if t2.Losses.PlusCount != 1 {
		t.Fatal("existing verifier should retain state")
	}
}

func TestMoveOpTrits(t *testing.T) {
	plusOps := []MoveOp{OpMoveTo, OpMint, OpSetFlag}
	minusOps := []MoveOp{OpMoveFrom, OpBurn, OpClearFlag}
	ergoOps := []MoveOp{OpBorrowGlobal, OpBorrowGlobalMut, OpTransfer}

	for _, op := range plusOps {
		if op.Trit() != 1 {
			t.Fatalf("op %d should be plus(1), got %d", op, op.Trit())
		}
	}
	for _, op := range minusOps {
		if op.Trit() != 2 {
			t.Fatalf("op %d should be minus(2), got %d", op, op.Trit())
		}
	}
	for _, op := range ergoOps {
		if op.Trit() != 0 {
			t.Fatalf("op %d should be ergodic(0), got %d", op, op.Trit())
		}
	}
}

func TestLabConversion(t *testing.T) {
	white := Color{255, 255, 255}
	lab := toLab(white)
	if lab[0] < 99 || lab[0] > 101 {
		t.Fatalf("white L* should be ~100, got %.2f", lab[0])
	}
	black := Color{0, 0, 0}
	labB := toLab(black)
	if labB[0] > 1 {
		t.Fatalf("black L* should be ~0, got %.2f", labB[0])
	}
}
