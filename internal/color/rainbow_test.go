//go:build darwin

package color_test

import (
	"io"
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"

	"github.com/bmorphism/boxxy/internal/color"
)

// forcedRenderer returns a lipgloss renderer that always emits TrueColor
// ANSI codes, even without a TTY (essential for CI/test environments).
func forcedRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard, termenv.WithProfile(termenv.TrueColor))
	r.SetColorProfile(termenv.TrueColor)
	return r
}

func TestGoldenAngle(t *testing.T) {
	if math.Abs(color.GoldenAnglePrecise-137.508) > 0.01 {
		t.Errorf("Golden angle should be ~137.508, got %.3f", color.GoldenAnglePrecise)
	}
}

func TestPlasticAngle(t *testing.T) {
	if math.Abs(color.PlasticAngle-205.14) > 0.5 {
		t.Errorf("Plastic angle should be ~205.14, got %.2f", color.PlasticAngle)
	}
}

func TestRainbowPaletteDistinctness(t *testing.T) {
	palette := color.RainbowPalette(8, 271.0, 0.7, 0.55)
	if len(palette) != 8 {
		t.Fatalf("Expected 8 colors, got %d", len(palette))
	}
	for i := 0; i < len(palette); i++ {
		for j := i + 1; j < len(palette); j++ {
			dist := palette[i].DistanceCIEDE2000(palette[j])
			t.Logf("Color %d (%s) <-> Color %d (%s): CIEDE2000=%.4f",
				i, palette[i].Hex(), j, palette[j].Hex(), dist)
			if dist < 0.02 {
				t.Errorf("Colors %d and %d too similar: %.4f", i, j, dist)
			}
		}
	}
}

func TestDefaultRainbowPalette(t *testing.T) {
	palette := color.DefaultRainbowPalette()
	if len(palette) != 8 {
		t.Fatalf("Expected 8 colors, got %d", len(palette))
	}
	h, _, _ := palette[0].Hcl()
	t.Logf("First rainbow color hue: %.1f (seeded from Gay MCP purple)", h)
	if h < 240 || h > 330 {
		t.Errorf("First color should be purple-ish (250-320), got %.1f", h)
	}
}

func TestPaletteToLipgloss(t *testing.T) {
	palette := color.DefaultRainbowPalette()
	styles := color.PaletteToLipgloss(palette)
	if len(styles) != len(palette) {
		t.Errorf("Style count mismatch: %d vs %d", len(styles), len(palette))
	}
}

func TestPaletteToLipglossForcedRenderer(t *testing.T) {
	palette := color.DefaultRainbowPalette()
	r := forcedRenderer()
	styles := color.PaletteToLipgloss(palette, r)
	if len(styles) != len(palette) {
		t.Fatalf("Style count mismatch")
	}
	// With forced TrueColor renderer, Render should produce ANSI
	rendered := styles[0].Render("(")
	if !strings.Contains(rendered, "\033[") {
		t.Errorf("Forced renderer should emit ANSI codes, got: %q", rendered)
	}
	t.Logf("Forced render paren: %q", rendered)
}

func TestRainbowParensBasic(t *testing.T) {
	palette := color.DefaultRainbowPalette()
	r := forcedRenderer()
	styles := color.PaletteToLipgloss(palette, r)

	input := "(+ 1 2)"
	result := color.RainbowParens(input, styles)

	if !strings.Contains(result, "\033[") {
		t.Error("Rainbow parens should contain ANSI escape codes with forced renderer")
	}
	if !strings.Contains(result, "+ 1 2") {
		t.Error("Non-paren content should be preserved")
	}
	t.Logf("Rainbow: %q", result)
}

func TestRainbowParensNested(t *testing.T) {
	palette := color.DefaultRainbowPalette()
	r := forcedRenderer()
	styles := color.PaletteToLipgloss(palette, r)

	input := "(def x (let [a 1] (+ a 2)))"
	result := color.RainbowParens(input, styles)

	if !strings.Contains(result, "\033[") {
		t.Error("Nested parens should have ANSI codes")
	}
	t.Logf("Nested rainbow: %q", result)
}

func TestRainbowParensStringLiterals(t *testing.T) {
	palette := color.DefaultRainbowPalette()
	styles := color.PaletteToLipgloss(palette)

	input := `(println "hello (world)")`
	result := color.RainbowParens(input, styles)
	if !strings.Contains(result, "hello (world)") {
		t.Error("Parens inside strings should not be rainbow-colored")
	}
}

func TestRainbowParensBrackets(t *testing.T) {
	palette := color.DefaultRainbowPalette()
	r := forcedRenderer()
	styles := color.PaletteToLipgloss(palette, r)

	input := "(let [a 1 b {:key val}] (+ a b))"
	result := color.RainbowParens(input, styles)

	if !strings.Contains(result, "\033[") {
		t.Error("Brackets and braces should also be rainbow-colored")
	}
	t.Logf("Mixed delimiters: %q", result)
}

func TestGoldenSpiralMaximalDispersion(t *testing.T) {
	palette := color.RainbowPalette(8, 0, 0.7, 0.55)
	hues := make([]float64, len(palette))
	for i, c := range palette {
		h, _, _ := c.Hcl()
		hues[i] = h
	}
	minDist := 360.0
	for i := 0; i < len(hues); i++ {
		for j := i + 1; j < len(hues); j++ {
			diff := math.Abs(hues[i] - hues[j])
			if diff > 180 {
				diff = 360 - diff
			}
			if diff < minDist {
				minDist = diff
			}
		}
	}
	t.Logf("Min hue distance across 8 golden-spiral colors: %.1f", minDist)
	if minDist < 15 {
		t.Errorf("Golden spiral should maintain >15 min hue distance, got %.1f", minDist)
	}
}

func TestGayMCPSeedColor(t *testing.T) {
	purple, err := colorful.Hex("#A855F7")
	if err != nil {
		t.Fatalf("Failed to parse #A855F7: %v", err)
	}
	h, c, l := purple.Hcl()
	t.Logf("Gay MCP purple: HCL(%.1f, %.3f, %.3f)", h, c, l)

	palette := color.RainbowPalette(1, h, 0.7, 0.55)
	firstH, _, _ := palette[0].Hcl()
	if math.Abs(firstH-h) > 1.0 {
		t.Errorf("First palette color hue should match seed: %.1f vs %.1f", firstH, h)
	}
}
