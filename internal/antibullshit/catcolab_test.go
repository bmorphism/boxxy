//go:build darwin

package antibullshit

import (
	"testing"

	"github.com/bmorphism/boxxy/internal/gf3"
)

func TestStandardTheory(t *testing.T) {
	theory := StandardTheory()

	if len(theory.ObTypes) != 3 {
		t.Fatalf("expected 3 ObTypes, got %d", len(theory.ObTypes))
	}
	if len(theory.MorTypes) != 3 {
		t.Fatalf("expected 3 MorTypes, got %d", len(theory.MorTypes))
	}
	if len(theory.ObOps) != 4 {
		t.Fatalf("expected 4 ObOps (frameworks), got %d", len(theory.ObOps))
	}
	if len(theory.MorOps) != 2 {
		t.Fatalf("expected 2 MorOps (cells), got %d", len(theory.MorOps))
	}

	// derives_from: Source → Claim
	src, ok := theory.SrcType("derives_from")
	if !ok || src != ObSource {
		t.Fatalf("derives_from source should be Source, got %s", src)
	}
	tgt, ok := theory.TgtType("derives_from")
	if !ok || tgt != ObClaim {
		t.Fatalf("derives_from target should be Claim, got %s", tgt)
	}

	// attests: Witness → Source
	src, _ = theory.SrcType("attests")
	if src != ObWitness {
		t.Fatalf("attests source should be Witness, got %s", src)
	}
}

func TestModelTypeValidation(t *testing.T) {
	model := NewEpistemicModel("empirical")

	model.AddObject("c1", ObClaim, gf3.One, "test claim")
	model.AddObject("s1", ObSource, gf3.Two, "test source")

	// Valid morphism: Source → Claim
	err := model.AddMorphism("m1", "derives_from", "s1", "c1", "deductive", 0.8)
	if err != nil {
		t.Fatalf("valid morphism should succeed: %v", err)
	}

	// Invalid morphism: Claim → Claim (wrong types for derives_from)
	err = model.AddMorphism("m2", "derives_from", "c1", "c1", "bad", 0.5)
	if err == nil {
		t.Fatal("morphism with wrong source type should fail")
	}

	// Invalid morphism type
	err = model.AddMorphism("m3", "nonexistent", "s1", "c1", "bad", 0.5)
	if err == nil {
		t.Fatal("morphism with unknown type should fail")
	}
}

func TestPathComposition(t *testing.T) {
	theory := StandardTheory()

	// Valid path: Witness →attests→ Source →derives_from→ Claim
	validPath := &Path{
		Segments: []PathSegment{
			{MorType: "attests", SourceID: "w1", TargetID: "s1", Strength: 0.9},
			{MorType: "derives_from", SourceID: "s1", TargetID: "c1", Strength: 0.8},
		},
	}
	if !validPath.Composes(theory) {
		t.Fatal("Witness→Source→Claim path should compose")
	}

	// Composite strength = 0.9 * 0.8 = 0.72
	cs := validPath.CompositeStrength()
	if cs < 0.71 || cs > 0.73 {
		t.Fatalf("composite strength should be ~0.72, got %f", cs)
	}

	// Invalid path: derives_from ; attests (Claim is not Source)
	invalidPath := &Path{
		Segments: []PathSegment{
			{MorType: "derives_from", SourceID: "s1", TargetID: "c1", Strength: 0.8},
			{MorType: "attests", SourceID: "w1", TargetID: "s1", Strength: 0.9},
		},
	}
	if invalidPath.Composes(theory) {
		t.Fatal("derives_from;attests should NOT compose (Claim ≠ Witness)")
	}

	// Identity path
	identity := &Path{}
	if !identity.IsIdentity() {
		t.Fatal("empty path should be identity")
	}
}

func TestAnalyzeWithCatColab(t *testing.T) {
	model := AnalyzeWithCatColab(
		"According to Dr. Smith, research from Harvard shows that exercise reduces stress",
		"empirical",
	)

	// Should have objects
	claimCount, sourceCount, witnessCount := 0, 0, 0
	for _, obj := range model.Objects {
		switch obj.Type {
		case ObClaim:
			claimCount++
		case ObSource:
			sourceCount++
		case ObWitness:
			witnessCount++
		}
	}

	if claimCount != 1 {
		t.Fatalf("expected 1 claim, got %d", claimCount)
	}
	if sourceCount == 0 {
		t.Fatal("expected at least 1 source")
	}
	if witnessCount == 0 {
		t.Fatal("expected at least 1 witness")
	}

	// Should have paths
	if len(model.Paths) == 0 {
		t.Fatal("expected derivation paths")
	}

	// All paths should compose
	for i, p := range model.Paths {
		if !p.Composes(model.Theory) {
			t.Fatalf("path %d does not compose", i)
		}
	}

	// Confidence should be positive
	if model.Confidence <= 0 {
		t.Fatalf("expected positive confidence, got %f", model.Confidence)
	}

	t.Logf("CatColab model: %d objects, %d morphisms, %d paths, confidence=%.3f",
		len(model.Objects), len(model.Morphisms), len(model.Paths), model.Confidence)
}

func TestUnsupportedClaimCatColab(t *testing.T) {
	model := AnalyzeWithCatColab("The moon is made of cheese", "empirical")

	h1, cocycles := model.SheafConsistency()
	if h1 == 0 {
		t.Fatal("unsupported claim should have H¹ > 0")
	}

	found := false
	for _, c := range cocycles {
		if c.Kind == "unsupported" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'unsupported' cocycle")
	}

	if model.Confidence > 0.2 {
		t.Fatalf("unsupported claim should have low confidence, got %f", model.Confidence)
	}
}

func TestMorOpConservationCell(t *testing.T) {
	model := NewEpistemicModel("pluralistic")

	// Add objects with trits that DON'T balance
	model.AddObject("c1", ObClaim, gf3.One, "claim")
	model.AddObject("c2", ObClaim, gf3.One, "claim2")
	// Two generators, no verifiers/coordinators → unbalanced

	model.Verify()

	// The gf3_conservation MorOp cell should detect the violation
	found := false
	for _, c := range model.Cocycles {
		if c.Kind == "trit-violation" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected trit-violation from conservation MorOp cell")
	}
}

func TestFrameworkObOps(t *testing.T) {
	text := "Study by MIT shows community benefit from sustainable energy integration"

	frameworks := []string{"empirical", "responsible", "harmonic", "pluralistic"}
	confidences := make(map[string]float64)

	for _, fw := range frameworks {
		model := AnalyzeWithCatColab(text, fw)
		confidences[fw] = model.Confidence
		t.Logf("framework=%s confidence=%.3f paths=%d cocycles=%d",
			fw, model.Confidence, len(model.Paths), len(model.Cocycles))
	}

	// Empirical should boost academic sources differently than responsible
	// (exact values depend on extraction, but they should all be positive)
	for fw, conf := range confidences {
		if conf <= 0 {
			t.Fatalf("framework %s should have positive confidence", fw)
		}
	}
}

func TestTheoryHasObType(t *testing.T) {
	theory := StandardTheory()

	if !theory.HasObType(ObClaim) {
		t.Fatal("theory should have Claim")
	}
	if theory.HasObType("Bogus") {
		t.Fatal("theory should not have Bogus")
	}
}
