//go:build darwin

// Package tape implements a TUI QuickTime alternative: causal tape recording
// at 1 FPS with Lamport clocks for distributed session convergence.
//
// The causal model: every participant maintains a Lamport clock. When a frame
// is captured (local event), the clock increments. When a frame is received
// over the network, the clock advances to max(local, remote)+1. This ensures
// all tapes converge to the same causal partial order regardless of wall-clock
// skew across machines.
package tape

import (
	"encoding/json"
	"sync"
	"sync/atomic"
)

// LamportClock is a monotonic logical clock for causal ordering.
// Two events a, b are causally ordered iff clock(a) < clock(b).
// Concurrent events (same clock value from different nodes) are
// ordered by NodeID as tiebreaker.
type LamportClock struct {
	tick   atomic.Uint64
	NodeID string
}

// NewLamportClock creates a clock for the given node.
func NewLamportClock(nodeID string) *LamportClock {
	return &LamportClock{NodeID: nodeID}
}

// Tick increments the clock for a local event and returns the new value.
func (lc *LamportClock) Tick() uint64 {
	return lc.tick.Add(1)
}

// Witness advances the clock on receiving a remote timestamp.
// Returns max(local, remote) + 1.
func (lc *LamportClock) Witness(remote uint64) uint64 {
	for {
		local := lc.tick.Load()
		next := remote
		if local > next {
			next = local
		}
		next++
		if lc.tick.CompareAndSwap(local, next) {
			return next
		}
	}
}

// Now returns the current clock value without incrementing.
func (lc *LamportClock) Now() uint64 {
	return lc.tick.Load()
}

// CausalOrder compares two timestamps from potentially different nodes.
// Returns -1 if a < b, 0 if concurrent, 1 if a > b.
func CausalOrder(aTick uint64, aNode string, bTick uint64, bNode string) int {
	if aTick < bTick {
		return -1
	}
	if aTick > bTick {
		return 1
	}
	if aNode < bNode {
		return -1
	}
	if aNode > bNode {
		return 1
	}
	return 0
}

// CausalFrame is a frame annotated with causal metadata for merging tapes.
type CausalFrame struct {
	Frame
	Delivered bool `json:"-"` // whether this frame has been delivered to the local tape
}

// MergeTapes merges multiple tapes into a single causally-ordered tape.
// Frames are sorted by Lamport timestamp, with NodeID as tiebreaker.
// This is the convergence point: all observers produce the same merged tape.
func MergeTapes(tapes ...*Tape) *Tape {
	if len(tapes) == 0 {
		return NewTape("merged", "merged")
	}

	merged := NewTape("merged", "merged")

	var allFrames []Frame
	for _, t := range tapes {
		t.mu.RLock()
		allFrames = append(allFrames, t.Frames...)
		t.mu.RUnlock()
	}

	// Sort by causal order
	sortFrames(allFrames)

	merged.mu.Lock()
	merged.Frames = allFrames
	merged.mu.Unlock()

	return merged
}

func sortFrames(frames []Frame) {
	n := len(frames)
	for i := 1; i < n; i++ {
		key := frames[i]
		j := i - 1
		for j >= 0 && CausalOrder(frames[j].LamportTS, frames[j].NodeID, key.LamportTS, key.NodeID) > 0 {
			frames[j+1] = frames[j]
			j--
		}
		frames[j+1] = key
	}
}

// VectorClock tracks causal dependencies across multiple nodes.
// Used for determining happens-before between frames from different sources.
type VectorClock struct {
	mu     sync.RWMutex
	clocks map[string]uint64
}

// NewVectorClock creates a vector clock.
func NewVectorClock() *VectorClock {
	return &VectorClock{clocks: make(map[string]uint64)}
}

// Increment advances the clock for a node.
func (vc *VectorClock) Increment(nodeID string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.clocks[nodeID]++
}

// Merge takes the component-wise max with a remote vector clock.
func (vc *VectorClock) Merge(remote map[string]uint64) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	for k, v := range remote {
		if v > vc.clocks[k] {
			vc.clocks[k] = v
		}
	}
}

// Snapshot returns a copy of the current vector clock.
func (vc *VectorClock) Snapshot() map[string]uint64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	snap := make(map[string]uint64, len(vc.clocks))
	for k, v := range vc.clocks {
		snap[k] = v
	}
	return snap
}

// MarshalJSON implements json.Marshaler.
func (vc *VectorClock) MarshalJSON() ([]byte, error) {
	return json.Marshal(vc.Snapshot())
}
