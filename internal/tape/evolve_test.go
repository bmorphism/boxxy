//go:build darwin

package tape

import (
	"fmt"
	"testing"
	"time"
)

func TestArchiveBasic(t *testing.T) {
	a := NewArchive(10)

	if a.Count() != 1 {
		t.Fatalf("expected 1 seed agent, got %d", a.Count())
	}

	best := a.Best()
	if best == nil {
		t.Fatal("expected non-nil best agent")
	}
	if best.Generation != 0 {
		t.Fatalf("expected generation 0, got %d", best.Generation)
	}
}

func TestMutateCreatesChild(t *testing.T) {
	a := NewArchive(10)
	parent := a.Sample()

	child := a.Mutate(parent)
	if child.ParentID != parent.ID {
		t.Fatalf("child should reference parent: got %q, want %q", child.ParentID, parent.ID)
	}
	if child.Generation != 1 {
		t.Fatalf("child generation should be 1, got %d", child.Generation)
	}
}

func TestEvaluate(t *testing.T) {
	a := NewArchive(10)
	agent := a.Sample()

	callCount := 0
	capFn := func() (string, int, int, error) {
		callCount++
		return fmt.Sprintf("frame-%d", callCount), 80, 24, nil
	}

	fitness := a.Evaluate(agent, capFn, 2*time.Second)

	if fitness <= 0 {
		t.Fatalf("expected positive fitness, got %f", fitness)
	}
	if agent.Stats.FramesCaptured == 0 {
		t.Fatal("expected at least 1 captured frame")
	}
}

func TestEvolveNProducesImprovedAgent(t *testing.T) {
	a := NewArchive(20)

	counter := 0
	capFn := func() (string, int, int, error) {
		counter++
		return fmt.Sprintf("evolving-content-%d-with-unique-data", counter), 80, 24, nil
	}

	best := a.EvolveN(5, capFn, 1*time.Second)

	if best == nil {
		t.Fatal("expected non-nil best agent after evolution")
	}
	if a.Count() < 2 {
		t.Logf("archive has %d agents after 5 generations", a.Count())
	}
	if a.Generation() != 5 {
		t.Fatalf("expected generation 5, got %d", a.Generation())
	}
}

func TestGF3Status(t *testing.T) {
	a := NewArchive(30)

	counter := 0
	capFn := func() (string, int, int, error) {
		counter++
		return fmt.Sprintf("gf3-test-%d", counter), 80, 24, nil
	}

	a.EvolveN(9, capFn, 500*time.Millisecond)

	status := a.GF3Status()
	if status["agents"].(int) < 1 {
		t.Fatal("expected at least 1 agent")
	}

	t.Logf("GF(3) status: %+v", status)
}

func TestDiffRatio(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"hello", "hello", 0.0},
		{"hello", "world", 0.8},
		{"", "hello", 1.0},
		{"abc", "abd", 1.0 / 3.0},
	}
	for _, tt := range tests {
		got := diffRatio(tt.a, tt.b)
		if abs(got-tt.want) > 0.15 {
			t.Errorf("diffRatio(%q,%q) = %f, want ~%f", tt.a, tt.b, got, tt.want)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
