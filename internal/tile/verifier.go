//go:build darwin

package tile

import (
	"fmt"
	"math"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// MoveOp represents resource state machine transitions.
// Matches the Zig MoveOp in color_winding.zig.
type MoveOp int

const (
	OpMoveTo MoveOp = iota
	OpMoveFrom
	OpBorrowGlobal
	OpBorrowGlobalMut
	OpMint
	OpBurn
	OpTransfer
	OpSetFlag
	OpClearFlag
)

// Trit returns the GF(3) trit for this operation.
func (op MoveOp) Trit() gf3.Elem {
	switch op {
	case OpMoveTo, OpMint, OpSetFlag:
		return gf3.One // plus
	case OpMoveFrom, OpBurn, OpClearFlag:
		return gf3.Two // minus
	default:
		return gf3.Zero // ergodic
	}
}

// ColorLossTracker tracks perceptual color fidelity metrics.
// Pure Go port of key metrics from color_losses.zig.
type ColorLossTracker struct {
	Colors     []Color
	PlusCount  int
	MinusCount int
	ErgoCount  int
	TotalSteps int
	DiffeoSteps int
}

// RecordColor appends a color observation.
func (t *ColorLossTracker) RecordColor(c Color) {
	t.Colors = append(t.Colors, c)
}

// RecordTrit records a GF(3) trit and updates diffeo tracking.
func (t *ColorLossTracker) RecordTrit(trit gf3.Elem) {
	switch trit {
	case gf3.One:
		t.PlusCount++
	case gf3.Two:
		t.MinusCount++
	default:
		t.ErgoCount++
	}
	t.TotalSteps++
	if trit == gf3.One {
		t.DiffeoSteps++
	}
}

// IsBalanced checks if plus and minus counts are equal (winding == 0).
func (t *ColorLossTracker) IsBalanced() bool {
	return t.PlusCount == t.MinusCount
}

// Residue returns the GF(3) residue of the trit stream.
func (t *ColorLossTracker) Residue() int {
	return ((t.PlusCount - t.MinusCount) % 3 + 3) % 3
}

// MeanDeltaE computes mean CIE76 delta-E from the first color.
func (t *ColorLossTracker) MeanDeltaE() float64 {
	if len(t.Colors) < 2 {
		return 0
	}
	ref := toLab(t.Colors[0])
	total := 0.0
	for _, c := range t.Colors[1:] {
		lab := toLab(c)
		dL := ref[0] - lab[0]
		da := ref[1] - lab[1]
		db := ref[2] - lab[2]
		total += math.Sqrt(dL*dL + da*da + db*db)
	}
	return total / float64(len(t.Colors)-1)
}

// lab is a simple CIE L*a*b* triple.
func toLab(c Color) [3]float64 {
	// sRGB → linear
	r := gammaExpand(float64(c.R) / 255.0)
	g := gammaExpand(float64(c.G) / 255.0)
	b := gammaExpand(float64(c.B) / 255.0)
	// linear RGB → XYZ (D65)
	x := 0.4124564*r + 0.3575761*g + 0.1804375*b
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := 0.0193339*r + 0.1191920*g + 0.9503041*b
	// XYZ → Lab
	xn, yn, zn := 0.95047, 1.0, 1.08883
	L := 116.0*labF(y/yn) - 16.0
	a := 500.0 * (labF(x/xn) - labF(y/yn))
	bv := 200.0 * (labF(y/yn) - labF(z/zn))
	return [3]float64{L, a, bv}
}

func gammaExpand(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func labF(t float64) float64 {
	if t > 0.008856 {
		return math.Cbrt(t)
	}
	return 7.787*t + 16.0/116.0
}

// TileVerifier provides unified verification for a tileable VM instance.
// Tracks resource lifecycle (winding via trit counts) and color fidelity
// from a single seed-derived identity.
// Pure Go port of TileVerifier from color_winding.zig.
type TileVerifier struct {
	Seed          uint64
	IdentityColor Color
	Losses        ColorLossTracker
	Transitions   uint64
}

// NewTileVerifier creates a verifier from a seed.
func NewTileVerifier(seed uint64) *TileVerifier {
	c := ColorFromSeed(seed)
	v := &TileVerifier{
		Seed:          seed,
		IdentityColor: c,
	}
	v.Losses.RecordColor(c)
	return v
}

// RecordTransition records a lifecycle transition with a color observation.
func (v *TileVerifier) RecordTransition(op MoveOp, observed Color) {
	v.Losses.RecordTrit(op.Trit())
	v.Losses.RecordColor(observed)
	v.Transitions++
}

// RecordOp records a lifecycle transition without color observation.
func (v *TileVerifier) RecordOp(op MoveOp) {
	v.Losses.RecordTrit(op.Trit())
	v.Transitions++
}

// IsLifecycleBalanced checks if plus and minus counts are equal.
func (v *TileVerifier) IsLifecycleBalanced() bool {
	return v.Losses.IsBalanced()
}

// IsDiffeomorphic checks if the color trajectory is orientation-preserving.
func (v *TileVerifier) IsDiffeomorphic() bool {
	if v.Losses.TotalSteps == 0 {
		return true
	}
	return v.Losses.DiffeoSteps > 0
}

// MeanDeltaE returns mean perceptual color difference from identity.
func (v *TileVerifier) MeanDeltaE() float64 {
	return v.Losses.MeanDeltaE()
}

// IsVerified checks all three verification conditions:
//  1. Resource lifecycle is balanced (winding == 0)
//  2. Color trajectory is orientation-preserving
//  3. GF(3) trit residue is zero
func (v *TileVerifier) IsVerified() bool {
	return v.IsLifecycleBalanced() && v.IsDiffeomorphic() && v.Losses.Residue() == 0
}

// Winding returns the winding number from embedded trit counts.
func (v *TileVerifier) Winding() int {
	return (v.Losses.PlusCount - v.Losses.MinusCount) / 3
}

// Report returns a compact verification summary.
func (v *TileVerifier) Report() string {
	diffeo := "ok"
	if !v.IsDiffeomorphic() {
		diffeo = "FLIP"
	}
	result := "PASS"
	if !v.IsVerified() {
		result = "FAIL"
	}
	return fmt.Sprintf("seed=%d wind=%d res=%d de=%.1f traj=%d diffeo=%s %s",
		v.Seed, v.Winding(), v.Losses.Residue(), v.MeanDeltaE(),
		len(v.Losses.Colors), diffeo, result)
}

// MultiTileVerifier tracks N tiles simultaneously and checks global conservation.
type MultiTileVerifier struct {
	Tiles map[uint64]*TileVerifier
}

// NewMultiTileVerifier creates an empty multi-tile verifier.
func NewMultiTileVerifier() *MultiTileVerifier {
	return &MultiTileVerifier{Tiles: make(map[uint64]*TileVerifier)}
}

// AddTile adds or retrieves a tile by seed.
func (m *MultiTileVerifier) AddTile(seed uint64) *TileVerifier {
	if tv, ok := m.Tiles[seed]; ok {
		return tv
	}
	tv := NewTileVerifier(seed)
	m.Tiles[seed] = tv
	return tv
}

// IsGloballyConserved checks if total plus == total minus across all tiles.
func (m *MultiTileVerifier) IsGloballyConserved() bool {
	totalPlus, totalMinus := 0, 0
	for _, tv := range m.Tiles {
		totalPlus += tv.Losses.PlusCount
		totalMinus += tv.Losses.MinusCount
	}
	return totalPlus == totalMinus
}

// AllVerified checks if every tile passes verification.
func (m *MultiTileVerifier) AllVerified() bool {
	for _, tv := range m.Tiles {
		if !tv.IsVerified() {
			return false
		}
	}
	return true
}
