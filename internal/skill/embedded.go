//go:build tinygo

// Package skill provides embedded-compatible skill validation for TinyGo MCU targets.
// This file removes glamour/lipgloss dependencies for firmware builds targeting
// STM32, nRF52840, RP2040, and ESP32 (where available).
//
// Uses only core Go stdlib: fmt, strings, filepath, crypto/sha256, encoding/binary.
// Suitable for medical device firmware with minimal binary footprint.
package skill

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
)

// EmbeddedSkill is the lightweight skill model for TinyGo MCU targets.
// Contains only what's necessary for validation and capability checking.
// Integrates OCAPN Sideref tokens for unforgeable capability authorization.
type EmbeddedSkill struct {
	Name        string // max 64 chars, lowercase+hyphens
	Description string // max 1024 chars, what skill does + when to use
	License     string // optional
	Compatibility string // optional, max 500 chars

	// Embedded-specific
	Path  string // filesystem path or resource ID
	Body  string // markdown instructions (advisory <500 lines, <5000 tokens)
	Trit  uint8  // GF(3) classification (0, 1, or 2)

	// OCAPN Capability Binding (Phase 1)
	Sideref *SiderefToken // Cryptographic authorization (unforgeable reference)
}

// ValidateEmbedded checks the skill against agentskills spec constraints.
// Returns nil if valid, or a list of validation errors.
//
// This is the TinyGo-compatible version with no reflection or fancy formatting.
func (s *EmbeddedSkill) ValidateEmbedded() []string {
	var errs []string

	// Name validation (required, 1-64 chars, lowercase+hyphens)
	if len(s.Name) == 0 {
		errs = append(errs, "name is required")
	} else if len(s.Name) > 64 {
		errs = append(errs, fmt.Sprintf("name too long: %d chars (max 64)", len(s.Name)))
	} else if !isValidSkillNameEmbedded(s.Name) {
		errs = append(errs, fmt.Sprintf("invalid name %q: must be lowercase alphanumeric and hyphens", s.Name))
	}

	// Description validation (required, 1-1024 chars)
	if len(s.Description) == 0 {
		errs = append(errs, "description is required")
	} else if len(s.Description) > 1024 {
		errs = append(errs, fmt.Sprintf("description too long: %d chars (max 1024)", len(s.Description)))
	}

	// Compatibility validation (optional, max 500 chars)
	if len(s.Compatibility) > 500 {
		errs = append(errs, fmt.Sprintf("compatibility too long: %d chars (max 500)", len(s.Compatibility)))
	}

	// Body validation (advisory: <500 lines, <5000 tokens)
	if len(s.Body) > 0 {
		lines := countLinesEmbedded(s.Body)
		if lines > 500 {
			errs = append(errs, fmt.Sprintf("body too long: %d lines (max 500)", lines))
		}
		tokens := estimateTokensEmbedded(len(s.Body))
		if tokens > 5000 {
			errs = append(errs, fmt.Sprintf("body exceeds token limit: ~%d tokens (recommended max 5000)", tokens))
		}
	}

	// Trit validation (0, 1, or 2 only)
	if s.Trit > 2 {
		errs = append(errs, fmt.Sprintf("invalid trit %d: must be 0, 1, or 2", s.Trit))
	}

	// OCAPN Sideref token validation (required for capability binding)
	if s.Sideref == nil {
		errs = append(errs, "sideref token required for OCAPN capability binding (unforgeable reference)")
	} else if s.Sideref.SkillName != s.Name {
		errs = append(errs, fmt.Sprintf("sideref skill name mismatch: token has %q, skill has %q", s.Sideref.SkillName, s.Name))
	}

	return errs
}

// isValidSkillNameEmbedded checks name constraints (lightweight version).
// Lowercase alphanumeric + hyphens, no leading/trailing/consecutive hyphens.
func isValidSkillNameEmbedded(name string) bool {
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

// countLinesEmbedded counts newlines in a string (lightweight version).
func countLinesEmbedded(s string) int {
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

// estimateTokensEmbedded estimates token count from character count.
// Conservative: ~4 characters per token.
func estimateTokensEmbedded(charCount int) int {
	const charsPerToken = 4
	return (charCount + charsPerToken - 1) / charsPerToken // ceiling division
}

// ComputeTriEmbedded computes the GF(3) trit from a skill name via SHA-256.
// Returns 0, 1, or 2 (never panics).
//
// SECURITY NOTE: This hash is deterministic and public, suitable only for capability
// classification. NOT suitable for security-critical operations (authentication,
// attestation, or firmware signing). For those, use ATECC608A secure element as
// documented in skills/embedded-medical-device/SKILL.md Part 4.
func ComputeTriEmbedded(name string) uint8 {
	h := sha256.Sum256([]byte(name))
	val := binary.LittleEndian.Uint64(h[0:8])
	return uint8(val % 3)
}

// ParseEmbeddedSkillLine parses a simple skill line format for resource-constrained parsing.
// Format: "name:description:trit"
//
// Example: "pdf-processing:Extract text from PDFs:1"
// This is useful for compact skill registries on MCU devices.
func ParseEmbeddedSkillLine(line string) (*EmbeddedSkill, error) {
	parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("skill: invalid format (want 'name:description[:trit]')")
	}

	s := &EmbeddedSkill{
		Name:        parts[0],
		Description: parts[1],
		Trit:        ComputeTriEmbedded(parts[0]),
	}

	if len(parts) == 3 {
		// Optional explicit trit override (for pre-computed values)
		var trit int
		_, err := fmt.Sscanf(parts[2], "%d", &trit)
		if err == nil && trit >= 0 && trit <= 2 {
			s.Trit = uint8(trit)
		}
	}

	return s, nil
}

// MaxSkillsPerDevice is the maximum number of skills a medical device firmware can register.
// Typical healthcare devices have 5-20 capabilities; 32 is a safe upper bound.
// Prevents memory exhaustion attacks on embedded firmware.
const MaxSkillsPerDevice = 32

// EmbeddedSkillRegistry holds a compact registry of skills for MCU enumeration.
type EmbeddedSkillRegistry struct {
	skills []*EmbeddedSkill
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *EmbeddedSkillRegistry {
	return &EmbeddedSkillRegistry{
		skills: make([]*EmbeddedSkill, 0, 32), // pre-allocate for ~32 skills
	}
}

// Register adds a skill to the registry.
// Returns error if name duplicates or registry is full (MaxSkillsPerDevice limit).
// Returns error to prevent resource exhaustion attacks on firmware.
func (r *EmbeddedSkillRegistry) Register(s *EmbeddedSkill) error {
	// Check for duplicates
	for _, existing := range r.skills {
		if existing.Name == s.Name {
			return fmt.Errorf("skill: duplicate name %q", s.Name)
		}
	}

	// Check registry capacity (resource exhaustion protection)
	if len(r.skills) >= MaxSkillsPerDevice {
		return fmt.Errorf("skill: registry full (%d skills, max %d)", len(r.skills), MaxSkillsPerDevice)
	}

	r.skills = append(r.skills, s)
	return nil
}

// Lookup finds a skill by name (binary search ready, but using linear for simplicity).
func (r *EmbeddedSkillRegistry) Lookup(name string) *EmbeddedSkill {
	for _, s := range r.skills {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// ByTrit returns all skills with a given trit.
func (r *EmbeddedSkillRegistry) ByTrit(trit uint8) []*EmbeddedSkill {
	var result []*EmbeddedSkill
	for _, s := range r.skills {
		if s.Trit == trit {
			result = append(result, s)
		}
	}
	return result
}

// Count returns the number of registered skills.
func (r *EmbeddedSkillRegistry) Count() int {
	return len(r.skills)
}

// IsBalanced checks if the sum of all trits ≡ 0 (mod 3).
// For medical device firmware: balanced skill sets satisfy GF(3) conservation.
func (r *EmbeddedSkillRegistry) IsBalanced() bool {
	sum := 0
	for _, s := range r.skills {
		sum += int(s.Trit)
	}
	return sum%3 == 0
}

// ValidateAll checks all registered skills and returns errors.
func (r *EmbeddedSkillRegistry) ValidateAll() []string {
	var errs []string
	for _, s := range r.skills {
		valErrs := s.ValidateEmbedded()
		if len(valErrs) > 0 {
			for _, e := range valErrs {
				errs = append(errs, fmt.Sprintf("%s: %s", s.Name, e))
			}
		}
	}
	return errs
}

// SerializeCompact produces a compact text representation suitable for EEPROM/flash.
// Format: one skill per line as "name:description:trit"
// Newlines are replaced with underscores in description to fit one line per skill.
func (r *EmbeddedSkillRegistry) SerializeCompact() string {
	var b strings.Builder
	for _, s := range r.skills {
		// Replace newlines with underscores for compact storage
		desc := strings.ReplaceAll(s.Description, "\n", "_")
		// Truncate description to 60 chars if needed (EEPROM constraint)
		if len(desc) > 60 {
			desc = desc[:60]
		}
		b.WriteString(fmt.Sprintf("%s:%s:%d\n", s.Name, desc, s.Trit))
	}
	return b.String()
}

// BindSideref binds an OCAPN Sideref token to the skill for capability authorization.
// The token becomes unforgeable once bound to the device secret.
func (s *EmbeddedSkill) BindSideref(deviceSecret [16]byte) {
	s.Sideref = NewSiderefToken(s.Name, deviceSecret)
}

// VerifySiderefBinding checks if the Sideref token is valid for this skill and device.
// Returns error if token is missing, mismatched, or invalid.
func (s *EmbeddedSkill) VerifySiderefBinding(deviceSecret [16]byte) error {
	if s.Sideref == nil {
		return fmt.Errorf("skill %q: no sideref token bound", s.Name)
	}
	return VerifySideref(s.Sideref, s.Name, deviceSecret)
}

// CompactFormat is the wire format for skill capabilities over serial/BLE.
// 2 bytes: [name_len:1][trit:1] followed by name string.
// Used for firmware advertisement over IEEE 11073 or custom medical protocols.
type CompactFormat struct {
	NameLen uint8
	Trit    uint8
	Name    [64]byte // max name length
}

// MarshalCompact encodes a skill into CompactFormat for wire transmission.
func (s *EmbeddedSkill) MarshalCompact() CompactFormat {
	cf := CompactFormat{
		NameLen: uint8(len(s.Name)),
		Trit:    s.Trit,
	}
	copy(cf.Name[:], s.Name)
	return cf
}

// UnmarshalCompact decodes CompactFormat into an EmbeddedSkill.
func UnmarshalCompact(cf CompactFormat) *EmbeddedSkill {
	if cf.NameLen > 64 || cf.Trit > 2 {
		return nil // invalid
	}
	name := string(cf.Name[:cf.NameLen])
	return &EmbeddedSkill{
		Name: name,
		Trit: cf.Trit,
	}
}
