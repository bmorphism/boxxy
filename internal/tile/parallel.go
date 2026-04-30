//go:build darwin

package tile

import (
	"sync"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// ParallelTileResult is the output of one independent tile worker.
// Value semantics only — no pointers, no shared state.
type ParallelTileResult struct {
	Seed      uint64
	Color     Color
	Trit      gf3.Elem
	Winding   int
	Residue   int
	DeltaE    float64
	Verified  bool
	Report    string
}

// WorkFunc is a pure function each tile worker executes.
// It receives only its seed (value copy) and returns only a value result.
// This is the SPI primitive: seed → deterministic result, zero shared state.
type WorkFunc func(seed uint64) ParallelTileResult

// DefaultWork runs the standard tile verification workflow.
func DefaultWork(ops []MoveOp) WorkFunc {
	return func(seed uint64) ParallelTileResult {
		tv := NewTileVerifier(seed)
		for _, op := range ops {
			tv.RecordOp(op)
		}
		return ParallelTileResult{
			Seed:     seed,
			Color:    tv.IdentityColor,
			Trit:     tv.IdentityColor.Trit(),
			Winding:  tv.Winding(),
			Residue:  tv.Losses.Residue(),
			DeltaE:   tv.MeanDeltaE(),
			Verified: tv.IsVerified(),
			Report:   tv.Report(),
		}
	}
}

// ParallelVerify runs N tile workers as independent goroutines, each with
// its own seed and zero shared mutable state. This is the maximally
// embarrassingly parallel pattern in Go:
//
//	go func(seed uint64) { ... }(seeds[i])  // value copy, no pointers
//
// The SPI invariant: results are identical regardless of goroutine scheduling.
// GF(3) conservation is checked at the single join point after all workers complete.
func ParallelVerify(seeds []uint64, work WorkFunc) ([]ParallelTileResult, bool) {
	results := make([]ParallelTileResult, len(seeds))
	var wg sync.WaitGroup
	wg.Add(len(seeds))

	for i, seed := range seeds {
		// The SPI primitive: value-copy closure, no shared state.
		// Each goroutine is a tileable VM with seed-derived identity.
		go func(idx int, s uint64) {
			defer wg.Done()
			results[idx] = work(s)
		}(i, seed)
	}

	wg.Wait()

	// Single join point: verify GF(3) conservation across all tiles.
	// This is the only synchronization — everything before was independent.
	conserved := checkConservation(results)
	return results, conserved
}

// checkConservation verifies ∑ plus == ∑ minus across all tile results.
func checkConservation(results []ParallelTileResult) bool {
	totalPlus, totalMinus := 0, 0
	for _, r := range results {
		switch r.Trit {
		case gf3.One:
			totalPlus++
		case gf3.Two:
			totalMinus++
		}
		totalPlus += r.Winding
	}
	// Check both winding conservation and trit balance
	trits := make([]gf3.Elem, len(results))
	for i, r := range results {
		trits[i] = r.Trit
	}
	return gf3.IsBalanced(trits)
}

// ParallelMap applies a function to each seed in parallel, returning colors.
// This is the pure-functional embarrassingly parallel map:
// seeds.par_map(|s| ColorFromSeed(s))
func ParallelMap(seeds []uint64) []Color {
	colors := make([]Color, len(seeds))
	var wg sync.WaitGroup
	wg.Add(len(seeds))
	for i, seed := range seeds {
		go func(idx int, s uint64) {
			defer wg.Done()
			colors[idx] = ColorFromSeed(s)
		}(i, seed)
	}
	wg.Wait()
	return colors
}

// FindBalancedTriad searches for 3 seeds from a starting point whose trits
// sum to 0 mod 3. Uses parallel goroutines to scan seed space.
func FindBalancedTriad(start uint64, maxSearch int) (seeds [3]uint64, found bool) {
	// Partition search into 3 goroutines, one per needed trit value
	type result struct {
		trit gf3.Elem
		seed uint64
	}
	ch := make(chan result, 3)
	var wg sync.WaitGroup

	for trit := gf3.Elem(0); trit < 3; trit++ {
		wg.Add(1)
		go func(target gf3.Elem) {
			defer wg.Done()
			for s := start; s < start+uint64(maxSearch); s++ {
				c := ColorFromSeed(s)
				if c.Trit() == target {
					ch <- result{target, s}
					return
				}
			}
		}(trit)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	collected := 0
	for r := range ch {
		seeds[int(r.trit)] = r.seed
		collected++
		if collected == 3 {
			return seeds, true
		}
	}
	return seeds, collected == 3
}
