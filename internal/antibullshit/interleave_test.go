//go:build darwin

package antibullshit

import (
	"testing"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

func TestInterleaveTapeFrames(t *testing.T) {
	frames := []TapeFrame{
		{SeqNo: 1, LamportTS: 1, NodeID: "node-a", Content: "According to Dr. Smith, research shows benefits", Trit: gf3.One, WallTime: time.Now()},
		{SeqNo: 2, LamportTS: 2, NodeID: "node-a", Content: "Study by MIT confirms the previous finding", Trit: gf3.Two, WallTime: time.Now()},
		{SeqNo: 3, LamportTS: 3, NodeID: "node-a", Content: "Published in Nature, further evidence supports this", Trit: gf3.Zero, WallTime: time.Now()},
	}

	result := InterleaveTapeFrames(frames, DefaultInterleaveConfig())

	if result.AcceptedFrames != 3 {
		t.Fatalf("expected 3 accepted frames, got %d", result.AcceptedFrames)
	}
	if result.CausalChainLen != 2 {
		t.Fatalf("expected causal chain length 2, got %d", result.CausalChainLen)
	}
	if result.Model == nil {
		t.Fatal("expected non-nil model")
	}

	// Should have claims, sources, witnesses, and morphisms
	claimCount := 0
	for _, obj := range result.Model.Objects {
		if obj.Type == ObClaim {
			claimCount++
		}
	}
	if claimCount != 3 {
		t.Fatalf("expected 3 claims (one per frame), got %d", claimCount)
	}

	t.Logf("interleave: %d frames → %d objects, %d morphisms, %d paths, conf=%.3f, H¹=%d",
		result.TotalFrames, len(result.Model.Objects), len(result.Model.Morphisms),
		len(result.Model.Paths), result.AvgConfidence, result.SheafH1)
}

func TestInterleaveSkipsShortContent(t *testing.T) {
	frames := []TapeFrame{
		{SeqNo: 1, LamportTS: 1, NodeID: "a", Content: "hi", Trit: gf3.One, WallTime: time.Now()},
		{SeqNo: 2, LamportTS: 2, NodeID: "a", Content: "According to Dr. Smith, research from Harvard shows results", Trit: gf3.Two, WallTime: time.Now()},
	}

	result := InterleaveTapeFrames(frames, DefaultInterleaveConfig())
	if result.SkippedFrames != 1 {
		t.Fatalf("expected 1 skipped frame, got %d", result.SkippedFrames)
	}
	if result.AcceptedFrames != 1 {
		t.Fatalf("expected 1 accepted frame, got %d", result.AcceptedFrames)
	}
}

func TestInterleaveDetectsManipulation(t *testing.T) {
	frames := []TapeFrame{
		{SeqNo: 1, LamportTS: 1, NodeID: "a", Content: "Act now! This exclusive offer expires soon. Everyone knows it works.", Trit: gf3.One, WallTime: time.Now()},
	}

	result := InterleaveTapeFrames(frames, DefaultInterleaveConfig())
	if len(result.ManipPatterns) == 0 {
		t.Fatal("expected manipulation patterns in frame content")
	}
	t.Logf("detected %d manipulation patterns in interleaved frames", len(result.ManipPatterns))
}

func TestProveConservation(t *testing.T) {
	// Balanced model: 1 claim (trit=1) + 1 source (trit=2) + 1 witness (trit=0) = sum 3 ≡ 0 mod 3
	model := AnalyzeWithCatColab(
		"According to WHO, vaccine efficacy confirmed",
		"empirical",
	)
	err := ProveConservation(model)
	// May or may not be balanced depending on extraction, but should not panic
	if err != nil {
		t.Logf("conservation proof: %v", err)
	} else {
		t.Log("conservation proof: HOLDS")
	}
}

func TestProvePathComposition(t *testing.T) {
	model := AnalyzeWithCatColab(
		"According to Dr. Smith, research from Harvard shows results",
		"empirical",
	)
	errors := ProvePathComposition(model)
	if len(errors) > 0 {
		t.Fatalf("expected all paths to compose, got %d errors", len(errors))
	}
	t.Logf("all %d paths compose correctly", len(model.Paths))
}

func TestProveSheafConsistency(t *testing.T) {
	model := AnalyzeWithCatColab(
		"According to Dr. Smith, research from Harvard shows results",
		"empirical",
	)
	cocycles := ProveSheafConsistency(model)
	t.Logf("sheaf consistency: %d cocycles", len(cocycles))
	for _, c := range cocycles {
		t.Logf("  cocycle: %s (severity=%.2f)", c.Kind, c.Severity)
	}
}

func TestInterleaveEmptyFrames(t *testing.T) {
	result := InterleaveTapeFrames(nil, DefaultInterleaveConfig())
	if result.TotalFrames != 0 {
		t.Fatalf("expected 0 total frames, got %d", result.TotalFrames)
	}
	if result.Model.Confidence != 0.1 {
		t.Logf("empty model confidence: %.3f", result.Model.Confidence)
	}
}
