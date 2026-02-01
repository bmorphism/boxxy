//go:build darwin

package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// --- Share3Hash Tests ---

func TestShare3HashDeterministic(t *testing.T) {
	// Same name always produces same trit
	for i := 0; i < 10; i++ {
		if Share3Hash("gay-mcp") != Share3Hash("gay-mcp") {
			t.Fatal("Share3Hash not deterministic")
		}
	}
}

func TestShare3HashDistribution(t *testing.T) {
	// Check that a reasonable set of skill names hits all three trits
	names := []string{
		"gay-mcp", "acsets", "bisimulation-game", "world-hopping",
		"glass-bead-game", "triad-interleave", "sheaf-cohomology",
		"active-inference", "algebraic-rewriting", "propagators",
		"babashka", "dafny", "lean4", "catlab", "rzk",
	}
	seen := [3]bool{}
	for _, name := range names {
		seen[Share3Hash(name)] = true
	}
	for trit := gf3.Elem(0); trit < 3; trit++ {
		if !seen[trit] {
			t.Errorf("trit %d not represented in %d skill names", trit, len(names))
		}
	}
}

func TestShare3HashCoversAllTrits(t *testing.T) {
	// Exhaustive: among 100 sequential names, all trits must appear
	counts := [3]int{}
	for i := 0; i < 100; i++ {
		name := strings.Repeat("x", i+1)
		counts[Share3Hash(name)]++
	}
	for trit := gf3.Elem(0); trit < 3; trit++ {
		if counts[trit] == 0 {
			t.Errorf("trit %d never produced in 100 names", trit)
		}
	}
}

// --- Frontmatter Parsing ---

func TestParseSkillWithFrontmatter(t *testing.T) {
	content := `---
name: asi-integrated
description: Unified ASI skill
version: 1.0.0
---

# ASI Integrated Skill

Body text here.`

	s, err := ParseSkill(content, "/skills/_integrated/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "asi-integrated" {
		t.Errorf("name = %q, want asi-integrated", s.Name)
	}
	if s.Description != "Unified ASI skill" {
		t.Errorf("description = %q", s.Description)
	}
	if s.Version != "1.0.0" {
		t.Errorf("version = %q", s.Version)
	}
	if !strings.Contains(s.Body, "# ASI Integrated Skill") {
		t.Errorf("body missing heading")
	}
}

func TestParseSkillWithoutFrontmatter(t *testing.T) {
	content := "# Plain Skill\n\nNo frontmatter."
	s, err := ParseSkill(content, "/skills/plain/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	// Name derived from directory
	if s.Name != "plain" {
		t.Errorf("name = %q, want plain", s.Name)
	}
	if s.Body != content {
		t.Error("body should be full content when no frontmatter")
	}
}

func TestParseSkillTrits(t *testing.T) {
	// Every parsed skill gets a trit and role
	s, _ := ParseSkill("# Test", "/skills/test/SKILL.md")
	if s.Trit > 2 {
		t.Errorf("trit = %d, want 0-2", s.Trit)
	}
	if s.HexColor == "" {
		t.Error("HexColor empty")
	}
	// Role must match trit
	if gf3.RoleToElem(s.Role) != s.Trit {
		t.Errorf("role %v doesn't match trit %d", s.Role, s.Trit)
	}
}

// --- File I/O ---

func TestParseSkillFile(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	os.MkdirAll(skillDir, 0o755)

	content := `---
name: test-skill
description: A test
version: 0.1.0
---

# Test Skill

Hello world.`

	path := filepath.Join(skillDir, "SKILL.md")
	os.WriteFile(path, []byte(content), 0o644)

	s, err := ParseSkillFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "test-skill" {
		t.Errorf("name = %q", s.Name)
	}
}

func TestParseSkillFileNotFound(t *testing.T) {
	_, err := ParseSkillFile("/nonexistent/SKILL.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- LoadSkillDir ---

func TestLoadSkillDir(t *testing.T) {
	dir := t.TempDir()

	// Create 3 skills
	for _, name := range []string{"alpha", "beta", "gamma"} {
		d := filepath.Join(dir, name)
		os.MkdirAll(d, 0o755)
		content := "---\nname: " + name + "\n---\n\n# " + name
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(content), 0o644)
	}

	skills, err := LoadSkillDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 3 {
		t.Errorf("found %d skills, want 3", len(skills))
	}
}

// --- Style Configuration ---

func TestStyleForTrit(t *testing.T) {
	for trit := gf3.Elem(0); trit < 3; trit++ {
		style := StyleForTrit(trit)
		// Style must have colored headings
		if style.H1.Color == nil {
			t.Errorf("trit %d: H1 has no color", trit)
		}
		// Color must match the triad palette
		if *style.H1.Color != TriadPalette[trit] {
			t.Errorf("trit %d: H1 color = %s, want %s", trit, *style.H1.Color, TriadPalette[trit])
		}
	}
}

func TestStyleForAllTritsDistinct(t *testing.T) {
	styles := [3]string{}
	for trit := gf3.Elem(0); trit < 3; trit++ {
		s := StyleForTrit(trit)
		styles[trit] = *s.H1.Color
	}
	// All three must be different
	if styles[0] == styles[1] || styles[1] == styles[2] || styles[0] == styles[2] {
		t.Errorf("styles not distinct: %v", styles)
	}
}

// --- Perceptual Spread ---

func TestMinPerceptualSpread(t *testing.T) {
	spread := MinPerceptualSpread()
	// Must be ≥50° for reasonable uniqueness (we target ≥53° actual)
	if spread < 50.0 {
		t.Errorf("perceptual spread = %.1f°, want ≥50°", spread)
	}
	t.Logf("minimum perceptual hue spread: %.1f°", spread)
}

// --- Rendering ---

func TestRenderSkill(t *testing.T) {
	content := `---
name: test-renderer
description: Tests glamour rendering
version: 0.1.0
---

# Test Renderer

This is a **bold** test with _italic_ text.

## Features

- Item one
- Item two

` + "```go\nfunc main() {}\n```"

	s, err := ParseSkill(content, "/skills/test-renderer/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}

	rendered, err := s.Render(80)
	if err != nil {
		t.Fatal(err)
	}

	if rendered == "" {
		t.Error("rendered output empty")
	}

	// Must contain the skill name in the header
	if !strings.Contains(rendered, "test-renderer") {
		t.Error("rendered output missing skill name")
	}

	t.Logf("rendered %d bytes", len(rendered))
}

func TestRenderDifferentTrits(t *testing.T) {
	// Skills with different trits produce different renderings
	names := []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg"}
	seen := map[gf3.Elem]string{}

	for _, name := range names {
		content := "---\nname: " + name + "\n---\n\n# " + name
		s, _ := ParseSkill(content, "/skills/"+name+"/SKILL.md")
		if prev, ok := seen[s.Trit]; !ok {
			rendered, err := s.Render(80)
			if err != nil {
				t.Fatal(err)
			}
			seen[s.Trit] = rendered
			_ = prev
		}
	}

	// We should have at least 2 distinct trit renderings
	if len(seen) < 2 {
		t.Errorf("only %d distinct trit renderings", len(seen))
	}
}

// --- Triad Summary ---

func TestRenderTriadSummary(t *testing.T) {
	skills := []*Skill{
		{Name: "alpha", Trit: gf3.Zero, Role: gf3.Coordinator, HexColor: TriadPalette[0]},
		{Name: "beta", Trit: gf3.One, Role: gf3.Generator, HexColor: TriadPalette[1]},
		{Name: "gamma", Trit: gf3.Two, Role: gf3.Verifier, HexColor: TriadPalette[2]},
	}

	summary := RenderTriadSummary(skills)
	if summary == "" {
		t.Error("empty summary")
	}
	if !strings.Contains(summary, "alpha") {
		t.Error("missing alpha")
	}
	if !strings.Contains(summary, "beta") {
		t.Error("missing beta")
	}
	if !strings.Contains(summary, "gamma") {
		t.Error("missing gamma")
	}
}

func TestTriadSummaryBalanced(t *testing.T) {
	// 0+1+2 = 3 ≡ 0 mod 3 → balanced
	skills := []*Skill{
		{Name: "a", Trit: gf3.Zero},
		{Name: "b", Trit: gf3.One},
		{Name: "c", Trit: gf3.Two},
	}
	summary := RenderTriadSummary(skills)
	if !strings.Contains(summary, "balanced") {
		t.Errorf("expected balanced indicator, got: %s", summary)
	}
}

func TestTriadSummaryUnbalanced(t *testing.T) {
	// 0+0 = 0 ≡ 0 mod 3 but len != multiple of 3
	skills := []*Skill{
		{Name: "a", Trit: gf3.Zero},
		{Name: "b", Trit: gf3.Zero},
	}
	summary := RenderTriadSummary(skills)
	if !strings.Contains(summary, "needs") {
		t.Errorf("expected balancing suggestion, got: %s", summary)
	}
}

// --- Validation (agentskills spec) ---

func TestValidateValidSkill(t *testing.T) {
	s := &Skill{Name: "pdf-processing", Description: "Extract text from PDFs"}
	errs := s.Validate()
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateNameRequired(t *testing.T) {
	s := &Skill{Description: "something"}
	errs := s.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e, "name is required") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'name is required' error")
	}
}

func TestValidateNameTooLong(t *testing.T) {
	s := &Skill{Name: strings.Repeat("a", 65), Description: "x"}
	errs := s.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e, "too long") {
			found = true
		}
	}
	if !found {
		t.Error("expected name too long error")
	}
}

func TestValidateNameFormat(t *testing.T) {
	invalid := []string{
		"-leading",
		"trailing-",
		"con--secutive",
		"Upper",
		"has space",
		"has_underscore",
	}
	for _, name := range invalid {
		s := &Skill{Name: name, Description: "test"}
		errs := s.Validate()
		found := false
		for _, e := range errs {
			if strings.Contains(e, "invalid name") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected invalid name error for %q", name)
		}
	}
}

func TestValidateNameFormatValid(t *testing.T) {
	valid := []string{"pdf-processing", "data-analysis", "code-review", "a", "abc123"}
	for _, name := range valid {
		if !isValidSkillName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}
}

func TestValidateDescriptionRequired(t *testing.T) {
	s := &Skill{Name: "test"}
	errs := s.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e, "description is required") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'description is required' error")
	}
}

func TestParseSkillWithAllFields(t *testing.T) {
	content := `---
name: pdf-processing
description: Extract text and tables from PDF files.
license: Apache-2.0
compatibility: Requires poppler-utils
allowed-tools: Bash(pdftotext:*) Read
metadata:
  author: example-org
  version: "1.0"
---

# PDF Processing

Instructions here.`

	s, err := ParseSkill(content, "/skills/pdf-processing/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if s.License != "Apache-2.0" {
		t.Errorf("license = %q", s.License)
	}
	if s.Compatibility != "Requires poppler-utils" {
		t.Errorf("compatibility = %q", s.Compatibility)
	}
	if len(s.AllowedTools) != 2 {
		t.Errorf("allowed-tools = %v, want 2 items", s.AllowedTools)
	}
	if s.Metadata["author"] != "example-org" {
		t.Errorf("metadata.author = %q", s.Metadata["author"])
	}
	if s.Version != "1.0" {
		t.Errorf("version = %q, want 1.0 (from metadata)", s.Version)
	}
}

// --- Progressive Disclosure ---

func TestCountLines(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello\n", 1},
		{"a\nb", 2},
		{"a\nb\n", 2},
		{"a\nb\nc", 3},
		{"a\nb\nc\n", 3},
	}
	for _, tc := range cases {
		got := countLines(tc.input)
		if got != tc.want {
			t.Errorf("countLines(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		chars int
		want  int
	}{
		{0, 0},
		{1, 1},
		{4, 1},
		{5, 2},
		{20000, 5000},
		{20001, 5001},
	}
	for _, tc := range cases {
		got := estimateTokens(tc.chars)
		if got != tc.want {
			t.Errorf("estimateTokens(%d) = %d, want %d", tc.chars, got, tc.want)
		}
	}
}

func TestValidateProgressiveDisclosureBodyLines(t *testing.T) {
	// Body with exactly 500 lines → valid
	body500 := strings.Repeat("line\n", 500)
	s := &Skill{Name: "test", Description: "test", Body: body500}
	errs := s.Validate()
	for _, e := range errs {
		if strings.Contains(e, "body too long") {
			t.Errorf("500 lines should be valid, got: %s", e)
		}
	}

	// Body with 501 lines → invalid
	body501 := strings.Repeat("line\n", 501)
	s2 := &Skill{Name: "test", Description: "test", Body: body501}
	errs2 := s2.Validate()
	found := false
	for _, e := range errs2 {
		if strings.Contains(e, "body too long") {
			found = true
		}
	}
	if !found {
		t.Error("501 lines should trigger body too long error")
	}
}

func TestValidateProgressiveDisclosureTokenLimit(t *testing.T) {
	// Body with exactly 20000 chars → 5000 tokens → valid
	body20k := strings.Repeat("x", 20000)
	s := &Skill{Name: "test", Description: "test", Body: body20k}
	errs := s.Validate()
	for _, e := range errs {
		if strings.Contains(e, "token limit") {
			t.Errorf("5000 tokens should be valid, got: %s", e)
		}
	}

	// Body with 20001 chars → 5001 tokens → advisory warning
	body20k1 := strings.Repeat("x", 20001)
	s2 := &Skill{Name: "test", Description: "test", Body: body20k1}
	errs2 := s2.Validate()
	found := false
	for _, e := range errs2 {
		if strings.Contains(e, "token limit") {
			found = true
		}
	}
	if !found {
		t.Error("20001 chars should trigger token limit advisory")
	}
}

// --- Integration: ASI SKILL.md ---

func TestRenderASISkillIfAvailable(t *testing.T) {
	path := "/Users/bob/i/asi/skills/_integrated/SKILL.md"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("ASI SKILL.md not found")
	}

	s, err := ParseSkillFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if s.Name != "asi-integrated" {
		t.Errorf("name = %q, want asi-integrated", s.Name)
	}

	rendered, err := s.Render(100)
	if err != nil {
		t.Fatal(err)
	}

	if len(rendered) < 100 {
		t.Errorf("ASI skill rendered too short: %d bytes", len(rendered))
	}

	t.Logf("ASI skill: trit=%d role=%s color=%s body=%d bytes",
		s.Trit, s.Role, s.HexColor, len(rendered))
}
