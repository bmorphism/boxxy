//go:build darwin && arm64

package vm

import (
	"testing"
)

func TestFlightPtrAccess(t *testing.T) {
	// 1KB buffer at 0x1000
	ptr := NewFlightPtr(0x1000, 0x1400, 0x1000, CapData)

	// In-bounds read
	if err := ptr.CheckAccess(256); err != nil {
		t.Fatalf("expected in-bounds access, got: %v", err)
	}

	// In-bounds write
	if err := ptr.CheckWrite(256); err != nil {
		t.Fatalf("expected in-bounds write, got: %v", err)
	}

	// OOB upper
	ptr.Intval = 0x1300
	if err := ptr.CheckAccess(0x200); err == nil {
		t.Fatal("expected oob-upper error")
	}

	// OOB lower
	ptr.Intval = 0x0FFF
	if err := ptr.CheckAccess(1); err == nil {
		t.Fatal("expected oob-lower error")
	}
}

func TestReadonlyCapDeniesWrite(t *testing.T) {
	ptr := NewFlightPtr(0x1000, 0x1400, 0x1000, CapReadonly)
	if err := ptr.CheckAccess(16); err != nil {
		t.Fatalf("readonly should allow read: %v", err)
	}
	if err := ptr.CheckWrite(16); err == nil {
		t.Fatal("readonly should deny write")
	}
}

func TestFunctionCapDeniesReadWrite(t *testing.T) {
	ptr := NewFlightPtr(0x2000, 0x2100, 0x2000, CapFunction)
	if err := ptr.CheckAccess(1); err == nil {
		t.Fatal("function cap should deny read")
	}
	if err := ptr.CheckWrite(1); err == nil {
		t.Fatal("function cap should deny write")
	}
}

func TestUseAfterFree(t *testing.T) {
	ptr := NewFlightPtr(0x1000, 0x1400, 0x1000, CapData)
	ptr.Free()
	if err := ptr.CheckAccess(1); err == nil {
		t.Fatal("expected use-after-free error")
	}
	if !ptr.Cap.Freed {
		t.Fatal("expected freed flag set")
	}
	if ptr.Cap.Upper != ptr.Cap.Lower {
		t.Fatal("expected upper == lower after free")
	}
}

func TestStoreAndLoad(t *testing.T) {
	fp := NewFlightPtr(0x1000, 0x1400, 0x1200, CapData)
	rp := Store(fp, 0, 0)
	if rp.Intval != 0x1200 {
		t.Fatalf("rest ptr intval mismatch: 0x%x", rp.Intval)
	}
	loaded := Load(rp)
	if loaded.Intval != fp.Intval {
		t.Fatal("load did not restore intval")
	}
	if loaded.Cap.Lower != fp.Cap.Lower || loaded.Cap.Upper != fp.Cap.Upper {
		t.Fatal("load did not restore capability bounds")
	}
}

func TestFFIBlanketNoSurprise(t *testing.T) {
	blanket := NewFFIBlanket()
	ptr := NewFlightPtr(0x1000, 0x2000, 0x1000, CapData)

	blanket.SnapshotBefore("buf", ptr)
	// Simulate FFI call that doesn't change capabilities
	surprise := blanket.VerifyAfter("buf", ptr)
	if surprise != nil {
		t.Fatalf("expected no surprise, got: %v", surprise)
	}
}

func TestFFIBlanketDetectsBoundsChange(t *testing.T) {
	blanket := NewFFIBlanket()
	ptr := NewFlightPtr(0x1000, 0x2000, 0x1000, CapData)

	blanket.SnapshotBefore("buf", ptr)
	// Simulate FFI corruption: upper bound changed
	ptr.Cap.Upper = 0x3000
	surprise := blanket.VerifyAfter("buf", ptr)
	if surprise == nil {
		t.Fatal("expected surprise for bounds change")
	}
	if surprise.Level != Awareness1 {
		t.Fatalf("expected awareness level 1, got %d", surprise.Level)
	}
}

func TestFFIBlanketDetectsStateChange(t *testing.T) {
	blanket := NewFFIBlanket()
	ptr := NewFlightPtr(0x1000, 0x2000, 0x1000, CapData)

	blanket.SnapshotBefore("buf", ptr)
	ptr.Cap.State = CapReadonly
	surprise := blanket.VerifyAfter("buf", ptr)
	if surprise == nil {
		t.Fatal("expected surprise for state change")
	}
}

func TestFFIBlanketGlue(t *testing.T) {
	blanket := NewFFIBlanket()
	// Two overlapping regions with same cap state: should glue
	fp1 := NewFlightPtr(0x1000, 0x2000, 0x1000, CapData)
	fp2 := NewFlightPtr(0x1800, 0x2800, 0x1800, CapData)
	surprise := blanket.VerifyGlue("a", fp1, "b", fp2)
	if surprise != nil {
		t.Fatalf("expected compatible overlap to glue, got: %v", surprise)
	}

	// Two overlapping regions with different cap states: should not glue
	fp3 := NewFlightPtr(0x1800, 0x2800, 0x1800, CapReadonly)
	surprise = blanket.VerifyGlue("a", fp1, "c", fp3)
	if surprise == nil {
		t.Fatal("expected surprise for incompatible overlap")
	}
	if surprise.Level != Awareness2 {
		t.Fatalf("expected awareness level 2, got %d", surprise.Level)
	}
}

func TestCapTable(t *testing.T) {
	ct := NewCapTable()
	ct.Register("kernel", 0x0, 0x100000, CapReadonly)
	ct.Register("heap", 0x100000, 0x400000, CapData)
	ct.Register("code", 0x500000, 0x100000, CapFunction)

	// Valid heap access
	if err := ct.CheckAccess("heap", 0x200000, 0x100); err != nil {
		t.Fatalf("heap access should succeed: %v", err)
	}

	// OOB heap access
	if err := ct.CheckAccess("heap", 0x500000, 1); err == nil {
		t.Fatal("out-of-bounds heap access should fail")
	}

	// Unknown region
	if err := ct.CheckAccess("nonexistent", 0, 1); err == nil {
		t.Fatal("unknown region should fail")
	}

	// Free heap
	if err := ct.Free("heap"); err != nil {
		t.Fatalf("free should succeed: %v", err)
	}
	if err := ct.CheckAccess("heap", 0x200000, 1); err == nil {
		t.Fatal("access after free should fail")
	}
}

func TestGF3Balance(t *testing.T) {
	// data(+1) + function(-1) + readonly(0) = 0 mod 3
	if !GF3Balance(TritPlus, TritMinus, TritErgodic) {
		t.Fatal("expected balanced triad")
	}
	// data(+1) + data(+1) + data(+1) = 3 = 0 mod 3
	if !GF3Balance(TritPlus, TritPlus, TritPlus) {
		t.Fatal("expected +1+1+1 = 0 mod 3")
	}
	// data(+1) + data(+1) + readonly(0) = 2 != 0 mod 3
	if GF3Balance(TritPlus, TritPlus, TritErgodic) {
		t.Fatal("expected unbalanced triad")
	}
}

func TestCapTableTriadBalance(t *testing.T) {
	ct := NewCapTable()
	ct.Register("heap", 0x100000, 0x400000, CapData)      // +1
	ct.Register("code", 0x500000, 0x100000, CapFunction)   // -1
	ct.Register("rodata", 0x600000, 0x50000, CapReadonly)   //  0

	balanced, err := ct.TriadBalance("heap", "code", "rodata")
	if err != nil {
		t.Fatalf("triad balance check failed: %v", err)
	}
	if !balanced {
		t.Fatal("data(+1) + function(-1) + readonly(0) should be balanced")
	}
}

func TestBlanketStats(t *testing.T) {
	blanket := NewFFIBlanket()
	ptr := NewFlightPtr(0x1000, 0x2000, 0x1000, CapData)

	blanket.SnapshotBefore("a", ptr)
	blanket.VerifyAfter("a", ptr)

	blanket.SnapshotBefore("b", ptr)
	ptr.Cap.Upper = 0x9999
	blanket.VerifyAfter("b", ptr)

	calls, surprises := blanket.Stats()
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	if surprises != 1 {
		t.Fatalf("expected 1 surprise, got %d", surprises)
	}
}
