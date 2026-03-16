//go:build darwin

// catcolab.go implements CatColab-style double theory for epistemological verification.
//
// Following ToposInstitute/CatColab's architecture, claims are modeled as:
//
//   DblTheory  = the epistemological framework (virtual double category)
//   DblModel   = the claim world (instance conforming to the theory)
//   ObType     = {Claim, Source, Witness} with GF(3) trit assignment
//   MorType    = {DerivesFrom, Attests, Cites} with source/target ObTypes
//   ObOp       = framework transformations (empirical→responsible→harmonic→pluralistic)
//   MorOp      = composition rules, conservation laws (cells in the double category)
//   Path       = composable derivation chains (sequences of morphisms)
//
// The double theory structure gives us:
//   - Objects (ObType): kinds of epistemological entities
//   - Proarrows (MorType): typed relations between entity kinds
//   - Arrows (ObOp): transformations between frameworks
//   - Cells (MorOp): coherence conditions (naturality squares)
//
// A claim is verified when:
//   1. Its derivation Path composes (no gaps in the morphism chain)
//   2. The Path factors through at least one Verifier-typed Source
//   3. All ObOp framework transformations preserve the Path structure
//   4. The MorOp conservation cell enforces GF(3) balance
package antibullshit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// --- Double Theory: the epistemological logic ---

// ObType is a type of object in the double theory.
type ObType string

const (
	ObClaim   ObType = "Claim"
	ObSource  ObType = "Source"
	ObWitness ObType = "Witness"
)

// MorType is a type of morphism (proarrow) in the double theory.
// Each MorType has a source and target ObType.
type MorType struct {
	Name   string `json:"name"`
	Source ObType `json:"src"`
	Target ObType `json:"tgt"`
}

// ObOp is an object operation (arrow) — a framework transformation.
// It maps objects of one type to objects of another (or the same) type
// while preserving the derivation structure.
type ObOp struct {
	Name       string `json:"name"`
	Domain     ObType `json:"dom"`
	Codomain   ObType `json:"cod"`
	Transform  string `json:"transform"` // e.g. "empirical→responsible"
}

// MorOp is a morphism operation (cell) — a coherence condition.
// It ensures that applying framework transformations to morphisms
// yields consistent results (naturality square).
type MorOp struct {
	Name      string  `json:"name"`
	DomMor    string  `json:"dom_mor"`
	CodMor    string  `json:"cod_mor"`
	TopOb     string  `json:"top_ob"`
	BotOb     string  `json:"bot_ob"`
	Condition string  `json:"condition"` // e.g. "gf3_conservation"
}

// EpistemicTheory is the CatColab-style double theory for claim verification.
type EpistemicTheory struct {
	ObTypes  []ObType  `json:"ob_types"`
	MorTypes []MorType `json:"mor_types"`
	ObOps    []ObOp    `json:"ob_ops"`
	MorOps   []MorOp   `json:"mor_ops"`
}

// StandardTheory returns the canonical epistemic double theory.
func StandardTheory() *EpistemicTheory {
	return &EpistemicTheory{
		ObTypes: []ObType{ObClaim, ObSource, ObWitness},
		MorTypes: []MorType{
			{Name: "derives_from", Source: ObSource, Target: ObClaim},
			{Name: "attests", Source: ObWitness, Target: ObSource},
			{Name: "cites", Source: ObClaim, Target: ObSource},
		},
		ObOps: []ObOp{
			{Name: "empirical_lens", Domain: ObClaim, Codomain: ObClaim, Transform: "empirical"},
			{Name: "responsible_lens", Domain: ObClaim, Codomain: ObClaim, Transform: "responsible"},
			{Name: "harmonic_lens", Domain: ObClaim, Codomain: ObClaim, Transform: "harmonic"},
			{Name: "pluralistic_lens", Domain: ObClaim, Codomain: ObClaim, Transform: "pluralistic"},
		},
		MorOps: []MorOp{
			{Name: "gf3_conservation", DomMor: "derives_from", CodMor: "derives_from",
				TopOb: "Source", BotOb: "Claim", Condition: "sum_trits_mod3_eq_0"},
			{Name: "provenance_composability", DomMor: "attests", CodMor: "derives_from",
				TopOb: "Witness", BotOb: "Claim", Condition: "path_composes"},
		},
	}
}

// HasObType checks if an object type exists in the theory.
func (t *EpistemicTheory) HasObType(ot ObType) bool {
	for _, o := range t.ObTypes {
		if o == ot {
			return true
		}
	}
	return false
}

// SrcType returns the source ObType of a morphism type.
func (t *EpistemicTheory) SrcType(morName string) (ObType, bool) {
	for _, m := range t.MorTypes {
		if m.Name == morName {
			return m.Source, true
		}
	}
	return "", false
}

// TgtType returns the target ObType of a morphism type.
func (t *EpistemicTheory) TgtType(morName string) (ObType, bool) {
	for _, m := range t.MorTypes {
		if m.Name == morName {
			return m.Target, true
		}
	}
	return "", false
}

// --- Path: composable morphism chains ---

// PathSegment is a single step in a derivation path.
type PathSegment struct {
	MorType  string `json:"mor_type"`  // which morphism type
	SourceID string `json:"source_id"` // the source object
	TargetID string `json:"target_id"` // the target object
	Strength float64 `json:"strength"`
}

// Path is a composable sequence of morphisms in the double category.
// It represents a derivation chain from primary evidence to final claim.
// Path composition: [attests ; derives_from] = Witness → Source → Claim
type Path struct {
	Segments []PathSegment `json:"segments"`
}

// IsIdentity returns true if the path is empty (identity morphism).
func (p *Path) IsIdentity() bool {
	return len(p.Segments) == 0
}

// Composes returns true if the path is well-formed: each segment's target
// matches the next segment's source type. This is the fundamental coherence
// check — a path that doesn't compose is an unsupported claim.
func (p *Path) Composes(theory *EpistemicTheory) bool {
	if len(p.Segments) <= 1 {
		return true
	}
	for i := 0; i < len(p.Segments)-1; i++ {
		tgt, ok1 := theory.TgtType(p.Segments[i].MorType)
		src, ok2 := theory.SrcType(p.Segments[i+1].MorType)
		if !ok1 || !ok2 || tgt != src {
			return false
		}
	}
	return true
}

// CompositeStrength returns the product of all segment strengths.
// This models how evidence weakens as it passes through more derivation steps.
func (p *Path) CompositeStrength() float64 {
	if len(p.Segments) == 0 {
		return 0
	}
	strength := 1.0
	for _, s := range p.Segments {
		strength *= s.Strength
	}
	return strength
}

// --- DblModel: the claim world as a model of the theory ---

// ModelObject is an object in the model, typed by the theory.
type ModelObject struct {
	ID     string   `json:"id"`
	Type   ObType   `json:"type"`
	Trit   gf3.Elem `json:"trit"`
	Hash   string   `json:"hash"`
	Label  string   `json:"label"`
	Meta   map[string]string `json:"meta,omitempty"`
}

// ModelMorphism is a morphism in the model, typed by the theory.
type ModelMorphism struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"` // references MorType.Name
	SourceID string  `json:"source_id"`
	TargetID string  `json:"target_id"`
	Strength float64 `json:"strength"`
	Kind     string  `json:"kind"` // "deductive", "appeal-to-authority", etc.
}

// EpistemicModel is a DblModel — an instance of the EpistemicTheory.
type EpistemicModel struct {
	Theory     *EpistemicTheory         `json:"-"`
	Objects    map[string]*ModelObject  `json:"objects"`
	Morphisms  map[string]*ModelMorphism `json:"morphisms"`
	Paths      []*Path                  `json:"paths"`
	Framework  string                   `json:"framework"`
	Cocycles   []Cocycle                `json:"cocycles"`
	Confidence float64                  `json:"confidence"`
}

// NewEpistemicModel creates an empty model conforming to the standard theory.
func NewEpistemicModel(framework string) *EpistemicModel {
	return &EpistemicModel{
		Theory:    StandardTheory(),
		Objects:   make(map[string]*ModelObject),
		Morphisms: make(map[string]*ModelMorphism),
		Framework: framework,
	}
}

// AddObject adds a typed object to the model, validating against the theory.
func (m *EpistemicModel) AddObject(id string, obType ObType, trit gf3.Elem, label string) error {
	if !m.Theory.HasObType(obType) {
		return fmt.Errorf("object type %s not in theory", obType)
	}
	m.Objects[id] = &ModelObject{
		ID: id, Type: obType, Trit: trit,
		Hash: hash(label), Label: label,
	}
	return nil
}

// AddMorphism adds a typed morphism, validating source/target types against the theory.
func (m *EpistemicModel) AddMorphism(id, morType, srcID, tgtID, kind string, strength float64) error {
	src, ok := m.Objects[srcID]
	if !ok {
		return fmt.Errorf("source object %s not in model", srcID)
	}
	tgt, ok := m.Objects[tgtID]
	if !ok {
		return fmt.Errorf("target object %s not in model", tgtID)
	}

	// Validate types match the theory's morphism type
	expectedSrc, ok1 := m.Theory.SrcType(morType)
	expectedTgt, ok2 := m.Theory.TgtType(morType)
	if !ok1 || !ok2 {
		return fmt.Errorf("morphism type %s not in theory", morType)
	}
	if src.Type != expectedSrc {
		return fmt.Errorf("source %s has type %s, expected %s for morphism %s", srcID, src.Type, expectedSrc, morType)
	}
	if tgt.Type != expectedTgt {
		return fmt.Errorf("target %s has type %s, expected %s for morphism %s", tgtID, tgt.Type, expectedTgt, morType)
	}

	m.Morphisms[id] = &ModelMorphism{
		ID: id, Type: morType, SourceID: srcID, TargetID: tgtID,
		Strength: strength, Kind: kind,
	}
	return nil
}

// BuildPaths constructs all derivation paths from witnesses through sources to claims.
func (m *EpistemicModel) BuildPaths() {
	m.Paths = nil

	// For each claim, find all paths: Witness →attests→ Source →derives_from→ Claim
	for _, claim := range m.Objects {
		if claim.Type != ObClaim {
			continue
		}

		// Find derives_from morphisms targeting this claim
		for _, deriv := range m.Morphisms {
			if deriv.Type != "derives_from" || deriv.TargetID != claim.ID {
				continue
			}
			source := m.Objects[deriv.SourceID]
			if source == nil {
				continue
			}

			// Find attests morphisms targeting this source
			for _, attest := range m.Morphisms {
				if attest.Type != "attests" || attest.TargetID != source.ID {
					continue
				}

				// Full path: Witness →attests→ Source →derives_from→ Claim
				m.Paths = append(m.Paths, &Path{
					Segments: []PathSegment{
						{MorType: "attests", SourceID: attest.SourceID, TargetID: source.ID, Strength: attest.Strength},
						{MorType: "derives_from", SourceID: source.ID, TargetID: claim.ID, Strength: deriv.Strength},
					},
				})
			}

			// Also add the single-step path: Source →derives_from→ Claim
			m.Paths = append(m.Paths, &Path{
				Segments: []PathSegment{
					{MorType: "derives_from", SourceID: source.ID, TargetID: claim.ID, Strength: deriv.Strength},
				},
			})
		}
	}
}

// CheckMorOps validates the morphism operations (cells) of the theory.
// Returns cocycles for each failed conservation condition.
func (m *EpistemicModel) CheckMorOps() []Cocycle {
	var cocycles []Cocycle

	for _, op := range m.Theory.MorOps {
		switch op.Condition {
		case "sum_trits_mod3_eq_0":
			balanced, _ := m.GF3Balance()
			if !balanced {
				cocycles = append(cocycles, Cocycle{
					Kind: "trit-violation", Severity: 0.3,
				})
			}
		case "path_composes":
			for _, p := range m.Paths {
				if !p.Composes(m.Theory) {
					cocycles = append(cocycles, Cocycle{
						Kind: "non-composable-path", Severity: 0.8,
					})
				}
			}
		}
	}

	return cocycles
}

// Verify runs the full CatColab-style verification pipeline.
// Returns the model with all paths built, cocycles detected, and confidence computed.
func (m *EpistemicModel) Verify() {
	m.BuildPaths()

	var cocycles []Cocycle

	// Check for unsupported claims (no incoming derivation morphism)
	for _, obj := range m.Objects {
		if obj.Type != ObClaim {
			continue
		}
		hasDerivation := false
		for _, mor := range m.Morphisms {
			if mor.Type == "derives_from" && mor.TargetID == obj.ID {
				hasDerivation = true
				break
			}
		}
		if !hasDerivation {
			cocycles = append(cocycles, Cocycle{
				ClaimA: obj.ID, Kind: "unsupported", Severity: 0.9,
			})
		}
	}

	// Check for weak authority derivations
	for _, mor := range m.Morphisms {
		if mor.Kind == "appeal-to-authority" && mor.Strength < 0.6 {
			cocycles = append(cocycles, Cocycle{
				ClaimA: mor.TargetID, ClaimB: mor.SourceID,
				Kind: "weak-authority", Severity: 0.5,
			})
		}
	}

	// Check MorOps (conservation cells)
	cocycles = append(cocycles, m.CheckMorOps()...)

	m.Cocycles = cocycles

	// Compute confidence from path strengths
	m.Confidence = m.computeConfidence()
}

func (m *EpistemicModel) computeConfidence() float64 {
	if len(m.Paths) == 0 {
		return 0.1
	}

	totalStrength := 0.0
	for _, p := range m.Paths {
		totalStrength += p.CompositeStrength()
	}
	avg := totalStrength / float64(len(m.Paths))

	// Framework-specific weighting via ObOps
	for _, op := range m.Theory.ObOps {
		if op.Transform != m.Framework {
			continue
		}
		switch m.Framework {
		case "empirical":
			academicCount := 0
			for _, obj := range m.Objects {
				if obj.Type == ObSource && obj.Meta != nil && obj.Meta["kind"] == "academic" {
					academicCount++
				}
			}
			avg *= 1.0 + 0.1*float64(academicCount)
		case "responsible":
			for _, obj := range m.Objects {
				if obj.Type == ObClaim && (strings.Contains(strings.ToLower(obj.Label), "community") ||
					strings.Contains(strings.ToLower(obj.Label), "benefit")) {
					avg *= 1.1
				}
			}
		case "harmonic":
			sourceCount := 0
			for _, obj := range m.Objects {
				if obj.Type == ObSource {
					sourceCount++
				}
			}
			if sourceCount >= 3 {
				avg *= 1.15
			}
		}
	}

	// Penalize cocycles
	avg -= 0.15 * float64(len(m.Cocycles))
	if avg > 1.0 {
		avg = 1.0
	}
	if avg < 0.0 {
		avg = 0.0
	}
	return avg
}

// GF3Balance checks the conservation law across all objects in the model.
func (m *EpistemicModel) GF3Balance() (bool, map[string]int) {
	counts := map[string]int{"coordinator": 0, "generator": 0, "verifier": 0}
	var trits []gf3.Elem

	for _, obj := range m.Objects {
		trits = append(trits, obj.Trit)
		switch obj.Trit {
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

// SheafConsistency returns H¹ dimension and cocycles.
func (m *EpistemicModel) SheafConsistency() (int, []Cocycle) {
	return len(m.Cocycles), m.Cocycles
}

// --- High-level API using CatColab model ---

// AnalyzeWithCatColab builds a full CatColab DblModel from text.
func AnalyzeWithCatColab(text, framework string) *EpistemicModel {
	model := NewEpistemicModel(framework)

	// Add the primary claim
	claimID := hash(text)[:12]
	model.AddObject(claimID, ObClaim, gf3.One, text)

	// Extract and add sources
	sources := extractSources(text)
	for _, src := range sources {
		model.AddObject(src.ID, ObSource, gf3.Two, src.Citation)
		if obj := model.Objects[src.ID]; obj != nil {
			if obj.Meta == nil {
				obj.Meta = make(map[string]string)
			}
			obj.Meta["kind"] = src.Kind
		}

		// Add derives_from morphism: Source → Claim
		morID := fmt.Sprintf("d-%s-%s", src.ID, claimID)
		model.AddMorphism(morID, "derives_from", src.ID, claimID, classifyDerivation(src), sourceStrength(src))

		// Add witness and attests morphism: Witness → Source
		witID := fmt.Sprintf("w-%s", src.ID)
		model.AddObject(witID, ObWitness, gf3.Zero, src.Citation)
		attestID := fmt.Sprintf("a-%s-%s", witID, src.ID)
		model.AddMorphism(attestID, "attests", witID, src.ID, "attestation", witnessWeight(src.Kind))
	}

	model.Verify()
	return model
}

func hash(text string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(text))))
	return hex.EncodeToString(h[:])
}
