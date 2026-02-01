package skill

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestNewASIRegistry creates an empty registry.
func TestNewASIRegistry(t *testing.T) {
	reg := NewASIRegistry()
	if reg == nil {
		t.Fatal("NewASIRegistry returned nil")
	}
	if reg.Count() != 0 {
		t.Errorf("expected empty registry, got %d skills", reg.Count())
	}
	if !reg.IsBalanced() {
		t.Error("empty registry should be balanced (0 ≡ 0 mod 3)")
	}
}

// TestAddSkill adds skills to registry and tracks balance.
func TestAddSkill(t *testing.T) {
	reg := NewASIRegistry()

	skill1 := ASISkill{ID: "s1", Name: "skill-one", Trit: 0}
	skill2 := ASISkill{ID: "s2", Name: "skill-two", Trit: 1}
	skill3 := ASISkill{ID: "s3", Name: "skill-three", Trit: 2}

	if err := reg.AddSkill(skill1); err != nil {
		t.Fatalf("failed to add skill1: %v", err)
	}
	if err := reg.AddSkill(skill2); err != nil {
		t.Fatalf("failed to add skill2: %v", err)
	}
	if err := reg.AddSkill(skill3); err != nil {
		t.Fatalf("failed to add skill3: %v", err)
	}

	if reg.Count() != 3 {
		t.Errorf("expected 3 skills, got %d", reg.Count())
	}

	if !reg.IsBalanced() {
		t.Errorf("registry should be balanced: sum=%d, mod 3 = %d", 0+1+2, (0+1+2)%3)
	}
}

// TestAddSkillDuplicate rejects duplicate skill names.
func TestAddSkillDuplicate(t *testing.T) {
	reg := NewASIRegistry()

	skill1 := ASISkill{ID: "s1", Name: "glucose-monitor", Trit: 0}
	skill2 := ASISkill{ID: "s2", Name: "glucose-monitor", Trit: 1}

	if err := reg.AddSkill(skill1); err != nil {
		t.Fatalf("failed to add first skill: %v", err)
	}

	err := reg.AddSkill(skill2)
	if err == nil {
		t.Error("expected error for duplicate skill name")
	}
}

// TestAddSkillInvalidTrit rejects invalid trit values.
func TestAddSkillInvalidTrit(t *testing.T) {
	reg := NewASIRegistry()
	skill := ASISkill{ID: "s1", Name: "invalid-trit", Trit: 5}

	err := reg.AddSkill(skill)
	if err == nil {
		t.Error("expected error for invalid trit value")
	}
}

// TestTriadStatus returns correct distribution.
func TestTriadStatus(t *testing.T) {
	reg := NewASIRegistry()

	// Add 2 coordinators, 1 generator, 1 verifier
	reg.AddSkill(ASISkill{Name: "coord1", Trit: 0})
	reg.AddSkill(ASISkill{Name: "coord2", Trit: 0})
	reg.AddSkill(ASISkill{Name: "gen1", Trit: 1})
	reg.AddSkill(ASISkill{Name: "ver1", Trit: 2})

	status := reg.TriadStatus()
	if total, ok := status["total_skills"].(int); ok {
		if total != 4 {
			t.Errorf("expected 4 skills, got %d", total)
		}
	} else {
		t.Error("total_skills not an int")
	}

	if coords, ok := status["coordinators"].(int); ok {
		if coords != 2 {
			t.Errorf("expected 2 coordinators, got %d", coords)
		}
	}
}

// TestIsBalanced validates GF(3) equilibrium.
func TestIsBalanced(t *testing.T) {
	reg := NewASIRegistry()

	// Start balanced (0 ≡ 0 mod 3)
	if !reg.IsBalanced() {
		t.Error("empty registry should be balanced")
	}

	// Add one from each trit
	reg.AddSkill(ASISkill{Name: "s1", Trit: 0})
	reg.AddSkill(ASISkill{Name: "s2", Trit: 1})
	reg.AddSkill(ASISkill{Name: "s3", Trit: 2})

	// Sum = 0 + 1 + 2 = 3 ≡ 0 (mod 3)
	if !reg.IsBalanced() {
		t.Error("registry with sum 3 should be balanced")
	}

	// Add another generator (sum becomes 4, not balanced)
	reg.AddSkill(ASISkill{Name: "s4", Trit: 1})
	if reg.IsBalanced() {
		t.Error("registry with sum 4 should not be balanced")
	}
}

// TestFindSkillsByTrit filters skills by trit value.
func TestFindSkillsByTrit(t *testing.T) {
	reg := NewASIRegistry()

	reg.AddSkill(ASISkill{Name: "coord1", Trit: 0})
	reg.AddSkill(ASISkill{Name: "coord2", Trit: 0})
	reg.AddSkill(ASISkill{Name: "gen1", Trit: 1})

	tritZero := reg.FindSkillsByTrit(0)
	if len(tritZero) != 2 {
		t.Errorf("expected 2 trit-0 skills, got %d", len(tritZero))
	}

	tritOne := reg.FindSkillsByTrit(1)
	if len(tritOne) != 1 {
		t.Errorf("expected 1 trit-1 skill, got %d", len(tritOne))
	}
}

// TestFindSkill locates a skill by name.
func TestFindSkill(t *testing.T) {
	reg := NewASIRegistry()
	reg.AddSkill(ASISkill{Name: "glucose-monitor", Trit: 1})

	found := reg.FindSkill("glucose-monitor")
	if found == nil {
		t.Fatal("expected to find glucose-monitor")
	}
	if found.Trit != 1 {
		t.Errorf("expected trit 1, got %d", found.Trit)
	}

	notFound := reg.FindSkill("nonexistent")
	if notFound != nil {
		t.Error("expected nil for nonexistent skill")
	}
}

// TestSelectBalancedSubset selects balanced subset from larger registry.
func TestSelectBalancedSubset(t *testing.T) {
	reg := NewASIRegistry()

	// Create a registry with 9 skills (3 of each trit)
	for i := 0; i < 3; i++ {
		reg.AddSkill(ASISkill{Name: "coord" + string(rune('0'+i)), Trit: 0})
		reg.AddSkill(ASISkill{Name: "gen" + string(rune('0'+i)), Trit: 1})
		reg.AddSkill(ASISkill{Name: "ver" + string(rune('0'+i)), Trit: 2})
	}

	// Select balanced subset of 6
	selected, err := reg.SelectBalancedSubset(6)
	if err != nil {
		t.Fatalf("SelectBalancedSubset failed: %v", err)
	}

	if len(selected) != 6 {
		t.Errorf("expected 6 skills, got %d", len(selected))
	}

	// Verify balance
	sum := 0
	for _, s := range selected {
		sum += int(s.Trit)
	}
	if sum%3 != 0 {
		t.Errorf("selected subset not balanced: sum=%d, mod 3=%d", sum, sum%3)
	}
}

// TestSelectBalancedSubset_TooLarge rejects request larger than registry.
func TestSelectBalancedSubset_TooLarge(t *testing.T) {
	reg := NewASIRegistry()
	reg.AddSkill(ASISkill{Name: "s1", Trit: 0})

	_, err := reg.SelectBalancedSubset(100)
	if err == nil {
		t.Error("expected error for subset larger than registry")
	}
}

// TestToJSON exports registry as JSON.
func TestToJSON(t *testing.T) {
	reg := NewASIRegistry()
	reg.AddSkill(ASISkill{ID: "s1", Name: "skill1", Trit: 0})
	reg.AddSkill(ASISkill{ID: "s2", Name: "skill2", Trit: 1})

	data, err := reg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("ToJSON returned empty data")
	}

	// Verify it's valid JSON
	var parsed []map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON output not valid: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("expected 2 skills in JSON, got %d", len(parsed))
	}
}

// TestFromJSON imports registry from JSON.
func TestFromJSON(t *testing.T) {
	jsonData := []byte(`[
    {"id": "s1", "name": "skill1", "trit": 0},
    {"id": "s2", "name": "skill2", "trit": 1},
    {"id": "s3", "name": "skill3", "trit": 2}
  ]`)

	reg := NewASIRegistry()
	err := reg.FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if reg.Count() != 3 {
		t.Errorf("expected 3 skills after import, got %d", reg.Count())
	}

	if !reg.IsBalanced() {
		t.Error("imported registry should be balanced")
	}
}

// TestBindSiderefs binds Sideref tokens to all skills.
func TestBindSiderefs(t *testing.T) {
	reg := NewASIRegistry()
	reg.AddSkill(ASISkill{Name: "glucose-monitor", Trit: 0})
	reg.AddSkill(ASISkill{Name: "heart-rate", Trit: 1})

	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	err := reg.BindSiderefs(deviceSecret)
	if err != nil {
		t.Fatalf("BindSiderefs failed: %v", err)
	}

	// Verify siderefs were bound
	skill := reg.FindSkill("glucose-monitor")
	if skill == nil {
		t.Fatal("skill not found")
	}
	if skill.Sideref == "" {
		t.Error("Sideref not bound to skill")
	}
	if len(skill.Sideref) != 64 {
		t.Errorf("Sideref hex should be 64 chars (32 bytes), got %d", len(skill.Sideref))
	}
}

// TestVerifySideref validates Sideref token for a skill.
func TestVerifySideref(t *testing.T) {
	reg := NewASIRegistry()
	reg.AddSkill(ASISkill{Name: "glucose-monitor", Trit: 0})

	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	reg.BindSiderefs(deviceSecret)

	// Verify with correct device secret
	valid, err := reg.VerifySideref("glucose-monitor", deviceSecret)
	if err != nil {
		t.Fatalf("VerifySideref failed: %v", err)
	}
	if !valid {
		t.Error("VerifySideref should return true for correct device secret")
	}

	// Verify with wrong device secret
	wrongSecret := [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	valid, err = reg.VerifySideref("glucose-monitor", wrongSecret)
	if err != nil {
		t.Fatalf("VerifySideref failed: %v", err)
	}
	if valid {
		t.Error("VerifySideref should return false for wrong device secret")
	}
}

// TestExportForEmbedded converts ASI skills to EmbeddedSkill format.
func TestExportForEmbedded(t *testing.T) {
	reg := NewASIRegistry()
	reg.AddSkill(ASISkill{Name: "glucose-monitor", Description: "Blood glucose tracking", Trit: 0})
	reg.AddSkill(ASISkill{Name: "heart-rate", Description: "Heart rate monitoring", Trit: 1})
	reg.AddSkill(ASISkill{Name: "step-counter", Description: "Step counting", Trit: 2})

	// Select balanced subset
	_, err := reg.SelectBalancedSubset(3)
	if err != nil {
		t.Fatalf("SelectBalancedSubset failed: %v", err)
	}

	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	embedded, err := reg.ExportForEmbedded(deviceSecret)
	if err != nil {
		t.Fatalf("ExportForEmbedded failed: %v", err)
	}

	if len(embedded) != 3 {
		t.Errorf("expected 3 embedded skills, got %d", len(embedded))
	}

	// Verify each embedded skill has Sideref bound
	for _, e := range embedded {
		if e.Sideref == nil {
			t.Errorf("embedded skill %q missing Sideref", e.Name)
		}
	}
}

// TestExportForEmbedded_NoSubset rejects export without selection.
func TestExportForEmbedded_NoSubset(t *testing.T) {
	reg := NewASIRegistry()
	reg.AddSkill(ASISkill{Name: "skill1", Trit: 0})

	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	_, err := reg.ExportForEmbedded(deviceSecret)
	if err == nil {
		t.Error("expected error when no subset selected")
	}
}

// TestGetTritRole returns correct role for trit value.
func TestGetTritRole(t *testing.T) {
	tests := []struct {
		trit     uint8
		expected string
	}{
		{0, "Coordinator"},
		{1, "Generator"},
		{2, "Verifier"},
		{3, "Unknown"},
	}

	for _, test := range tests {
		got := getTritRole(test.trit)
		if got != test.expected {
			t.Errorf("getTritRole(%d) = %q, want %q", test.trit, got, test.expected)
		}
	}
}

// BenchmarkSelectBalancedSubset measures subset selection performance.
func BenchmarkSelectBalancedSubset(b *testing.B) {
	reg := NewASIRegistry()

	// Create registry with many skills
	for i := 0; i < 315; i++ {
		reg.AddSkill(ASISkill{
			Name: "skill" + string(rune('0'+(i%10))),
			Trit: uint8(i % 3),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = reg.SelectBalancedSubset(27) // Select 27 balanced skills
	}
}

// BenchmarkBindSiderefs measures token binding performance.
func BenchmarkBindSiderefs(b *testing.B) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	reg := NewASIRegistry()
	for i := 0; i < 100; i++ {
		reg.AddSkill(ASISkill{Name: "skill" + string(rune('0'+(i%10))), Trit: uint8(i % 3)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.BindSiderefs(deviceSecret)
	}
}

// TestRegistryJSONRoundtrip verifies JSON export/import consistency.
func TestRegistryJSONRoundtrip(t *testing.T) {
	reg1 := NewASIRegistry()
	reg1.AddSkill(ASISkill{ID: "s1", Name: "skill1", Trit: 0, Category: "medical"})
	reg1.AddSkill(ASISkill{ID: "s2", Name: "skill2", Trit: 1, Category: "fitness"})
	reg1.AddSkill(ASISkill{ID: "s3", Name: "skill3", Trit: 2, Category: "health"})

	// Export to JSON
	data, err := reg1.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Import to new registry
	reg2 := NewASIRegistry()
	if err := reg2.FromJSON(data); err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// Verify equality
	if reg1.Count() != reg2.Count() {
		t.Errorf("count mismatch: %d vs %d", reg1.Count(), reg2.Count())
	}

	status1 := reg1.TriadStatus()
	status2 := reg2.TriadStatus()

	if !bytes.Equal(mustMarshal(status1), mustMarshal(status2)) {
		t.Error("triad status mismatch after roundtrip")
	}
}

// Helper function for marshaling in tests
func mustMarshal(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
