package demon

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestDemonSortMinimizesEntropy(t *testing.T) {
	cfg := DefaultConfig(4)
	cfg.Epsilon = 0 // Pure exploit for deterministic test
	cfg.ProbeInterval = 5 * time.Millisecond

	d := New(cfg)

	// Set up simulated path profiles: path 0 is fastest
	sim := &SimulatedProber{}
	sim.SetProfiles([]PathProfile{
		{BaseLatency: 5 * time.Millisecond, Jitter: 1 * time.Millisecond},
		{BaseLatency: 15 * time.Millisecond, Jitter: 3 * time.Millisecond},
		{BaseLatency: 30 * time.Millisecond, Jitter: 5 * time.Millisecond},
		{BaseLatency: 100 * time.Millisecond, Jitter: 20 * time.Millisecond},
	})
	d.prober.SetBackend(sim)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := d.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Let probes run to establish path metrics
	time.Sleep(200 * time.Millisecond)

	// Sort 100 packets
	for i := 0; i < 100; i++ {
		pkt := &Packet{
			Data:      []byte("test"),
			StreamID:  uint64(i),
			Timestamp: time.Now(),
			Size:      4,
		}
		d.Sort(pkt)
	}

	stats := d.Stats()

	// Verify: path 0 should get the most packets (lowest RTT)
	if len(stats.Paths) < 4 {
		t.Fatalf("expected 4 paths, got %d", len(stats.Paths))
	}

	// With ε=0 (pure exploit), path 0 should get all packets
	if stats.Paths[0].SendCount < 90 {
		t.Errorf("path 0 (fastest) got only %d packets, expected ≥90", stats.Paths[0].SendCount)
	}

	// Entropy after sorting should be less than max entropy
	maxEntropy := math.Log2(4)
	if stats.EntropyAfter >= maxEntropy {
		t.Errorf("entropy after sorting (%.3f) should be < max entropy (%.3f)",
			stats.EntropyAfter, maxEntropy)
	}

	// Landauer cost should be positive
	if stats.LandauerBitsErased <= 0 {
		t.Error("Landauer bits erased should be positive")
	}

	t.Logf("Stats: sorted=%d, probes=%d, entropy=%.3f->%.3f, landauer=%.1f bits",
		stats.TotalSorted, stats.TotalProbes,
		stats.EntropyBefore, stats.EntropyAfter, stats.LandauerBitsErased)

	d.Stop()
}

func TestDemonBatchStriping(t *testing.T) {
	cfg := DefaultConfig(3)
	cfg.Epsilon = 0
	cfg.ProbeInterval = 5 * time.Millisecond

	d := New(cfg)

	sim := &SimulatedProber{}
	sim.SetProfiles([]PathProfile{
		{BaseLatency: 10 * time.Millisecond, Jitter: 1 * time.Millisecond},
		{BaseLatency: 20 * time.Millisecond, Jitter: 2 * time.Millisecond},
		{BaseLatency: 40 * time.Millisecond, Jitter: 4 * time.Millisecond},
	})
	d.prober.SetBackend(sim)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.Start(ctx)
	time.Sleep(200 * time.Millisecond)

	// Batch sort: should stripe proportional to inverse RTT
	batch := make([]*Packet, 60)
	for i := range batch {
		batch[i] = &Packet{
			Data:      []byte("batch"),
			StreamID:  uint64(i),
			Timestamp: time.Now(),
			Size:      5,
		}
	}

	d.SortBatch(batch)

	// Check distribution: path 0 should get the most
	counts := [3]int{}
	for _, p := range batch {
		counts[p.AssignedPath]++
	}

	t.Logf("Batch distribution: path0=%d, path1=%d, path2=%d", counts[0], counts[1], counts[2])

	if counts[0] <= counts[2] {
		t.Errorf("path 0 (fastest) got %d packets, path 2 (slowest) got %d; expected path 0 > path 2",
			counts[0], counts[2])
	}

	d.Stop()
}

func TestTopologyEmbedding(t *testing.T) {
	builder := NewTopologyBuilder()

	// Simulate 4 paths between 2 endpoints
	for i := 0; i < 4; i++ {
		for j := 0; j < 10; j++ {
			builder.AddFlow(FlowRecord{
				Timestamp: time.Now(),
				SrcAddr:   "10.0.0.1",
				DstAddr:   "10.0.0.2",
				PathID:    PathID(i),
				RTT:       time.Duration(5+i*10) * time.Millisecond,
				Size:      1000,
			})
		}
	}

	// Add a third endpoint to create triangles
	for i := 0; i < 2; i++ {
		builder.AddFlow(FlowRecord{
			Timestamp: time.Now(),
			SrcAddr:   "10.0.0.1",
			DstAddr:   "10.0.0.3",
			PathID:    PathID(i),
			RTT:       time.Duration(8+i*5) * time.Millisecond,
			Size:      500,
		})
		builder.AddFlow(FlowRecord{
			Timestamp: time.Now(),
			SrcAddr:   "10.0.0.2",
			DstAddr:   "10.0.0.3",
			PathID:    PathID(i),
			RTT:       time.Duration(12+i*7) * time.Millisecond,
			Size:      500,
		})
	}

	sc := builder.Build()

	if len(sc.Vertices) != 3 {
		t.Errorf("expected 3 vertices, got %d", len(sc.Vertices))
	}

	if len(sc.Edges) < 4 {
		t.Errorf("expected ≥4 edges, got %d", len(sc.Edges))
	}

	t.Logf("Topology: %d vertices, %d edges, %d triangles",
		len(sc.Vertices), len(sc.Edges), len(sc.Triangles))
	t.Logf("Betti numbers: β0=%d, β1=%d, β2=%d",
		sc.BettiNumbers[0], sc.BettiNumbers[1], sc.BettiNumbers[2])
	t.Logf("Euler characteristic: %d", sc.EulerChar)
	t.Logf("H0 pairs: %d, H1 pairs: %d", len(sc.H0), len(sc.H1))

	// β0 should be 1 (all connected)
	if sc.BettiNumbers[0] != 1 {
		t.Errorf("expected β0=1 (connected), got %d", sc.BettiNumbers[0])
	}

	// Persistence diagram should have entries
	diagram := sc.PersistenceDiagram()
	if len(diagram) == 0 {
		t.Error("persistence diagram should not be empty")
	}
	t.Logf("Persistence diagram: %d pairs", len(diagram))
}

func TestSpectacleRendering(t *testing.T) {
	cfg := DefaultConfig(3)
	d := New(cfg)

	sim := &SimulatedProber{}
	sim.SetProfiles(DefaultProfiles(3))
	d.prober.SetBackend(sim)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	arena := DefaultArenaConfig(3)
	spec := NewSpectacle(arena, d)

	frame := spec.RenderFrame()

	if frame.Tick != 1 {
		t.Errorf("expected tick 1, got %d", frame.Tick)
	}

	if len(frame.PathStats) != 3 {
		t.Errorf("expected 3 path stats, got %d", len(frame.PathStats))
	}

	// Should have block updates for initial render
	if len(frame.Blocks) == 0 {
		t.Log("warning: no block updates in first frame (paths may not be probed yet)")
	}

	jsonStr := frame.JSON()
	if len(jsonStr) < 10 {
		t.Error("frame JSON too short")
	}

	t.Logf("Frame: tick=%d, blocks=%d, particles=%d, entropy=%.3f",
		frame.Tick, len(frame.Blocks), len(frame.Particles), frame.Entropy)

	d.Stop()
}

func TestStripeByWeight(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		weights []float64
	}{
		{"uniform", 12, []float64{1, 1, 1}},
		{"weighted", 10, []float64{0.5, 0.3, 0.2}},
		{"single", 5, []float64{1.0}},
		{"zero_total", 4, []float64{0, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripeByWeight(tt.n, tt.weights)
			total := 0
			for _, c := range result {
				total += c
				if c < 0 {
					t.Errorf("negative assignment: %v", result)
				}
			}
			if total != tt.n {
				t.Errorf("total %d != expected %d, assignments=%v", total, tt.n, result)
			}
		})
	}
}
