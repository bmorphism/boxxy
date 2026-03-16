//go:build darwin

package tape

import (
	"fmt"
	"testing"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

func BenchmarkMergeTapes(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("frames=%d", n), func(b *testing.B) {
			t1 := makeBenchTape("a", n/2)
			t2 := makeBenchTape("b", n/2)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				MergeTapes(t1, t2)
			}
		})
	}
}

func BenchmarkLamportClockTick(b *testing.B) {
	c := NewLamportClock("bench")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Tick()
	}
}

func BenchmarkLamportClockWitness(b *testing.B) {
	c := NewLamportClock("bench")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Witness(uint64(i))
	}
}

func BenchmarkVectorClockMerge(b *testing.B) {
	vc := NewVectorClock()
	remote := map[string]uint64{"a": 100, "b": 200, "c": 300}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vc.Merge(remote)
	}
}

func BenchmarkCausalOrder(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CausalOrder(uint64(i), "node-a", uint64(i+1), "node-b")
	}
}

func BenchmarkSheafConsistency(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("frames=%d", n), func(b *testing.B) {
			t1 := makeBenchTape("a", n/2)
			t2 := makeBenchTape("b", n/2)
			world := NewTapeWorld(t1, t2)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CheckSheafConsistency(world)
			}
		})
	}
}

func BenchmarkGF3Balance(b *testing.B) {
	tape := makeBenchTape("bench", 1000)
	world := NewTapeWorld(tape)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		world.GF3Conservation()
	}
}

func BenchmarkSaveJSONL(b *testing.B) {
	tape := makeBenchTape("bench", 100)
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tape.SaveJSONL(fmt.Sprintf("%s/bench-%d.jsonl", dir, i))
	}
}

func BenchmarkDiffRatio(b *testing.B) {
	a := "the quick brown fox jumps over the lazy dog and more text here for length"
	c := "the quick brown cat jumps over the lazy dog and more text here for width"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		diffRatio(a, c)
	}
}

func BenchmarkArchiveSampleMutate(b *testing.B) {
	a := NewArchive(50)
	capFn := func() (string, int, int, error) { return "bench", 80, 24, nil }
	a.EvolveN(10, capFn, 100*time.Millisecond)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parent := a.Sample()
		a.Mutate(parent)
	}
}

func makeBenchTape(nodeID string, n int) *Tape {
	t := NewTape(nodeID, "bench")
	for i := 0; i < n; i++ {
		t.Frames = append(t.Frames, Frame{
			SeqNo:     uint64(i),
			LamportTS: uint64(i * 2),
			NodeID:    nodeID,
			WallTime:  time.Now(),
			Width:     80,
			Height:    24,
			Content:   fmt.Sprintf("frame %d content with some data", i),
			Trit:      gf3.Elem(i % 3),
		})
	}
	return t
}
