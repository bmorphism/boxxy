//go:build darwin

// autopoiesis.go implements the self-maintaining, self-modifying recorder.
//
// An AutopoieticRecorder wraps a standard Recorder and a DGM Daemon.
// It continuously records at 1 FPS while the daemon evolves capture
// strategies in the background. When the daemon discovers a better
// strategy, the recorder hot-swaps its capture parameters.
//
// This realizes Maturana & Varela's autopoiesis: the system that
// produces its own components (frames) also modifies the production
// process (capture strategy). The organizational closure is:
//
//   record → evaluate → evolve → adapt → record (improved)
//
// The recorder never stops; it gets better over time.
package tape

import (
	"fmt"
	"sync"
	"time"
)

// AutopoieticRecorder combines recording with self-evolving capture.
type AutopoieticRecorder struct {
	mu       sync.Mutex
	recorder *Recorder
	daemon   *Daemon
	gossip   *GossipState
	colors   *SessionColorStream
	events   []DaemonEvent
	maxEvents int
}

// AutopoieticConfig configures the self-evolving recorder.
type AutopoieticConfig struct {
	NodeID     string
	Label      string
	CaptureFn  CaptureFunc
	Daemon     DaemonConfig
	GossipNode bool // enable gossip protocol
}

// NewAutopoieticRecorder creates a recorder that evolves itself.
func NewAutopoieticRecorder(cfg AutopoieticConfig) (*AutopoieticRecorder, error) {
	rec := NewRecorder(cfg.NodeID, cfg.Label, cfg.CaptureFn)

	daemon, err := NewDaemon(rec, cfg.CaptureFn, cfg.Daemon)
	if err != nil {
		return nil, fmt.Errorf("create daemon: %w", err)
	}

	ar := &AutopoieticRecorder{
		recorder:  rec,
		daemon:    daemon,
		colors:    NewSessionColorStream(),
		maxEvents: 100,
	}

	if cfg.GossipNode {
		ar.gossip = NewGossipState(cfg.NodeID)
	}

	// Wire color stream to frames
	rec.OnFrame(func(f Frame) {
		ar.colors.FeedFrame(f)
	})

	// Collect daemon events
	daemon.OnEvolve(func(ev DaemonEvent) {
		ar.mu.Lock()
		ar.events = append(ar.events, ev)
		if len(ar.events) > ar.maxEvents {
			ar.events = ar.events[len(ar.events)-ar.maxEvents:]
		}
		ar.mu.Unlock()
	})

	return ar, nil
}

// Start begins both recording and self-evolution.
func (ar *AutopoieticRecorder) Start() error {
	if err := ar.recorder.Start(); err != nil {
		return err
	}
	return ar.daemon.Start()
}

// Stop halts recording and evolution, persisting the archive.
func (ar *AutopoieticRecorder) Stop() *Tape {
	ar.daemon.Stop()
	return ar.recorder.Stop()
}

// Recorder returns the underlying recorder.
func (ar *AutopoieticRecorder) Recorder() *Recorder {
	return ar.recorder
}

// Daemon returns the underlying daemon.
func (ar *AutopoieticRecorder) Daemon() *Daemon {
	return ar.daemon
}

// Gossip returns the gossip state (nil if not enabled).
func (ar *AutopoieticRecorder) Gossip() *GossipState {
	return ar.gossip
}

// Colors returns the session color stream.
func (ar *AutopoieticRecorder) Colors() *SessionColorStream {
	return ar.colors
}

// Events returns the recent daemon events.
func (ar *AutopoieticRecorder) Events() []DaemonEvent {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	events := make([]DaemonEvent, len(ar.events))
	copy(events, ar.events)
	return events
}

// Status returns the full autopoietic status.
func (ar *AutopoieticRecorder) Status() map[string]interface{} {
	tape := ar.recorder.Tape()
	dStats := ar.daemon.Stats()
	archive := ar.daemon.Archive()

	status := map[string]interface{}{
		"node_id":     tape.NodeID,
		"label":       tape.Label,
		"frames":      tape.Len(),
		"duration":    tape.Duration().String(),
		"lamport":     ar.recorder.clock.Now(),
		"color_seed":  ar.colors.Seed(),
		"daemon": map[string]interface{}{
			"generations": dStats.TotalGenerations,
			"hot_swaps":   dStats.HotSwaps,
			"saves":       dStats.TotalSaves,
			"best_fitness": dStats.BestFitness,
			"best_agent":  dStats.BestAgentID,
			"uptime":      dStats.Uptime.String(),
		},
		"archive": archive.GF3Status(),
	}

	if ar.gossip != nil {
		status["gossip"] = ar.gossip.ConvergenceStatus()
	}

	return status
}

// WaitForEvolution blocks until at least N generations have completed.
func (ar *AutopoieticRecorder) WaitForEvolution(generations int, timeout time.Duration) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return false
		case <-ticker.C:
			if ar.daemon.Stats().TotalGenerations >= generations {
				return true
			}
		}
	}
}
