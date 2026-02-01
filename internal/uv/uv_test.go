package uv_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/colors"
	"github.com/lucasb-eyer/go-colorful"
)

// Ultraviolet is Charmbracelet's low-level TUI primitives library,
// powering Bubble Tea v2 and Lip Gloss v2. It's currently pre-v1
// with cellbuf API instability, so we test via the stable layers:
// x/colors (adaptive palette), lipgloss (styling), go-colorful (perceptual).

func TestXColorsAdaptivePalette(t *testing.T) {
	// x/colors provides AdaptiveColor pairs that switch between
	// light and dark terminal backgrounds.
	testCases := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"Indigo", colors.Indigo},
		{"IndigoDim", colors.IndigoDim},
		{"IndigoSubtle", colors.IndigoSubtle},
		{"Fuschia", colors.Fuschia},
		{"FuchsiaDim", colors.FuchsiaDim},
		{"Red", colors.Red},
		{"RedDull", colors.RedDull},
		{"Gray", colors.Gray},
		{"GrayDark", colors.GrayDark},
		{"Normal", colors.Normal},
		{"NormalDim", colors.NormalDim},
		{"YellowGreen", colors.YellowGreen},
		{"YellowGreenDull", colors.YellowGreenDull},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.color.Light == "" {
				t.Errorf("%s: light color is empty", tc.name)
			}
			if tc.color.Dark == "" {
				t.Errorf("%s: dark color is empty", tc.name)
			}
			t.Logf("%s: light=%s dark=%s", tc.name, tc.color.Light, tc.color.Dark)
		})
	}
}

func TestXColorsGreenIsLipglossColor(t *testing.T) {
	// colors.Green is a plain lipgloss.Color — same type Gay MCP hex values produce.
	green := colors.Green
	if string(green) != "#04B575" {
		t.Errorf("Green: expected #04B575, got %s", string(green))
	}
}

func TestLipglossColorFromGayHex(t *testing.T) {
	// Gay MCP produces hex strings like "#A855F7".
	// lipgloss.Color accepts these directly — no conversion needed.
	gayColors := []struct {
		name string
		hex  string
	}{
		{"purple", "#A855F7"},
		{"blue", "#2E5FA3"},
		{"amber", "#F59E0B"},
		{"emerald", "#10B981"},
		{"red", "#EF4444"},
		{"indigo", "#6366F1"},
	}

	for _, gc := range gayColors {
		t.Run(gc.name, func(t *testing.T) {
			lgc := lipgloss.Color(gc.hex)
			if string(lgc) != gc.hex {
				t.Errorf("lipgloss.Color(%s) roundtrip failed: got %s", gc.hex, string(lgc))
			}
		})
	}
}

func TestAdaptiveColorFromGayHex(t *testing.T) {
	// Build adaptive colors from Gay MCP hex — lighter variant for dark bg.
	gayHex := "#A855F7"
	cf, err := colorful.Hex(gayHex)
	if err != nil {
		t.Fatalf("go-colorful parse failed: %v", err)
	}

	// Create a lighter variant for dark backgrounds (20% blend toward white)
	white := colorful.Color{R: 1, G: 1, B: 1}
	lighter := cf.BlendHcl(white, 0.2)

	adaptive := lipgloss.AdaptiveColor{
		Light: gayHex,
		Dark:  lighter.Hex(),
	}

	t.Logf("Adaptive Gay purple: light=%s dark=%s", adaptive.Light, adaptive.Dark)

	if adaptive.Light != gayHex {
		t.Errorf("Light should be original hex")
	}
	if adaptive.Dark == adaptive.Light {
		t.Errorf("Dark variant should differ from light")
	}

	// Parse the dark variant to verify it's valid hex
	darkCf, err := colorful.Hex(adaptive.Dark)
	if err != nil {
		t.Fatalf("Dark variant is invalid hex: %v", err)
	}

	// Dark variant should be lighter (higher L in HCL)
	_, _, origL := cf.Hcl()
	_, _, darkL := darkCf.Hcl()
	t.Logf("Lightness: original=%.3f, dark-variant=%.3f", origL, darkL)
	if darkL <= origL {
		t.Errorf("Dark variant should be lighter: got %.3f <= %.3f", darkL, origL)
	}
}

func TestPerceptualColorDistance(t *testing.T) {
	// Verify that Gay MCP palette colors are perceptually distinct.
	// Uses CIEDE2000 — the gold standard for color difference.
	hexColors := []string{"#A855F7", "#2E5FA3", "#F59E0B", "#10B981", "#EF4444", "#6366F1"}

	parsed := make([]colorful.Color, len(hexColors))
	for i, h := range hexColors {
		c, err := colorful.Hex(h)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", h, err)
		}
		parsed[i] = c
	}

	// Every pair should be perceptually distinct
	for i := 0; i < len(parsed); i++ {
		for j := i + 1; j < len(parsed); j++ {
			dist := parsed[i].DistanceCIEDE2000(parsed[j])
			t.Logf("%s ↔ %s: CIEDE2000=%.4f", hexColors[i], hexColors[j], dist)
			if dist < 0.05 {
				t.Errorf("Colors %s and %s too similar: CIEDE2000=%.4f",
					hexColors[i], hexColors[j], dist)
			}
		}
	}
}
