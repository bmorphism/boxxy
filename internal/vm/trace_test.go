package vm

import (
	"math"
	"testing"
)

func TestTraceCacheSplitMix64Deterministic(t *testing.T) {
	tc := NewTraceCache()
	inputs := []uint64{0, 1, 42, math.MaxUint64, 0x9e3779b97f4a7c15}

	t1 := tc.RecordSplitMix64(inputs)
	t2 := tc.RecordSplitMix64(inputs)

	if t1.Fingerprint != t2.Fingerprint {
		t.Fatal("same inputs must produce identical fingerprints")
	}
}

func TestTraceCacheSplitMix64DifferentInputs(t *testing.T) {
	tc := NewTraceCache()
	a := tc.RecordSplitMix64([]uint64{0, 1, 2})
	tc2 := NewTraceCache()
	b := tc2.RecordSplitMix64([]uint64{3, 4, 5})

	if a.Fingerprint == b.Fingerprint {
		t.Fatal("different inputs must produce different fingerprints")
	}
}

func TestTraceCacheColorAtDeterministic(t *testing.T) {
	tc := NewTraceCache()
	pairs := [][2]uint64{{0, 0}, {42, 1}, {math.MaxUint64, 100}}

	t1 := tc.RecordColorAt(pairs)
	t2 := tc.RecordColorAt(pairs)

	if t1.Fingerprint != t2.Fingerprint {
		t.Fatal("colorAt traces must be deterministic")
	}
}

func TestTraceCacheSeedFromNameDeterministic(t *testing.T) {
	tc := NewTraceCache()
	names := []string{"alice", "bob", "carol", "", "🎨"}

	t1 := tc.RecordSeedFromName(names)
	t2 := tc.RecordSeedFromName(names)

	if t1.Fingerprint != t2.Fingerprint {
		t.Fatal("seedFromName traces must be deterministic")
	}
}

func TestBehavioralEquality(t *testing.T) {
	tc := NewTraceCache()
	inputs := []uint64{0, 1, 2, 3, 4}

	tc.RecordSplitMix64(inputs)

	// Record the same function under a different name by manually inserting
	entries := make([]TraceEntry, len(inputs))
	for i, in := range inputs {
		out := splitmix64(in)
		entries[i] = TraceEntry{
			InputHash:  hashUint64(in),
			OutputHash: hashUint64(out),
		}
	}
	bt := &BehavioralTrace{
		Name:        "splitmix64_clone",
		Entries:     entries,
		Fingerprint: fingerprint(entries),
	}
	tc.mu.Lock()
	tc.traces["splitmix64_clone"] = bt
	tc.mu.Unlock()

	eq, err := tc.BehaviorallyEqual("splitmix64", "splitmix64_clone")
	if err != nil {
		t.Fatal(err)
	}
	if !eq {
		t.Fatal("identical implementations must be behaviorally equal")
	}
}

func TestBehavioralEqualityMissing(t *testing.T) {
	tc := NewTraceCache()
	_, err := tc.BehaviorallyEqual("missing_a", "missing_b")
	if err == nil {
		t.Fatal("expected error for unrecorded traces")
	}
}

func TestFingerprintOrderIndependent(t *testing.T) {
	// Insertion order must not affect fingerprint
	e1 := []TraceEntry{
		{InputHash: hashUint64(1), OutputHash: hashUint64(splitmix64(1))},
		{InputHash: hashUint64(2), OutputHash: hashUint64(splitmix64(2))},
	}
	e2 := []TraceEntry{
		{InputHash: hashUint64(2), OutputHash: hashUint64(splitmix64(2))},
		{InputHash: hashUint64(1), OutputHash: hashUint64(splitmix64(1))},
	}
	if fingerprint(e1) != fingerprint(e2) {
		t.Fatal("fingerprint must be order-independent")
	}
}

// --- Functoriality tests ---

func TestFunctorialityColorAt(t *testing.T) {
	tc := NewTraceCache()
	inputs := make([][2]uint64, 1000)
	for i := range inputs {
		inputs[i] = [2]uint64{uint64(i * 7), uint64(i * 13)}
	}

	r := tc.VerifyFunctoriality(inputs)
	if !r.Preserved {
		t.Fatalf("functoriality violated: %s", r)
	}
	t.Logf("%s", r)
}

func TestFunctorialityEdgeCases(t *testing.T) {
	tc := NewTraceCache()
	inputs := [][2]uint64{
		{0, 0},
		{math.MaxUint64, math.MaxUint64},
		{0, math.MaxUint64},
		{math.MaxUint64, 0},
		{1, 1},
		{0x9e3779b97f4a7c15, 42},
	}

	r := tc.VerifyFunctoriality(inputs)
	if !r.Preserved {
		t.Fatalf("functoriality violated at edges: %s", r)
	}
}

func TestEndToEndFunctoriality(t *testing.T) {
	tc := NewTraceCache()
	names := []string{
		"alice", "bob", "carol", "dave", "eve",
		"", "🎨", "a very long name with spaces and unicode: ñ",
		"splitmix64", "boxxy",
	}

	r := tc.VerifyEndToEndFunctoriality(names, 0)
	if !r.Preserved {
		t.Fatalf("e2e functoriality violated at index=0: %s", r)
	}
	t.Logf("%s", r)

	// Different index
	r2 := tc.VerifyEndToEndFunctoriality(names, 999)
	if !r2.Preserved {
		t.Fatalf("e2e functoriality violated at index=999: %s", r2)
	}

	// Fingerprints should differ between index=0 and index=999
	if r.ComposedFingerprint == r2.ComposedFingerprint {
		t.Fatal("different indices should produce different fingerprints")
	}
}

func TestEndToEndFunctorialityMaxIndex(t *testing.T) {
	tc := NewTraceCache()
	names := []string{"test-vm-0", "test-vm-1"}

	r := tc.VerifyEndToEndFunctoriality(names, math.MaxUint64)
	if !r.Preserved {
		t.Fatalf("e2e functoriality violated at MaxUint64: %s", r)
	}
}

func TestMemoizationO1Lookup(t *testing.T) {
	tc := NewTraceCache()
	tc.RecordSplitMix64([]uint64{1, 2, 3})

	// First lookup
	bt, ok := tc.Get("splitmix64")
	if !ok {
		t.Fatal("expected memoized trace")
	}
	fp1 := bt.Fingerprint

	// Second lookup — should be O(1), same pointer
	bt2, ok := tc.Get("splitmix64")
	if !ok {
		t.Fatal("expected memoized trace on second lookup")
	}
	if fp1 != bt2.Fingerprint {
		t.Fatal("memoized fingerprint must be stable")
	}
}

// --- Fuzz targets ---

func FuzzFunctoriality(f *testing.F) {
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(42), uint64(1))
	f.Add(uint64(math.MaxUint64), uint64(math.MaxUint64))
	f.Fuzz(func(t *testing.T, seed, index uint64) {
		tc := NewTraceCache()
		r := tc.VerifyFunctoriality([][2]uint64{{seed, index}})
		if !r.Preserved {
			t.Fatalf("functoriality violated for seed=%d index=%d", seed, index)
		}
	})
}
