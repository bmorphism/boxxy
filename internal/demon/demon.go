// Package demon implements Maxwell's demon for QUIC multipath path probing.
//
// The demon sits at the boundary between Hyperion's proxy egress and the
// network, sorting packets onto paths to minimize delivery-time entropy.
// Each path is continuously probed; the demon's "memory" is the path quality
// state table, and the Landauer erasure cost manifests as probe overhead.
package demon

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// PathID identifies a network path.
type PathID uint32

// Packet is a unit of work the demon must sort onto a path.
type Packet struct {
	Data      []byte
	StreamID  uint64
	Order     uint32
	Timestamp time.Time
	Size      int
	// Set by the demon after sorting:
	AssignedPath PathID
	SortedAt     time.Time
}

// DemonConfig configures the Maxwell's demon instance.
type DemonConfig struct {
	// Paths is the number of available network paths.
	Paths int
	// ProbeInterval controls how often PATH_CHALLENGE probes are sent.
	ProbeInterval time.Duration
	// Epsilon for ε-greedy exploration (0 = pure exploit, 1 = pure explore).
	Epsilon float64
	// ReorderBufferSize is the max packets to hold while waiting for slow paths.
	ReorderBufferSize int
	// FECRedundancy is the fraction of redundant packets (0 = none, 0.5 = 50% overhead).
	FECRedundancy float64
	// OnSort is called each time the demon assigns a packet to a path.
	OnSort func(pkt *Packet, metrics *PathMetrics)
	// OnProbe is called each time a probe completes.
	OnProbe func(pathID PathID, rtt time.Duration)
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig(numPaths int) DemonConfig {
	return DemonConfig{
		Paths:             numPaths,
		ProbeInterval:     10 * time.Millisecond,
		Epsilon:           0.05,
		ReorderBufferSize: 256,
		FECRedundancy:     0.0,
	}
}

// Demon is the Maxwell's demon — an entropy-sorting QUIC path selector.
type Demon struct {
	config  DemonConfig
	metrics []*PathMetrics
	prober  *Prober
	sel     *Selector

	// Statistics
	totalSorted   atomic.Int64
	totalProbes   atomic.Int64
	entropyBefore atomic.Int64 // fixed-point entropy * 1000
	entropyAfter  atomic.Int64

	mu      sync.RWMutex
	running atomic.Bool
	cancel  context.CancelFunc
}

// New creates a new Maxwell's demon.
func New(cfg DemonConfig) *Demon {
	metrics := make([]*PathMetrics, cfg.Paths)
	for i := range metrics {
		metrics[i] = NewPathMetrics(PathID(i))
	}

	d := &Demon{
		config:  cfg,
		metrics: metrics,
	}

	d.prober = NewProber(cfg, metrics)
	d.sel = NewSelector(cfg, metrics)

	return d
}

// Start begins the demon's probing and sorting loops.
func (d *Demon) Start(ctx context.Context) error {
	if d.running.Load() {
		return fmt.Errorf("demon already running")
	}

	ctx, d.cancel = context.WithCancel(ctx)
	d.running.Store(true)

	go d.prober.Run(ctx)

	return nil
}

// Stop halts the demon.
func (d *Demon) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	d.running.Store(false)
}

// Sort assigns a packet to the optimal path, minimizing delivery-time entropy.
// This is the demon's trapdoor operation.
func (d *Demon) Sort(pkt *Packet) PathID {
	// Measure entropy before sorting (uniform distribution assumption)
	beforeEntropy := d.currentEntropy()

	// Select path via ε-greedy with entropy minimization
	pathID := d.sel.Select(pkt)
	pkt.AssignedPath = pathID
	pkt.SortedAt = time.Now()

	// Update metrics
	d.metrics[pathID].RecordSend(pkt.Size)
	d.totalSorted.Add(1)

	// Measure entropy after
	afterEntropy := d.pathEntropy()

	d.entropyBefore.Store(int64(beforeEntropy * 1000))
	d.entropyAfter.Store(int64(afterEntropy * 1000))

	if d.config.OnSort != nil {
		d.config.OnSort(pkt, d.metrics[pathID])
	}

	return pathID
}

// SortBatch sorts a batch of packets, optimizing for minimum aggregate jitter.
// This is the bulk operation for Hyperion's tick-aligned egress buffers.
func (d *Demon) SortBatch(pkts []*Packet) {
	if len(pkts) == 0 {
		return
	}

	// Stripe packets across paths weighted by inverse RTT.
	// Faster paths get more packets so all arrive ~simultaneously.
	weights := d.sel.PathWeights()
	assignments := stripeByWeight(len(pkts), weights)

	idx := 0
	for pathID, count := range assignments {
		for i := 0; i < count && idx < len(pkts); i++ {
			pkts[idx].AssignedPath = PathID(pathID)
			pkts[idx].SortedAt = time.Now()
			d.metrics[pathID].RecordSend(pkts[idx].Size)
			idx++
		}
	}

	d.totalSorted.Add(int64(len(pkts)))
}

// currentEntropy computes the entropy of the current path load distribution.
// Maximum entropy = uniform distribution = demon is not sorting.
// Minimum entropy = all traffic on one path = maximum sorting.
// The demon aims for the entropy that minimizes delivery-time variance.
func (d *Demon) currentEntropy() float64 {
	n := len(d.metrics)
	if n <= 1 {
		return 0
	}
	// Maximum entropy for n paths
	return math.Log2(float64(n))
}

// pathEntropy computes the actual entropy of packet distribution across paths.
func (d *Demon) pathEntropy() float64 {
	total := int64(0)
	counts := make([]int64, len(d.metrics))
	for i, m := range d.metrics {
		c := m.SendCount()
		counts[i] = c
		total += c
	}
	if total == 0 {
		return 0
	}

	entropy := 0.0
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / float64(total)
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// Stats returns the demon's current operating statistics.
func (d *Demon) Stats() DemonStats {
	stats := DemonStats{
		TotalSorted:   d.totalSorted.Load(),
		TotalProbes:   d.prober.TotalProbes(),
		EntropyBefore: float64(d.entropyBefore.Load()) / 1000.0,
		EntropyAfter:  float64(d.entropyAfter.Load()) / 1000.0,
		PathCount:     len(d.metrics),
		Running:       d.running.Load(),
	}

	stats.Paths = make([]PathStats, len(d.metrics))
	for i, m := range d.metrics {
		stats.Paths[i] = m.Stats()
	}

	// Compute Landauer cost: kT ln 2 per bit of path-state information
	// In our model: bits = log2(numPaths) per packet sorted
	bitsPerSort := math.Log2(float64(len(d.metrics)))
	stats.LandauerBitsErased = bitsPerSort * float64(stats.TotalSorted)

	return stats
}

// DemonStats holds the demon's operating statistics.
type DemonStats struct {
	TotalSorted        int64       `json:"total_sorted"`
	TotalProbes        int64       `json:"total_probes"`
	EntropyBefore      float64     `json:"entropy_before"`
	EntropyAfter       float64     `json:"entropy_after"`
	EntropyReduction   float64     `json:"entropy_reduction"`
	LandauerBitsErased float64     `json:"landauer_bits_erased"`
	PathCount          int         `json:"path_count"`
	Running            bool        `json:"running"`
	Paths              []PathStats `json:"paths"`
}

// stripeByWeight distributes n items across paths proportional to weights.
func stripeByWeight(n int, weights []float64) []int {
	if len(weights) == 0 {
		return nil
	}

	total := 0.0
	for _, w := range weights {
		total += w
	}
	if total == 0 {
		// Uniform fallback
		result := make([]int, len(weights))
		each := n / len(weights)
		remainder := n % len(weights)
		for i := range result {
			result[i] = each
			if i < remainder {
				result[i]++
			}
		}
		return result
	}

	result := make([]int, len(weights))
	assigned := 0
	for i, w := range weights {
		result[i] = int(math.Round(float64(n) * w / total))
		assigned += result[i]
	}

	// Fix rounding errors
	diff := n - assigned
	for diff > 0 {
		// Give extra to the highest-weight path
		best := 0
		for i, w := range weights {
			if w > weights[best] {
				best = i
			}
		}
		result[best]++
		diff--
	}
	for diff < 0 {
		// Take from the lowest-weight path that has assignments
		worst := -1
		for i, w := range weights {
			if result[i] > 0 && (worst == -1 || w < weights[worst]) {
				worst = i
			}
		}
		if worst >= 0 {
			result[worst]--
		}
		diff++
	}

	return result
}
