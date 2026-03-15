//go:build darwin

package tape

import (
	"testing"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

func TestSheafConsistencySingleNode(t *testing.T) {
	tape := NewTape("node-a", "test")
	for i := uint64(1); i <= 5; i++ {
		tape.Append(Frame{
			SeqNo: i, LamportTS: i, NodeID: "node-a",
			Trit: gf3.Elem(i % 3), Content: "frame",
			WallTime: time.Now(),
		})
	}

	world := NewTapeWorld(tape)
	result := CheckSheafConsistency(world)

	if !result.Consistent {
		t.Fatalf("single node tape should be consistent, got %d cocycles", result.H1Dim)
	}
	if result.Sections != 1 {
		t.Fatalf("expected 1 section, got %d", result.Sections)
	}
	if result.Coverage != 1.0 {
		t.Fatalf("expected full coverage, got %.2f", result.Coverage)
	}
}

func TestSheafConsistencyTwoNodes(t *testing.T) {
	t1 := NewTape("node-a", "s1")
	t2 := NewTape("node-b", "s2")

	// Interleaved Lamport timestamps (both nodes active simultaneously)
	t1.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "node-a", Trit: 0, Content: "a1", WallTime: time.Now()})
	t2.Append(Frame{SeqNo: 1, LamportTS: 2, NodeID: "node-b", Trit: 1, Content: "b1", WallTime: time.Now()})
	t1.Append(Frame{SeqNo: 2, LamportTS: 3, NodeID: "node-a", Trit: 2, Content: "a2", WallTime: time.Now()})
	t2.Append(Frame{SeqNo: 2, LamportTS: 4, NodeID: "node-b", Trit: 0, Content: "b2", WallTime: time.Now()})

	world := NewTapeWorld(t1, t2)
	result := CheckSheafConsistency(world)

	t.Logf("sheaf check: consistent=%v sections=%d cocycles=%d coverage=%.2f",
		result.Consistent, result.Sections, result.H1Dim, result.Coverage)

	if result.Sections < 2 {
		t.Fatalf("expected at least 2 sections (one per node), got %d", result.Sections)
	}
}

func TestSheafEmptyWorld(t *testing.T) {
	world := NewTapeWorld()
	result := CheckSheafConsistency(world)

	if !result.Consistent {
		t.Fatal("empty world should be consistent")
	}
	if result.Coverage != 1.0 {
		t.Fatalf("empty world should have full coverage, got %.2f", result.Coverage)
	}
}

func TestSheafGF3Conservation(t *testing.T) {
	tape := NewTape("node", "test")
	// Trits: 0, 1, 2 -> balanced
	tape.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "node", Trit: 0, WallTime: time.Now()})
	tape.Append(Frame{SeqNo: 2, LamportTS: 2, NodeID: "node", Trit: 1, WallTime: time.Now()})
	tape.Append(Frame{SeqNo: 3, LamportTS: 3, NodeID: "node", Trit: 2, WallTime: time.Now()})

	world := NewTapeWorld(tape)
	result := CheckSheafConsistency(world)

	if !result.GF3Balanced {
		t.Fatal("balanced trits should pass GF(3) check")
	}
}
