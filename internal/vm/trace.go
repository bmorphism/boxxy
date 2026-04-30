package vm

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
)

// TraceEntry records a single (input → output) observation of a pure function.
type TraceEntry struct {
	InputHash  [32]byte
	OutputHash [32]byte
}

// BehavioralTrace is a content-addressed memo of a function's observed behavior.
// Two traces with the same Fingerprint are behaviorally identical.
type BehavioralTrace struct {
	Name        string
	Entries     []TraceEntry
	Fingerprint [32]byte // sha256 of sorted (input_hash, output_hash) pairs
}

// TraceCache provides O(1) behavioral identity checks after first observation.
type TraceCache struct {
	mu     sync.RWMutex
	traces map[string]*BehavioralTrace // keyed by function name
}

func NewTraceCache() *TraceCache {
	return &TraceCache{traces: make(map[string]*BehavioralTrace)}
}

// hashUint64 produces a content hash for a uint64 value.
func hashUint64(v uint64) [32]byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	return sha256.Sum256(buf[:])
}

// hashFloat64Triple produces a content hash for an (h, s, l) triple.
func hashFloat64Triple(a, b, c float64) [32]byte {
	var buf [24]byte
	binary.LittleEndian.PutUint64(buf[0:8], math.Float64bits(a))
	binary.LittleEndian.PutUint64(buf[8:16], math.Float64bits(b))
	binary.LittleEndian.PutUint64(buf[16:24], math.Float64bits(c))
	return sha256.Sum256(buf[:])
}

// hashString produces a content hash for a string.
func hashString(s string) [32]byte {
	return sha256.Sum256([]byte(s))
}

// fingerprint computes the content-addressed behavioral identity of a trace.
// Sorted by InputHash so insertion order doesn't matter.
func fingerprint(entries []TraceEntry) [32]byte {
	sorted := make([]TraceEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		for k := 0; k < 32; k++ {
			if sorted[i].InputHash[k] != sorted[j].InputHash[k] {
				return sorted[i].InputHash[k] < sorted[j].InputHash[k]
			}
		}
		return false
	})
	h := sha256.New()
	for _, e := range sorted {
		h.Write(e.InputHash[:])
		h.Write(e.OutputHash[:])
	}
	var fp [32]byte
	copy(fp[:], h.Sum(nil))
	return fp
}

// RecordSplitMix64 observes splitmix64 over a set of inputs and memoizes the trace.
func (tc *TraceCache) RecordSplitMix64(inputs []uint64) *BehavioralTrace {
	entries := make([]TraceEntry, len(inputs))
	for i, in := range inputs {
		out := splitmix64(in)
		entries[i] = TraceEntry{
			InputHash:  hashUint64(in),
			OutputHash: hashUint64(out),
		}
	}
	bt := &BehavioralTrace{
		Name:        "splitmix64",
		Entries:     entries,
		Fingerprint: fingerprint(entries),
	}
	tc.mu.Lock()
	tc.traces["splitmix64"] = bt
	tc.mu.Unlock()
	return bt
}

// RecordColorAt observes colorAt over a set of (seed, index) pairs.
func (tc *TraceCache) RecordColorAt(pairs [][2]uint64) *BehavioralTrace {
	entries := make([]TraceEntry, len(pairs))
	for i, p := range pairs {
		h, s, l := colorAt(p[0], p[1])
		var ibuf [16]byte
		binary.LittleEndian.PutUint64(ibuf[0:8], p[0])
		binary.LittleEndian.PutUint64(ibuf[8:16], p[1])
		entries[i] = TraceEntry{
			InputHash:  sha256.Sum256(ibuf[:]),
			OutputHash: hashFloat64Triple(h, s, l),
		}
	}
	bt := &BehavioralTrace{
		Name:        "colorAt",
		Entries:     entries,
		Fingerprint: fingerprint(entries),
	}
	tc.mu.Lock()
	tc.traces["colorAt"] = bt
	tc.mu.Unlock()
	return bt
}

// RecordSeedFromName observes seedFromName over a set of names.
func (tc *TraceCache) RecordSeedFromName(names []string) *BehavioralTrace {
	entries := make([]TraceEntry, len(names))
	for i, n := range names {
		out := seedFromName(n)
		entries[i] = TraceEntry{
			InputHash:  hashString(n),
			OutputHash: hashUint64(out),
		}
	}
	bt := &BehavioralTrace{
		Name:        "seedFromName",
		Entries:     entries,
		Fingerprint: fingerprint(entries),
	}
	tc.mu.Lock()
	tc.traces["seedFromName"] = bt
	tc.mu.Unlock()
	return bt
}

// BehaviorallyEqual returns true iff two named traces have the same fingerprint.
// O(1) after both have been recorded.
func (tc *TraceCache) BehaviorallyEqual(a, b string) (bool, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	ta, ok := tc.traces[a]
	if !ok {
		return false, fmt.Errorf("trace %q not recorded", a)
	}
	tb, ok := tc.traces[b]
	if !ok {
		return false, fmt.Errorf("trace %q not recorded", b)
	}
	return ta.Fingerprint == tb.Fingerprint, nil
}

// Get retrieves a memoized trace by name.
func (tc *TraceCache) Get(name string) (*BehavioralTrace, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	bt, ok := tc.traces[name]
	return bt, ok
}

// --- Functoriality verification ---

// VerifyFunctoriality checks that trace(g ∘ f) == trace(g) ∘ trace(f)
// for the decomposition: colorAt(seed, index) = hslExtract(splitmix64(seed ^ index))
//
// This is the core functoriality property: the functor from "composed operations"
// to "behavioral traces" preserves composition.
//
// F(g ∘ f) = F(g) ∘ F(f)
//
// where F maps each function to its behavioral trace fingerprint.
func (tc *TraceCache) VerifyFunctoriality(inputs [][2]uint64) *FunctorialityResult {
	r := &FunctorialityResult{InputCount: len(inputs)}

	// Step 1: Record trace of the composed operation (colorAt)
	composedEntries := make([]TraceEntry, len(inputs))
	for i, p := range inputs {
		h, s, l := colorAt(p[0], p[1])
		var ibuf [16]byte
		binary.LittleEndian.PutUint64(ibuf[0:8], p[0])
		binary.LittleEndian.PutUint64(ibuf[8:16], p[1])
		composedEntries[i] = TraceEntry{
			InputHash:  sha256.Sum256(ibuf[:]),
			OutputHash: hashFloat64Triple(h, s, l),
		}
	}
	r.ComposedFingerprint = fingerprint(composedEntries)

	// Step 2: Record trace of the decomposed pipeline:
	//   f: (seed, index) → splitmix64(seed ^ index)     [mixing stage]
	//   g: mixed → (h, s, l)                              [extraction stage]
	//   g ∘ f == colorAt
	decomposedEntries := make([]TraceEntry, len(inputs))
	for i, p := range inputs {
		// f stage: mix
		mixed := splitmix64(p[0] ^ p[1])
		// g stage: extract HSL from mixed (same arithmetic as colorAt body)
		hue := float64(mixed&0xFFFF) / 65536.0 * 360.0
		sat := 0.5 + float64((mixed>>16)&0xFFFF)/65536.0*0.5
		lit := 0.4 + float64((mixed>>32)&0xFFFF)/65536.0*0.2

		var ibuf [16]byte
		binary.LittleEndian.PutUint64(ibuf[0:8], p[0])
		binary.LittleEndian.PutUint64(ibuf[8:16], p[1])
		decomposedEntries[i] = TraceEntry{
			InputHash:  sha256.Sum256(ibuf[:]),
			OutputHash: hashFloat64Triple(hue, sat, lit),
		}
	}
	r.DecomposedFingerprint = fingerprint(decomposedEntries)

	r.Preserved = r.ComposedFingerprint == r.DecomposedFingerprint
	return r
}

// FunctorialityResult reports whether F(g ∘ f) == F(g) ∘ F(f).
type FunctorialityResult struct {
	InputCount             int
	ComposedFingerprint    [32]byte // F(colorAt)
	DecomposedFingerprint  [32]byte // F(hslExtract) ∘ F(splitmix64)
	Preserved              bool     // true iff functoriality holds
}

func (r *FunctorialityResult) String() string {
	return fmt.Sprintf("functoriality(n=%d): composed=%x decomposed=%x preserved=%v",
		r.InputCount, r.ComposedFingerprint[:8], r.DecomposedFingerprint[:8], r.Preserved)
}

// VerifyEndToEndFunctoriality checks the full pipeline:
//   name → seedFromName → (seed, index) → colorAt → hex
// verifying that the composed pipeline's trace equals the decomposed trace
// at each stage boundary.
func (tc *TraceCache) VerifyEndToEndFunctoriality(names []string, index uint64) *EndToEndResult {
	r := &EndToEndResult{Names: names, Index: index}

	// Composed: name → hex (single shot)
	composedEntries := make([]TraceEntry, len(names))
	for i, n := range names {
		seed := seedFromName(n)
		h, s, l := colorAt(seed, index)
		hex := hslToHex(clampHue(h), clamp01(s), clamp01(l))
		composedEntries[i] = TraceEntry{
			InputHash:  hashString(n),
			OutputHash: hashString(hex),
		}
	}
	r.ComposedFingerprint = fingerprint(composedEntries)

	// Decomposed: name → seed → mixed → hsl → hex (staged)
	decomposedEntries := make([]TraceEntry, len(names))
	for i, n := range names {
		seed := seedFromName(n)                       // stage 1: name→seed
		mixed := splitmix64(seed ^ index)             // stage 2: seed→mixed
		h := float64(mixed&0xFFFF) / 65536.0 * 360.0 // stage 3: mixed→hsl
		s := 0.5 + float64((mixed>>16)&0xFFFF)/65536.0*0.5
		l := 0.4 + float64((mixed>>32)&0xFFFF)/65536.0*0.2
		hex := hslToHex(clampHue(h), clamp01(s), clamp01(l)) // stage 4: hsl→hex
		decomposedEntries[i] = TraceEntry{
			InputHash:  hashString(n),
			OutputHash: hashString(hex),
		}
	}
	r.DecomposedFingerprint = fingerprint(decomposedEntries)

	r.Preserved = r.ComposedFingerprint == r.DecomposedFingerprint
	return r
}

// EndToEndResult reports functoriality of the full name→hex pipeline.
type EndToEndResult struct {
	Names                 []string
	Index                 uint64
	ComposedFingerprint   [32]byte
	DecomposedFingerprint [32]byte
	Preserved             bool
}

func (r *EndToEndResult) String() string {
	return fmt.Sprintf("e2e-functoriality(names=%d, index=%d): composed=%x decomposed=%x preserved=%v",
		len(r.Names), r.Index, r.ComposedFingerprint[:8], r.DecomposedFingerprint[:8], r.Preserved)
}
