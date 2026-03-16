//go:build darwin

// interleave.go bridges the tape causal recording system with the CatColab
// anti-bullshit engine. Every recorded frame becomes a claim in the epistemic
// model; the causal ordering provides the derivation morphisms; the gossip
// protocol provides the witness attestation. The sheaf consistency check
// from tape/ and the cocycle detection from antibullshit/ are unified into
// a single verification pipeline.
//
// This is the interleaving: tape frames flow into the epistemic model,
// and the epistemic model's confidence feeds back into the DGM fitness
// evaluation. The autopoietic cycle now spans both systems.
package antibullshit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// TapeFrame mirrors tape.Frame without importing to avoid circular deps.
type TapeFrame struct {
	SeqNo     uint64
	LamportTS uint64
	NodeID    string
	Content   string
	Width     int
	Height    int
	Trit      gf3.Elem
	WallTime  time.Time
}

// InterleaveConfig controls how tape frames map to epistemic claims.
type InterleaveConfig struct {
	Framework       string  // epistemological framework for analysis
	MinContentLen   int     // skip frames with less content
	ConfidenceDecay float64 // how fast old frames lose confidence (0-1)
}

// DefaultInterleaveConfig returns sensible defaults.
func DefaultInterleaveConfig() InterleaveConfig {
	return InterleaveConfig{
		Framework:       "pluralistic",
		MinContentLen:   10,
		ConfidenceDecay: 0.95,
	}
}

// InterleaveResult is the unified verification result.
type InterleaveResult struct {
	Model           *EpistemicModel
	TotalFrames     int
	AcceptedFrames  int
	SkippedFrames   int
	AvgConfidence   float64
	SheafH1         int
	GF3Balanced     bool
	ManipPatterns   []ManipulationPattern
	CausalChainLen  int
}

// InterleaveTapeFrames converts a sequence of tape frames into a verified
// CatColab epistemic model. Each frame's content is analyzed for claims
// and sources; the Lamport ordering provides causal derivation morphisms.
func InterleaveTapeFrames(frames []TapeFrame, cfg InterleaveConfig) *InterleaveResult {
	model := NewEpistemicModel(cfg.Framework)
	result := &InterleaveResult{TotalFrames: len(frames)}

	var prevClaimID string
	var allManip []ManipulationPattern

	for i, frame := range frames {
		if len(frame.Content) < cfg.MinContentLen {
			result.SkippedFrames++
			continue
		}
		result.AcceptedFrames++

		// Each frame becomes a claim in the epistemic model
		claimID := frameHash(frame)[:12]
		model.AddObject(claimID, ObClaim, frame.Trit, frame.Content)

		// Extract sources from frame content and add them
		sources := extractSources(frame.Content)
		for _, src := range sources {
			model.AddObject(src.ID, ObSource, gf3.Two, src.Citation)
			if obj := model.Objects[src.ID]; obj != nil {
				if obj.Meta == nil {
					obj.Meta = make(map[string]string)
				}
				obj.Meta["kind"] = src.Kind
			}
			morID := fmt.Sprintf("d-%s-%s", src.ID, claimID)
			model.AddMorphism(morID, "derives_from", src.ID, claimID,
				classifyDerivation(src), sourceStrength(src))

			witID := fmt.Sprintf("w-%s", src.ID)
			model.AddObject(witID, ObWitness, gf3.Zero, src.Citation)
			attestID := fmt.Sprintf("a-%s-%s", witID, src.ID)
			model.AddMorphism(attestID, "attests", witID, src.ID,
				"attestation", witnessWeight(src.Kind))
		}

		// Causal chain: if this isn't the first frame, add a cites morphism
		// from this claim to a source representing the previous frame
		if prevClaimID != "" && i > 0 {
			prevSourceID := fmt.Sprintf("causal-%s", prevClaimID)
			model.AddObject(prevSourceID, ObSource, gf3.Two,
				fmt.Sprintf("causal predecessor at lamport=%d", frames[i-1].LamportTS))
			if obj := model.Objects[prevSourceID]; obj != nil {
				if obj.Meta == nil {
					obj.Meta = make(map[string]string)
				}
				obj.Meta["kind"] = "causal"
			}
			causalMorID := fmt.Sprintf("causal-%s-%s", prevSourceID, claimID)
			model.AddMorphism(causalMorID, "derives_from", prevSourceID, claimID,
				"direct", cfg.ConfidenceDecay)
			result.CausalChainLen++
		}

		// Detect manipulation in frame content
		manip := DetectManipulation(frame.Content)
		allManip = append(allManip, manip...)

		prevClaimID = claimID
	}

	// Verify the unified model
	model.Verify()

	result.Model = model
	result.SheafH1 = len(model.Cocycles)
	result.GF3Balanced, _ = model.GF3Balance()
	result.ManipPatterns = allManip
	result.AvgConfidence = model.Confidence

	return result
}

// ProveConservation formally checks the GF(3) conservation invariant
// across the interleaved model. Returns nil if the invariant holds,
// or an error describing the violation.
func ProveConservation(model *EpistemicModel) error {
	balanced, counts := model.GF3Balance()
	if balanced {
		return nil
	}

	var trits []gf3.Elem
	for _, obj := range model.Objects {
		trits = append(trits, obj.Trit)
	}
	sum := gf3.SeqSum(trits)

	return fmt.Errorf("GF(3) conservation violated: sum=%d mod 3 = %d (need 0), counts=%v",
		sum, sum%3, counts)
}

// ProvePathComposition checks that every derivation path in the model
// composes correctly (each morphism's target type matches the next's source type).
func ProvePathComposition(model *EpistemicModel) []error {
	var errors []error
	for i, path := range model.Paths {
		if !path.Composes(model.Theory) {
			errors = append(errors, fmt.Errorf("path %d does not compose: %d segments, types mismatch", i, len(path.Segments)))
		}
	}
	return errors
}

// ProveSheafConsistency checks that the sheaf condition holds (H¹ = 0).
func ProveSheafConsistency(model *EpistemicModel) []Cocycle {
	_, cocycles := model.SheafConsistency()
	return cocycles
}

func frameHash(f TapeFrame) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s",
		strings.ToLower(f.NodeID), f.LamportTS, strings.ToLower(f.Content))))
	return hex.EncodeToString(h[:])
}
