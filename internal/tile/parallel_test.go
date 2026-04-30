//go:build darwin

package tile

import (
	"testing"

	"github.com/bmorphism/boxxy/internal/gf3"
)

func TestParallelVerifyDeterministic(t *testing.T) {
	seeds := []uint64{1069, 42, 7, 256, 999}
	ops := []MoveOp{OpMint, OpBurn} // balanced lifecycle

	work := DefaultWork(ops)

	// Run twice — SPI guarantees identical results regardless of scheduling
	results1, _ := ParallelVerify(seeds, work)
	results2, _ := ParallelVerify(seeds, work)

	for i := range results1 {
		if results1[i].Color != results2[i].Color {
			t.Fatalf("seed %d: color mismatch between runs: %s vs %s",
				seeds[i], results1[i].Color.Hex(), results2[i].Color.Hex())
		}
		if results1[i].Verified != results2[i].Verified {
			t.Fatalf("seed %d: verification mismatch between runs", seeds[i])
		}
		if results1[i].Report != results2[i].Report {
			t.Fatalf("seed %d: report mismatch between runs", seeds[i])
		}
	}
}

func TestParallelVerifyAllVerified(t *testing.T) {
	seeds := []uint64{1069, 42, 7}
	ops := []MoveOp{OpMoveTo, OpMoveFrom} // balanced

	work := DefaultWork(ops)
	results, _ := ParallelVerify(seeds, work)

	for _, r := range results {
		if !r.Verified {
			t.Fatalf("seed %d should be verified with balanced ops", r.Seed)
		}
	}
}

func TestParallelVerifyUnbalancedDetected(t *testing.T) {
	seeds := []uint64{100, 200, 300}
	ops := []MoveOp{OpMint} // unbalanced — no matching burn

	work := DefaultWork(ops)
	results, _ := ParallelVerify(seeds, work)

	for _, r := range results {
		if r.Verified {
			t.Fatalf("seed %d should NOT be verified with unbalanced ops", r.Seed)
		}
	}
}

func TestParallelMapDeterministic(t *testing.T) {
	seeds := []uint64{1069, 42, 7, 0, 999, 12345}

	colors1 := ParallelMap(seeds)
	colors2 := ParallelMap(seeds)

	for i := range colors1 {
		if colors1[i] != colors2[i] {
			t.Fatalf("seed %d: parallel map not deterministic: %s vs %s",
				seeds[i], colors1[i].Hex(), colors2[i].Hex())
		}
		// Also verify against sequential
		expected := ColorFromSeed(seeds[i])
		if colors1[i] != expected {
			t.Fatalf("seed %d: parallel result %s differs from sequential %s",
				seeds[i], colors1[i].Hex(), expected.Hex())
		}
	}
}

func TestFindBalancedTriad(t *testing.T) {
	seeds, found := FindBalancedTriad(0, 100)
	if !found {
		t.Fatal("could not find balanced triad in first 100 seeds")
	}

	// Verify balance
	trits := make([]gf3.Elem, 3)
	for i, s := range seeds {
		trits[i] = ColorFromSeed(s).Trit()
	}
	if !gf3.IsBalanced(trits) {
		t.Fatalf("triad not balanced: trits=%v seeds=%v", trits, seeds)
	}
	t.Logf("balanced triad: seeds=%v trits=%v", seeds, trits)
}

func TestParallelVerifyLargeScale(t *testing.T) {
	// 1000 independent tiles — embarrassingly parallel
	seeds := make([]uint64, 1000)
	for i := range seeds {
		seeds[i] = uint64(i)
	}

	ops := []MoveOp{OpMint, OpTransfer, OpBurn} // balanced lifecycle
	work := DefaultWork(ops)

	results, _ := ParallelVerify(seeds, work)

	verified := 0
	for _, r := range results {
		if r.Verified {
			verified++
		}
	}
	t.Logf("%d/%d tiles verified at scale", verified, len(seeds))
	if verified != len(seeds) {
		t.Fatalf("expected all %d tiles verified, got %d", len(seeds), verified)
	}
}

func BenchmarkParallelVerify100(b *testing.B) {
	seeds := make([]uint64, 100)
	for i := range seeds {
		seeds[i] = uint64(i)
	}
	ops := []MoveOp{OpMint, OpBurn}
	work := DefaultWork(ops)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParallelVerify(seeds, work)
	}
}

func BenchmarkParallelMap1000(b *testing.B) {
	seeds := make([]uint64, 1000)
	for i := range seeds {
		seeds[i] = uint64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParallelMap(seeds)
	}
}
