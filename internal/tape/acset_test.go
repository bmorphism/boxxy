//go:build darwin

package tape

import (
	"testing"
	"time"
)

func TestTapeWorldSchema(t *testing.T) {
	schema := TapeWorldSchema()

	if len(schema.Objects) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(schema.Objects))
	}
	if len(schema.Morphisms) != 4 {
		t.Fatalf("expected 4 morphisms, got %d", len(schema.Morphisms))
	}
	if len(schema.Attributes) != 4 {
		t.Fatalf("expected 4 attributes, got %d", len(schema.Attributes))
	}
}

func TestNewTapeWorld(t *testing.T) {
	t1 := NewTape("node-a", "session-1")
	t1.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "node-a", Content: "hello", Trit: 0, WallTime: time.Now()})
	t1.Append(Frame{SeqNo: 2, LamportTS: 3, NodeID: "node-a", Content: "world", Trit: 1, WallTime: time.Now()})

	t2 := NewTape("node-b", "session-2")
	t2.Append(Frame{SeqNo: 1, LamportTS: 2, NodeID: "node-b", Content: "foo", Trit: 2, WallTime: time.Now()})

	world := NewTapeWorld(t1, t2)

	if len(world.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(world.Nodes))
	}
	if len(world.Frames) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(world.Frames))
	}
	if len(world.Edges) != 2 {
		t.Fatalf("expected 2 causal edges, got %d", len(world.Edges))
	}

	// Verify causal edges exist with correct relations
	// Lamport order: 1(node-a) -> 2(node-b) -> 3(node-a)
	for _, edge := range world.Edges {
		if edge.Relation != "causal" && edge.Relation != "concurrent" {
			t.Errorf("unexpected edge relation: %s", edge.Relation)
		}
	}
}

func TestGF3Conservation(t *testing.T) {
	tape := NewTape("node", "test")
	// Add frames with trits 0, 1, 2 -> sum = 3 ≡ 0 mod 3
	tape.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "node", Trit: 0, WallTime: time.Now()})
	tape.Append(Frame{SeqNo: 2, LamportTS: 2, NodeID: "node", Trit: 1, WallTime: time.Now()})
	tape.Append(Frame{SeqNo: 3, LamportTS: 3, NodeID: "node", Trit: 2, WallTime: time.Now()})

	world := NewTapeWorld(tape)
	balanced, counts := world.GF3Conservation()

	if !balanced {
		t.Fatalf("expected balanced GF(3), got counts=%v", counts)
	}
	if counts["coordinator"] != 1 || counts["generator"] != 1 || counts["verifier"] != 1 {
		t.Fatalf("unexpected distribution: %v", counts)
	}
}

func TestBisimulationVerify(t *testing.T) {
	makeTape := func() *Tape {
		tp := NewTape("node-a", "s1")
		tp.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "node-a", Trit: 0, WallTime: time.Now()})
		tp.Append(Frame{SeqNo: 2, LamportTS: 2, NodeID: "node-a", Trit: 1, WallTime: time.Now()})
		return tp
	}

	w1 := NewTapeWorld(makeTape())
	w2 := NewTapeWorld(makeTape())

	if !BisimulationVerify(w1, w2) {
		t.Fatal("identical tape worlds should be bisimilar")
	}

	// Add extra frame to break bisimilarity
	w2.Frames = append(w2.Frames, &TWFrame{ID: 99, LamportTS: 99, Trit: 2})
	if BisimulationVerify(w1, w2) {
		t.Fatal("different worlds should not be bisimilar")
	}
}

func TestToASIRegistry(t *testing.T) {
	tape := NewTape("alice", "demo")
	tape.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "alice", Trit: 0, WallTime: time.Now()})

	world := NewTapeWorld(tape)
	entries := world.ToASIRegistry()

	if len(entries) != 1 {
		t.Fatalf("expected 1 registry entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry["category"] != "tape-recording" {
		t.Fatalf("expected category tape-recording, got %v", entry["category"])
	}
}

func TestTapeWorldJSON(t *testing.T) {
	tape := NewTape("json-node", "json-test")
	tape.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "json-node", Trit: 0, Content: "test", WallTime: time.Now()})

	world := NewTapeWorld(tape)
	data, err := world.ToJSON()
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}

func TestURIScheme(t *testing.T) {
	tape := NewTape("my-node", "demo")
	world := NewTapeWorld(tape)

	uri := world.URIScheme()
	if uri != "tape://my-node/world" {
		t.Fatalf("expected tape://my-node/world, got %s", uri)
	}
}
