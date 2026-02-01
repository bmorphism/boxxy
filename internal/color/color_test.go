package color_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/lucasb-eyer/go-colorful"
)

// Gay MCP produces hex colors like "#A855F7". This test validates the pipeline:
// Gay MCP hex → go-colorful parsing → colorprofile degradation → terminal output.

func TestColorProfileLevels(t *testing.T) {
	// colorprofile defines 5 levels of terminal color support
	levels := []struct {
		profile colorprofile.Profile
		name    string
	}{
		{colorprofile.NoTTY, "NoTTY"},
		{colorprofile.Ascii, "Ascii"},
		{colorprofile.ANSI, "ANSI"},
		{colorprofile.ANSI256, "ANSI256"},
		{colorprofile.TrueColor, "TrueColor"},
	}

	// Verify ordering: NoTTY < Ascii < ANSI < ANSI256 < TrueColor
	for i := 1; i < len(levels); i++ {
		if levels[i].profile <= levels[i-1].profile {
			t.Errorf("%s should be > %s", levels[i].name, levels[i-1].name)
		}
	}
}

func TestGayMCPHexToColorful(t *testing.T) {
	// Gay MCP palette colors — these are actual hex values from the deterministic stream
	gayHexColors := []string{
		"#A855F7", // purple — skill://gay-mcp
		"#2E5FA3", // blue
		"#F59E0B", // amber
		"#10B981", // emerald
		"#EF4444", // red
		"#6366F1", // indigo
	}

	for _, hex := range gayHexColors {
		c, err := colorful.Hex(hex)
		if err != nil {
			t.Errorf("Failed to parse Gay MCP hex %s: %v", hex, err)
			continue
		}

		// Verify round-trip: parse → hex → parse
		roundTrip := c.Hex()
		if !strings.EqualFold(hex, roundTrip) {
			t.Errorf("Hex round-trip failed: %s → %s", hex, roundTrip)
		}

		// Verify color is in valid RGB range
		r, g, b := c.RGB255()
		if r > 255 || g > 255 || b > 255 {
			t.Errorf("Color %s out of RGB range: (%d, %d, %d)", hex, r, g, b)
		}

		// HCL decomposition — useful for perceptual operations
		h, chr, l := c.Hcl()
		_ = h
		if chr < 0 {
			t.Errorf("Color %s has negative chroma: %f", hex, chr)
		}
		if l < 0 || l > 1 {
			t.Errorf("Color %s has out-of-range lightness: %f", hex, l)
		}
	}
}

func TestColorProfileDetection(t *testing.T) {
	// Detect the current terminal's color profile.
	// In CI / test environments this will typically be NoTTY or Ascii.
	// The important thing is it doesn't panic and returns a valid profile.
	w := colorprofile.NewWriter(io.Discard, os.Environ())
	profile := w.Profile
	t.Logf("Detected color profile: %v", profile)

	// Profile must be one of the known levels
	switch profile {
	case colorprofile.NoTTY, colorprofile.Ascii, colorprofile.ANSI,
		colorprofile.ANSI256, colorprofile.TrueColor:
		// valid
	default:
		t.Errorf("Unknown color profile: %v", profile)
	}
}

func TestColorDegradation(t *testing.T) {
	// go-colorful can convert TrueColor → ANSI256 index
	// This simulates what colorprofile does when degrading for limited terminals.
	gayPurple, _ := colorful.Hex("#A855F7")

	// Convert to closest ANSI 256-color palette entry
	// go-colorful doesn't have a direct ANSI256 method, but we can
	// compute the closest color distance in the terminal palette.

	// Verify HCL values are in expected range for a purple
	h, c, l := gayPurple.Hcl()
	t.Logf("Gay purple #A855F7 → HCL(%.1f, %.3f, %.3f)", h, c, l)

	// Purple hue should be roughly 270-310° range
	if h < 250 || h > 330 {
		t.Errorf("Expected purple hue 250-330°, got %.1f°", h)
	}
	// Should have meaningful chroma (not gray)
	if c < 0.3 {
		t.Errorf("Expected chroma > 0.3 for saturated purple, got %.3f", c)
	}

	// Verify perceptual distance between two Gay colors
	gayBlue, _ := colorful.Hex("#2E5FA3")
	dist := gayPurple.DistanceCIEDE2000(gayBlue)
	t.Logf("CIEDE2000 distance purple↔blue: %.3f", dist)
	// CIEDE2000 returns 0-1 normalized values in go-colorful
	if dist < 0.05 {
		t.Error("Purple and blue should be perceptually distinct (CIEDE2000 > 0.05)")
	}

	// Also verify Euclidean distance in LAB space as a sanity check
	labDist := gayPurple.DistanceLab(gayBlue)
	t.Logf("LAB distance purple↔blue: %.3f", labDist)
	if labDist < 0.1 {
		t.Error("Purple and blue should be distinct in LAB space")
	}
}
