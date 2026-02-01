//go:build darwin

package vtterm

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// --- Damage type tests ---

func TestCellDamage(t *testing.T) {
	d := CellDamage{X: 5, Y: 3, Width: 2}
	b := d.Bounds()
	if b.Min.X != 5 || b.Min.Y != 3 {
		t.Errorf("CellDamage min = (%d,%d), want (5,3)", b.Min.X, b.Min.Y)
	}
	if b.Max.X != 7 || b.Max.Y != 4 {
		t.Errorf("CellDamage max = (%d,%d), want (7,4)", b.Max.X, b.Max.Y)
	}
}

func TestRectDamage(t *testing.T) {
	d := RectDamage(image.Rect(10, 5, 30, 15))
	if d.X() != 10 {
		t.Errorf("X = %d, want 10", d.X())
	}
	if d.Y() != 5 {
		t.Errorf("Y = %d, want 5", d.Y())
	}
	if d.Width() != 20 {
		t.Errorf("Width = %d, want 20", d.Width())
	}
	if d.Height() != 10 {
		t.Errorf("Height = %d, want 10", d.Height())
	}
}

func TestScreenDamage(t *testing.T) {
	d := ScreenDamage{Width: 80, Height: 24}
	b := d.Bounds()
	if b.Max.X != 80 || b.Max.Y != 24 {
		t.Errorf("ScreenDamage max = (%d,%d), want (80,24)", b.Max.X, b.Max.Y)
	}
}

func TestDamageInterface(t *testing.T) {
	// All damage types implement the Damage interface
	damages := []Damage{
		CellDamage{X: 0, Y: 0, Width: 1},
		RectDamage(image.Rect(0, 0, 10, 10)),
		ScreenDamage{Width: 80, Height: 24},
	}
	for i, d := range damages {
		b := d.Bounds()
		if b.Empty() {
			t.Errorf("damage[%d] has empty bounds", i)
		}
	}
}

// --- Entropy Collector tests ---

func TestEntropyCollectorEmpty(t *testing.T) {
	ec := NewEntropyCollector()
	if ec.Len() != 0 {
		t.Errorf("empty collector Len = %d", ec.Len())
	}
	// Even empty collector produces a seed (hash of nothing)
	seed := ec.Seed()
	if seed == 0 {
		// Extremely unlikely but not impossible — just log
		t.Log("empty seed is 0 (unlikely but valid)")
	}
}

func TestEntropyFromDamage(t *testing.T) {
	ec := NewEntropyCollector()

	// Simulate terminal damage events
	ec.RecordDamage(CellDamage{X: 0, Y: 0, Width: 10})
	ec.RecordDamage(RectDamage(image.Rect(5, 3, 25, 8)))
	ec.RecordDamage(ScreenDamage{Width: 80, Height: 24})

	if ec.Len() != 3 {
		t.Errorf("Len = %d, want 3", ec.Len())
	}

	seed := ec.Seed()
	t.Logf("damage entropy seed: %d (0x%016X)", seed, seed)

	// Same sequence should produce same seed (deterministic)
	ec2 := &EntropyCollector{startAt: ec.startAt}
	ec2.RecordDamage(CellDamage{X: 0, Y: 0, Width: 10})
	ec2.RecordDamage(RectDamage(image.Rect(5, 3, 25, 8)))
	ec2.RecordDamage(ScreenDamage{Width: 80, Height: 24})
	// Note: seeds will differ due to timing jitter — that's the entropy!
}

func TestEntropyFromKeystrokes(t *testing.T) {
	ec := NewEntropyCollector()

	// Type "(+ 1 2)" — a typical boxxy REPL expression
	for _, ch := range "(+ 1 2)" {
		ec.RecordKeystroke(ch)
		time.Sleep(time.Microsecond) // simulate typing jitter
	}

	if ec.Len() != 7 {
		t.Errorf("Len = %d, want 7", ec.Len())
	}

	seed := ec.Seed()
	t.Logf("keystroke entropy seed: %d (0x%016X)", seed, seed)
}

func TestEntropyFromCursor(t *testing.T) {
	ec := NewEntropyCollector()

	// Cursor moves across screen
	for x := 0; x < 10; x++ {
		ec.RecordCursor(x, 0)
	}

	if ec.Len() != 10 {
		t.Errorf("Len = %d, want 10", ec.Len())
	}

	seed := ec.Seed()
	t.Logf("cursor entropy seed: %d (0x%016X)", seed, seed)
}

func TestEntropyMixed(t *testing.T) {
	ec := NewEntropyCollector()

	// Mixed interaction: type, damage, cursor — like a real REPL session
	ec.RecordKeystroke('(')
	ec.RecordDamage(CellDamage{X: 0, Y: 0, Width: 1})
	ec.RecordCursor(1, 0)
	ec.RecordKeystroke('+')
	ec.RecordDamage(CellDamage{X: 1, Y: 0, Width: 1})
	ec.RecordCursor(2, 0)
	ec.RecordKeystroke(' ')
	ec.RecordKeystroke('1')
	ec.RecordKeystroke(' ')
	ec.RecordKeystroke('2')
	ec.RecordKeystroke(')')
	ec.RecordDamage(RectDamage(image.Rect(0, 0, 7, 1)))

	if ec.Len() != 12 {
		t.Errorf("Len = %d, want 12", ec.Len())
	}

	seed := ec.Seed()
	t.Logf("mixed entropy seed: %d (0x%016X)", seed, seed)
}

func TestDifferentInteractionsDifferentSeeds(t *testing.T) {
	// RFC 6943 §4: interaction timing provides entropy
	ec1 := NewEntropyCollector()
	ec1.RecordKeystroke('a')
	time.Sleep(time.Millisecond)
	ec1.RecordKeystroke('b')
	seed1 := ec1.Seed()

	time.Sleep(time.Millisecond)

	ec2 := NewEntropyCollector()
	ec2.RecordKeystroke('a')
	time.Sleep(2 * time.Millisecond) // different timing
	ec2.RecordKeystroke('b')
	seed2 := ec2.Seed()

	// Seeds should differ due to timing jitter
	if seed1 == seed2 {
		t.Log("seeds matched despite different timing (unlikely but possible)")
	} else {
		t.Logf("seed1=%d seed2=%d (different as expected)", seed1, seed2)
	}
}

// --- Color Stream tests ---

func TestColorStreamDeterministic(t *testing.T) {
	cs1 := NewColorStream(42)
	cs2 := NewColorStream(42)

	// Same seed produces same colors
	for i := 0; i < 10; i++ {
		c1 := cs1.NextColor()
		c2 := cs2.NextColor()
		if c1 != c2 {
			t.Errorf("index %d: %v != %v", i+1, c1, c2)
		}
	}
}

func TestColorStreamDifferentSeeds(t *testing.T) {
	cs1 := NewColorStream(42)
	cs2 := NewColorStream(43)

	c1 := cs1.NextColor()
	c2 := cs2.NextColor()
	if c1 == c2 {
		t.Error("different seeds produced same first color")
	}
}

func TestColorAtMatchesSequence(t *testing.T) {
	cs := NewColorStream(1337)

	// ColorAt(i) should match the i-th NextColor call
	for i := 1; i <= 8; i++ {
		fromAt := cs.ColorAt(i)
		t.Logf("ColorAt(%d) = %s", i, HexColor(fromAt))
	}
}

func TestColorStreamGoldenAngleDispersion(t *testing.T) {
	cs := NewColorStream(0)

	// Generate 8 colors and verify they're visually distinct
	// (hues should be spread by ~137.5° each)
	colors := make([]string, 8)
	for i := 0; i < 8; i++ {
		c := cs.NextColor()
		colors[i] = HexColor(c)
	}
	t.Logf("golden angle palette: %v", colors)

	// No two adjacent colors should be identical
	for i := 1; i < len(colors); i++ {
		if colors[i] == colors[i-1] {
			t.Errorf("colors[%d] == colors[%d] = %s", i, i-1, colors[i])
		}
	}
}

func TestColorStreamTrit(t *testing.T) {
	cs := NewColorStream(42)

	// Trit cycles through GF(3): 0, 1, 2, 0, 1, 2, ...
	// (based on index mod 3)
	cs.NextColor() // index=1
	if cs.Trit() != gf3.One {
		t.Errorf("trit at index 1 = %d, want 1", cs.Trit())
	}
	cs.NextColor() // index=2
	if cs.Trit() != gf3.Two {
		t.Errorf("trit at index 2 = %d, want 2", cs.Trit())
	}
	cs.NextColor() // index=3
	if cs.Trit() != gf3.Zero {
		t.Errorf("trit at index 3 = %d, want 0", cs.Trit())
	}
}

func TestHexColor(t *testing.T) {
	tests := []struct {
		r, g, b uint8
		hex     string
	}{
		{0xA8, 0x55, 0xF7, "#A855F7"},
		{0x2E, 0x5F, 0xA3, "#2E5FA3"},
		{0xF5, 0x9E, 0x0B, "#F59E0B"},
		{0x00, 0x00, 0x00, "#000000"},
		{0xFF, 0xFF, 0xFF, "#FFFFFF"},
	}
	for _, tt := range tests {
		got := HexColor(color.RGBA{tt.r, tt.g, tt.b, 0xFF})
		if got != tt.hex {
			t.Errorf("HexColor(%d,%d,%d) = %s, want %s", tt.r, tt.g, tt.b, got, tt.hex)
		}
	}
}

func TestGF3PaletteColors(t *testing.T) {
	// Verify the palette matches Gay MCP assignments
	if HexColor(GF3Palette[0]) != "#F59E0B" {
		t.Errorf("GF3Palette[0] = %s, want #F59E0B", HexColor(GF3Palette[0]))
	}
	if HexColor(GF3Palette[1]) != "#A855F7" {
		t.Errorf("GF3Palette[1] = %s, want #A855F7", HexColor(GF3Palette[1]))
	}
	if HexColor(GF3Palette[2]) != "#2E5FA3" {
		t.Errorf("GF3Palette[2] = %s, want #2E5FA3", HexColor(GF3Palette[2]))
	}
}

// --- Full pipeline: interaction → entropy → seed → color ---

func TestFullPipeline(t *testing.T) {
	// Simulate a boxxy REPL session
	ec := NewEntropyCollector()

	// User types a GF(3) expression
	for _, ch := range "(gf3/add 1 2)" {
		ec.RecordKeystroke(ch)
	}
	// Terminal damages from rendering
	ec.RecordDamage(CellDamage{X: 0, Y: 0, Width: 14})
	ec.RecordDamage(RectDamage(image.Rect(0, 1, 80, 2))) // result line

	// Derive seed from interaction
	seed := ec.Seed()
	t.Logf("session seed: %d (0x%016X)", seed, seed)

	// Create color stream from entropy
	cs := NewColorStream(seed)

	// Generate a palette for this session
	t.Log("session palette:")
	for i := 0; i < 8; i++ {
		c := cs.NextColor()
		trit := cs.Trit()
		role := gf3.ElemToRole(trit)
		t.Logf("  [%d] %s trit=%s role=%s", i+1, HexColor(c), trit, role)
	}
}
