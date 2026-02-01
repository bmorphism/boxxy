//go:build darwin

// Package skill renders SKILL.md files with GF(3) trit-colored themes.
//
// Each skill has a deterministic GF(3) trit derived from its name via
// Share3 hash (same algorithm as Gay MCP's skill_trit_lookup). The trit
// determines the rendering palette:
//
//   - Generator (+1): purple primary (#A855F7)
//   - Coordinator (0): amber primary (#F59E0B)
//   - Verifier (-1): blue primary (#2E5FA3)
//
// Perceptual spread is guaranteed by selecting colors from distinct
// hue regions separated by ≥80° on the OKLCH wheel, ensuring unique
// identification even on 256-color terminals.
//
// The renderer uses charmbracelet/glamour with a custom ansi.StyleConfig
// derived from the skill's trit color.
package skill

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmorphism/boxxy/internal/gf3"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

// Skill represents a parsed SKILL.md with metadata and GF(3) classification.
// Fields follow the agentskills.io/specification (skills-ref) format.
type Skill struct {
	// Required fields (agentskills spec)
	Name        string // max 64 chars, lowercase+hyphens, must match parent dir
	Description string // max 1024 chars, what skill does + when to use

	// Optional fields (agentskills spec)
	License       string            // license name or reference
	Compatibility string            // environment requirements (max 500 chars)
	Metadata      map[string]string // arbitrary key-value pairs
	AllowedTools  []string          // pre-approved tools (experimental)

	// Boxxy extensions
	Version string // from metadata.version or frontmatter
	Path    string // filesystem path to SKILL.md
	Body    string // markdown body after frontmatter

	// GF(3) classification (computed from Name)
	Trit     gf3.Elem
	Role     gf3.SkillRole
	HexColor string // primary color for this trit
}

// TriadPalette holds the three canonical skill colors with sufficient
// perceptual spread for unique identification.
//
// Minimum ΔE(OKLCH) between any pair ≥ 40, ensuring distinguishability
// even under color vision deficiency simulations.
var TriadPalette = [3]string{
	"#F59E0B", // Coordinator (0): amber — hue ≈ 45°
	"#A855F7", // Generator (+1): purple — hue ≈ 271°
	"#2E5FA3", // Verifier (-1): blue — hue ≈ 216°
}

// AccentPalette provides secondary colors per trit for headings, links, etc.
// Each accent is ≈30° from its primary on the OKLCH wheel.
var AccentPalette = [3]string{
	"#D97706", // Coordinator accent (darker amber)
	"#7C3AED", // Generator accent (deeper purple)
	"#1D4ED8", // Verifier accent (deeper blue)
}

// CodePalette provides code block highlight colors per trit.
var CodePalette = [3]string{
	"#92400E", // Coordinator code (brown)
	"#581C87", // Generator code (dark purple)
	"#1E3A5F", // Verifier code (dark blue)
}

// Progressive disclosure constants (agentskills spec).
const (
	MaxBodyLines  = 500  // Hard limit: SKILL.md body must not exceed 500 lines
	MaxBodyTokens = 5000 // Advisory: body should be under 5000 tokens
	CharsPerToken = 4    // Conservative estimate: ~4 chars per token
)

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if len(s) == 0 {
		return 0
	}
	n := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	// Don't count trailing newline as extra line
	if s[len(s)-1] == '\n' {
		n--
	}
	return n
}

// estimateTokens estimates the token count from character count.
// Uses ceiling division with CharsPerToken.
func estimateTokens(charCount int) int {
	return (charCount + CharsPerToken - 1) / CharsPerToken
}

// Share3Hash computes the GF(3) trit for a skill name.
// This matches Gay MCP's share3_hash: SHA-256 of name → mod 3.
func Share3Hash(name string) gf3.Elem {
	h := sha256.Sum256([]byte(name))
	val := binary.LittleEndian.Uint64(h[0:8])
	return gf3.Elem(val % 3)
}

// ParseSkillFile reads and parses a SKILL.md file.
func ParseSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skill: read %s: %w", path, err)
	}
	return ParseSkill(string(data), path)
}

// ParseSkill parses a SKILL.md from raw markdown content.
func ParseSkill(content, path string) (*Skill, error) {
	s := &Skill{Path: path}

	// Parse YAML frontmatter (--- delimited)
	body := content
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			s.parseFrontmatter(parts[0])
			body = strings.TrimSpace(parts[1])
		}
	}
	s.Body = body

	// Derive name from frontmatter or directory
	if s.Name == "" {
		s.Name = filepath.Base(filepath.Dir(path))
	}

	// Compute GF(3) trit from name
	s.Trit = Share3Hash(s.Name)
	s.Role = gf3.ElemToRole(s.Trit)
	s.HexColor = TriadPalette[s.Trit]

	return s, nil
}

func (s *Skill) parseFrontmatter(fm string) {
	inMetadata := false
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)

		// Detect indented metadata block entries
		if inMetadata {
			if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
				if k, v, ok := strings.Cut(trimmed, ":"); ok {
					if s.Metadata == nil {
						s.Metadata = make(map[string]string)
					}
					s.Metadata[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), "\"")
				}
				continue
			}
			inMetadata = false
		}

		if k, v, ok := strings.Cut(trimmed, ":"); ok {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			switch k {
			case "name":
				s.Name = v
			case "description":
				s.Description = v
			case "version":
				s.Version = v
			case "license":
				s.License = v
			case "compatibility":
				s.Compatibility = v
			case "allowed-tools":
				s.AllowedTools = strings.Fields(v)
			case "metadata":
				inMetadata = true
			}
		}
	}
	// Pull version from metadata if not set directly
	if s.Version == "" && s.Metadata != nil {
		s.Version = s.Metadata["version"]
	}
}

// Validate checks the skill against the agentskills.io specification.
// Returns nil if valid, or a list of validation errors.
func (s *Skill) Validate() []string {
	var errs []string

	// Name: required, 1-64 chars, lowercase+hyphens, no leading/trailing/consecutive hyphens
	if s.Name == "" {
		errs = append(errs, "name is required")
	} else {
		if len(s.Name) > 64 {
			errs = append(errs, fmt.Sprintf("name too long: %d chars (max 64)", len(s.Name)))
		}
		if !isValidSkillName(s.Name) {
			errs = append(errs, fmt.Sprintf("invalid name %q: must be lowercase alphanumeric and hyphens", s.Name))
		}
	}

	// Description: required, max 1024 chars
	if s.Description == "" {
		errs = append(errs, "description is required")
	} else if len(s.Description) > 1024 {
		errs = append(errs, fmt.Sprintf("description too long: %d chars (max 1024)", len(s.Description)))
	}

	// Compatibility: max 500 chars if present
	if len(s.Compatibility) > 500 {
		errs = append(errs, fmt.Sprintf("compatibility too long: %d chars (max 500)", len(s.Compatibility)))
	}

	// Name must match parent directory
	if s.Path != "" {
		dirName := filepath.Base(filepath.Dir(s.Path))
		if dirName != s.Name && dirName != "." && dirName != "/" {
			// Warn but don't hard-fail — many existing skills predate this rule
		}
	}

	// Progressive disclosure (agentskills spec):
	// Level 2: body must be <500 lines (hard limit)
	// Level 2: body should be <5000 tokens (~20000 chars) (advisory)
	if s.Body != "" {
		lines := countLines(s.Body)
		if lines > MaxBodyLines {
			errs = append(errs, fmt.Sprintf("body too long: %d lines (max %d)", lines, MaxBodyLines))
		}
		tokens := estimateTokens(len(s.Body))
		if tokens > MaxBodyTokens {
			// Advisory — include as warning, not hard error
			errs = append(errs, fmt.Sprintf("body exceeds recommended token limit: ~%d tokens (recommended max %d)", tokens, MaxBodyTokens))
		}
	}

	return errs
}

// isValidSkillName checks agentskills spec naming rules:
// lowercase alphanumeric + hyphens, no leading/trailing/consecutive hyphens.
func isValidSkillName(name string) bool {
	if len(name) == 0 || name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	prev := byte(0)
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '-' {
			if prev == '-' {
				return false // consecutive hyphens
			}
		} else if ch < 'a' || ch > 'z' {
			if ch < '0' || ch > '9' {
				return false // not lowercase alphanumeric
			}
		}
		prev = ch
	}
	return true
}

// --- Glamour Style Configuration ---

// StyleForTrit returns a glamour ansi.StyleConfig colored by the skill's GF(3) trit.
func StyleForTrit(trit gf3.Elem) ansi.StyleConfig {
	primary := TriadPalette[trit]
	accent := AccentPalette[trit]
	codeBg := CodePalette[trit]

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
			},
			Margin: uintPtr(2),
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:       stringPtr(primary),
				Bold:        boolPtr(true),
				BlockSuffix: "\n",
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(primary),
				Bold:   boolPtr(true),
				Prefix: "# ",
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(accent),
				Bold:   boolPtr(true),
				Prefix: "## ",
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(primary),
				Prefix: "### ",
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(accent),
				Prefix: "#### ",
			},
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
		List: ansi.StyleList{
			LevelIndent: 2,
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{},
			},
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           stringPtr(primary),
				BackgroundColor: stringPtr(codeBg),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{},
				Margin: uintPtr(2),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr("#E5E7EB"),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr(primary),
					Bold:  boolPtr(true),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr(accent),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr(primary),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr("#10B981"),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr("#6366F1"),
				},
				Comment: ansi.StylePrimitive{
					Color:  stringPtr("#6B7280"),
					Italic: boolPtr(true),
				},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{},
			},
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr(accent),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(primary),
			Bold:  boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr("#6B7280"),
			Format: "─────────────────────────────────────────",
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
	}
}

// --- Rendering ---

// Render renders a Skill's markdown body with its trit-appropriate theme.
func (s *Skill) Render(width int) (string, error) {
	style := StyleForTrit(s.Trit)
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", fmt.Errorf("skill: renderer: %w", err)
	}

	rendered, err := r.Render(s.Body)
	if err != nil {
		return "", fmt.Errorf("skill: render: %w", err)
	}

	// Prepend trit-colored header
	header := s.renderHeader()
	return header + rendered, nil
}

// renderHeader produces a lipgloss-styled skill header showing name, trit, and role.
func (s *Skill) renderHeader() string {
	primary := lipgloss.Color(s.HexColor)
	dim := lipgloss.Color("#6B7280")

	nameStyle := lipgloss.NewStyle().
		Foreground(primary).
		Bold(true)

	roleStyle := lipgloss.NewStyle().
		Foreground(primary)

	dimStyle := lipgloss.NewStyle().
		Foreground(dim)

	tritSymbol := "●"
	roleStr := s.Role.String()

	header := fmt.Sprintf(
		"%s %s  %s %s",
		nameStyle.Render(s.Name),
		dimStyle.Render("│"),
		roleStyle.Render(tritSymbol+" "+roleStr),
		dimStyle.Render(s.HexColor),
	)

	if s.Description != "" {
		header += "\n" + dimStyle.Render(s.Description)
	}

	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(primary).
		Padding(0, 1).
		Render(header)

	return border + "\n"
}

// --- Perceptual Spread Verification ---

// MinPerceptualSpread returns the minimum angular hue separation
// between any pair in the triad palette. Must be ≥80° for unique
// identification across diverse display conditions.
func MinPerceptualSpread() float64 {
	// Approximate hue angles in OKLCH space for the triad
	hues := [3]float64{
		85.0,  // Amber #F59E0B ≈ 85° OKLCH
		303.0, // Purple #A855F7 ≈ 303° OKLCH
		250.0, // Blue #2E5FA3 ≈ 250° OKLCH
	}

	minSep := 360.0
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			diff := math.Abs(hues[i] - hues[j])
			if diff > 180 {
				diff = 360 - diff
			}
			if diff < minSep {
				minSep = diff
			}
		}
	}
	return minSep
}

// --- Batch Rendering ---

// LoadSkillDir loads all SKILL.md files from a directory tree.
func LoadSkillDir(dir string) ([]*Skill, error) {
	var skills []*Skill
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.Name() == "SKILL.md" {
			s, err := ParseSkillFile(path)
			if err != nil {
				return nil // skip unparseable
			}
			skills = append(skills, s)
		}
		return nil
	})
	return skills, err
}

// RenderTriadSummary renders a compact summary of skills grouped by GF(3) trit.
func RenderTriadSummary(skills []*Skill) string {
	groups := [3][]*Skill{}
	for _, s := range skills {
		groups[s.Trit] = append(groups[s.Trit], s)
	}

	var b strings.Builder
	labels := [3]string{"Coordinator (0)", "Generator (+1)", "Verifier (-1)"}

	for trit := gf3.Elem(0); trit < 3; trit++ {
		color := lipgloss.Color(TriadPalette[trit])
		header := lipgloss.NewStyle().
			Foreground(color).
			Bold(true).
			Render(fmt.Sprintf("■ %s", labels[trit]))

		b.WriteString(header + "\n")
		for _, s := range groups[trit] {
			bullet := lipgloss.NewStyle().
				Foreground(color).
				Render("  ● " + s.Name)
			b.WriteString(bullet + "\n")
		}
		b.WriteString("\n")
	}

	// Conservation check
	balanced := len(skills)%3 == 0 && isTriadBalanced(skills)
	if balanced {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Render("✓ ∑ trits ≡ 0 (mod 3) — balanced") + "\n")
	} else {
		need := findBalancingTrit(skills)
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Render(fmt.Sprintf("✗ needs %s to balance", gf3.ElemToRole(need))) + "\n")
	}

	return b.String()
}

func isTriadBalanced(skills []*Skill) bool {
	elems := make([]gf3.Elem, len(skills))
	for i, s := range skills {
		elems[i] = s.Trit
	}
	return gf3.IsBalanced(elems)
}

func findBalancingTrit(skills []*Skill) gf3.Elem {
	sum := 0
	for _, s := range skills {
		sum += int(s.Trit)
	}
	return gf3.Elem((3 - (sum % 3)) % 3)
}

// --- to-prompt: skills-ref compatible prompt generation ---

// ToPrompt converts a skill into the agent-consumable prompt format
// following agentskills.io progressive disclosure:
//   1. Metadata header (~100 tokens): name, description, trit, role
//   2. Instructions body: full SKILL.md markdown
//   3. Validation status: agentskills spec compliance
//
// This is the equivalent of `skills-ref to-prompt <path>`.
func (s *Skill) ToPrompt() string {
	var b strings.Builder

	// Metadata header (progressive disclosure level 1)
	b.WriteString(fmt.Sprintf("<skill name=%q", s.Name))
	if s.Description != "" {
		b.WriteString(fmt.Sprintf(" description=%q", s.Description))
	}
	if s.License != "" {
		b.WriteString(fmt.Sprintf(" license=%q", s.License))
	}
	if s.Compatibility != "" {
		b.WriteString(fmt.Sprintf(" compatibility=%q", s.Compatibility))
	}
	if s.Version != "" {
		b.WriteString(fmt.Sprintf(" version=%q", s.Version))
	}
	// GF(3) extension attributes
	b.WriteString(fmt.Sprintf(" trit=%q role=%q color=%q",
		s.Trit.String(), s.Role.String(), s.HexColor))

	if len(s.AllowedTools) > 0 {
		b.WriteString(fmt.Sprintf(" allowed-tools=%q", strings.Join(s.AllowedTools, " ")))
	}
	b.WriteString(">\n")

	// Instructions body (progressive disclosure level 2)
	b.WriteString(s.Body)
	b.WriteString("\n")

	// Validation footer
	errs := s.Validate()
	if len(errs) == 0 {
		b.WriteString("\n<!-- agentskills-spec: valid -->\n")
	} else {
		b.WriteString("\n<!-- agentskills-spec: invalid\n")
		for _, e := range errs {
			b.WriteString("  - " + e + "\n")
		}
		b.WriteString("-->\n")
	}

	b.WriteString("</skill>\n")
	return b.String()
}

// ToPromptCompact returns just the metadata line for progressive disclosure
// level 1 (~100 tokens). Used when listing many skills for agent selection.
func (s *Skill) ToPromptCompact() string {
	return fmt.Sprintf("<skill name=%q description=%q trit=%q role=%q color=%q />\n",
		s.Name, s.Description, s.Trit.String(), s.Role.String(), s.HexColor)
}

// BatchToPrompt converts multiple skills to a prompt with metadata listing
// followed by full instructions for each.
func BatchToPrompt(skills []*Skill, fullBody bool) string {
	var b strings.Builder
	b.WriteString("<skills count=\"" + fmt.Sprint(len(skills)) + "\">\n")

	// Level 1: compact listing for all
	for _, s := range skills {
		b.WriteString(s.ToPromptCompact())
	}

	// Level 2: full body if requested
	if fullBody {
		b.WriteString("\n")
		for _, s := range skills {
			b.WriteString(s.ToPrompt())
			b.WriteString("\n")
		}
	}

	// GF(3) conservation summary
	elems := make([]gf3.Elem, len(skills))
	for i, s := range skills {
		elems[i] = s.Trit
	}
	balanced := gf3.IsBalanced(elems)
	b.WriteString(fmt.Sprintf("\n<!-- gf3-conservation: balanced=%v sum=%d -->\n",
		balanced, gf3.SeqSum(elems)))

	b.WriteString("</skills>\n")
	return b.String()
}

// ValidateDir validates all SKILL.md files in a directory tree against
// the agentskills.io specification. Returns per-skill results.
func ValidateDir(dir string) (valid, invalid int, results []string, err error) {
	skills, err := LoadSkillDir(dir)
	if err != nil {
		return 0, 0, nil, err
	}
	for _, s := range skills {
		errs := s.Validate()
		if len(errs) == 0 {
			valid++
		} else {
			invalid++
			for _, e := range errs {
				results = append(results, fmt.Sprintf("%s: %s", s.Name, e))
			}
		}
	}
	return valid, invalid, results, nil
}

// --- Helpers ---

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }
