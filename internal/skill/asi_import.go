// Package skill provides ASI skill registry integration for Phase 3.
// Implements GF(3)-balanced skill selection from the 315-skill ASI registry.

package skill

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ASISkill represents a skill from the 315-skill ASI registry.
// Integrates with OCAPN Sideref tokens for capability binding.
type ASISkill struct {
	ID          string `json:"id"`                    // Unique skill identifier
	Name        string `json:"name"`                  // Skill name (matches embedded SKILL.md)
	Description string `json:"description"`          // What the skill does
	Trit        uint8  `json:"trit"`                  // GF(3) value (0, 1, or 2)
	Role        string `json:"role"`                  // Role name (Coordinator, Generator, Verifier)
	Category    string `json:"category"`              // Skill category for organization
	Sideref     string `json:"sideref_hex,omitempty"` // Optional: OCAPN Sideref token (hex)
}

// ASIRegistry holds the 315-skill ASI registry with balance tracking.
// Enforces GF(3) conservation: sum of trits ≡ 0 (mod 3).
type ASIRegistry struct {
	skills           []ASISkill
	tritCounts       [3]int // Count of each trit value
	tritSum          int
	balanced         bool
	selectedSubset   []ASISkill // Current balanced selection
	subsetTrits      [3]int
	subsetSum        int
}

// NewASIRegistry creates an empty ASI registry.
// An empty registry is balanced (0 ≡ 0 mod 3).
func NewASIRegistry() *ASIRegistry {
	return &ASIRegistry{
		skills:   make([]ASISkill, 0, 315),
		balanced: true, // empty registry is balanced
	}
}

// AddSkill adds a skill to the registry and updates balance state.
// Returns error if skill would exceed 315 limit or if name duplicates.
func (r *ASIRegistry) AddSkill(skill ASISkill) error {
	if len(r.skills) >= 315 {
		return fmt.Errorf("asi: registry full (315 skills max)")
	}

	// Check for duplicates
	for _, s := range r.skills {
		if s.Name == skill.Name {
			return fmt.Errorf("asi: duplicate skill name %q", skill.Name)
		}
	}

	// Validate trit value
	if skill.Trit > 2 {
		return fmt.Errorf("asi: invalid trit %d for skill %q (must be 0, 1, or 2)", skill.Trit, skill.Name)
	}

	// Add to registry
	r.skills = append(r.skills, skill)

	// Update balance tracking
	r.tritCounts[skill.Trit]++
	r.tritSum += int(skill.Trit)
	r.balanced = r.tritSum%3 == 0

	// Set role based on trit
	if skill.Role == "" {
		skill.Role = getTritRole(skill.Trit)
		r.skills[len(r.skills)-1].Role = skill.Role
	}

	return nil
}

// GetRole returns the human-readable role for a trit value.
func getTritRole(trit uint8) string {
	switch trit {
	case 0:
		return "Coordinator"
	case 1:
		return "Generator"
	case 2:
		return "Verifier"
	default:
		return "Unknown"
	}
}

// IsBalanced returns true if registry GF(3) sum ≡ 0 (mod 3).
func (r *ASIRegistry) IsBalanced() bool {
	return r.balanced
}

// TriadStatus returns the current trit distribution and balance info.
func (r *ASIRegistry) TriadStatus() map[string]interface{} {
	return map[string]interface{}{
		"total_skills": len(r.skills),
		"coordinators": r.tritCounts[0],
		"generators":   r.tritCounts[1],
		"verifiers":    r.tritCounts[2],
		"trit_sum":     r.tritSum,
		"balanced":     r.balanced,
		"equilibrium":  r.tritSum % 3 == 0,
	}
}

// SelectBalancedSubset selects a balanced subset of N skills where sum ≡ 0 (mod 3).
// Uses greedy algorithm to select diverse trits while maintaining balance.
// Returns selected skills or error if impossible.
func (r *ASIRegistry) SelectBalancedSubset(n int) ([]ASISkill, error) {
	if n > len(r.skills) {
		return nil, fmt.Errorf("asi: requested %d skills but registry has only %d", n, len(r.skills))
	}

	if n%3 != 0 {
		// Try to find closest balanced size (multiple of 3 for perfect balance)
		possibleSizes := []int{}
		for i := 0; i <= n; i++ {
			if i%3 == 0 {
				possibleSizes = append(possibleSizes, i)
			}
		}
		if len(possibleSizes) > 0 {
			n = possibleSizes[len(possibleSizes)-1]
		}
	}

	// Sort skills by trit for balanced selection
	sorted := make([]ASISkill, len(r.skills))
	copy(sorted, r.skills)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Trit != sorted[j].Trit {
			return sorted[i].Trit < sorted[j].Trit
		}
		return sorted[i].Name < sorted[j].Name
	})

	// Greedy selection: take balanced distribution
	selected := []ASISkill{}
	counts := [3]int{}

	// First pass: take from each trit equally
	itemsPerTrit := n / 3
	for _, skill := range sorted {
		if counts[skill.Trit] < itemsPerTrit {
			selected = append(selected, skill)
			counts[skill.Trit]++
		}
	}

	// Second pass: fill remaining slots maintaining balance
	remainder := n - len(selected)
	for _, skill := range sorted {
		if remainder == 0 {
			break
		}
		// Check if already selected
		alreadySelected := false
		for _, s := range selected {
			if s.Name == skill.Name {
				alreadySelected = true
				break
			}
		}
		if !alreadySelected {
			selected = append(selected, skill)
			remainder--
		}
	}

	// Verify balance
	sum := 0
	for _, s := range selected {
		sum += int(s.Trit)
	}
	if sum%3 != 0 {
		return nil, fmt.Errorf("asi: could not create balanced subset of %d skills", n)
	}

	r.selectedSubset = selected
	return selected, nil
}

// FindSkillsByTrit returns all skills with a given trit value.
func (r *ASIRegistry) FindSkillsByTrit(trit uint8) []ASISkill {
	var result []ASISkill
	for _, s := range r.skills {
		if s.Trit == trit {
			result = append(result, s)
		}
	}
	return result
}

// FindSkill finds a skill by name.
func (r *ASIRegistry) FindSkill(name string) *ASISkill {
	for i, s := range r.skills {
		if s.Name == name {
			return &r.skills[i]
		}
	}
	return nil
}

// Count returns the number of skills in the registry.
func (r *ASIRegistry) Count() int {
	return len(r.skills)
}

// ToJSON exports the registry as JSON.
func (r *ASIRegistry) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r.skills, "", "  ")
}

// FromJSON imports a registry from JSON.
func (r *ASIRegistry) FromJSON(data []byte) error {
	var skills []ASISkill
	if err := json.Unmarshal(data, &skills); err != nil {
		return fmt.Errorf("asi: failed to parse JSON: %w", err)
	}

	for _, skill := range skills {
		if err := r.AddSkill(skill); err != nil {
			return err
		}
	}

	return nil
}

// BindSiderefs binds OCAPN Sideref tokens to all skills in the registry.
// Each skill gets a unique token based on device secret.
func (r *ASIRegistry) BindSiderefs(deviceSecret [16]byte) error {
	for i := range r.skills {
		token := NewSiderefToken(r.skills[i].Name, deviceSecret)
		// Store token as hex string in registry
		r.skills[i].Sideref = fmt.Sprintf("%x", token.Token[:])
	}
	return nil
}

// VerifySideref checks if a skill's Sideref token is valid.
func (r *ASIRegistry) VerifySideref(skillName string, deviceSecret [16]byte) (bool, error) {
	skill := r.FindSkill(skillName)
	if skill == nil {
		return false, fmt.Errorf("asi: skill %q not found", skillName)
	}

	// If Sideref is not bound, return false
	if skill.Sideref == "" {
		return false, nil
	}

	// Regenerate token and compare
	expectedToken := NewSiderefToken(skillName, deviceSecret)
	return skill.Sideref == fmt.Sprintf("%x", expectedToken.Token[:]), nil
}

// ExportForEmbedded converts selected subset to EmbeddedSkill format for firmware.
func (r *ASIRegistry) ExportForEmbedded(deviceSecret [16]byte) ([]*EmbeddedSkill, error) {
	if len(r.selectedSubset) == 0 {
		return nil, fmt.Errorf("asi: no balanced subset selected (call SelectBalancedSubset first)")
	}

	var embedded []*EmbeddedSkill

	for _, asiSkill := range r.selectedSubset {
		embSkill := &EmbeddedSkill{
			Name:        asiSkill.Name,
			Description: asiSkill.Description,
			Trit:        asiSkill.Trit,
		}

		// Bind Sideref for capability authorization
		embSkill.BindSideref(deviceSecret)

		embedded = append(embedded, embSkill)
	}

	return embedded, nil
}
