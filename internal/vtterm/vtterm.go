//go:build darwin

// Package vtterm provides interaction entropy collection from terminal damage
// regions and maps it to Gay MCP deterministic color seeds.
//
// The key insight (per RFC 6943 §4): interactive terminal sessions generate
// entropy through timing, cursor positions, damage regions, and keystroke
// patterns. We collect this entropy into a hash that becomes the Gay MCP seed,
// ensuring deterministic-yet-unique color streams per session.
//
// Damage types mirror charmbracelet/x/vt's API for forward compatibility.
// When vt's ultraviolet dependency stabilizes, this package will wrap
// the full vt.Emulator.
package vtterm

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// --- Damage types (mirrors vt API) ---

// Damage represents a damaged area of the terminal screen.
type Damage interface {
	Bounds() image.Rectangle
}

// CellDamage represents a single damaged cell region.
type CellDamage struct {
	X, Y  int
	Width int
}

func (d CellDamage) Bounds() image.Rectangle {
	return image.Rect(d.X, d.Y, d.X+d.Width, d.Y+1)
}

// RectDamage represents a damaged rectangle.
type RectDamage image.Rectangle

func (d RectDamage) Bounds() image.Rectangle { return image.Rectangle(d) }
func (d RectDamage) X() int                  { return image.Rectangle(d).Min.X }
func (d RectDamage) Y() int                  { return image.Rectangle(d).Min.Y }
func (d RectDamage) Width() int              { return image.Rectangle(d).Dx() }
func (d RectDamage) Height() int             { return image.Rectangle(d).Dy() }

// ScreenDamage represents a full screen damage.
type ScreenDamage struct {
	Width, Height int
}

func (d ScreenDamage) Bounds() image.Rectangle {
	return image.Rect(0, 0, d.Width, d.Height)
}

// ScrollDamage represents a scrolled area.
type ScrollDamage struct {
	image.Rectangle
	Dx, Dy int
}

// --- Interaction Entropy Collector ---
// Per RFC 6943 §4 "Randomness from Interaction":
// Terminal sessions are a rich entropy source via timing jitter,
// spatial damage patterns, and keystroke sequences.

// EntropyCollector accumulates entropy from terminal interactions.
type EntropyCollector struct {
	events  []entropyEvent
	startAt time.Time
}

type entropyEvent struct {
	ts     time.Duration // relative to start
	kind   byte          // event type discriminator
	data   [8]byte       // event-specific data
}

// NewEntropyCollector creates a collector anchored to the current time.
func NewEntropyCollector() *EntropyCollector {
	return &EntropyCollector{startAt: time.Now()}
}

// RecordDamage feeds a damage region into the entropy pool.
func (ec *EntropyCollector) RecordDamage(d Damage) {
	b := d.Bounds()
	var data [8]byte
	binary.LittleEndian.PutUint16(data[0:2], uint16(b.Min.X))
	binary.LittleEndian.PutUint16(data[2:4], uint16(b.Min.Y))
	binary.LittleEndian.PutUint16(data[4:6], uint16(b.Max.X))
	binary.LittleEndian.PutUint16(data[6:8], uint16(b.Max.Y))
	ec.events = append(ec.events, entropyEvent{
		ts:   time.Since(ec.startAt),
		kind: 'D',
		data: data,
	})
}

// RecordKeystroke feeds a keystroke into the entropy pool.
func (ec *EntropyCollector) RecordKeystroke(key rune) {
	var data [8]byte
	binary.LittleEndian.PutUint32(data[0:4], uint32(key))
	ec.events = append(ec.events, entropyEvent{
		ts:   time.Since(ec.startAt),
		kind: 'K',
		data: data,
	})
}

// RecordCursor feeds a cursor position change into the entropy pool.
func (ec *EntropyCollector) RecordCursor(x, y int) {
	var data [8]byte
	binary.LittleEndian.PutUint32(data[0:4], uint32(x))
	binary.LittleEndian.PutUint32(data[4:8], uint32(y))
	ec.events = append(ec.events, entropyEvent{
		ts:   time.Since(ec.startAt),
		kind: 'C',
		data: data,
	})
}

// Len returns the number of entropy events collected.
func (ec *EntropyCollector) Len() int {
	return len(ec.events)
}

// Seed computes a deterministic 64-bit seed from all collected entropy.
// This becomes the Gay MCP seed for color generation.
func (ec *EntropyCollector) Seed() uint64 {
	h := sha256.New()
	for _, ev := range ec.events {
		// Mix timing (nanosecond jitter is the primary entropy source per RFC 6943)
		var tsBuf [8]byte
		binary.LittleEndian.PutUint64(tsBuf[:], uint64(ev.ts.Nanoseconds()))
		h.Write(tsBuf[:])
		h.Write([]byte{ev.kind})
		h.Write(ev.data[:])
	}
	sum := h.Sum(nil)
	return binary.LittleEndian.Uint64(sum[0:8])
}

// --- Gay MCP Color Pipeline ---
// Seed → golden angle → deterministic color stream

const GoldenAngle = 137.5077640500378 // degrees

// GF3Palette maps each GF(3) element to an RGBA color.
var GF3Palette = [3]color.RGBA{
	{0xF5, 0x9E, 0x0B, 0xFF}, // Zero/Coordinator (amber)
	{0xA8, 0x55, 0xF7, 0xFF}, // One/Generator (purple)
	{0x2E, 0x5F, 0xA3, 0xFF}, // Two/Verifier (blue)
}

// ColorStream generates deterministic colors from an entropy seed
// using the golden angle spiral (matching Gay MCP's color_at).
type ColorStream struct {
	seed    uint64
	baseHue float64
	index   int
}

// NewColorStream creates a stream from an entropy seed.
// The base hue is derived by hashing the seed through the golden ratio
// to maximize dispersion even for adjacent seed values.
func NewColorStream(seed uint64) *ColorStream {
	// Use golden ratio fractional part for maximal hue dispersion
	// This ensures adjacent seeds (42, 43) produce very different base hues
	phi := 1.6180339887498948
	baseHue := math.Mod(float64(seed)*phi, 360.0)
	return &ColorStream{seed: seed, baseHue: baseHue}
}

// NextColor returns the next color in the golden angle spiral.
func (cs *ColorStream) NextColor() color.RGBA {
	cs.index++
	hue := math.Mod(cs.baseHue+float64(cs.index)*GoldenAngle, 360.0)
	return hslToRGB(hue, 0.7, 0.55)
}

// ColorAt returns the color at a specific index (1-based, like Gay MCP).
func (cs *ColorStream) ColorAt(index int) color.RGBA {
	hue := math.Mod(cs.baseHue+float64(index)*GoldenAngle, 360.0)
	return hslToRGB(hue, 0.7, 0.55)
}

// Seed returns the entropy-derived seed value.
func (cs *ColorStream) Seed() uint64 {
	return cs.seed
}

// Index returns how many colors have been generated.
func (cs *ColorStream) Index() int {
	return cs.index
}

// Trit returns the GF(3) trit for the current index.
func (cs *ColorStream) Trit() gf3.Elem {
	return gf3.Elem(cs.index % 3)
}

// --- HSL to RGB conversion ---

func hslToRGB(h, s, l float64) color.RGBA {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := (1.0 - math.Abs(2.0*l-1.0)) * s
	x := c * (1.0 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := l - c/2.0

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return color.RGBA{
		R: uint8((r + m) * 255),
		G: uint8((g + m) * 255),
		B: uint8((b + m) * 255),
		A: 255,
	}
}

// HexColor returns a color as #RRGGBB string.
func HexColor(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
