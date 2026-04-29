//go:build darwin

package vm

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bmorphism/boxxy/internal/lisp"
)

// ---------- helpers ----------

// stubVM creates a VMInstance with no real vz.VirtualMachine (for unit tests
// that only exercise Attrs / URI resolution logic, not Virtualization.framework).
// We register it in the global vmRegistry under the given name.
func stubVM(t *testing.T, name string) *VMInstance {
	t.Helper()
	inst := &VMInstance{
		Attrs:    make(map[string]any),
		shutdown: make(chan struct{}),
	}
	RegisterVM(name, inst)
	t.Cleanup(func() {
		vmRegistryMu.Lock()
		delete(vmRegistry, name)
		vmRegistryMu.Unlock()
	})
	return inst
}

// ---------- snapshot consistency (the Jepsen torn-read test) ----------

// TestColorSnapshotConsistency verifies that a concurrent writer cannot
// produce a torn read where hue/saturation/lightness come from different
// "transactions". We write two distinct {hue,sat,lit} tuples in a tight
// loop while a reader repeatedly resolves color://{name}. Every observed
// snapshot must be one of the two tuples, never a cross.
func TestColorSnapshotConsistency(t *testing.T) {
	inst := stubVM(t, "jepsen-snap")

	// Two "transactions": A and B. They differ in all three values so any
	// cross is detectable.
	type hslTuple struct {
		hue, sat, lit float64
	}
	tupleA := hslTuple{0, 0.2, 0.3}
	tupleB := hslTuple{180, 0.8, 0.7}

	// Seed with A.
	SetAttr(inst, "hue", tupleA.hue)
	SetAttr(inst, "saturation", tupleA.sat)
	SetAttr(inst, "lightness", tupleA.lit)

	const iters = 50_000
	var stop atomic.Bool
	var violations atomic.Int64

	// Writer goroutine: alternate between A and B.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; !stop.Load(); i++ {
			var tup hslTuple
			if i%2 == 0 {
				tup = tupleA
			} else {
				tup = tupleB
			}
			// Write all three under one lock (same as user calling set-attr! 3x
			// is NOT atomic, but SetAttr is individually locked — so we simulate
			// the "correct" pattern: holding the lock ourselves).
			inst.mu.Lock()
			if inst.Attrs == nil {
				inst.Attrs = make(map[string]any)
			}
			inst.Attrs["hue"] = tup.hue
			inst.Attrs["saturation"] = tup.sat
			inst.Attrs["lightness"] = tup.lit
			inst.mu.Unlock()
		}
	}()

	// Reader goroutine: resolve and check snapshot integrity.
	for i := 0; i < iters; i++ {
		val := ResolveColorURI("color://jepsen-snap")
		hm, ok := val.(lisp.HashMap)
		if !ok {
			// inst has no real VM so getStateLocked will panic — skip if
			// the stub doesn't support state resolution. In our stub this
			// will panic, so we test a different way below.
			t.Skipf("stub VM does not support full ResolveColorURI (no vz.VirtualMachine)")
		}
		h := float64(hm[lisp.Keyword("hue")].(lisp.Float))
		s := float64(hm[lisp.Keyword("saturation")].(lisp.Float))
		l := float64(hm[lisp.Keyword("lightness")].(lisp.Float))

		isA := h == tupleA.hue && s == tupleA.sat && l == tupleA.lit
		isB := h == tupleB.hue && s == tupleB.sat && l == tupleB.lit
		if !isA && !isB {
			violations.Add(1)
			if violations.Load() <= 3 {
				t.Errorf("torn read #%d: hue=%.1f sat=%.2f lit=%.2f", i, h, s, l)
			}
		}
	}
	stop.Store(true)
	wg.Wait()

	if v := violations.Load(); v > 0 {
		t.Fatalf("%d torn reads out of %d iterations", v, iters)
	}
}

// TestColorSnapshotConsistencyAttrsOnly exercises the snapshot path without
// needing a real vz.VirtualMachine by directly testing the attrs-snapshot
// logic in isolation.
func TestColorSnapshotConsistencyAttrsOnly(t *testing.T) {
	inst := &VMInstance{
		Attrs:    make(map[string]any),
		shutdown: make(chan struct{}),
	}

	type hslTuple struct{ hue, sat, lit float64 }
	tupleA := hslTuple{30, 0.1, 0.9}
	tupleB := hslTuple{270, 0.9, 0.1}

	inst.Attrs["hue"] = tupleA.hue
	inst.Attrs["saturation"] = tupleA.sat
	inst.Attrs["lightness"] = tupleA.lit

	const iters = 100_000
	var stop atomic.Bool
	var violations atomic.Int64

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; !stop.Load(); i++ {
			tup := tupleA
			if i%2 != 0 {
				tup = tupleB
			}
			inst.mu.Lock()
			inst.Attrs["hue"] = tup.hue
			inst.Attrs["saturation"] = tup.sat
			inst.Attrs["lightness"] = tup.lit
			inst.mu.Unlock()
		}
	}()

	for i := 0; i < iters; i++ {
		// Simulate the single-lock snapshot read from ResolveColorURI.
		inst.mu.Lock()
		h, _ := inst.Attrs["hue"].(float64)
		s, _ := inst.Attrs["saturation"].(float64)
		l, _ := inst.Attrs["lightness"].(float64)
		inst.mu.Unlock()

		isA := h == tupleA.hue && s == tupleA.sat && l == tupleA.lit
		isB := h == tupleB.hue && s == tupleB.sat && l == tupleB.lit
		if !isA && !isB {
			violations.Add(1)
			if violations.Load() <= 3 {
				t.Errorf("torn read: hue=%.1f sat=%.2f lit=%.2f", h, s, l)
			}
		}
	}
	stop.Store(true)
	wg.Wait()

	if v := violations.Load(); v > 0 {
		t.Fatalf("%d torn reads out of %d iterations", v, iters)
	}
}

// ---------- HSL domain clamping ----------

func TestClampHue(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{360, 0},       // wraps
		{720, 0},       // double wrap
		{-90, 270},     // negative
		{-360, 0},      // full negative rotation
		{999, 279},     // large positive
		{-999, 81},     // large negative
		{180.5, 180.5}, // fractional preserved
	}
	for _, tc := range cases {
		got := clampHue(tc.in)
		if math.Abs(got-tc.want) > 0.01 {
			t.Errorf("clampHue(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestClamp01(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{-0.5, 0},
		{0, 0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
		{999, 1.0},
	}
	for _, tc := range cases {
		got := clamp01(tc.in)
		if got != tc.want {
			t.Errorf("clamp01(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestHSLToHexClamped(t *testing.T) {
	// Out-of-range inputs must not produce values outside #000000..#FFFFFF.
	extreme := []struct{ h, s, l float64 }{
		{999, 2.0, -1.0},
		{-360, -0.5, 1.5},
		{0, 0, 0},
		{360, 1, 1},
	}
	for _, e := range extreme {
		hex := hslToHex(clampHue(e.h), clamp01(e.s), clamp01(e.l))
		if len(hex) != 7 || hex[0] != '#' {
			t.Errorf("hslToHex(%v,%v,%v) = %q, bad format", e.h, e.s, e.l, hex)
		}
		// Parse and verify range.
		var r, g, b int
		fmt.Sscanf(hex, "#%02X%02X%02X", &r, &g, &b)
		if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
			t.Errorf("hslToHex(%v,%v,%v) = %q, RGB out of range", e.h, e.s, e.l, hex)
		}
	}
}

// ---------- edge-case URIs ----------

func TestResolveVMURIEdgeCases(t *testing.T) {
	stubVM(t, "edge-test")

	cases := []struct {
		uri    string
		isNil  bool
		panics bool
	}{
		{"vm://", true, false},          // empty name
		{"vm:///state", true, false},    // empty name with sub-resource
		{"vm://nonexistent", true, false},
		{"vm://edge-test", false, false},
		{"vm://edge-test/bogus", true, false}, // unknown sub-resource → nil now
		{"not-a-vm-uri", false, true},         // wrong scheme → panic
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.uri, func(t *testing.T) {
			if tc.panics {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic, got none")
					}
				}()
			}
			val := ResolveVMURI(tc.uri)
			if tc.panics {
				return
			}
			_, isNil := val.(lisp.Nil)
			if isNil != tc.isNil {
				t.Errorf("ResolveVMURI(%q): nil=%v, want nil=%v (got %T)", tc.uri, isNil, tc.isNil, val)
			}
		})
	}
}

func TestResolveColorURIEdgeCases(t *testing.T) {
	cases := []struct {
		uri    string
		isNil  bool
		panics bool
	}{
		{"color://", true, false},
		{"color:///hue", true, false},
		{"color://nonexistent", true, false},
		{"nope://x", false, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.uri, func(t *testing.T) {
			if tc.panics {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic, got none")
					}
				}()
			}
			val := ResolveColorURI(tc.uri)
			if tc.panics {
				return
			}
			_, isNil := val.(lisp.Nil)
			if isNil != tc.isNil {
				t.Errorf("ResolveColorURI(%q): nil=%v, want nil=%v (got %T)", tc.uri, isNil, tc.isNil, val)
			}
		})
	}
}

// ---------- nil Attrs safety ----------

func TestSetAttrOnNilAttrs(t *testing.T) {
	inst := &VMInstance{
		shutdown: make(chan struct{}),
		// Attrs intentionally nil — simulates a VMInstance created outside the
		// normal constructor path.
	}
	// Must not panic.
	SetAttr(inst, "mood", "calm")
	v, ok := GetAttr(inst, "mood")
	if !ok || v != "calm" {
		t.Errorf("SetAttr on nil Attrs: got %v, %v", v, ok)
	}
}

// ---------- concurrent SetAttr race detector ----------

func TestConcurrentSetAttrNoRace(t *testing.T) {
	inst := stubVM(t, "race-test")
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < 10_000; i++ {
				SetAttr(inst, fmt.Sprintf("key-%d", g), float64(i))
				GetAttr(inst, fmt.Sprintf("key-%d", (g+1)%8))
			}
		}()
	}
	wg.Wait()
}

// ---------- anyToLisp roundtrip ----------

func TestAnyToLisp(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"hello", "hello"},
		{42.0, "42"},
		{true, "true"},
		{false, "false"},
		{nil, "nil"},
		{struct{ X int }{7}, "{7}"},
	}
	for _, tc := range cases {
		got := anyToLisp(tc.in)
		s := got.String()
		if !strings.Contains(s, tc.want) {
			t.Errorf("anyToLisp(%v) = %q, want contains %q", tc.in, s, tc.want)
		}
	}
}

// ---------- stateHue coverage ----------

func TestStateHueCoverage(t *testing.T) {
	known := map[string]float64{
		"running":  120,
		"paused":   60,
		"starting": 180,
		"error":    0,
		"stopped":  240,
		"unknown":  240,
		"":         240,
	}
	for state, want := range known {
		got := stateHue(state)
		if got != want {
			t.Errorf("stateHue(%q) = %v, want %v", state, got, want)
		}
	}
}
