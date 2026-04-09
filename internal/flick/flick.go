//go:build darwin

// Package flick implements the Flick time unit (1/705600000 second) with
// GF(3) trit-tick extension for polarity-tagged frame timing.
//
// A Flick is the smallest time unit that evenly divides all standard
// frame rates (24, 25, 30, 48, 50, 60, 90, 100, 120, 144 Hz).
// 705600000 = 2^9 × 3^2 × 5^5 × 7^2
//
// A TritTick is a Flick tagged with a GF(3) trit, encoding both
// WHEN (temporal position) and WHAT POLARITY (trit value).
package flick

import "github.com/bmorphism/boxxy/internal/gf3"

// Flick is a time unit equal to 1/705600000 of a second.
type Flick int64

// FlicksPerSecond is the number of flicks in one second.
const FlicksPerSecond Flick = 705600000

// Flicks-per-frame constants for standard frame rates.
const (
	FlicksPer24  Flick = FlicksPerSecond / 24  // 29400000
	FlicksPer25  Flick = FlicksPerSecond / 25  // 28224000
	FlicksPer30  Flick = FlicksPerSecond / 30  // 23520000
	FlicksPer48  Flick = FlicksPerSecond / 48  // 14700000
	FlicksPer50  Flick = FlicksPerSecond / 50  // 14112000
	FlicksPer60  Flick = FlicksPerSecond / 60  // 11760000
	FlicksPer90  Flick = FlicksPerSecond / 90  // 7840000
	FlicksPer100 Flick = FlicksPerSecond / 100 // 7056000
	FlicksPer120 Flick = FlicksPerSecond / 120 // 5880000
	FlicksPer144 Flick = FlicksPerSecond / 144 // 4900000
)

// FromHz returns the number of flicks per frame for a given frame rate.
// Panics if hz is zero.
func FromHz(hz int) Flick {
	if hz == 0 {
		panic("flick: zero Hz")
	}
	return FlicksPerSecond / Flick(hz)
}

// ToSeconds converts flicks to seconds as a float64.
func (f Flick) ToSeconds() float64 {
	return float64(f) / float64(FlicksPerSecond)
}

// ToMillis converts flicks to milliseconds as a float64.
func (f Flick) ToMillis() float64 {
	return f.ToSeconds() * 1000.0
}

// TritTick is a Flick tagged with a GF(3) trit, encoding both temporal
// position (When) and polarity (Trit).
type TritTick struct {
	When Flick
	Trit gf3.Elem
}

// NewTritTick constructs a TritTick from a flick position and GF(3) element.
func NewTritTick(when Flick, trit gf3.Elem) TritTick {
	return TritTick{When: when, Trit: trit}
}

// FrameTriad generates the three trit-ticks for one frame duration,
// placing Plus, Ergodic, and Minus at equal sub-frame offsets (0/3, 1/3, 2/3).
func FrameTriad(frameFlick Flick) [3]TritTick {
	third := frameFlick / 3
	return [3]TritTick{
		{When: 0, Trit: gf3.One},            // Plus  at offset 0
		{When: third, Trit: gf3.Zero},        // Ergodic at offset 1/3
		{When: 2 * third, Trit: gf3.Two},     // Minus at offset 2/3
	}
}

// IsBalancedFrame checks GF(3) conservation within a frame: the sum of
// the three trit values must be 0 (mod 3).
func IsBalancedFrame(ticks [3]TritTick) bool {
	return gf3.IsBalanced([]gf3.Elem{ticks[0].Trit, ticks[1].Trit, ticks[2].Trit})
}

// ChainParity computes Möbius parity (-1)^len for a chain of frame durations,
// returned as a GF(3) balanced trit: Plus (+1) for even length, Minus (-1)
// for odd length.
func ChainParity(frames []Flick) gf3.BalancedTrit {
	if len(frames)%2 == 0 {
		return gf3.Plus
	}
	return gf3.Minus
}
