//go:build darwin && arm64

// Package vm's invisicap.go implements Fil-C InvisiCaps as a Go runtime
// for boxxy VM pointer capability tracking.
//
// Fil-C's InvisiCap model: every pointer carries an invisible capability
// (lower bound, upper bound, aux flags) stored outside the C address space.
// Every memory access is checked: ptr >= lower && ptr+size <= upper.
//
// This maps to boxxy's existing architecture:
//   - VM memory regions have capabilities (like InvisiCaps)
//   - Pinhole proxy is the FFI boundary (Markov blanket)
//   - Demon probe is the fuzzer (active inference)
//   - GF(3) trits classify capability types
//
// Perception-Action-Space:
//   Perception = capability snapshot (efference copy before FFI)
//   Action     = VM memory access or pinhole crossing
//   Space      = n-awareness levels (0: exists, 1: preserved, 2: glues, 3: descent)
package vm

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Trit is a GF(3) value for capability classification.
type Trit int8

const (
	TritMinus   Trit = -1 // Function/readonly -- cannot write
	TritErgodic Trit = 0  // Balanced/readonly -- can read
	TritPlus    Trit = 1  // Data -- can read and write
)

func (t Trit) Add(other Trit) Trit {
	return Trit(((int8(t) + int8(other)) % 3 + 3) % 3)
}

func (t Trit) String() string {
	switch t {
	case TritMinus:
		return "MINUS(-1)"
	case TritErgodic:
		return "ERGODIC(0)"
	case TritPlus:
		return "PLUS(+1)"
	default:
		return fmt.Sprintf("TRIT(%d)", t)
	}
}

// CapState is the aux word flag encoding what a capability permits.
type CapState uint8

const (
	CapData     CapState = iota // +1: can read and write
	CapReadonly                 //  0: can read only
	CapFunction                // -1: can call only
	CapFreed                   // dead: upper == lower, all access panics
	CapSpecial                 // runtime internal (threads, jmp_buf)
)

func (cs CapState) Trit() Trit {
	switch cs {
	case CapData:
		return TritPlus
	case CapReadonly:
		return TritErgodic
	case CapFunction:
		return TritMinus
	default:
		return TritErgodic
	}
}

func (cs CapState) CanRead() bool  { return cs == CapData || cs == CapReadonly }
func (cs CapState) CanWrite() bool { return cs == CapData }
func (cs CapState) CanCall() bool  { return cs == CapFunction }

func (cs CapState) String() string {
	switch cs {
	case CapData:
		return "data(+1)"
	case CapReadonly:
		return "readonly(0)"
	case CapFunction:
		return "function(-1)"
	case CapFreed:
		return "freed"
	case CapSpecial:
		return "special"
	default:
		return "unknown"
	}
}

// InvisiCap is the invisible capability attached to every pointer.
// In Fil-C, this is stored outside the C address space (in aux allocation).
// In boxxy, this is stored alongside VM memory region metadata.
type InvisiCap struct {
	Lower  uint64   // Trusted lower bound (points at object header)
	Upper  uint64   // Trusted upper bound
	State  CapState // Aux word flags
	Aux    uint64   // Aux allocation pointer (for pointers-at-rest)
	Freed  bool     // Use-after-free detection
}

// FlightPtr is a pointer in flight (in registers, not stored to memory).
// It has two parts: the trusted Lower (invisible) and the untrusted Intval (visible to C).
type FlightPtr struct {
	Cap    InvisiCap // The invisible capability
	Intval uint64    // The raw integer value visible to the program
}

// NewFlightPtr creates a flight pointer with bounds.
func NewFlightPtr(lower, upper, intval uint64, state CapState) FlightPtr {
	return FlightPtr{
		Cap: InvisiCap{
			Lower: lower,
			Upper: upper,
			State: state,
		},
		Intval: intval,
	}
}

// CheckAccess verifies that accessing size bytes at the pointer's intval
// is within the capability bounds. Returns nil on success, error on violation.
// This is the cut-elimination step: negative ray (access) meets positive ray (capability).
func (fp *FlightPtr) CheckAccess(size uint64) error {
	if fp.Cap.Freed {
		return fmt.Errorf("invisicap: use-after-free at 0x%x (bounds [0x%x, 0x%x])",
			fp.Intval, fp.Cap.Lower, fp.Cap.Upper)
	}
	if fp.Intval < fp.Cap.Lower {
		return fmt.Errorf("invisicap: oob-lower at 0x%x < 0x%x",
			fp.Intval, fp.Cap.Lower)
	}
	if fp.Intval+size > fp.Cap.Upper {
		return fmt.Errorf("invisicap: oob-upper at 0x%x+%d > 0x%x",
			fp.Intval, size, fp.Cap.Upper)
	}
	if size > 0 && !fp.Cap.State.CanRead() {
		return fmt.Errorf("invisicap: read denied on %s capability", fp.Cap.State)
	}
	return nil
}

// CheckWrite verifies write access.
func (fp *FlightPtr) CheckWrite(size uint64) error {
	if err := fp.CheckAccess(size); err != nil {
		return err
	}
	if !fp.Cap.State.CanWrite() {
		return fmt.Errorf("invisicap: write denied on %s capability", fp.Cap.State)
	}
	return nil
}

// Free marks the capability as freed (upper = lower, zero-width bounds).
// All subsequent accesses will panic deterministically.
func (fp *FlightPtr) Free() {
	fp.Cap.Upper = fp.Cap.Lower
	fp.Cap.Freed = true
}

// RestPtr is a pointer at rest (stored in heap memory).
// The intval is in the payload, the capability is in the aux allocation.
// This is the InvisiCap split: the ray divides but the constellation binds them.
type RestPtr struct {
	PayloadOffset uint64    // Where intval is stored in the object
	AuxOffset     uint64    // Where lower is stored in the aux allocation
	Intval        uint64    // The stored intval (visible)
	Cap           InvisiCap // The stored capability (invisible)
}

// Store splits a flight pointer into a rest pointer.
func Store(fp FlightPtr, payloadOff, auxOff uint64) RestPtr {
	return RestPtr{
		PayloadOffset: payloadOff,
		AuxOffset:     auxOff,
		Intval:        fp.Intval,
		Cap:           fp.Cap,
	}
}

// Load reunites a rest pointer into a flight pointer.
func Load(rp RestPtr) FlightPtr {
	return FlightPtr{
		Cap:    rp.Cap,
		Intval: rp.Intval,
	}
}

// CapSnapshot is an efference copy taken before an FFI boundary crossing.
type CapSnapshot struct {
	Lower     uint64
	Upper     uint64
	State     CapState
	Timestamp time.Time
}

// FFIBlanket is the Markov blanket around an FFI call.
// It snapshots capabilities before the call and verifies after.
//
// Perception = snapshot (efference copy)
// Action     = external call
// Comparator = diff before/after
// Space      = n-awareness levels
type FFIBlanket struct {
	mu        sync.Mutex
	snapshots map[string]CapSnapshot // keyed by pointer name
	calls     atomic.Int64
	surprises atomic.Int64
}

// NewFFIBlanket creates a new FFI boundary monitor.
func NewFFIBlanket() *FFIBlanket {
	return &FFIBlanket{
		snapshots: make(map[string]CapSnapshot),
	}
}

// SnapshotBefore takes an efference copy of a pointer's capability.
func (b *FFIBlanket) SnapshotBefore(name string, fp FlightPtr) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.snapshots[name] = CapSnapshot{
		Lower:     fp.Cap.Lower,
		Upper:     fp.Cap.Upper,
		State:     fp.Cap.State,
		Timestamp: time.Now(),
	}
}

// AwarenessLevel is the verification depth after an FFI call.
type AwarenessLevel int

const (
	Awareness0 AwarenessLevel = iota // Did it return?
	Awareness1                       // Capabilities preserved?
	Awareness2                       // Overlapping regions glue?
	Awareness3                       // Full session descent
)

// Surprise is a prediction error detected by the comparator.
type Surprise struct {
	Level    AwarenessLevel
	Name     string
	Expected CapSnapshot
	Actual   CapSnapshot
	Message  string
}

func (s Surprise) String() string {
	return fmt.Sprintf("surprise(level=%d, name=%s): %s", s.Level, s.Name, s.Message)
}

// VerifyAfter checks a pointer's capability against the efference copy.
// Returns nil if no surprise, or a Surprise describing the prediction error.
func (b *FFIBlanket) VerifyAfter(name string, fp FlightPtr) *Surprise {
	b.mu.Lock()
	snap, ok := b.snapshots[name]
	b.mu.Unlock()

	b.calls.Add(1)

	// Level 0: did the call return at all? (if we got here, yes)
	if !ok {
		return &Surprise{
			Level:   Awareness0,
			Name:    name,
			Message: "no efference copy found (pointer not snapshotted before call)",
		}
	}

	actual := CapSnapshot{
		Lower:     fp.Cap.Lower,
		Upper:     fp.Cap.Upper,
		State:     fp.Cap.State,
		Timestamp: time.Now(),
	}

	// Level 1: capabilities preserved?
	if snap.Lower != actual.Lower {
		b.surprises.Add(1)
		return &Surprise{
			Level:    Awareness1,
			Name:     name,
			Expected: snap,
			Actual:   actual,
			Message:  fmt.Sprintf("lower bound changed: 0x%x -> 0x%x", snap.Lower, actual.Lower),
		}
	}
	if snap.Upper != actual.Upper {
		b.surprises.Add(1)
		return &Surprise{
			Level:    Awareness1,
			Name:     name,
			Expected: snap,
			Actual:   actual,
			Message:  fmt.Sprintf("upper bound changed: 0x%x -> 0x%x", snap.Upper, actual.Upper),
		}
	}
	if snap.State != actual.State {
		b.surprises.Add(1)
		return &Surprise{
			Level:    Awareness1,
			Name:     name,
			Expected: snap,
			Actual:   actual,
			Message:  fmt.Sprintf("cap state changed: %s -> %s", snap.State, actual.State),
		}
	}

	return nil // No surprise -- reafference matches efference copy
}

// VerifyGlue checks that two pointers' capabilities are compatible
// at an overlapping region (Level 2 awareness).
func (b *FFIBlanket) VerifyGlue(name1 string, fp1 FlightPtr, name2 string, fp2 FlightPtr) *Surprise {
	// Two regions overlap if lower1 < upper2 AND lower2 < upper1
	overlaps := fp1.Cap.Lower < fp2.Cap.Upper && fp2.Cap.Lower < fp1.Cap.Upper
	if !overlaps {
		return nil // No overlap, no gluing needed
	}

	// Overlapping regions must have compatible capabilities
	if fp1.Cap.State != fp2.Cap.State {
		b.surprises.Add(1)
		return &Surprise{
			Level:   Awareness2,
			Name:    fmt.Sprintf("%s+%s", name1, name2),
			Message: fmt.Sprintf("overlapping regions have incompatible caps: %s vs %s", fp1.Cap.State, fp2.Cap.State),
		}
	}

	return nil
}

// Stats returns the blanket's operational statistics.
func (b *FFIBlanket) Stats() (calls, surprises int64) {
	return b.calls.Load(), b.surprises.Load()
}

// CapRegion tracks a contiguous memory region with a capability.
// This is the VM-level analog of a Fil-C object with its InvisiCap.
type CapRegion struct {
	Name    string
	Base    uint64
	Size    uint64
	State   CapState
	Trit    Trit
	Ptr     FlightPtr
}

// NewCapRegion creates a tracked memory region.
func NewCapRegion(name string, base, size uint64, state CapState) *CapRegion {
	return &CapRegion{
		Name:  name,
		Base:  base,
		Size:  size,
		State: state,
		Trit:  state.Trit(),
		Ptr:   NewFlightPtr(base, base+size, base, state),
	}
}

// CapTable tracks all capability regions in a VM, analogous to Fil-C's
// aux allocation table. The table itself is outside the VM's address space
// (invisible to the guest).
type CapTable struct {
	mu      sync.RWMutex
	regions map[string]*CapRegion
	blanket *FFIBlanket
}

// NewCapTable creates a capability tracking table for a VM.
func NewCapTable() *CapTable {
	return &CapTable{
		regions: make(map[string]*CapRegion),
		blanket: NewFFIBlanket(),
	}
}

// Register adds a memory region to the capability table.
func (ct *CapTable) Register(name string, base, size uint64, state CapState) *CapRegion {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	r := NewCapRegion(name, base, size, state)
	ct.regions[name] = r
	return r
}

// Lookup finds a region by name.
func (ct *CapTable) Lookup(name string) (*CapRegion, bool) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	r, ok := ct.regions[name]
	return r, ok
}

// CheckAccess verifies an access against the capability table.
func (ct *CapTable) CheckAccess(name string, addr, size uint64) error {
	r, ok := ct.Lookup(name)
	if !ok {
		return fmt.Errorf("invisicap: no capability for region %q", name)
	}
	ptr := r.Ptr
	ptr.Intval = addr
	return ptr.CheckAccess(size)
}

// Free marks a region as freed.
func (ct *CapTable) Free(name string) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	r, ok := ct.regions[name]
	if !ok {
		return fmt.Errorf("invisicap: no region %q to free", name)
	}
	r.Ptr.Free()
	r.State = CapFreed
	return nil
}

// Blanket returns the FFI boundary monitor.
func (ct *CapTable) Blanket() *FFIBlanket {
	return ct.blanket
}

// GF3Balance checks that a triad of capability regions sums to zero mod 3.
func GF3Balance(a, b, c Trit) bool {
	sum := a.Add(b).Add(c)
	return sum == TritErgodic
}

// TriadBalance checks a named triad from the cap table.
func (ct *CapTable) TriadBalance(name1, name2, name3 string) (bool, error) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	r1, ok1 := ct.regions[name1]
	r2, ok2 := ct.regions[name2]
	r3, ok3 := ct.regions[name3]
	if !ok1 || !ok2 || !ok3 {
		return false, fmt.Errorf("invisicap: missing region(s) for triad balance")
	}
	return GF3Balance(r1.Trit, r2.Trit, r3.Trit), nil
}
