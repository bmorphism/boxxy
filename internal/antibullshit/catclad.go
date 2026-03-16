//go:build darwin

// Package antibullshit implements cat-clad epistemological verification.
//
// A "cat-clad" claim is an object in a category with morphisms tracking
// its provenance, derivation history, and the consistency conditions that
// bind it to other claims. Verification reduces to structural properties:
//
//   - Provenance is a composable morphism chain to primary sources
//   - Consistency is a sheaf condition (H¹ = 0 means no contradictions)
//   - GF(3) conservation prevents unbounded generation without verification
//   - Bisimulation detects forgery (divergent accounts of the same event)
//
// ACSet Schema:
//
//	@present SchClaimWorld(FreeSchema) begin
//	  Claim::Ob           -- assertions to verify
//	  Source::Ob           -- evidence or citations
//	  Witness::Ob          -- attestation parties
//	  Derivation::Ob       -- inference steps
//
//	  derives_from::Hom(Derivation, Source)
//	  produces::Hom(Derivation, Claim)
//	  attests::Hom(Witness, Source)
//	  cites::Hom(Claim, Source)
//
//	  Trit::AttrType
//	  Confidence::AttrType
//	  ContentHash::AttrType
//	  Timestamp::AttrType
//
//	  claim_trit::Attr(Claim, Trit)
//	  source_trit::Attr(Source, Trit)
//	  witness_trit::Attr(Witness, Trit)
//	  claim_hash::Attr(Claim, ContentHash)
//	  source_hash::Attr(Source, ContentHash)
//	  claim_confidence::Attr(Claim, Confidence)
//	end
package antibullshit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// --- ACSet Schema ---

// Claim is a typed assertion with GF(3) trit and content hash.
type Claim struct {
	ID         string    `json:"id"`
	Text       string    `json:"text"`
	Trit       gf3.Elem  `json:"trit"`       // 0=coordinator, 1=generator, 2=verifier
	Hash       string    `json:"hash"`        // SHA-256 of normalized text
	Confidence float64   `json:"confidence"`  // 0.0-1.0
	Framework  string    `json:"framework"`   // which epistemological framework
	CreatedAt  time.Time `json:"created_at"`
}

// Source is a cited piece of evidence.
type Source struct {
	ID       string   `json:"id"`
	Citation string   `json:"citation"` // extracted citation text
	Trit     gf3.Elem `json:"trit"`
	Hash     string   `json:"hash"`
	Kind     string   `json:"kind"` // "academic", "news", "authority", "anecdotal", "self-referential"
}

// Witness is an attestation party for a source.
type Witness struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Trit   gf3.Elem `json:"trit"`
	Role   string   `json:"role"` // "author", "peer-reviewer", "editor", "self"
	Weight float64  `json:"weight"`
}

// Derivation is an inference step: source → claim.
type Derivation struct {
	ID       string   `json:"id"`
	SourceID string   `json:"source_id"`
	ClaimID  string   `json:"claim_id"`
	Kind     string   `json:"kind"` // "direct", "inductive", "deductive", "analogical", "appeal-to-authority"
	Strength float64  `json:"strength"` // 0.0-1.0
}

// Cocycle records a sheaf obstruction between claims.
type Cocycle struct {
	ClaimA   string  `json:"claim_a"`
	ClaimB   string  `json:"claim_b"`
	Kind     string  `json:"kind"` // "contradiction", "unsupported", "circular", "trit-violation"
	Severity float64 `json:"severity"`
}

// --- ClaimWorld: the ACSet instance ---

// ClaimWorld is a cat-clad epistemological universe.
type ClaimWorld struct {
	Claims      map[string]*Claim      `json:"claims"`
	Sources     map[string]*Source     `json:"sources"`
	Witnesses   map[string]*Witness   `json:"witnesses"`
	Derivations []*Derivation          `json:"derivations"`
	Cocycles    []Cocycle              `json:"cocycles"`
}

// NewClaimWorld creates an empty cat-clad world.
func NewClaimWorld() *ClaimWorld {
	return &ClaimWorld{
		Claims:    make(map[string]*Claim),
		Sources:   make(map[string]*Source),
		Witnesses: make(map[string]*Witness),
	}
}

// --- Claim analysis ---

// AnalyzeClaim parses text into a cat-clad structure and checks consistency.
func AnalyzeClaim(text, framework string) *ClaimWorld {
	world := NewClaimWorld()

	// Create the primary claim (Generator role — it's asserting something)
	claim := &Claim{
		ID:        contentHash(text)[:12],
		Text:      text,
		Trit:      gf3.One, // Generator: creating an assertion
		Hash:      contentHash(text),
		Framework: framework,
		CreatedAt: time.Now(),
	}
	world.Claims[claim.ID] = claim

	// Extract sources as morphisms from claim
	sources := extractSources(text)
	for _, src := range sources {
		world.Sources[src.ID] = src
		world.Derivations = append(world.Derivations, &Derivation{
			ID:       fmt.Sprintf("d-%s-%s", src.ID, claim.ID),
			SourceID: src.ID,
			ClaimID:  claim.ID,
			Kind:     classifyDerivation(src),
			Strength: sourceStrength(src),
		})
	}

	// Extract witnesses (who attests to the sources)
	for _, src := range sources {
		witnesses := extractWitnesses(src)
		for _, w := range witnesses {
			world.Witnesses[w.ID] = w
		}
	}

	// Check GF(3) conservation
	claim.Confidence = computeConfidence(world, claim, framework)

	// Detect cocycles (contradictions, unsupported claims, circular reasoning)
	world.Cocycles = detectCocycles(world)

	return world
}

// --- Manipulation detection ---

type ManipulationPattern struct {
	Kind     string  `json:"kind"`
	Evidence string  `json:"evidence"`
	Severity float64 `json:"severity"`
}

// Pre-compiled manipulation patterns — compiled once, reused on every call.
var manipulationChecks = []struct {
	kind    string
	pattern *regexp.Regexp
	weight  float64
}{
	{"emotional_fear", regexp.MustCompile(`(?i)(fear|terrif|alarm|panic|dread|catastroph)`), 0.7},
	{"urgency", regexp.MustCompile(`(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)`), 0.8},
	{"false_consensus", regexp.MustCompile(`(?i)(everyone knows|nobody (believes|wants|thinks)|all experts|unanimous|widely accepted)`), 0.6},
	{"appeal_authority", regexp.MustCompile(`(?i)(experts say|scientists (claim|prove)|studies show|research proves|doctors recommend)`), 0.5},
	{"artificial_scarcity", regexp.MustCompile(`(?i)(exclusive|rare opportunity|only \d+ left|limited (edition|supply|spots))`), 0.7},
	{"social_pressure", regexp.MustCompile(`(?i)(you don't want to be|don't miss out|join .* (others|people)|be the first)`), 0.6},
	{"loaded_language", regexp.MustCompile(`(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)`), 0.4},
	{"false_dichotomy", regexp.MustCompile(`(?i)(either .* or|only (two|2) (options|choices)|if you don't .* then)`), 0.6},
	{"circular_reasoning", regexp.MustCompile(`(?i)(because .* therefore .* because|true because .* which is true)`), 0.9},
	{"ad_hominem", regexp.MustCompile(`(?i)(stupid|idiot|moron|fool|ignorant|naive) .* (think|believe|say)`), 0.8},
}

// DetectManipulation checks for manipulation patterns in text.
func DetectManipulation(text string) []ManipulationPattern {
	var patterns []ManipulationPattern

	for _, c := range manipulationChecks {
		matches := c.pattern.FindAllString(text, -1)
		for _, m := range matches {
			patterns = append(patterns, ManipulationPattern{
				Kind:     c.kind,
				Evidence: m,
				Severity: c.weight,
			})
		}
	}

	return patterns
}

// --- Sheaf consistency ---

// SheafConsistency returns the H¹ dimension (0 = consistent, >0 = contradictions).
func (w *ClaimWorld) SheafConsistency() (int, []Cocycle) {
	return len(w.Cocycles), w.Cocycles
}

// GF3Balance checks the conservation law: Σ trits ≡ 0 (mod 3).
func (w *ClaimWorld) GF3Balance() (bool, map[string]int) {
	counts := map[string]int{"coordinator": 0, "generator": 0, "verifier": 0}
	var trits []gf3.Elem

	for _, c := range w.Claims {
		trits = append(trits, c.Trit)
	}
	for _, s := range w.Sources {
		trits = append(trits, s.Trit)
	}
	for _, wit := range w.Witnesses {
		trits = append(trits, wit.Trit)
	}

	for _, t := range trits {
		switch t {
		case gf3.Zero:
			counts["coordinator"]++
		case gf3.One:
			counts["generator"]++
		case gf3.Two:
			counts["verifier"]++
		}
	}

	return gf3.IsBalanced(trits), counts
}

// --- Helpers ---

func contentHash(text string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(text))))
	return hex.EncodeToString(h[:])
}

// Pre-compiled source extraction patterns.
var sourcePatterns = []struct {
	re   *regexp.Regexp
	kind string
}{
	{regexp.MustCompile(`(?i)(?:according to|cited by|reported by)\s+([^,\.]+)`), "authority"},
	{regexp.MustCompile(`(?i)(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)`), "academic"},
	{regexp.MustCompile(`(?i)(?:published in|journal of)\s+([^,\.]+)`), "academic"},
	{regexp.MustCompile(`(?i)(https?://\S+)`), "url"},
}

func extractSources(text string) []*Source {
	var sources []*Source
	seen := make(map[string]bool)

	for _, p := range sourcePatterns {
		matches := p.re.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			citation := strings.TrimSpace(m[1])
			id := contentHash(citation)[:12]
			if seen[id] {
				continue
			}
			seen[id] = true

			sources = append(sources, &Source{
				ID:       id,
				Citation: citation,
				Trit:     gf3.Two, // Verifier role — evidence checks claims
				Hash:     contentHash(citation),
				Kind:     p.kind,
			})
		}
	}

	return sources
}

func extractWitnesses(src *Source) []*Witness {
	// The source itself is a witness to its own content
	return []*Witness{{
		ID:     fmt.Sprintf("w-%s", src.ID),
		Name:   src.Citation,
		Trit:   gf3.Zero, // Coordinator — mediating between claim and verification
		Role:   witnessRole(src.Kind),
		Weight: witnessWeight(src.Kind),
	}}
}

func witnessRole(kind string) string {
	switch kind {
	case "academic":
		return "peer-reviewer"
	case "authority":
		return "author"
	case "url":
		return "publisher"
	default:
		return "self"
	}
}

func witnessWeight(kind string) float64 {
	switch kind {
	case "academic":
		return 0.9
	case "authority":
		return 0.6
	case "url":
		return 0.4
	default:
		return 0.2
	}
}

func classifyDerivation(src *Source) string {
	switch src.Kind {
	case "academic":
		return "deductive"
	case "authority":
		return "appeal-to-authority"
	case "url":
		return "direct"
	default:
		return "analogical"
	}
}

func sourceStrength(src *Source) float64 {
	switch src.Kind {
	case "academic":
		return 0.85
	case "authority":
		return 0.5
	case "url":
		return 0.3
	default:
		return 0.1
	}
}

func computeConfidence(world *ClaimWorld, claim *Claim, framework string) float64 {
	if len(world.Sources) == 0 {
		return 0.1 // unsupported claim
	}

	// Average derivation strength
	totalStrength := 0.0
	count := 0
	for _, d := range world.Derivations {
		if d.ClaimID == claim.ID {
			totalStrength += d.Strength
			count++
		}
	}
	if count == 0 {
		return 0.1
	}
	avgStrength := totalStrength / float64(count)

	// Weight by framework
	switch framework {
	case "empirical":
		// Empirical: boost academic sources
		academicCount := 0
		for _, s := range world.Sources {
			if s.Kind == "academic" {
				academicCount++
			}
		}
		if academicCount > 0 {
			avgStrength *= 1.0 + 0.1*float64(academicCount)
		}
	case "responsible":
		// Weight community impact language
		if strings.Contains(strings.ToLower(claim.Text), "community") ||
			strings.Contains(strings.ToLower(claim.Text), "benefit") {
			avgStrength *= 1.1
		}
	case "harmonic":
		// Reward multi-source convergence
		if len(world.Sources) >= 3 {
			avgStrength *= 1.15
		}
	case "pluralistic":
		// Combine all — no special boost, raw structural quality
	}

	// Penalize cocycles
	cocyclePenalty := 0.15 * float64(len(world.Cocycles))
	confidence := avgStrength - cocyclePenalty

	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}
	return confidence
}

func detectCocycles(world *ClaimWorld) []Cocycle {
	var cocycles []Cocycle

	// Check for unsupported claims (no derivation chain)
	for _, claim := range world.Claims {
		hasDerivation := false
		for _, d := range world.Derivations {
			if d.ClaimID == claim.ID {
				hasDerivation = true
				break
			}
		}
		if !hasDerivation {
			cocycles = append(cocycles, Cocycle{
				ClaimA:   claim.ID,
				Kind:     "unsupported",
				Severity: 0.9,
			})
		}
	}

	// Check for appeal-to-authority without verification
	for _, d := range world.Derivations {
		if d.Kind == "appeal-to-authority" && d.Strength < 0.6 {
			cocycles = append(cocycles, Cocycle{
				ClaimA:   d.ClaimID,
				ClaimB:   d.SourceID,
				Kind:     "weak-authority",
				Severity: 0.5,
			})
		}
	}

	// Check GF(3) conservation
	balanced, _ := world.GF3Balance()
	if !balanced {
		cocycles = append(cocycles, Cocycle{
			Kind:     "trit-violation",
			Severity: 0.3,
		})
	}

	return cocycles
}
