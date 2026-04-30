//go:build darwin

// Package tile provides color-dispatched tileable VMs.
//
// Each VM gets a deterministic color identity from a seed via SplitMix64,
// the same algorithm used in zig-syrup's gay/splitmix.zig. That color:
//   1. Identifies the VM in the tile lattice
//   2. Encodes as zig-syrup compatible <'gay:color r g b> wire format
//   3. Seeds the golden-angle rainbow palette for that VM's REPL parens
//
// The referential chain: seed → splitmix64 → Color → Syrup wire bytes → paren palette
package tile

import (
	"fmt"
	"math"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/bmorphism/boxxy/internal/color"
	"github.com/bmorphism/boxxy/internal/gf3"
	"github.com/bmorphism/boxxy/internal/vm"
)

// SplitMix64 constants — must match zig-syrup/src/gay/splitmix.zig exactly.
const (
	smGolden = 0x9e3779b97f4a7c15
	smMix1   = 0xbf58476d1ce4e5b9
	smMix2   = 0x94d049bb133111eb
)

// splitmix64 is a bijective hash — same algorithm as Gay MCP / zig-syrup.
func splitmix64(state uint64) (next uint64, value uint64) {
	state += smGolden
	z := state
	z = (z ^ (z >> 30)) * smMix1
	z = (z ^ (z >> 27)) * smMix2
	z = z ^ (z >> 31)
	return state, z
}

// Color is an RGB color derived from a seed — mirrors zig-syrup Color struct.
type Color struct {
	R uint8
	G uint8
	B uint8
}

// ColorFromSeed generates a deterministic color from a seed using SplitMix64.
// Matches zig-syrup/src/gay/splitmix.zig Color.fromSeed().
func ColorFromSeed(seed uint64) Color {
	_, val := splitmix64(seed)
	return Color{
		R: uint8(val & 0xFF),
		G: uint8((val >> 8) & 0xFF),
		B: uint8((val >> 16) & 0xFF),
	}
}

// Hex returns the CSS hex string, e.g. "#A855F7".
func (c Color) Hex() string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// Hue returns the hue in degrees [0, 360) for seeding the rainbow palette.
func (c Color) Hue() float64 {
	col := colorful.Color{
		R: float64(c.R) / 255.0,
		G: float64(c.G) / 255.0,
		B: float64(c.B) / 255.0,
	}
	h, _, _ := col.Hcl()
	if math.IsNaN(h) {
		return 0
	}
	return h
}

// Trit returns the GF(3) trit for this color: (R + G + B) mod 3.
// Matches the convention in zig-syrup/src/tileable_cc.zig.
func (c Color) Trit() gf3.Elem {
	sum := int(c.R) + int(c.G) + int(c.B)
	return gf3.Elem(((sum % 3) + 3) % 3)
}

// ColorIdentity bundles a VM's seed, derived color, trit, and wire encoding.
type ColorIdentity struct {
	Seed    uint64
	Color   Color
	Trit    gf3.Elem
	Role    gf3.SkillRole
	HexCode string
}

// NewColorIdentity creates a full identity from a seed.
func NewColorIdentity(seed uint64) ColorIdentity {
	c := ColorFromSeed(seed)
	trit := c.Trit()
	return ColorIdentity{
		Seed:    seed,
		Color:   c,
		Trit:    trit,
		Role:    gf3.ElemToRole(trit),
		HexCode: c.Hex(),
	}
}

// RainbowPalette returns the golden-angle palette seeded from this identity's hue.
func (ci ColorIdentity) RainbowPalette(depth int) []colorful.Color {
	return color.RainbowPalette(depth, ci.Color.Hue(), 0.7, 0.55)
}

// --- Syrup Wire Encoding ---

// EncodeSyrupColor encodes the color in zig-syrup compatible format:
//
//	<'gay:color R G B>
//
// where R, G, B are Syrup integers (length-prefixed).
// This matches zig-syrup/src/gay/serialization.zig encodeColor().
func (ci ColorIdentity) EncodeSyrupColor() []byte {
	var buf []byte
	// Record open
	buf = append(buf, '<')
	// Symbol: gay:color
	sym := "gay:color"
	buf = append(buf, syrupEncodeSymbol(sym)...)
	// Three integer fields: R, G, B
	buf = append(buf, syrupEncodeInt(int64(ci.Color.R))...)
	buf = append(buf, syrupEncodeInt(int64(ci.Color.G))...)
	buf = append(buf, syrupEncodeInt(int64(ci.Color.B))...)
	// Record close
	buf = append(buf, '>')
	return buf
}

// EncodeSyrupCheckpoint encodes a full checkpoint record:
//
//	<'gay:ckpt seed worker_id invocation_count>
func (ci ColorIdentity) EncodeSyrupCheckpoint(workerID, invocationCount uint64) []byte {
	var buf []byte
	buf = append(buf, '<')
	buf = append(buf, syrupEncodeSymbol("gay:ckpt")...)
	buf = append(buf, syrupEncodeInt(int64(ci.Seed))...)
	buf = append(buf, syrupEncodeInt(int64(workerID))...)
	buf = append(buf, syrupEncodeInt(int64(invocationCount))...)
	buf = append(buf, '>')
	return buf
}

// EncodeMessageFrame wraps Syrup payload in a 4-byte big-endian length prefix,
// matching zig-syrup/src/message_frame.zig.
func EncodeMessageFrame(payload []byte) []byte {
	n := uint32(len(payload))
	frame := make([]byte, 4+len(payload))
	frame[0] = byte(n >> 24)
	frame[1] = byte(n >> 16)
	frame[2] = byte(n >> 8)
	frame[3] = byte(n)
	copy(frame[4:], payload)
	return frame
}

// syrupEncodeSymbol encodes a Syrup symbol: len'sym
func syrupEncodeSymbol(s string) []byte {
	prefix := fmt.Sprintf("%d'", len(s))
	return append([]byte(prefix), []byte(s)...)
}

// syrupEncodeInt encodes a Syrup integer: digits+
func syrupEncodeInt(n int64) []byte {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return []byte(s + "+")
	}
	return []byte(s + "+")
}

// --- Tileable VM ---

// TileableVM wraps a boxxy VM instance with its color identity.
type TileableVM struct {
	Identity ColorIdentity
	Name     string
	VMConfig vm.Config
	Instance *vm.VMInstance
}

// NewTileableVM creates a tile-ready VM with a color identity derived from seed.
func NewTileableVM(name string, seed uint64, cfg vm.Config) *TileableVM {
	return &TileableVM{
		Identity: NewColorIdentity(seed),
		Name:     name,
		VMConfig: cfg,
	}
}

// Start creates and starts the underlying VM.
func (t *TileableVM) Start() error {
	instance, err := vm.CreateVM(t.VMConfig)
	if err != nil {
		return fmt.Errorf("tile %s [%s]: create: %w", t.Name, t.Identity.HexCode, err)
	}
	t.Instance = instance
	vm.RegisterVM(t.Name, instance)
	if err := vm.StartVM(instance); err != nil {
		return fmt.Errorf("tile %s [%s]: start: %w", t.Name, t.Identity.HexCode, err)
	}
	return nil
}

// --- Tile Lattice ---

// TileLattice manages a set of tileable VMs and enforces GF(3) balance.
type TileLattice struct {
	Tiles []*TileableVM
}

// NewTileLattice creates an empty lattice.
func NewTileLattice() *TileLattice {
	return &TileLattice{}
}

// Add registers a tile in the lattice.
func (tl *TileLattice) Add(t *TileableVM) {
	tl.Tiles = append(tl.Tiles, t)
}

// IsBalanced checks if the lattice's trits sum to 0 mod 3.
func (tl *TileLattice) IsBalanced() bool {
	elems := make([]gf3.Elem, len(tl.Tiles))
	for i, t := range tl.Tiles {
		elems[i] = t.Identity.Trit
	}
	return gf3.IsBalanced(elems)
}

// FindBalancerSeed brute-searches for a seed whose trit would balance the lattice.
// Returns 0 if already balanced. Searches up to maxSearch seeds starting from start.
func (tl *TileLattice) FindBalancerSeed(start uint64, maxSearch int) (uint64, bool) {
	if tl.IsBalanced() {
		return 0, true
	}
	elems := make([]gf3.Elem, len(tl.Tiles))
	for i, t := range tl.Tiles {
		elems[i] = t.Identity.Trit
	}
	partial := 0
	for _, e := range elems {
		partial += int(e)
	}
	needed := gf3.Elem(((3 - (partial % 3)) % 3 + 3) % 3)

	for i := 0; i < maxSearch; i++ {
		candidate := start + uint64(i)
		ci := NewColorIdentity(candidate)
		if ci.Trit == needed {
			return candidate, true
		}
	}
	return 0, false
}

// WireColors returns all tile colors encoded as Syrup records.
func (tl *TileLattice) WireColors() [][]byte {
	result := make([][]byte, len(tl.Tiles))
	for i, t := range tl.Tiles {
		result[i] = t.Identity.EncodeSyrupColor()
	}
	return result
}

// --- Triangulation: 3-party behavioral distance with triangle inequality ---

// BehaviorTrace records the observable trace of a computation.
// In the REBEL sense, this is the compositional primitive: the sequence of
// operations that can be matched across implementations via IR similarity.
type BehaviorTrace struct {
	Seed       uint64
	CallCount  int
	MemoHits   int
	MemoMisses int
	TraceHash  uint64 // SplitMix64 of the call sequence
}

// NewBehaviorTrace creates a trace for a seed.
func NewBehaviorTrace(seed uint64) *BehaviorTrace {
	return &BehaviorTrace{Seed: seed, TraceHash: seed}
}

// Record adds a call to the trace, advancing the hash.
func (bt *BehaviorTrace) Record(hit bool) {
	bt.CallCount++
	if hit {
		bt.MemoHits++
	} else {
		bt.MemoMisses++
	}
	// Advance trace hash — same as SplitMix64 step
	_, val := splitmix64(bt.TraceHash + uint64(bt.CallCount))
	bt.TraceHash = val
}

// BehavioralDistance computes the distance between two traces.
// Distance = Hamming-like metric on memoization patterns.
// When two programs have identical memo hit/miss sequences, d=0,
// which is what REBEL's IRSimilarityIdentifier would surface.
func BehavioralDistance(a, b *BehaviorTrace) float64 {
	if a.CallCount == 0 && b.CallCount == 0 {
		return 0
	}
	// Ratio of memo hits as proxy for structural similarity
	maxCalls := a.CallCount
	if b.CallCount > maxCalls {
		maxCalls = b.CallCount
	}
	if maxCalls == 0 {
		return 0
	}
	hitDiff := math.Abs(float64(a.MemoHits) - float64(b.MemoHits))
	countDiff := math.Abs(float64(a.CallCount) - float64(b.CallCount))
	hashDiff := float64(0)
	if a.TraceHash != b.TraceHash {
		hashDiff = 1.0
	}
	return (hitDiff + countDiff + hashDiff) / float64(3*maxCalls+1)
}

// Triangulation holds three parties and checks triangle inequality.
type Triangulation struct {
	A, B, C *BehaviorTrace
	DAB     float64 // d(A,B)
	DBC     float64 // d(B,C)
	DAC     float64 // d(A,C)
}

// NewTriangulation computes all pairwise distances.
func NewTriangulation(a, b, c *BehaviorTrace) Triangulation {
	return Triangulation{
		A:   a,
		B:   b,
		C:   c,
		DAB: BehavioralDistance(a, b),
		DBC: BehavioralDistance(b, c),
		DAC: BehavioralDistance(a, c),
	}
}

// StrongTriangleInequality checks d(A,C) <= d(A,B) + d(B,C).
func (t Triangulation) StrongTriangleInequality() bool {
	return t.DAC <= t.DAB+t.DBC+1e-10
}

// WeakTriangleInequality (ultrametric) checks d(A,C) <= max(d(A,B), d(B,C)).
func (t Triangulation) WeakTriangleInequality() bool {
	maxDist := t.DAB
	if t.DBC > maxDist {
		maxDist = t.DBC
	}
	return t.DAC <= maxDist+1e-10
}

// IsSemanticallyEquivalent returns true if all three have zero distance,
// meaning they are IR-identical compositional primitives (REBEL match).
func (t Triangulation) IsSemanticallyEquivalent() bool {
	return t.DAB < 1e-10 && t.DBC < 1e-10 && t.DAC < 1e-10
}
