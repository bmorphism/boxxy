//go:build darwin

// Package color provides Gay MCP golden-thread rainbow parentheses
// and lipgloss-based syntax highlighting for S-expressions.
package color

import (
	"math"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// GoldenAngle is φ⁻¹ × 360° ≈ 137.508° — the angle that maximizes
// perceptual dispersion on the hue wheel (sunflower spiral).
const GoldenAngle = 360.0 / (1.0 + (1.0+math.Sqrt2*0.41421356237) + 0.41421356237)

// We use the exact value: 360 / φ² = 360 / 2.618... ≈ 137.508°
const GoldenAnglePrecise = 137.5077640500378

// PlasticAngle ≈ 205.14° — the ternary/3D analog from the plastic
// constant ρ (x³ = x + 1, ρ ≈ 1.3247). Matches Gay MCP plastic_thread.
const PlasticAngle = 205.1442270324102

// RainbowPalette generates n colors along the golden-angle spiral,
// starting from a base hue. Each color is maximally perceptually
// dispersed from its neighbors — the same algorithm as Gay MCP's
// golden_thread tool.
func RainbowPalette(n int, baseHue float64, saturation, lightness float64) []colorful.Color {
	colors := make([]colorful.Color, n)
	for i := 0; i < n; i++ {
		hue := math.Mod(baseHue+float64(i)*GoldenAnglePrecise, 360.0)
		colors[i] = colorful.Hcl(hue, saturation, lightness)
	}
	return colors
}

// DefaultRainbowPalette returns the standard 8-depth rainbow parentheses
// palette derived from Gay MCP's purple (#A855F7) as the seed hue.
func DefaultRainbowPalette() []colorful.Color {
	// #A855F7 has hue ≈ 271° in HCL space
	purple, _ := colorful.Hex("#A855F7")
	h, _, _ := purple.Hcl()
	return RainbowPalette(8, h, 0.7, 0.55)
}

// PaletteToLipgloss converts go-colorful colors to lipgloss.Color values.
// If a renderer is provided, styles are scoped to it (useful for forcing
// TrueColor output in tests or non-TTY contexts).
func PaletteToLipgloss(palette []colorful.Color, renderer ...*lipgloss.Renderer) []lipgloss.Style {
	styles := make([]lipgloss.Style, len(palette))
	for i, c := range palette {
		if len(renderer) > 0 && renderer[0] != nil {
			styles[i] = renderer[0].NewStyle().Foreground(lipgloss.Color(c.Hex()))
		} else {
			styles[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
		}
	}
	return styles
}

// ParenStyle returns the lipgloss style for a parenthesis at the given
// nesting depth, cycling through the rainbow palette.
func ParenStyle(depth int, palette []lipgloss.Style) lipgloss.Style {
	if len(palette) == 0 {
		return lipgloss.NewStyle()
	}
	return palette[depth%len(palette)]
}

// RainbowParens colorizes matched parentheses/brackets/braces in an
// S-expression string. Each nesting level gets a distinct color from
// the golden-thread palette.
func RainbowParens(input string, styles []lipgloss.Style) string {
	if len(styles) == 0 {
		return input
	}

	var result []byte
	depth := 0
	inString := false
	escape := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if escape {
			result = append(result, ch)
			escape = false
			continue
		}

		if ch == '\\' && inString {
			result = append(result, ch)
			escape = true
			continue
		}

		if ch == '"' {
			inString = !inString
			result = append(result, ch)
			continue
		}

		if inString {
			result = append(result, ch)
			continue
		}

		switch ch {
		case '(', '[', '{':
			styled := styles[depth%len(styles)].Render(string(ch))
			result = append(result, []byte(styled)...)
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			styled := styles[depth%len(styles)].Render(string(ch))
			result = append(result, []byte(styled)...)
		default:
			result = append(result, ch)
		}
	}

	return string(result)
}
