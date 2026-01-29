//go:build tinygo

package skill

import (
	"fmt"
	"testing"
)

func TestEmbeddedSkillValidate(t *testing.T) {
	tests := []struct {
		name    string
		skill   *EmbeddedSkill
		wantErr bool
	}{
		{
			name: "valid skill",
			skill: &EmbeddedSkill{
				Name:        "pulse-oximetry",
				Description: "Read SpO2 from MAX30102 sensor",
				Trit:        1,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			skill: &EmbeddedSkill{
				Description: "A test skill",
				Trit:        0,
			},
			wantErr: true,
		},
		{
			name: "name too long",
			skill: &EmbeddedSkill{
				Name:        "this-is-a-very-long-skill-name-that-exceeds-the-sixty-four-character-limit-imposed-by-agentskills",
				Description: "Test",
				Trit:        0,
			},
			wantErr: true,
		},
		{
			name: "invalid name chars",
			skill: &EmbeddedSkill{
				Name:        "pulse_oximetry",
				Description: "Invalid underscore",
				Trit:        0,
			},
			wantErr: true,
		},
		{
			name: "name with leading hyphen",
			skill: &EmbeddedSkill{
				Name:        "-pulse-ox",
				Description: "Invalid leading hyphen",
				Trit:        0,
			},
			wantErr: true,
		},
		{
			name: "description too long",
			skill: &EmbeddedSkill{
				Name:        "test",
				Description: string(make([]byte, 1025)),
				Trit:        0,
			},
			wantErr: true,
		},
		{
			name: "invalid trit",
			skill: &EmbeddedSkill{
				Name:        "test",
				Description: "Test",
				Trit:        5,
			},
			wantErr: true,
		},
		{
			name: "body too long",
			skill: &EmbeddedSkill{
				Name:        "test",
				Description: "Test",
				Body:        string(make([]byte, 25001)), // ~6251 tokens (exceeds 5000)
				Trit:        0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.skill.ValidateEmbedded()
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ValidateEmbedded() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestIsValidSkillNameEmbedded(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"pulse-oximetry", true},
		{"ecg-monitor", true},
		{"bp-monitor", true},
		{"temp", true},
		{"a", true},
		{"0-device", true},
		{"device-123", true},
		{"-invalid", false},
		{"invalid-", false},
		{"in--valid", false},
		{"UPPERCASE", false},
		{"with_underscore", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidSkillNameEmbedded(tt.name); got != tt.want {
				t.Errorf("isValidSkillNameEmbedded(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCountLinesEmbedded(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"single line", "hello", 1},
		{"single line with newline", "hello\n", 1},
		{"two lines", "hello\nworld", 2},
		{"two lines with trailing newline", "hello\nworld\n", 2},
		{"multiple", "a\nb\nc\n", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countLinesEmbedded(tt.s); got != tt.want {
				t.Errorf("countLinesEmbedded(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestEstimateTokensEmbedded(t *testing.T) {
	tests := []struct {
		chars int
		want  int
	}{
		{0, 0},
		{1, 1},
		{4, 1},
		{5, 2},
		{8, 2},
		{20000, 5000},
		{20001, 5001},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := estimateTokensEmbedded(tt.chars); got != tt.want {
				t.Errorf("estimateTokensEmbedded(%d) = %v, want %v", tt.chars, got, tt.want)
			}
		})
	}
}

func TestComputeTritEmbedded(t *testing.T) {
	// Same hash should produce same trit
	trit1 := ComputetritEmbedded("pulse-oximetry")
	trit2 := ComputetritEmbedded("pulse-oximetry")
	if trit1 != trit2 {
		t.Errorf("ComputetritEmbedded not deterministic: %d != %d", trit1, trit2)
	}

	// Trit should be valid (0, 1, or 2)
	if trit1 > 2 {
		t.Errorf("ComputetritEmbedded returned invalid trit: %d", trit1)
	}
}

func TestParseEmbeddedSkillLine(t *testing.T) {
	tests := []struct {
		line    string
		wantErr bool
		wantName string
	}{
		{
			line:     "pulse-oximetry:Read SpO2 from sensor:1",
			wantErr:  false,
			wantName: "pulse-oximetry",
		},
		{
			line:     "ecg-monitor:Monitor heart activity",
			wantErr:  false,
			wantName: "ecg-monitor",
		},
		{
			line:    "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			skill, err := ParseEmbeddedSkillLine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEmbeddedSkillLine error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && skill.Name != tt.wantName {
				t.Errorf("ParseEmbeddedSkillLine Name = %v, want %v", skill.Name, tt.wantName)
			}
		})
	}
}

func TestEmbeddedSkillRegistry(t *testing.T) {
	registry := NewRegistry()

	// Register skills
	s1 := &EmbeddedSkill{Name: "pulse-ox", Description: "Pulse oximetry", Trit: 1}
	s2 := &EmbeddedSkill{Name: "ecg", Description: "ECG monitoring", Trit: 0}
	s3 := &EmbeddedSkill{Name: "temp", Description: "Temperature", Trit: 2}

	registry.Register(s1)
	registry.Register(s2)
	registry.Register(s3)

	if registry.Count() != 3 {
		t.Errorf("Count = %d, want 3", registry.Count())
	}

	// Lookup
	if skill := registry.Lookup("pulse-ox"); skill == nil || skill.Name != "pulse-ox" {
		t.Errorf("Lookup failed")
	}

	// By trit
	byTrit := registry.ByTrit(1)
	if len(byTrit) != 1 || byTrit[0].Name != "pulse-ox" {
		t.Errorf("ByTrit(1) = %v, want [pulse-ox]", byTrit)
	}

	// IsBalanced: 1 + 0 + 2 = 3 ≡ 0 (mod 3)
	if !registry.IsBalanced() {
		t.Errorf("IsBalanced = false, want true")
	}

	// Duplicate registration should fail
	err := registry.Register(s1)
	if err == nil {
		t.Errorf("Register duplicate should error")
	}
}

func TestSerializeCompact(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&EmbeddedSkill{Name: "pulse-ox", Description: "Read SpO2", Trit: 1})
	registry.Register(&EmbeddedSkill{Name: "ecg", Description: "Monitor ECG", Trit: 0})

	compact := registry.SerializeCompact()
	if len(compact) == 0 {
		t.Errorf("SerializeCompact returned empty string")
	}

	// Should contain both skill names
	if len(compact) == 0 || (compact[0] != 'p' && compact[0] != 'e') {
		t.Errorf("SerializeCompact format unexpected: %q", compact)
	}
}

func TestMarshalUnmarshalCompact(t *testing.T) {
	original := &EmbeddedSkill{
		Name: "pulse-oximetry",
		Trit: 1,
	}

	cf := original.MarshalCompact()
	restored := UnmarshalCompact(cf)

	if restored.Name != original.Name {
		t.Errorf("Restored name = %q, want %q", restored.Name, original.Name)
	}
	if restored.Trit != original.Trit {
		t.Errorf("Restored trit = %d, want %d", restored.Trit, original.Trit)
	}
}

func TestMedicalDeviceCapabilities(t *testing.T) {
	// Simulate nRF52840 medical device firmware
	registry := NewRegistry()

	// Device capabilities (balanced GF(3) skill set)
	registry.Register(&EmbeddedSkill{
		Name:        "pulse-oximetry",
		Description: "SpO2 measurement via MAX30102 I2C sensor",
		Trit:        1, // Generator
	})

	registry.Register(&EmbeddedSkill{
		Name:        "heart-rate",
		Description: "HR calculation from PPG waveform",
		Trit:        0, // Coordinator
	})

	registry.Register(&EmbeddedSkill{
		Name:        "ble-transport",
		Description: "Advertise readings via BLE GATT",
		Trit:        2, // Verifier
	})

	// Check balance
	if !registry.IsBalanced() {
		t.Errorf("Medical device capabilities not balanced")
	}

	// Serialize for BLE advertisement
	compact := registry.SerializeCompact()
	if len(compact) == 0 {
		t.Errorf("Failed to serialize medical device capabilities")
	}

	// Verify all registrations are valid
	errs := registry.ValidateAll()
	if len(errs) > 0 {
		t.Errorf("Medical device validation errors: %v", errs)
	}
}

func TestRegistryCapacityLimit(t *testing.T) {
	// Test resource exhaustion protection
	registry := NewRegistry()

	// Fill registry to capacity
	for i := 0; i < MaxSkillsPerDevice; i++ {
		name := fmt.Sprintf("skill-%d", i)
		err := registry.Register(&EmbeddedSkill{
			Name:        name,
			Description: "Test skill",
			Trit:        uint8(i % 3),
		})
		if err != nil {
			t.Errorf("Failed to register skill %d: %v", i, err)
		}
	}

	if registry.Count() != MaxSkillsPerDevice {
		t.Errorf("Registry count = %d, want %d", registry.Count(), MaxSkillsPerDevice)
	}

	// Try to exceed capacity
	err := registry.Register(&EmbeddedSkill{
		Name:        "extra-skill",
		Description: "Should fail",
		Trit:        0,
	})

	if err == nil {
		t.Errorf("Expected error when exceeding registry capacity, got nil")
	}

	// Verify count didn't change
	if registry.Count() != MaxSkillsPerDevice {
		t.Errorf("Registry count changed after failed registration")
	}
}
