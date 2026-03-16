//go:build darwin

package antibullshit

import (
	"testing"
)

func TestAnalyzeClaimWithSources(t *testing.T) {
	world := AnalyzeClaim(
		"According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%",
		"empirical",
	)

	if len(world.Claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(world.Claims))
	}

	if len(world.Sources) == 0 {
		t.Fatal("expected at least 1 extracted source")
	}

	// Should have derivations linking sources to claim
	if len(world.Derivations) == 0 {
		t.Fatal("expected derivations from sources to claim")
	}

	// Check that the claim has confidence > 0
	for _, c := range world.Claims {
		if c.Confidence <= 0 {
			t.Fatalf("expected positive confidence, got %f", c.Confidence)
		}
		t.Logf("claim confidence: %.2f, framework: %s", c.Confidence, c.Framework)
	}
}

func TestAnalyzeUnsupportedClaim(t *testing.T) {
	world := AnalyzeClaim("The moon is made of cheese", "empirical")

	if len(world.Sources) != 0 {
		t.Fatalf("expected 0 sources for unsupported claim, got %d", len(world.Sources))
	}

	// Should detect unsupported cocycle
	h1, cocycles := world.SheafConsistency()
	if h1 == 0 {
		t.Fatal("unsupported claim should produce H¹ > 0")
	}

	foundUnsupported := false
	for _, c := range cocycles {
		if c.Kind == "unsupported" {
			foundUnsupported = true
		}
	}
	if !foundUnsupported {
		t.Fatal("expected 'unsupported' cocycle")
	}

	// Confidence should be very low
	for _, c := range world.Claims {
		if c.Confidence > 0.2 {
			t.Fatalf("unsupported claim should have low confidence, got %f", c.Confidence)
		}
	}
}

func TestGF3BalanceWithFullTriad(t *testing.T) {
	world := AnalyzeClaim(
		"According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy",
		"pluralistic",
	)

	balanced, counts := world.GF3Balance()
	t.Logf("GF(3): balanced=%v coord=%d gen=%d ver=%d",
		balanced, counts["coordinator"], counts["generator"], counts["verifier"])

	// Should have generators (claims), verifiers (sources), coordinators (witnesses)
	if counts["generator"] == 0 {
		t.Fatal("expected at least 1 generator (claim)")
	}
	if counts["verifier"] == 0 {
		t.Fatal("expected at least 1 verifier (source)")
	}
}

func TestDetectManipulation(t *testing.T) {
	patterns := DetectManipulation(
		"Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven.",
	)

	if len(patterns) == 0 {
		t.Fatal("expected manipulation patterns")
	}

	kinds := make(map[string]bool)
	for _, p := range patterns {
		kinds[p.Kind] = true
		t.Logf("detected: %s (severity=%.1f) evidence=%q", p.Kind, p.Severity, p.Evidence)
	}

	if !kinds["urgency"] {
		t.Fatal("expected urgency pattern")
	}
	if !kinds["artificial_scarcity"] {
		t.Fatal("expected scarcity pattern")
	}
	if !kinds["appeal_authority"] {
		t.Fatal("expected appeal to authority pattern")
	}
}

func TestDetectNoManipulation(t *testing.T) {
	patterns := DetectManipulation(
		"The temperature today is 72 degrees Fahrenheit with partly cloudy skies.",
	)

	if len(patterns) != 0 {
		t.Fatalf("expected 0 manipulation patterns for neutral text, got %d", len(patterns))
	}
}

func TestContentHashDeterministic(t *testing.T) {
	h1 := contentHash("hello world")
	h2 := contentHash("hello world")
	h3 := contentHash("Hello World") // case-insensitive

	if h1 != h2 {
		t.Fatal("same text should produce same hash")
	}
	if h1 != h3 {
		t.Fatal("hashing should be case-insensitive")
	}
}

func TestMultipleFrameworks(t *testing.T) {
	text := "Study by MIT shows community benefit from sustainable energy integration"

	frameworks := []string{"empirical", "responsible", "harmonic", "pluralistic"}
	for _, fw := range frameworks {
		world := AnalyzeClaim(text, fw)

		for _, c := range world.Claims {
			if c.Framework != fw {
				t.Fatalf("expected framework %s, got %s", fw, c.Framework)
			}
			t.Logf("framework=%s confidence=%.2f sources=%d cocycles=%d",
				fw, c.Confidence, len(world.Sources), len(world.Cocycles))
		}
	}
}

func TestSourceKindClassification(t *testing.T) {
	world := AnalyzeClaim(
		"A study by Stanford published in Nature, and according to the CDC, plus https://example.com/data",
		"empirical",
	)

	kinds := make(map[string]int)
	for _, s := range world.Sources {
		kinds[s.Kind]++
	}

	t.Logf("source kinds: %v", kinds)

	if kinds["academic"] == 0 && kinds["authority"] == 0 && kinds["url"] == 0 {
		t.Fatal("expected at least one classified source")
	}
}
