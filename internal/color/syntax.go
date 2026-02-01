//go:build darwin

package color

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/colors"
)

// Theme holds all the lipgloss styles for syntax highlighting S-expressions.
type Theme struct {
	// Structural
	Paren []lipgloss.Style // rainbow depth-indexed

	// Atoms
	Symbol  lipgloss.Style
	Keyword lipgloss.Style
	String  lipgloss.Style
	Number  lipgloss.Style
	Bool    lipgloss.Style
	Nil     lipgloss.Style
	Comment lipgloss.Style

	// Special forms
	SpecialForm lipgloss.Style
	Builtin     lipgloss.Style
	Namespace   lipgloss.Style

	// REPL chrome
	Prompt     lipgloss.Style
	Result     lipgloss.Style
	Error      lipgloss.Style
	Banner     lipgloss.Style
	BannerDim  lipgloss.Style
	HelpTitle  lipgloss.Style
	HelpCmd    lipgloss.Style
}

// specialForms lists Clojure/boxxy special forms for highlighting.
var specialForms = map[string]bool{
	"def": true, "let": true, "fn": true, "if": true, "do": true,
	"quote": true, "require": true, "ns": true,
}

// builtins lists commonly-used built-in functions.
var builtins = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
	"=": true, "<": true, ">": true,
	"println": true, "print": true, "str": true,
	"count": true, "first": true, "rest": true, "nth": true,
	"conj": true, "vector": true, "get": true, "assoc": true,
	"nil?": true, "string?": true, "number?": true, "vector?": true,
	"type": true, "not": true, "or": true, "and": true,
	"getenv": true,
}

// DefaultTheme returns a lipgloss theme using Gay MCP colors and
// charmbracelet/x/colors adaptive palette.
func DefaultTheme() *Theme {
	palette := DefaultRainbowPalette()
	parenStyles := PaletteToLipgloss(palette)

	return &Theme{
		Paren: parenStyles,

		Symbol:  lipgloss.NewStyle().Foreground(colors.Normal),
		Keyword: lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true),
		String:  lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")),
		Number:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6366F1")),
		Bool:    lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true),
		Nil:     lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Italic(true),
		Comment: lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true),

		SpecialForm: lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Bold(true),
		Builtin:     lipgloss.NewStyle().Foreground(lipgloss.Color("#2E5FA3")),
		Namespace:   lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Italic(true),

		Prompt:    lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Bold(true),
		Result:    lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true),
		Banner:    lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Bold(true),
		BannerDim: lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")),
		HelpTitle: lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true).Underline(true),
		HelpCmd:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6366F1")),
	}
}

// HighlightExpr applies syntax highlighting to an S-expression string.
// It combines rainbow parentheses with token-level coloring.
func (t *Theme) HighlightExpr(input string) string {
	var result strings.Builder
	i := 0
	depth := 0
	firstInList := false
	inString := false
	escape := false

	for i < len(input) {
		ch := input[i]

		// Handle string state
		if escape {
			result.WriteByte(ch)
			escape = false
			i++
			continue
		}
		if ch == '\\' && inString {
			result.WriteString(t.String.Render("\\"))
			escape = true
			i++
			continue
		}
		if ch == '"' {
			if inString {
				// End of string — scan back to get the whole thing was already emitted char by char
				result.WriteString(t.String.Render("\""))
				inString = false
				i++
				continue
			}
			// Start of string — collect the whole string token
			j := i + 1
			for j < len(input) {
				if input[j] == '\\' {
					j += 2
					continue
				}
				if input[j] == '"' {
					j++
					break
				}
				j++
			}
			result.WriteString(t.String.Render(input[i:j]))
			i = j
			continue
		}

		// Comment
		if ch == ';' {
			j := i
			for j < len(input) && input[j] != '\n' {
				j++
			}
			result.WriteString(t.Comment.Render(input[i:j]))
			i = j
			continue
		}

		// Parens/brackets/braces
		if ch == '(' || ch == '[' || ch == '{' {
			style := ParenStyle(depth, t.Paren)
			result.WriteString(style.Render(string(ch)))
			depth++
			if ch == '(' {
				firstInList = true
			}
			i++
			continue
		}
		if ch == ')' || ch == ']' || ch == '}' {
			if depth > 0 {
				depth--
			}
			style := ParenStyle(depth, t.Paren)
			result.WriteString(style.Render(string(ch)))
			i++
			continue
		}

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == ',' {
			result.WriteByte(ch)
			i++
			continue
		}

		// Token: collect until terminator
		j := i
		for j < len(input) {
			c := input[j]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' ||
				c == '(' || c == ')' || c == '[' || c == ']' ||
				c == '{' || c == '}' || c == '"' || c == ';' {
				break
			}
			j++
		}

		token := input[i:j]
		result.WriteString(t.colorToken(token, firstInList))
		firstInList = false
		i = j
	}

	return result.String()
}

func (t *Theme) colorToken(token string, isHead bool) string {
	// Keywords :foo
	if len(token) > 0 && token[0] == ':' {
		return t.Keyword.Render(token)
	}

	// Booleans
	if token == "true" || token == "false" {
		return t.Bool.Render(token)
	}

	// Nil
	if token == "nil" {
		return t.Nil.Render(token)
	}

	// Numbers
	if isNumber(token) {
		return t.Number.Render(token)
	}

	// Namespace-qualified: vz/foo, agm/foo
	if strings.Contains(token, "/") && !strings.HasPrefix(token, "/") {
		return t.Namespace.Render(token)
	}

	// Head position in list
	if isHead {
		if specialForms[token] {
			return t.SpecialForm.Render(token)
		}
		if builtins[token] {
			return t.Builtin.Render(token)
		}
	}

	return t.Symbol.Render(token)
}

func isNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		if len(s) == 1 {
			return false
		}
		start = 1
	}
	hasDot := false
	for i := start; i < len(s); i++ {
		if s[i] == '.' {
			if hasDot {
				return false
			}
			hasDot = true
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
