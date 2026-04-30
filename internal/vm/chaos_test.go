//go:build darwin

package vm

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

)

// ==================== Adversarial Chaos Tests ====================
// These tests try to break things: boundary values, concurrent chaos,
// pathological inputs, and brute-force invariant checks.

// --- SplitMix64 adversarial ---

func TestSplitMix64BoundaryValues(t *testing.T) {
	// Extremal uint64 values must not panic and must produce unique outputs.
	inputs := []uint64{0, 1, 2, ^uint64(0), ^uint64(0) - 1, 1 << 63, (1 << 63) - 1, 0xdeadbeefcafebabe}
	seen := make(map[uint64]bool)
	for _, x := range inputs {
		out := splitmix64(x)
		if seen[out] {
			t.Errorf("splitmix64 collision: two different inputs mapped to %d", out)
		}
		seen[out] = true
	}
}

func TestSplitMix64InjectivityBrute(t *testing.T) {
	// Brute-force check: 100K consecutive inputs must all produce unique outputs.
	// This verifies bijection over a large range.
	const n = 100_000
	seen := make(map[uint64]bool, n)
	for i := uint64(0); i < n; i++ {
		out := splitmix64(i)
		if seen[out] {
			t.Fatalf("splitmix64 collision at input %d", i)
		}
		seen[out] = true
	}
}

func TestSplitMix64AvoidsTrivialFixedPoints(t *testing.T) {
	// A good hash should have no trivial fixed points (f(x) == x).
	for i := uint64(0); i < 100_000; i++ {
		if splitmix64(i) == i {
			t.Errorf("trivial fixed point at %d", i)
		}
	}
}

func TestSplitMix64AvalancheProperty(t *testing.T) {
	// Flipping one input bit should flip ~50% of output bits (avalanche).
	// We test over 10K inputs and average the bit-flip ratio.
	const trials = 10_000
	var totalFlips int
	for i := uint64(0); i < trials; i++ {
		base := splitmix64(i)
		for bit := 0; bit < 64; bit++ {
			perturbed := splitmix64(i ^ (1 << bit))
			diff := base ^ perturbed
			totalFlips += popcount64(diff)
		}
	}
	avgFlips := float64(totalFlips) / float64(trials*64)
	// Perfect avalanche = 32.0 flips per bit-flip.
	// Accept anything in [28, 36] (generous tolerance).
	if avgFlips < 28 || avgFlips > 36 {
		t.Errorf("poor avalanche: avg bit flips = %.2f (want ~32)", avgFlips)
	}
}

func popcount64(x uint64) int {
	// Kernighan's bit count.
	n := 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}

// --- colorAt adversarial ---

func TestColorAtMaxUint64(t *testing.T) {
	// Must not panic or produce NaN/Inf.
	h, s, l := colorAt(^uint64(0), ^uint64(0))
	if math.IsNaN(h) || math.IsInf(h, 0) {
		t.Error("hue is NaN/Inf at max uint64")
	}
	if math.IsNaN(s) || math.IsInf(s, 0) {
		t.Error("sat is NaN/Inf at max uint64")
	}
	if math.IsNaN(l) || math.IsInf(l, 0) {
		t.Error("lit is NaN/Inf at max uint64")
	}
	if h < 0 || h >= 360 || s < 0.5 || s >= 1.0 || l < 0.4 || l >= 0.6 {
		t.Errorf("colorAt(max,max) out of range: h=%.6f s=%.6f l=%.6f", h, s, l)
	}
}

func TestColorAtAllZeros(t *testing.T) {
	h, s, l := colorAt(0, 0)
	if h < 0 || h >= 360 || s < 0.5 || s >= 1.0 || l < 0.4 || l >= 0.6 {
		t.Errorf("colorAt(0,0) out of range: h=%.4f s=%.4f l=%.4f", h, s, l)
	}
}

func TestColorAtBruteRange(t *testing.T) {
	// 50K random (seed, index) pairs — every result must be in valid HSL range.
	rng := rand.New(rand.NewSource(12345))
	for i := 0; i < 50_000; i++ {
		seed := rng.Uint64()
		idx := rng.Uint64()
		h, s, l := colorAt(seed, idx)
		if h < 0 || h >= 360 {
			t.Fatalf("hue out of [0,360) at seed=%d idx=%d: %f", seed, idx, h)
		}
		if s < 0.5 || s >= 1.0 {
			t.Fatalf("sat out of [0.5,1.0) at seed=%d idx=%d: %f", seed, idx, s)
		}
		if l < 0.4 || l >= 0.6 {
			t.Fatalf("lit out of [0.4,0.6) at seed=%d idx=%d: %f", seed, idx, l)
		}
	}
}

// --- HSL→Hex adversarial ---

func TestHSLToHexAdversarial(t *testing.T) {
	// Feed every combination of extremal/pathological HSL values.
	hues := []float64{-1e18, -720, -360, -0.001, 0, 0.001, 120, 180, 240, 359.999, 360, 720, 1e18, math.NaN(), math.Inf(1), math.Inf(-1)}
	sats := []float64{-1e18, -1, 0, 0.5, 1, 2, 1e18, math.NaN(), math.Inf(1)}
	lits := []float64{-1e18, -1, 0, 0.5, 1, 2, 1e18, math.NaN(), math.Inf(1)}

	for _, h := range hues {
		for _, s := range sats {
			for _, l := range lits {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("hslToHex(%.4g, %.4g, %.4g) panicked: %v", h, s, l, r)
						}
					}()
					hex := hslToHex(clampHue(h), clamp01(s), clamp01(l))
					if len(hex) != 7 || hex[0] != '#' {
						t.Errorf("hslToHex(%.4g, %.4g, %.4g) = %q bad format", h, s, l, hex)
					}
				}()
			}
		}
	}
}

// --- URI adversarial ---

func TestResolveColorURIPathological(t *testing.T) {
	// Register a VM so non-empty names can resolve.
	stubVM(t, "chaos-vm")

	// Pathological URIs that must not panic.
	pathological := []string{
		"color://",
		"color:///",
		"color:////",
		"color://chaos-vm",
		"color://chaos-vm/",
		"color://chaos-vm/spi",
		"color://chaos-vm/walk",
		"color://chaos-vm/nonexistent-sub",
		"color://chaos-vm/spi/extra/segments",
		"color://" + string(make([]byte, 10000)), // huge name
		"color://\x00\x01\x02",                   // binary garbage
		"color://chaos-vm/\x00",
	}

	for _, uri := range pathological {
		t.Run(uri[:min(len(uri), 40)], func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ResolveColorURI(%q) panicked: %v", uri[:min(len(uri), 40)], r)
				}
			}()
			_ = ResolveColorURI(uri)
		})
	}
}

func TestResolveVMURIPathological(t *testing.T) {
	stubVM(t, "chaos-vm2")

	pathological := []string{
		"vm://",
		"vm:///",
		"vm:////",
		"vm://chaos-vm2",
		"vm://chaos-vm2/",
		"vm://chaos-vm2/state",
		"vm://chaos-vm2/nonexistent",
		"vm://" + string(make([]byte, 10000)),
		"vm://\x00\x01",
	}

	for _, uri := range pathological {
		t.Run(uri[:min(len(uri), 40)], func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ResolveVMURI(%q) panicked: %v", uri[:min(len(uri), 40)], r)
				}
			}()
			_ = ResolveVMURI(uri)
		})
	}
}

// --- Concurrent chaos: SetAttr + ResolveColorURI + RandomWalk simultaneously ---

func TestChaosTripleContention(t *testing.T) {
	// Hammer a single VM with concurrent writes, reads, and random walks.
	const workers = 16
	const opsPerWorker = 5_000
	inst := stubVM(t, "chaos-triple")

	// Also register some neighbors for random walk.
	for i := 0; i < 5; i++ {
		stubVM(t, fmt.Sprintf("chaos-neighbor-%d", i))
	}

	var wg sync.WaitGroup
	var panics atomic.Int64

	// Writers
	for w := 0; w < workers/2; w++ {
		wg.Add(1)
		w := w
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
					t.Errorf("writer %d panic: %v", w, r)
				}
			}()
			for i := 0; i < opsPerWorker; i++ {
				SetAttr(inst, fmt.Sprintf("chaos-key-%d", i%20), float64(i))
			}
		}()
	}

	// Readers (color URI)
	for w := 0; w < workers/4; w++ {
		wg.Add(1)
		w := w
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
					t.Errorf("reader %d panic: %v", w, r)
				}
			}()
			for i := 0; i < opsPerWorker; i++ {
				_ = ResolveColorURI("color://chaos-triple")
				_ = ResolveColorURI("color://chaos-triple/spi")
			}
		}()
	}

	// Walkers
	for w := 0; w < workers/4; w++ {
		wg.Add(1)
		w := w
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
					t.Errorf("walker %d panic: %v", w, r)
				}
			}()
			for i := 0; i < opsPerWorker/10; i++ {
				_ = RandomWalk("chaos-triple", 8, uint64(w*1000+i))
			}
		}()
	}

	wg.Wait()
	if p := panics.Load(); p > 0 {
		t.Fatalf("%d goroutines panicked during chaos test", p)
	}
}

// --- SPI fingerprint: order independence within same count ---

func TestSPIFingerprintIsOrderIndependent(t *testing.T) {
	// The fingerprint depends on (seed XOR invocation_counter), NOT on attr values/keys.
	// So two VMs with same seed and same invocation count must have identical fingerprints,
	// regardless of which keys/values were written.
	instA := &VMInstance{Attrs: make(map[string]any), Seed: 42, shutdown: make(chan struct{})}
	instB := &VMInstance{Attrs: make(map[string]any), Seed: 42, shutdown: make(chan struct{})}

	// Different keys, different values, same count.
	for i := 0; i < 500; i++ {
		SetAttr(instA, fmt.Sprintf("alpha-%d", i), "aaa")
		SetAttr(instB, fmt.Sprintf("beta-%d", i), float64(i))
	}

	if instA.Fingerprint != instB.Fingerprint {
		t.Errorf("SPI violation: same seed + same count should produce same fingerprint. A=%d B=%d",
			instA.Fingerprint, instB.Fingerprint)
	}
	if instA.Invocation != instB.Invocation {
		t.Errorf("invocation mismatch: A=%d B=%d", instA.Invocation, instB.Invocation)
	}
}

// --- RandomWalk: cycle detection ---

func TestRandomWalkNoPanicOnSingleVM(t *testing.T) {
	// A registry with only one VM: walk should visit the same VM every step.
	vmRegistryMu.Lock()
	saved := vmRegistry
	vmRegistry = make(map[string]*VMInstance)
	vmRegistryMu.Unlock()
	defer func() {
		vmRegistryMu.Lock()
		vmRegistry = saved
		vmRegistryMu.Unlock()
	}()

	solo := &VMInstance{Attrs: make(map[string]any), Seed: 1, shutdown: make(chan struct{})}
	vmRegistryMu.Lock()
	vmRegistry["solo"] = solo
	vmRegistryMu.Unlock()

	trail := RandomWalk("solo", 100, 7)
	if trail == nil {
		t.Fatal("single-VM walk should not be nil")
	}
	for i, step := range trail {
		if step.Name != "solo" {
			t.Errorf("step %d: expected 'solo', got %q", i, step.Name)
		}
	}
}

// --- seedFromName: collision resistance ---

func TestSeedFromNameCollisionResistance(t *testing.T) {
	// 10K unique names should produce 10K unique seeds.
	const n = 10_000
	seen := make(map[uint64]bool, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("vm-%d-%s", i, []string{"alpha", "beta", "gamma", "delta"}[i%4])
		s := seedFromName(name)
		if seen[s] {
			t.Fatalf("seed collision at name %q", name)
		}
		seen[s] = true
	}
}

// --- Go fuzzing targets ---

func FuzzSplitMix64(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(42))
	f.Add(^uint64(0))
	f.Add(uint64(0xdeadbeefcafebabe))
	f.Fuzz(func(t *testing.T, x uint64) {
		y := splitmix64(x)
		// Must be deterministic.
		if splitmix64(x) != y {
			t.Errorf("non-deterministic at %d", x)
		}
		// Must not be a fixed point (extremely unlikely for good hash).
		// Skip this check: there could theoretically be one, just flag it.
		if y == x {
			t.Logf("NOTE: fixed point at %d", x)
		}
	})
}

func FuzzColorAt(f *testing.F) {
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(1), uint64(1))
	f.Add(^uint64(0), ^uint64(0))
	f.Add(uint64(0xCAFE), uint64(9999))
	f.Fuzz(func(t *testing.T, seed, idx uint64) {
		h, s, l := colorAt(seed, idx)
		if math.IsNaN(h) || math.IsInf(h, 0) || h < 0 || h >= 360 {
			t.Errorf("hue out of range: %f (seed=%d, idx=%d)", h, seed, idx)
		}
		if math.IsNaN(s) || math.IsInf(s, 0) || s < 0.5 || s >= 1.0 {
			t.Errorf("sat out of range: %f (seed=%d, idx=%d)", s, seed, idx)
		}
		if math.IsNaN(l) || math.IsInf(l, 0) || l < 0.4 || l >= 0.6 {
			t.Errorf("lit out of range: %f (seed=%d, idx=%d)", l, seed, idx)
		}
		// Determinism.
		h2, s2, l2 := colorAt(seed, idx)
		if h != h2 || s != s2 || l != l2 {
			t.Errorf("non-deterministic at seed=%d idx=%d", seed, idx)
		}
	})
}

func FuzzResolveColorURI(f *testing.F) {
	f.Add("color://")
	f.Add("color:///")
	f.Add("color://x")
	f.Add("color://x/spi")
	f.Add("color://x/walk")
	f.Add("color://x/nonexistent")
	f.Add("color://\x00\x01\x02\x03")
	f.Fuzz(func(t *testing.T, uri string) {
		// Must never panic regardless of input.
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic on URI %q: %v", uri, r)
			}
		}()
		_ = ResolveColorURI(uri)
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
