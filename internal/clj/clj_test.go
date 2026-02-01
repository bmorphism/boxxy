package clj_test

import (
	"encoding/json"
	"testing"

	"github.com/goloop/trit"
)

// Joker (candid82/joker) is a Clojure interpreter written in Go,
// used as a tool binary (not an embeddable library — its core package
// requires codegen). The established Clojurian pattern for Go interop
// is: EDN as wire format, trit values as GF(3) atoms, and maps for
// provider configs.
//
// These tests validate the Go-side data structures that interop with
// Joker-evaluated Clojure via EDN/JSON serialization.

// ProviderConfig mirrors a Joker EDN map like:
//
//	{:name "macos-vf" :trit 1 :color "#A855F7" :backend :virtualization-framework}
type ProviderConfig struct {
	Name    string    `json:"name"`
	Trit    trit.Trit `json:"trit"`
	Color   string    `json:"color"`
	Backend string    `json:"backend"`
}

func TestProviderConfigMarshal(t *testing.T) {
	// Construct a provider config — the Go-side representation
	// of what Joker would produce from EDN evaluation
	cfg := ProviderConfig{
		Name:    "macos-vf",
		Trit:    trit.True, // +1 — generation/creation
		Color:   "#A855F7",
		Backend: "virtualization-framework",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	t.Logf("JSON: %s", string(data))

	// Roundtrip
	var decoded ProviderConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Name != "macos-vf" {
		t.Errorf("Name: expected macos-vf, got %s", decoded.Name)
	}
	if decoded.Color != "#A855F7" {
		t.Errorf("Color: expected #A855F7, got %s", decoded.Color)
	}
	if decoded.Backend != "virtualization-framework" {
		t.Errorf("Backend: expected virtualization-framework, got %s", decoded.Backend)
	}
}

func TestProviderQuadBalance(t *testing.T) {
	// A boxxy quad: 4 providers whose trit values sum ≡ 0 (mod 3).
	// This is the Clojurian pattern from Gay MCP's skill quad system,
	// now in Go structs ready for Joker EDN interop.
	providers := []ProviderConfig{
		{Name: "macos-vf", Trit: trit.True, Color: "#A855F7"},         // +1
		{Name: "cloud-hypervisor", Trit: trit.True, Color: "#2E5FA3"}, // +1
		{Name: "qemu-riscv", Trit: trit.False, Color: "#F59E0B"},      // -1
		{Name: "firecracker", Trit: trit.False, Color: "#EF4444"},      // -1
	}

	sum := 0
	for _, p := range providers {
		sum += int(p.Trit.Int())
	}
	// Balanced: +1 +1 -1 -1 = 0
	mod := ((sum % 3) + 3) % 3
	if mod != 0 {
		t.Errorf("Provider quad not balanced: sum=%d, mod3=%d", sum, mod)
	}
	t.Logf("Provider quad balanced: sum=%d ≡ 0 (mod 3)", sum)

	// Verify all have colors (Gay MCP hex)
	for _, p := range providers {
		if p.Color == "" || p.Color[0] != '#' {
			t.Errorf("Provider %s has invalid color: %s", p.Name, p.Color)
		}
	}
}

func TestProviderTritJSONInterop(t *testing.T) {
	// Trit marshals as JSON boolean: true/false/null
	// This maps to Clojure's true/false/nil — exact EDN equivalents.
	cases := []struct {
		trit     trit.Trit
		expected string
		clj      string // Clojure equivalent
	}{
		{trit.True, "true", "true"},
		{trit.False, "false", "false"},
		{trit.Unknown, "null", "nil"},
	}

	for _, tc := range cases {
		data, err := tc.trit.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed for %v: %v", tc.trit, err)
		}
		if string(data) != tc.expected {
			t.Errorf("Trit %v: expected JSON %s (Clojure %s), got %s",
				tc.trit, tc.expected, tc.clj, string(data))
		}
	}
}
