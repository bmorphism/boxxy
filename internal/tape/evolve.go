//go:build darwin

// evolve.go implements Darwin Godel Machine patterns for self-evolving
// tape capture strategies. Capture functions are treated as agents in an
// archive; each generation mutates capture parameters (timing, encoding,
// compression, diff detection) and evaluates fitness by frame information
// density per byte.
//
// The DGM loop:
//   1. Sample capture agent from archive (fitness-proportionate)
//   2. Mutate: adjust parameters (fps jitter, diff threshold, compression)
//   3. Evaluate: measure information density of captured frames
//   4. If novel and fit, add to archive
//   5. Prune archive to maintain diversity
package tape

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// CaptureAgent is an evolved capture strategy in the DGM archive.
type CaptureAgent struct {
	ID         string            `json:"id"`
	Generation int               `json:"generation"`
	ParentID   string            `json:"parent_id,omitempty"`
	Fitness    float64           `json:"fitness"`
	Trit       gf3.Elem          `json:"trit"`
	Params     CaptureParams     `json:"params"`
	Stats      CaptureStats      `json:"stats"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// CaptureParams are the evolvable parameters of a capture strategy.
type CaptureParams struct {
	IntervalMs     int     `json:"interval_ms"`     // Base capture interval (1000 = 1 FPS)
	JitterMs       int     `json:"jitter_ms"`       // Random jitter ±ms added to interval
	DiffThreshold  float64 `json:"diff_threshold"`  // Min change ratio to store frame (0.0 = all)
	MaxContentLen  int     `json:"max_content_len"` // Truncate content beyond this
	CompressFrames bool    `json:"compress_frames"` // Deduplicate identical runs
	CursorTracking bool    `json:"cursor_tracking"` // Include cursor position data
}

// DefaultCaptureParams returns the baseline 1-FPS strategy.
func DefaultCaptureParams() CaptureParams {
	return CaptureParams{
		IntervalMs:     1000,
		JitterMs:       0,
		DiffThreshold:  0.0,
		MaxContentLen:  16384,
		CompressFrames: false,
		CursorTracking: false,
	}
}

// CaptureStats tracks evaluation metrics for a capture agent.
type CaptureStats struct {
	FramesCaptured    int     `json:"frames_captured"`
	FramesSkipped     int     `json:"frames_skipped"`
	TotalBytes        int64   `json:"total_bytes"`
	UniqueBytes       int64   `json:"unique_bytes"`
	AvgInfoDensity    float64 `json:"avg_info_density"`
	CaptureLatencyUs  int64   `json:"capture_latency_us"`
	EvalDurationMs    int64   `json:"eval_duration_ms"`
}

// Archive holds the population of capture agents for the DGM loop.
type Archive struct {
	mu             sync.RWMutex
	agents         []*CaptureAgent
	maxSize        int
	generation     int
	diversityThresh float64
	bestFitness    float64
}

// NewArchive creates a DGM archive seeded with the default agent.
func NewArchive(maxSize int) *Archive {
	seed := &CaptureAgent{
		ID:         agentID("seed", 0),
		Generation: 0,
		Fitness:    0.1,
		Trit:       gf3.Zero,
		Params:     DefaultCaptureParams(),
	}

	return &Archive{
		agents:          []*CaptureAgent{seed},
		maxSize:         maxSize,
		diversityThresh: 0.3,
	}
}

// Sample selects an agent from the archive via fitness-proportionate selection.
func (a *Archive) Sample() *CaptureAgent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.agents) == 0 {
		return nil
	}
	if len(a.agents) == 1 {
		return a.agents[0]
	}

	// Fitness-proportionate selection
	total := 0.0
	for _, ag := range a.agents {
		total += math.Max(ag.Fitness, 0.01)
	}

	// Deterministic selection using generation as seed
	h := sha256.Sum256([]byte(fmt.Sprintf("sample-%d", a.generation)))
	rval := float64(binary.LittleEndian.Uint64(h[:8])) / float64(math.MaxUint64)
	target := rval * total

	cumulative := 0.0
	for _, ag := range a.agents {
		cumulative += math.Max(ag.Fitness, 0.01)
		if cumulative >= target {
			return ag
		}
	}
	return a.agents[len(a.agents)-1]
}

// Mutate creates a child agent by perturbing the parent's parameters.
// Mutations are bounded to maintain valid configurations.
func (a *Archive) Mutate(parent *CaptureAgent) *CaptureAgent {
	a.mu.Lock()
	a.generation++
	gen := a.generation
	a.mu.Unlock()

	child := &CaptureAgent{
		ID:         agentID(parent.ID, gen),
		Generation: gen,
		ParentID:   parent.ID,
		Trit:       gf3.Elem(gen % 3),
		Params:     parent.Params,
	}

	// Deterministic mutations based on generation
	h := sha256.Sum256([]byte(fmt.Sprintf("mutate-%s-%d", parent.ID, gen)))
	mutations := binary.LittleEndian.Uint64(h[:8])

	// Mutate interval: ±200ms, clamped to [200, 5000]
	if mutations&1 != 0 {
		delta := int((mutations>>1)%401) - 200
		child.Params.IntervalMs = clampInt(parent.Params.IntervalMs+delta, 200, 5000)
	}

	// Mutate jitter: ±100ms, clamped to [0, 500]
	if mutations&2 != 0 {
		delta := int((mutations>>2)%201) - 100
		child.Params.JitterMs = clampInt(parent.Params.JitterMs+delta, 0, 500)
	}

	// Mutate diff threshold: ±0.1, clamped to [0.0, 0.9]
	if mutations&4 != 0 {
		delta := float64(int((mutations>>3)%21)-10) / 100.0
		child.Params.DiffThreshold = clampFloat(parent.Params.DiffThreshold+delta, 0.0, 0.9)
	}

	// Mutate max content length: ±4096, clamped to [1024, 65536]
	if mutations&8 != 0 {
		delta := int((mutations>>4)%8193) - 4096
		child.Params.MaxContentLen = clampInt(parent.Params.MaxContentLen+delta, 1024, 65536)
	}

	// Toggle compression
	if mutations&16 != 0 {
		child.Params.CompressFrames = !parent.Params.CompressFrames
	}

	// Toggle cursor tracking
	if mutations&32 != 0 {
		child.Params.CursorTracking = !parent.Params.CursorTracking
	}

	return child
}

// Evaluate measures a capture agent's fitness by running it for a trial period.
// Fitness = information density = unique bytes / total bytes, weighted by latency.
func (a *Archive) Evaluate(agent *CaptureAgent, capFn CaptureFunc, trialDuration time.Duration) float64 {
	start := time.Now()

	var frames []Frame
	var totalBytes, uniqueBytes int64
	var lastContent string
	var skipped int

	interval := time.Duration(agent.Params.IntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deadline := time.After(trialDuration)
	seq := uint64(0)

	for {
		select {
		case <-deadline:
			goto done
		case <-ticker.C:
			content, w, h, err := capFn()
			if err != nil {
				continue
			}

			// Apply max content length
			if len(content) > agent.Params.MaxContentLen {
				content = content[:agent.Params.MaxContentLen]
			}

			// Apply diff threshold
			if agent.Params.DiffThreshold > 0 && lastContent != "" {
				diff := diffRatio(lastContent, content)
				if diff < agent.Params.DiffThreshold {
					skipped++
					continue
				}
			}

			seq++
			totalBytes += int64(len(content))

			// Measure unique content
			if content != lastContent {
				uniqueBytes += int64(len(content))
			}

			// Apply compression (deduplicate runs)
			if agent.Params.CompressFrames && content == lastContent {
				skipped++
				continue
			}

			frames = append(frames, Frame{
				SeqNo:   seq,
				NodeID:  "eval",
				Width:   w,
				Height:  h,
				Content: content,
			})
			lastContent = content
		}
	}

done:
	elapsed := time.Since(start)

	agent.Stats = CaptureStats{
		FramesCaptured:   len(frames),
		FramesSkipped:    skipped,
		TotalBytes:       totalBytes,
		UniqueBytes:      uniqueBytes,
		EvalDurationMs:   elapsed.Milliseconds(),
	}

	if totalBytes == 0 {
		agent.Fitness = 0.01
		return agent.Fitness
	}

	// Fitness = information density * frame rate efficiency
	infoDensity := float64(uniqueBytes) / float64(totalBytes)
	frameEfficiency := float64(len(frames)) / float64(len(frames)+skipped+1)

	agent.Fitness = infoDensity*0.7 + frameEfficiency*0.3
	agent.Stats.AvgInfoDensity = infoDensity

	return agent.Fitness
}

// Insert adds an agent to the archive if it's novel and fit enough.
// Returns true if the agent was accepted.
func (a *Archive) Insert(agent *CaptureAgent) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check novelty
	for _, existing := range a.agents {
		if paramsSimilarity(existing.Params, agent.Params) > (1 - a.diversityThresh) {
			return false
		}
	}

	a.agents = append(a.agents, agent)

	if agent.Fitness > a.bestFitness {
		a.bestFitness = agent.Fitness
	}

	// Prune if over capacity
	if len(a.agents) > a.maxSize {
		sort.Slice(a.agents, func(i, j int) bool {
			return a.agents[i].Fitness > a.agents[j].Fitness
		})
		a.agents = a.agents[:a.maxSize]
	}

	return true
}

// Best returns the highest-fitness agent in the archive.
func (a *Archive) Best() *CaptureAgent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.agents) == 0 {
		return nil
	}

	best := a.agents[0]
	for _, ag := range a.agents[1:] {
		if ag.Fitness > best.Fitness {
			best = ag
		}
	}
	return best
}

// Count returns the number of agents in the archive.
func (a *Archive) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.agents)
}

// Generation returns the current generation counter.
func (a *Archive) Generation() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.generation
}

// GF3Status returns the trit distribution across the archive.
func (a *Archive) GF3Status() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var trits [3]int
	for _, ag := range a.agents {
		trits[ag.Trit]++
	}

	elems := make([]gf3.Elem, len(a.agents))
	for i, ag := range a.agents {
		elems[i] = ag.Trit
	}

	return map[string]interface{}{
		"agents":       len(a.agents),
		"generation":   a.generation,
		"coordinators": trits[0],
		"generators":   trits[1],
		"verifiers":    trits[2],
		"balanced":     gf3.IsBalanced(elems),
		"best_fitness": a.bestFitness,
	}
}

// EvolveN runs N generations of the DGM loop.
func (a *Archive) EvolveN(n int, capFn CaptureFunc, trialDuration time.Duration) *CaptureAgent {
	for i := 0; i < n; i++ {
		parent := a.Sample()
		if parent == nil {
			continue
		}
		child := a.Mutate(parent)
		a.Evaluate(child, capFn, trialDuration)
		a.Insert(child)
	}
	return a.Best()
}

// --- Helpers ---

func agentID(parentID string, gen int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", parentID, gen)))
	return fmt.Sprintf("agent-%x", h[:4])
}

func diffRatio(a, b string) float64 {
	if a == b {
		return 0.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 1.0
	}

	// Simple character-level diff ratio
	shorter, longer := a, b
	if len(a) > len(b) {
		shorter, longer = b, a
	}

	matches := 0
	for i := 0; i < len(shorter); i++ {
		if shorter[i] == longer[i] {
			matches++
		}
	}
	return 1.0 - float64(matches)/float64(len(longer))
}

func paramsSimilarity(a, b CaptureParams) float64 {
	score := 0.0
	dims := 0.0

	// Interval similarity
	dims++
	maxInterval := 5000.0
	score += 1.0 - math.Abs(float64(a.IntervalMs-b.IntervalMs))/maxInterval

	// Jitter similarity
	dims++
	maxJitter := 500.0
	score += 1.0 - math.Abs(float64(a.JitterMs-b.JitterMs))/maxJitter

	// Diff threshold similarity
	dims++
	score += 1.0 - math.Abs(a.DiffThreshold-b.DiffThreshold)

	// Content length similarity
	dims++
	maxLen := 65536.0
	score += 1.0 - math.Abs(float64(a.MaxContentLen-b.MaxContentLen))/maxLen

	// Boolean features
	dims++
	if a.CompressFrames == b.CompressFrames {
		score++
	}
	dims++
	if a.CursorTracking == b.CursorTracking {
		score++
	}

	return score / dims
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
