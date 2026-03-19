package demon

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"sort"
)

// Selector implements the ε-greedy entropy-minimizing path selection strategy.
//
// From the SCION paper (arxiv:2509.05938): "Hybrid strategies, particularly
// Epsilon-Greedy, effectively balance efficiency and stability."
//
// The demon's goal: minimize delivery-time variance across paths.
// Pure greedy (ε=0): always pick the lowest-RTT path → causes congestion collapse.
// Pure explore (ε=1): random → no sorting, maximum entropy.
// ε-greedy: exploit the best path (1-ε) of the time, explore others ε of the time.
type Selector struct {
	config  DemonConfig
	metrics []*PathMetrics
}

// NewSelector creates a selector.
func NewSelector(cfg DemonConfig, metrics []*PathMetrics) *Selector {
	return &Selector{
		config:  cfg,
		metrics: metrics,
	}
}

// Select chooses the optimal path for a single packet.
func (s *Selector) Select(pkt *Packet) PathID {
	alive := s.alivePaths()
	if len(alive) == 0 {
		return 0 // Fallback to path 0
	}
	if len(alive) == 1 {
		return alive[0]
	}

	// ε-greedy: explore with probability ε
	if s.shouldExplore() {
		return alive[s.randInt(len(alive))]
	}

	// Exploit: pick the path with the best (lowest) score
	return s.bestPath(alive)
}

// SelectWeighted chooses a path with probability proportional to inverse score.
// Used for batch striping where we want smooth distribution.
func (s *Selector) SelectWeighted() PathID {
	weights := s.PathWeights()
	return s.weightedChoice(weights)
}

// PathWeights returns the weight for each path, proportional to path quality.
// Higher weight = more packets should be sent on this path.
// Weight = 1/score, normalized to sum to 1.
func (s *Selector) PathWeights() []float64 {
	weights := make([]float64, len(s.metrics))
	total := 0.0

	for i, m := range s.metrics {
		if !m.IsAlive() {
			weights[i] = 0
			continue
		}
		score := m.Score()
		if score <= 0 || score == math.MaxFloat64 {
			weights[i] = 0
			continue
		}
		// Inverse score: better paths (lower score) get higher weight
		w := 1.0 / score
		weights[i] = w
		total += w
	}

	// Normalize
	if total > 0 {
		for i := range weights {
			weights[i] /= total
		}
	} else {
		// Uniform fallback
		n := float64(len(weights))
		for i := range weights {
			weights[i] = 1.0 / n
		}
	}

	return weights
}

// DeliveryTimeVariance estimates the variance in delivery times if packets
// are distributed according to the current weights. The demon's objective
// function: minimize this.
func (s *Selector) DeliveryTimeVariance() float64 {
	weights := s.PathWeights()
	if len(weights) == 0 {
		return 0
	}

	// Expected delivery time per path = smoothed_rtt
	// Variance = Σ w_i * (rtt_i - mean_rtt)^2
	meanRTT := 0.0
	for i, m := range s.metrics {
		meanRTT += weights[i] * m.SmoothedRTT().Seconds()
	}

	variance := 0.0
	for i, m := range s.metrics {
		diff := m.SmoothedRTT().Seconds() - meanRTT
		variance += weights[i] * diff * diff
	}

	return variance
}

// EntropyReduction computes how much entropy the demon has removed from the
// path distribution relative to uniform (maximum entropy). This is the
// demon's "work done" — and by Landauer's principle, has a thermodynamic cost.
func (s *Selector) EntropyReduction() float64 {
	maxEntropy := math.Log2(float64(len(s.metrics)))
	if maxEntropy == 0 {
		return 0
	}

	weights := s.PathWeights()
	actualEntropy := 0.0
	for _, w := range weights {
		if w > 0 {
			actualEntropy -= w * math.Log2(w)
		}
	}

	return maxEntropy - actualEntropy
}

func (s *Selector) alivePaths() []PathID {
	var alive []PathID
	for i, m := range s.metrics {
		if m.IsAlive() {
			alive = append(alive, PathID(i))
		}
	}
	return alive
}

func (s *Selector) bestPath(candidates []PathID) PathID {
	if len(candidates) == 0 {
		return 0
	}

	type scored struct {
		id    PathID
		score float64
	}
	paths := make([]scored, len(candidates))
	for i, id := range candidates {
		paths[i] = scored{id, s.metrics[id].Score()}
	}
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].score < paths[j].score
	})

	return paths[0].id
}

func (s *Selector) shouldExplore() bool {
	if s.config.Epsilon <= 0 {
		return false
	}
	if s.config.Epsilon >= 1 {
		return true
	}
	var b [8]byte
	rand.Read(b[:])
	roll := float64(binary.LittleEndian.Uint64(b[:])) / float64(^uint64(0))
	return roll < s.config.Epsilon
}

func (s *Selector) randInt(n int) int {
	var b [8]byte
	rand.Read(b[:])
	return int(binary.LittleEndian.Uint64(b[:]) % uint64(n))
}

func (s *Selector) weightedChoice(weights []float64) PathID {
	var b [8]byte
	rand.Read(b[:])
	roll := float64(binary.LittleEndian.Uint64(b[:])) / float64(^uint64(0))

	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if roll <= cumulative {
			return PathID(i)
		}
	}
	return PathID(len(weights) - 1)
}
