//go:build darwin

// acset.go implements the ACSets (Attributed C-Sets) schema for tape frames,
// integrating with plurigrid/asi's categorical data model.
//
// The schema:
//   @present SchTapeWorld(FreeSchema) begin
//     Node::Ob         -- recording participants
//     Frame::Ob        -- captured terminal snapshots
//     Tape::Ob         -- ordered sequence of frames
//     Edge::Ob         -- causal edges between frames
//
//     source::Hom(Edge, Frame)
//     target::Hom(Edge, Frame)
//     tape_frame::Hom(Frame, Tape)
//     node_tape::Hom(Tape, Node)
//
//     Seed::AttrType
//     Trit::AttrType
//     Timestamp::AttrType
//     Content::AttrType
//
//     seed::Attr(Node, Seed)
//     trit::Attr(Frame, Trit)
//     lamport::Attr(Frame, Timestamp)
//     content::Attr(Frame, Content)
//   end
package tape

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// ACSetSchema describes the categorical schema for tape worlds.
type ACSetSchema struct {
	Objects     []string            `json:"objects"`
	Morphisms   []ACSetMorphism     `json:"morphisms"`
	AttrTypes   []string            `json:"attr_types"`
	Attributes  []ACSetAttribute    `json:"attributes"`
}

// ACSetMorphism is a hom in the schema: source -> target.
type ACSetMorphism struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// ACSetAttribute is an attribute assignment: object -> type.
type ACSetAttribute struct {
	Name   string `json:"name"`
	Object string `json:"object"`
	Type   string `json:"type"`
}

// TapeWorldSchema returns the canonical ACSets schema for tape recording.
func TapeWorldSchema() ACSetSchema {
	return ACSetSchema{
		Objects: []string{"Node", "Frame", "Tape", "Edge"},
		Morphisms: []ACSetMorphism{
			{Name: "source", Source: "Edge", Target: "Frame"},
			{Name: "target", Source: "Edge", Target: "Frame"},
			{Name: "tape_frame", Source: "Frame", Target: "Tape"},
			{Name: "node_tape", Source: "Tape", Target: "Node"},
		},
		AttrTypes: []string{"Seed", "Trit", "Timestamp", "Content"},
		Attributes: []ACSetAttribute{
			{Name: "seed", Object: "Node", Type: "Seed"},
			{Name: "trit", Object: "Frame", Type: "Trit"},
			{Name: "lamport", Object: "Frame", Type: "Timestamp"},
			{Name: "content", Object: "Frame", Type: "Content"},
		},
	}
}

// TapeWorld is an ACSet instance conforming to TapeWorldSchema.
type TapeWorld struct {
	Schema  ACSetSchema          `json:"schema"`
	Nodes   map[string]*TWNode   `json:"nodes"`
	Tapes   map[string]*TWTape   `json:"tapes"`
	Frames  []*TWFrame           `json:"frames"`
	Edges   []*TWEdge            `json:"edges"`
}

// TWNode is an ACSet node (recording participant).
type TWNode struct {
	ID   string   `json:"id"`
	Seed uint64   `json:"seed"`
	Trit gf3.Elem `json:"trit"`
}

// TWTape is an ACSet tape (ordered frame sequence from one node).
type TWTape struct {
	ID     string `json:"id"`
	NodeID string `json:"node_id"`
	Label  string `json:"label"`
}

// TWFrame is an ACSet frame with causal metadata.
type TWFrame struct {
	ID        int      `json:"id"`
	TapeID    string   `json:"tape_id"`
	LamportTS uint64   `json:"lamport"`
	NodeID    string   `json:"node_id"`
	Trit      gf3.Elem `json:"trit"`
	Content   string   `json:"content"`
	Width     int      `json:"w"`
	Height    int      `json:"h"`
}

// TWEdge is a causal edge: source frame happens-before target frame.
type TWEdge struct {
	ID       int    `json:"id"`
	SourceID int    `json:"source_id"`
	TargetID int    `json:"target_id"`
	Relation string `json:"relation"` // "causal", "concurrent", "merge"
}

// NewTapeWorld creates a world instance from tape data.
func NewTapeWorld(tapes ...*Tape) *TapeWorld {
	w := &TapeWorld{
		Schema: TapeWorldSchema(),
		Nodes:  make(map[string]*TWNode),
		Tapes:  make(map[string]*TWTape),
	}

	frameID := 0
	for _, t := range tapes {
		// Register node
		if _, exists := w.Nodes[t.NodeID]; !exists {
			w.Nodes[t.NodeID] = &TWNode{
				ID:   t.NodeID,
				Trit: gf3.Elem(len(w.Nodes) % 3),
			}
		}

		// Register tape
		tapeKey := fmt.Sprintf("%s/%s", t.NodeID, t.Label)
		w.Tapes[tapeKey] = &TWTape{
			ID:     tapeKey,
			NodeID: t.NodeID,
			Label:  t.Label,
		}

		// Register frames
		t.mu.RLock()
		for _, f := range t.Frames {
			frameID++
			w.Frames = append(w.Frames, &TWFrame{
				ID:        frameID,
				TapeID:    tapeKey,
				LamportTS: f.LamportTS,
				NodeID:    f.NodeID,
				Trit:      f.Trit,
				Content:   f.Content,
				Width:     f.Width,
				Height:    f.Height,
			})
		}
		t.mu.RUnlock()
	}

	// Build causal edges from Lamport ordering
	w.buildCausalEdges()

	return w
}

// buildCausalEdges constructs edges based on Lamport happens-before.
func (w *TapeWorld) buildCausalEdges() {
	// Sort frames by Lamport timestamp
	sorted := make([]*TWFrame, len(w.Frames))
	copy(sorted, w.Frames)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].LamportTS == sorted[j].LamportTS {
			return sorted[i].NodeID < sorted[j].NodeID
		}
		return sorted[i].LamportTS < sorted[j].LamportTS
	})

	edgeID := 0
	for i := 1; i < len(sorted); i++ {
		prev := sorted[i-1]
		curr := sorted[i]

		relation := "causal"
		if prev.LamportTS == curr.LamportTS {
			relation = "concurrent"
		}

		edgeID++
		w.Edges = append(w.Edges, &TWEdge{
			ID:       edgeID,
			SourceID: prev.ID,
			TargetID: curr.ID,
			Relation: relation,
		})
	}
}

// GF3Conservation checks that the trit sum across all frames is balanced.
func (w *TapeWorld) GF3Conservation() (bool, map[string]int) {
	counts := map[string]int{
		"coordinator": 0,
		"generator":   0,
		"verifier":    0,
	}

	elems := make([]gf3.Elem, len(w.Frames))
	for i, f := range w.Frames {
		elems[i] = f.Trit
		switch f.Trit {
		case gf3.Zero:
			counts["coordinator"]++
		case gf3.One:
			counts["generator"]++
		case gf3.Two:
			counts["verifier"]++
		}
	}

	return gf3.IsBalanced(elems), counts
}

// BisimulationVerify checks that two tape worlds are observationally
// equivalent: same causal structure, same trit assignments.
// Returns true if bisimilar (convergent).
func BisimulationVerify(a, b *TapeWorld) bool {
	// Check frame count
	if len(a.Frames) != len(b.Frames) {
		return false
	}

	// Check causal edge count
	if len(a.Edges) != len(b.Edges) {
		return false
	}

	// Check Lamport ordering is identical
	aSorted := sortedLamports(a.Frames)
	bSorted := sortedLamports(b.Frames)

	for i := range aSorted {
		if aSorted[i] != bSorted[i] {
			return false
		}
	}

	// Check trit assignments match
	for i := range a.Frames {
		if a.Frames[i].Trit != b.Frames[i].Trit {
			return false
		}
	}

	return true
}

func sortedLamports(frames []*TWFrame) []uint64 {
	ts := make([]uint64, len(frames))
	for i, f := range frames {
		ts[i] = f.LamportTS
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	return ts
}

// ToASIRegistry exports the tape world's skills as ASI registry entries.
func (w *TapeWorld) ToASIRegistry() []map[string]interface{} {
	var entries []map[string]interface{}

	for _, node := range w.Nodes {
		entries = append(entries, map[string]interface{}{
			"id":       fmt.Sprintf("tape-node-%s", node.ID),
			"name":     fmt.Sprintf("tape-recorder-%s", node.ID),
			"trit":     node.Trit,
			"role":     gf3.ElemToRole(node.Trit).String(),
			"category": "tape-recording",
		})
	}

	return entries
}

// ToJSON serializes the world to JSON.
func (w *TapeWorld) ToJSON() ([]byte, error) {
	return json.MarshalIndent(w, "", "  ")
}

// URIScheme returns the tape world's URI in the plurigrid/asi namespace.
func (w *TapeWorld) URIScheme() string {
	if len(w.Nodes) == 0 {
		return "tape://local/default"
	}
	for id := range w.Nodes {
		return fmt.Sprintf("tape://%s/world", id)
	}
	return "tape://unknown/world"
}
