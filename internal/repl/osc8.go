//go:build darwin

package repl

import (
	"fmt"
	"math"
	"strings"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/bmorphism/boxxy/internal/tile"
)

// OSC 8 terminal hyperlink escape sequences.
// Format: ESC ] 8 ; params ; URI ST  TEXT  ESC ] 8 ; ; ST
// ST = ESC \ (we use BEL \a for wider terminal compat)
//
// Supported by: Ghostty, iTerm2, WezTerm, foot, Kitty, Windows Terminal,
// GNOME Terminal 3.26+, VTE 0.50+.

const (
	osc8Open  = "\033]8;;"
	osc8Close = "\033]8;;\a"
	osc8Sep   = "\a"
)

// OSC8Link wraps text in an OSC 8 terminal hyperlink.
func OSC8Link(url, text string) string {
	return osc8Open + url + osc8Sep + text + osc8Close
}

// OSC8LinkParams wraps text with optional id= param for link grouping.
func OSC8LinkParams(url, text, id string) string {
	params := ""
	if id != "" {
		params = "id=" + id
	}
	return "\033]8;" + params + ";" + url + osc8Sep + text + osc8Close
}

// TileURI returns a canonical tile:// URI for a seed.
// tile://seed/<seed>  — the VM's identity
func TileURI(seed uint64) string {
	return fmt.Sprintf("tile://seed/%d", seed)
}

// ColorURI returns an https link to a color viewer for the hex code.
func ColorURI(hex string) string {
	h := strings.TrimPrefix(hex, "#")
	return fmt.Sprintf("https://www.colorhexa.com/%s", strings.ToLower(h))
}

// SyrupURI returns a tile:// URI for the Syrup wire representation.
func SyrupURI(seed uint64) string {
	return fmt.Sprintf("tile://syrup/%d", seed)
}

// LinkedColor returns the hex code as a clickable OSC 8 link, ANSI-colored
// in the color itself (24-bit truecolor).
func LinkedColor(ci tile.ColorIdentity) string {
	url := ColorURI(ci.HexCode)
	ansi := fmt.Sprintf("\033[38;2;%d;%d;%dm", ci.Color.R, ci.Color.G, ci.Color.B)
	reset := "\033[0m"
	return ansi + OSC8Link(url, ci.HexCode) + reset
}

// LinkedTile returns a tile identity summary as a clickable link:
// [#A855F7 +1]  where the hex is linked and ANSI-colored.
func LinkedTile(ci tile.ColorIdentity) string {
	url := TileURI(ci.Seed)
	ansi := fmt.Sprintf("\033[38;2;%d;%d;%dm", ci.Color.R, ci.Color.G, ci.Color.B)
	reset := "\033[0m"
	trit := ci.Trit
	sign := "0"
	if trit == 1 {
		sign = "+1"
	} else if trit == 2 {
		sign = "-1"
	}
	label := fmt.Sprintf("%s %s", ci.HexCode, sign)
	return ansi + OSC8Link(url, label) + reset
}

// LinkedSeed wraps a seed number as a clickable tile:// link.
func LinkedSeed(seed uint64) string {
	ci := tile.NewColorIdentity(seed)
	url := TileURI(seed)
	ansi := fmt.Sprintf("\033[38;2;%d;%d;%dm", ci.Color.R, ci.Color.G, ci.Color.B)
	reset := "\033[0m"
	return ansi + OSC8Link(url, fmt.Sprintf("seed:%d", seed)) + reset
}

// --- OSC 8 Rainbow Parentheses ---

// GoldenAnglePrecise matches color.GoldenAnglePrecise.
const goldenAngle = 137.5077640500378

// LinkedRainbowParens colorizes parentheses with ANSI truecolor AND wraps
// each bracket pair in an OSC 8 link carrying the depth and seed as context.
//
// The link format: tile://paren/<seed>/<depth>
// This lets a smart terminal handler (Ghostty, custom protocol handler)
// show the tile context on hover/click.
func LinkedRainbowParens(input string, seed uint64) string {
	ci := tile.NewColorIdentity(seed)
	palette := ci.RainbowPalette(8)

	var result strings.Builder
	depth := 0
	inString := false
	escape := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if escape {
			result.WriteByte(ch)
			escape = false
			continue
		}
		if ch == '\\' && inString {
			result.WriteByte(ch)
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			result.WriteByte(ch)
			continue
		}
		if inString {
			result.WriteByte(ch)
			continue
		}

		switch ch {
		case '(', '[', '{':
			c := palette[depth%len(palette)]
			r8, g8, b8 := colorTo8bit(c)
			url := fmt.Sprintf("tile://paren/%d/%d", seed, depth)
			ansi := fmt.Sprintf("\033[38;2;%d;%d;%dm", r8, g8, b8)
			result.WriteString(ansi + OSC8Link(url, string(ch)) + "\033[0m")
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			c := palette[depth%len(palette)]
			r8, g8, b8 := colorTo8bit(c)
			url := fmt.Sprintf("tile://paren/%d/%d", seed, depth)
			ansi := fmt.Sprintf("\033[38;2;%d;%d;%dm", r8, g8, b8)
			result.WriteString(ansi + OSC8Link(url, string(ch)) + "\033[0m")
		default:
			result.WriteByte(ch)
		}
	}
	return result.String()
}

// LinkedHighlightExpr does full syntax highlighting with OSC 8 links on
// namespace-qualified symbols (tile/*, vz/*, agm/*) linking to help URIs,
// and rainbow parens linked to tile://paren/ as above.
func LinkedHighlightExpr(input string, seed uint64) string {
	ci := tile.NewColorIdentity(seed)
	palette := ci.RainbowPalette(8)

	var result strings.Builder
	i := 0
	depth := 0
	firstInList := false
	inString := false
	escaping := false

	for i < len(input) {
		ch := input[i]

		if escaping {
			result.WriteByte(ch)
			escaping = false
			i++
			continue
		}
		if ch == '\\' && inString {
			result.WriteByte(ch)
			escaping = true
			i++
			continue
		}

		// Strings
		if ch == '"' {
			if inString {
				result.WriteString(ansiColor(16, 185, 129, "\""))
				inString = false
				i++
				continue
			}
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
			result.WriteString(ansiColor(16, 185, 129, input[i:j]))
			i = j
			continue
		}

		// Comment
		if ch == ';' {
			j := i
			for j < len(input) && input[j] != '\n' {
				j++
			}
			result.WriteString(ansiColor(107, 114, 128, input[i:j]))
			i = j
			continue
		}

		// Parens with OSC 8
		if ch == '(' || ch == '[' || ch == '{' {
			c := palette[depth%len(palette)]
			r8, g8, b8 := colorTo8bit(c)
			url := fmt.Sprintf("tile://paren/%d/%d", seed, depth)
			result.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm", r8, g8, b8))
			result.WriteString(OSC8Link(url, string(ch)))
			result.WriteString("\033[0m")
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
			c := palette[depth%len(palette)]
			r8, g8, b8 := colorTo8bit(c)
			url := fmt.Sprintf("tile://paren/%d/%d", seed, depth)
			result.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm", r8, g8, b8))
			result.WriteString(OSC8Link(url, string(ch)))
			result.WriteString("\033[0m")
			i++
			continue
		}

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == ',' {
			result.WriteByte(ch)
			i++
			continue
		}

		// Token
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
		result.WriteString(linkedToken(token, seed, firstInList))
		firstInList = false
		i = j
	}

	return result.String()
}

func linkedToken(token string, seed uint64, isHead bool) string {
	// Keywords
	if len(token) > 0 && token[0] == ':' {
		return ansiColor(245, 158, 11, token)
	}
	// Booleans
	if token == "true" || token == "false" {
		return ansiBold(239, 68, 68, token)
	}
	// Nil
	if token == "nil" {
		return ansiColor(239, 68, 68, token)
	}
	// Numbers
	if isNumber(token) {
		return ansiColor(99, 102, 241, token)
	}
	// Namespace-qualified → OSC 8 link to help
	if strings.Contains(token, "/") && !strings.HasPrefix(token, "/") {
		parts := strings.SplitN(token, "/", 2)
		ns := parts[0]
		url := fmt.Sprintf("tile://help/%s/%s", ns, parts[1])
		colored := ansiColor(168, 85, 247, token)
		return OSC8Link(url, colored)
	}
	// Special forms
	if isHead && isSpecialForm(token) {
		return ansiBold(168, 85, 247, token)
	}
	// Builtins
	if isHead && isBuiltin(token) {
		return ansiColor(46, 95, 163, token)
	}
	return token
}

func isSpecialForm(s string) bool {
	switch s {
	case "def", "let", "fn", "if", "do", "quote", "require", "ns":
		return true
	}
	return false
}

func isBuiltin(s string) bool {
	switch s {
	case "+", "-", "*", "/", "=", "<", ">",
		"println", "print", "str", "count", "first", "rest", "nth",
		"conj", "vector", "get", "assoc",
		"nil?", "string?", "number?", "vector?",
		"type", "not", "or", "and", "getenv":
		return true
	}
	return false
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

// ansiColor wraps text in 24-bit ANSI truecolor.
func ansiColor(r, g, b uint8, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, text)
}

// ansiBold wraps text in bold 24-bit ANSI truecolor.
func ansiBold(r, g, b uint8, text string) string {
	return fmt.Sprintf("\033[1;38;2;%d;%d;%dm%s\033[0m", r, g, b, text)
}

// colorTo8bit extracts 0-255 RGB from a go-colorful Color.
func colorTo8bit(c colorful.Color) (uint8, uint8, uint8) {
	return uint8(math.Round(c.R * 255)),
		uint8(math.Round(c.G * 255)),
		uint8(math.Round(c.B * 255))
}
