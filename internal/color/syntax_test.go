//go:build darwin

package color_test

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/bmorphism/boxxy/internal/color"
)

// forcedTheme returns a Theme with a TrueColor renderer so ANSI codes
// are emitted even in CI/test (NoTTY) environments.
func forcedTheme() *color.Theme {
	r := lipgloss.NewRenderer(io.Discard, termenv.WithProfile(termenv.TrueColor))
	r.SetColorProfile(termenv.TrueColor)

	palette := color.DefaultRainbowPalette()
	parenStyles := color.PaletteToLipgloss(palette, r)

	return &color.Theme{
		Paren:       parenStyles,
		Symbol:      r.NewStyle(),
		Keyword:     r.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true),
		String:      r.NewStyle().Foreground(lipgloss.Color("#10B981")),
		Number:      r.NewStyle().Foreground(lipgloss.Color("#6366F1")),
		Bool:        r.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true),
		Nil:         r.NewStyle().Foreground(lipgloss.Color("#EF4444")).Italic(true),
		Comment:     r.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true),
		SpecialForm: r.NewStyle().Foreground(lipgloss.Color("#A855F7")).Bold(true),
		Builtin:     r.NewStyle().Foreground(lipgloss.Color("#2E5FA3")),
		Namespace:   r.NewStyle().Foreground(lipgloss.Color("#A855F7")).Italic(true),
		Prompt:      r.NewStyle().Foreground(lipgloss.Color("#A855F7")).Bold(true),
		Result:      r.NewStyle().Foreground(lipgloss.Color("#10B981")),
		Error:       r.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true),
		Banner:      r.NewStyle().Foreground(lipgloss.Color("#A855F7")).Bold(true),
		BannerDim:   r.NewStyle().Foreground(lipgloss.Color("#6B7280")),
		HelpTitle:   r.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Underline(true),
		HelpCmd:     r.NewStyle().Foreground(lipgloss.Color("#6366F1")),
	}
}

func TestDefaultTheme(t *testing.T) {
	theme := color.DefaultTheme()
	if theme == nil {
		t.Fatal("DefaultTheme returned nil")
	}
	if len(theme.Paren) != 8 {
		t.Errorf("Expected 8 paren styles, got %d", len(theme.Paren))
	}
}

func TestHighlightSpecialForms(t *testing.T) {
	theme := forcedTheme()
	cases := []string{"def", "let", "fn", "if", "do", "quote"}
	for _, sf := range cases {
		input := "(" + sf + " x 1)"
		result := theme.HighlightExpr(input)
		if !strings.Contains(result, "\033[") {
			t.Errorf("Special form '%s' should be highlighted with ANSI", sf)
		}
		t.Logf("Special form %s: %q", sf, result)
	}
}

func TestHighlightKeywords(t *testing.T) {
	theme := forcedTheme()
	input := ":my-keyword"
	result := theme.HighlightExpr(input)
	if !strings.Contains(result, "\033[") {
		t.Error("Keywords should be highlighted")
	}
	t.Logf("Keyword: %q", result)
}

func TestHighlightStrings(t *testing.T) {
	theme := forcedTheme()
	input := `(println "hello world")`
	result := theme.HighlightExpr(input)
	if !strings.Contains(result, "\033[") {
		t.Error("Strings should be highlighted")
	}
	if !strings.Contains(result, "hello world") {
		t.Error("String content should be preserved")
	}
	t.Logf("String: %q", result)
}

func TestHighlightNumbers(t *testing.T) {
	theme := forcedTheme()
	input := "(+ 42 3.14 -7)"
	result := theme.HighlightExpr(input)
	if !strings.Contains(result, "\033[") {
		t.Error("Numbers should be highlighted")
	}
	t.Logf("Numbers: %q", result)
}

func TestHighlightBooleans(t *testing.T) {
	theme := forcedTheme()
	input := "(if true 1 false)"
	result := theme.HighlightExpr(input)
	if !strings.Contains(result, "\033[") {
		t.Error("Booleans should be highlighted")
	}
	t.Logf("Booleans: %q", result)
}

func TestHighlightNamespace(t *testing.T) {
	theme := forcedTheme()
	input := "(vz/new-vm config)"
	result := theme.HighlightExpr(input)
	if !strings.Contains(result, "\033[") {
		t.Error("Namespace-qualified symbols should be highlighted")
	}
	t.Logf("Namespace: %q", result)
}

func TestHighlightComment(t *testing.T) {
	theme := forcedTheme()
	input := "(+ 1 2) ; this is a comment"
	result := theme.HighlightExpr(input)
	if !strings.Contains(result, "\033[") {
		t.Error("Comments should be highlighted")
	}
	if !strings.Contains(result, "this is a comment") {
		t.Error("Comment content should be preserved")
	}
	t.Logf("Comment: %q", result)
}

func TestHighlightComplexExpression(t *testing.T) {
	theme := forcedTheme()
	input := `(let [vm (vz/new-vm-config 4 8 boot platform)] (if (nil? vm) (println "failed") (vz/start-vm! vm)))`
	result := theme.HighlightExpr(input)
	if len(result) < len(input) {
		t.Error("Highlighted output should be longer than input (ANSI codes)")
	}
	t.Logf("Complex: %q", result)
}

func TestHighlightAGMExpressions(t *testing.T) {
	theme := forcedTheme()
	exprs := []string{
		"(def K (agm/new-belief-set))",
		"(agm/expand K 'vm-running)",
		"(agm/revise K 'not-vm-running)",
		"(agm/contract K 'disk-attached)",
		"(agm/worlds K)",
	}
	for _, expr := range exprs {
		result := theme.HighlightExpr(expr)
		if !strings.Contains(result, "\033[") {
			t.Errorf("AGM expression should be highlighted: %s", expr)
		}
		t.Logf("AGM: %q", result)
	}
}

func TestHighlightEmptyInput(t *testing.T) {
	theme := color.DefaultTheme()
	result := theme.HighlightExpr("")
	if result != "" {
		t.Errorf("Empty input should produce empty output, got: %q", result)
	}
}

func TestHighlightPreservesWhitespace(t *testing.T) {
	theme := forcedTheme()
	input := "(def  x  1)"
	result := theme.HighlightExpr(input)
	if !strings.Contains(result, "  ") {
		t.Error("Whitespace should be preserved")
	}
}
