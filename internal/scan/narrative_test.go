//go:build darwin

package scan

import (
	"testing"
	"time"
)

func TestNarrativeEmpty(t *testing.T) {
	n := NewNarrative()
	if !n.IsBalanced() {
		t.Error("empty narrative should be balanced")
	}
	violations := n.VerifySheafCondition()
	if len(violations) != 0 {
		t.Errorf("empty narrative has violations: %v", violations)
	}
}

func fakeScanResult(devices ...Device) *ScanResult {
	return &ScanResult{
		Timestamp: time.Now(),
		Devices:   devices,
	}
}

func TestNarrativeSingleSnapshot(t *testing.T) {
	n := NewNarrative()
	n.AddSnapshot(fakeScanResult(
		Device{MAC: "aa:bb:cc:dd:ee:01", GF3Trit: 1},
		Device{MAC: "aa:bb:cc:dd:ee:02", GF3Trit: -1},
	))

	// F([0,0]) should be 1 + (-1) = 0
	v, ok := n.SheafValue(0, 0)
	if !ok {
		t.Fatal("missing value for [0,0]")
	}
	if v.GF3 != 0 {
		t.Errorf("F([0,0]) = %d, want 0", v.GF3)
	}
	if !n.IsBalanced() {
		t.Error("single balanced snapshot should yield balanced narrative")
	}
}

func TestNarrativeSheafCondition(t *testing.T) {
	n := NewNarrative()

	// Three snapshots with trits: +1, -1, +1
	n.AddSnapshot(fakeScanResult(Device{MAC: "a", GF3Trit: 1}))
	n.AddSnapshot(fakeScanResult(Device{MAC: "b", GF3Trit: -1}))
	n.AddSnapshot(fakeScanResult(Device{MAC: "c", GF3Trit: 1}))

	// Check sheaf condition: F([a,b]) = F([a,p]) + F([p,b])
	violations := n.VerifySheafCondition()
	if len(violations) != 0 {
		t.Errorf("sheaf violations: %v", violations)
	}

	// F([0,0]) = 1
	v00, _ := n.SheafValue(0, 0)
	if v00.GF3 != 1 {
		t.Errorf("F([0,0]) = %d, want 1", v00.GF3)
	}

	// F([1,1]) = -1
	v11, _ := n.SheafValue(1, 1)
	if v11.GF3 != -1 {
		t.Errorf("F([1,1]) = %d, want -1", v11.GF3)
	}

	// F([0,1]) = 1 + (-1) = 0
	v01, _ := n.SheafValue(0, 1)
	if v01.GF3 != 0 {
		t.Errorf("F([0,1]) = %d, want 0", v01.GF3)
	}

	// F([0,2]) = 1 + (-1) + 1 = 1
	v02, _ := n.SheafValue(0, 2)
	if v02.GF3 != 1 {
		t.Errorf("F([0,2]) = %d, want 1", v02.GF3)
	}

	// F([1,2]) = -1 + 1 = 0
	v12, _ := n.SheafValue(1, 2)
	if v12.GF3 != 0 {
		t.Errorf("F([1,2]) = %d, want 0", v12.GF3)
	}
}

func TestNarrativeBalanced(t *testing.T) {
	n := NewNarrative()

	// Three snapshots summing to 0: +1, +1, +1 → sum = 3 ≡ 0 (mod 3)
	n.AddSnapshot(fakeScanResult(Device{MAC: "a", GF3Trit: 1}))
	n.AddSnapshot(fakeScanResult(Device{MAC: "b", GF3Trit: 1}))
	n.AddSnapshot(fakeScanResult(Device{MAC: "c", GF3Trit: 1}))

	// tritAdd(1, tritAdd(1, 1)) = tritAdd(1, -1) = 0
	// Because in balanced ternary: 1+1 = 2 → 2-3 = -1, then 1+(-1) = 0
	if !n.IsBalanced() {
		v, _ := n.SheafValue(0, 2)
		t.Errorf("narrative should be balanced, F([0,2]) = %d", v.GF3)
	}
}

func TestNarrativeUnbalanced(t *testing.T) {
	n := NewNarrative()

	// Two snapshots: +1, +1 → sum = -1 (in balanced ternary: 2-3 = -1)
	n.AddSnapshot(fakeScanResult(Device{MAC: "a", GF3Trit: 1}))
	n.AddSnapshot(fakeScanResult(Device{MAC: "b", GF3Trit: 1}))

	if n.IsBalanced() {
		t.Error("narrative with two +1 should be unbalanced")
	}

	v, ok := n.SheafValue(0, 1)
	if !ok {
		t.Fatal("missing [0,1]")
	}
	if v.GF3 != -1 {
		t.Errorf("F([0,1]) = %d, want -1", v.GF3)
	}
}

func TestNarrativeGF9Frobenius(t *testing.T) {
	n := NewNarrative()

	// Device with GF(3)-embeddable nonet: (1,0) is in the image of trit_to_nonet
	// Frobenius((a,b)) = (a,-b), so (1,0) is fixed
	n.AddSnapshot(fakeScanResult(Device{
		MAC:      "a",
		GF3Trit:  1,
		GF9Nonet: &Nonet{Score: 1, Confidence: 0},
	}))

	fixed := n.FrobeniusFixed()
	if len(fixed) != 1 {
		t.Errorf("expected 1 Frobenius-fixed interval, got %d", len(fixed))
	}

	// Now add a non-fixed device: (1,1), Frobenius = (1,-1) ≠ (1,1)
	n.AddSnapshot(fakeScanResult(Device{
		MAC:      "b",
		GF3Trit:  0,
		GF9Nonet: &Nonet{Score: 1, Confidence: 1},
	}))

	// [1,1] should NOT be Frobenius-fixed
	v11, _ := n.SheafValue(1, 1)
	frob := v11.GF9.Frobenius()
	if frob == v11.GF9 {
		t.Error("[1,1] should not be Frobenius-fixed when confidence ≠ 0")
	}
}

func TestNarrativeInterval(t *testing.T) {
	iv := Interval{2, 5}
	if iv.String() != "[2,5]" {
		t.Errorf("Interval.String() = %q, want [2,5]", iv.String())
	}
}

func TestNarrativeSaveLoad(t *testing.T) {
	dir := t.TempDir()

	n := NewNarrative()
	n.AddSnapshot(fakeScanResult(
		Device{MAC: "aa:bb:cc:dd:ee:01", GF3Trit: 1},
		Device{MAC: "aa:bb:cc:dd:ee:02", GF3Trit: -1},
	))
	n.AddSnapshot(fakeScanResult(
		Device{MAC: "aa:bb:cc:dd:ee:03", GF3Trit: 0},
	))

	if err := n.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadNarrative(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Snapshots) != 2 {
		t.Errorf("loaded %d snapshots, want 2", len(loaded.Snapshots))
	}
	if len(loaded.Values) != len(n.Values) {
		t.Errorf("loaded %d values, want %d", len(loaded.Values), len(n.Values))
	}

	// Verify sheaf condition survives round-trip
	violations := loaded.VerifySheafCondition()
	if len(violations) != 0 {
		t.Errorf("loaded narrative has violations: %v", violations)
	}
}
