//go:build darwin

// daemon.go implements the self-evolving daemon: a continuous DGM loop
// that runs in the background, autonomously improving capture strategies.
//
// The daemon implements the autopoietic cycle from the self-evolving-agent
// pattern: it observes its own performance, mutates its strategies, and
// replaces the running capture function when a better strategy is found.
//
// The DGM loop runs forever:
//   1. Sample parent from archive (fitness-proportionate)
//   2. Mutate: perturb capture parameters
//   3. Evaluate: measure information density over a trial
//   4. If novel and fit, insert into archive
//   5. If best agent changed, hot-swap the active recorder's strategy
//   6. Periodically persist archive for cross-session memory
//
// This is the core autopoietic structure: the system that maintains
// itself also evolves itself.
package tape

import (
	"fmt"
	"sync"
	"time"
)

// DaemonConfig controls the self-evolving daemon behavior.
type DaemonConfig struct {
	ArchivePath     string        // Path for cross-session persistence
	ArchiveMaxSize  int           // Maximum agents in archive
	TrialDuration   time.Duration // How long each agent evaluation runs
	EvolveInterval  time.Duration // Time between evolution rounds
	SaveInterval    int           // Save archive every N generations
	HotSwap         bool          // Whether to hot-swap recorder params
	MaxGenerations  int           // 0 = infinite
}

// DefaultDaemonConfig returns sensible defaults for the self-evolving daemon.
func DefaultDaemonConfig() DaemonConfig {
	return DaemonConfig{
		ArchivePath:    DefaultArchivePath(),
		ArchiveMaxSize: 30,
		TrialDuration:  3 * time.Second,
		EvolveInterval: 10 * time.Second,
		SaveInterval:   10,
		HotSwap:        true,
		MaxGenerations: 0,
	}
}

// Daemon is the self-evolving tape capture daemon.
type Daemon struct {
	mu       sync.Mutex
	config   DaemonConfig
	archive  *Archive
	recorder *Recorder
	capFn    CaptureFunc
	stop     chan struct{}
	done     chan struct{}
	running  bool
	stats    DaemonStats
	onEvolve func(DaemonEvent)
}

// DaemonStats tracks the daemon's self-evolution progress.
type DaemonStats struct {
	TotalGenerations int           `json:"total_generations"`
	TotalSaves       int           `json:"total_saves"`
	HotSwaps         int           `json:"hot_swaps"`
	BestFitness      float64       `json:"best_fitness"`
	BestAgentID      string        `json:"best_agent_id"`
	Uptime           time.Duration `json:"uptime"`
	LastEvolveAt     time.Time     `json:"last_evolve_at"`
	LastSaveAt       time.Time     `json:"last_save_at"`
}

// DaemonEvent is emitted when the daemon makes progress.
type DaemonEvent struct {
	Type       string  `json:"type"` // "evolved", "hot-swap", "saved", "started", "stopped"
	Generation int     `json:"generation"`
	Fitness    float64 `json:"fitness"`
	AgentID    string  `json:"agent_id"`
	Message    string  `json:"message"`
}

// NewDaemon creates a self-evolving daemon.
func NewDaemon(recorder *Recorder, capFn CaptureFunc, config DaemonConfig) (*Daemon, error) {
	archive, err := LoadArchive(config.ArchivePath, config.ArchiveMaxSize)
	if err != nil {
		return nil, fmt.Errorf("load archive: %w", err)
	}

	return &Daemon{
		config:   config,
		archive:  archive,
		recorder: recorder,
		capFn:    capFn,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}, nil
}

// OnEvolve registers a callback for daemon events.
func (d *Daemon) OnEvolve(fn func(DaemonEvent)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onEvolve = fn
}

// Start begins the self-evolving daemon loop.
func (d *Daemon) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("daemon already running")
	}
	d.running = true
	d.mu.Unlock()

	d.emit(DaemonEvent{
		Type:    "started",
		Message: fmt.Sprintf("DGM daemon started, archive has %d agents", d.archive.Count()),
	})

	go d.loop()
	return nil
}

// Stop halts the daemon and persists the final archive state.
func (d *Daemon) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	d.mu.Unlock()

	close(d.stop)
	<-d.done

	// Final save
	if err := d.archive.SaveArchive(d.config.ArchivePath); err == nil {
		d.mu.Lock()
		d.stats.TotalSaves++
		d.stats.LastSaveAt = time.Now()
		d.mu.Unlock()
	}

	d.emit(DaemonEvent{
		Type:    "stopped",
		Message: fmt.Sprintf("DGM daemon stopped after %d generations", d.stats.TotalGenerations),
	})
}

// Archive returns the current DGM archive.
func (d *Daemon) Archive() *Archive {
	return d.archive
}

// Stats returns the daemon's progress statistics.
func (d *Daemon) Stats() DaemonStats {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stats
}

func (d *Daemon) loop() {
	defer close(d.done)
	startTime := time.Now()

	ticker := time.NewTicker(d.config.EvolveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
			d.evolveStep(startTime)
		}
	}
}

func (d *Daemon) evolveStep(startTime time.Time) {
	// Check generation limit
	if d.config.MaxGenerations > 0 && d.stats.TotalGenerations >= d.config.MaxGenerations {
		return
	}

	prevBest := d.archive.Best()
	prevBestID := ""
	if prevBest != nil {
		prevBestID = prevBest.ID
	}

	// DGM step: sample → mutate → evaluate → insert
	parent := d.archive.Sample()
	if parent == nil {
		return
	}

	child := d.archive.Mutate(parent)
	d.archive.Evaluate(child, d.capFn, d.config.TrialDuration)
	inserted := d.archive.Insert(child)

	d.mu.Lock()
	d.stats.TotalGenerations++
	d.stats.Uptime = time.Since(startTime)
	d.stats.LastEvolveAt = time.Now()
	d.mu.Unlock()

	if inserted {
		d.emit(DaemonEvent{
			Type:       "evolved",
			Generation: child.Generation,
			Fitness:    child.Fitness,
			AgentID:    child.ID,
			Message:    fmt.Sprintf("gen %d: new agent %s (fitness=%.3f)", child.Generation, child.ID, child.Fitness),
		})
	}

	// Check for hot-swap: if a better agent is found, apply its params to the live recorder
	newBest := d.archive.Best()
	if d.config.HotSwap && newBest != nil && newBest.ID != prevBestID {
		// Actually apply the evolved params to the running recorder
		d.recorder.ApplyParams(newBest.Params)

		d.mu.Lock()
		d.stats.HotSwaps++
		d.stats.BestFitness = newBest.Fitness
		d.stats.BestAgentID = newBest.ID
		d.mu.Unlock()

		d.emit(DaemonEvent{
			Type:       "hot-swap",
			Generation: newBest.Generation,
			Fitness:    newBest.Fitness,
			AgentID:    newBest.ID,
			Message:    fmt.Sprintf("hot-swap: applied agent %s (fitness=%.3f, interval=%dms, diff=%.2f)", newBest.ID, newBest.Fitness, newBest.Params.IntervalMs, newBest.Params.DiffThreshold),
		})
	}

	// Auto-save
	if d.archive.AutoSave(d.config.ArchivePath, d.config.SaveInterval) {
		d.mu.Lock()
		d.stats.TotalSaves++
		d.stats.LastSaveAt = time.Now()
		d.mu.Unlock()

		d.emit(DaemonEvent{
			Type:    "saved",
			Message: fmt.Sprintf("archive saved (%d agents, gen %d)", d.archive.Count(), d.archive.Generation()),
		})
	}
}

func (d *Daemon) emit(ev DaemonEvent) {
	d.mu.Lock()
	fn := d.onEvolve
	d.mu.Unlock()
	if fn != nil {
		fn(ev)
	}
}
