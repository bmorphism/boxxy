package demon

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ProbeFrame mirrors QUIC's PATH_CHALLENGE / PATH_RESPONSE mechanism.
// The demon sends challenges on each path; the response RTT is the
// "molecule velocity measurement" that informs sorting.
type ProbeFrame struct {
	PathID    PathID
	Challenge [8]byte
	SentAt    time.Time
}

// ProbeResult is the demon's observation of a single path measurement.
type ProbeResult struct {
	PathID   PathID
	RTT      time.Duration
	Success  bool
	SentAt   time.Time
	RecvAt   time.Time
}

// PathProber is the interface that path-specific probing backends must implement.
// Real implementations send actual QUIC PATH_CHALLENGE frames;
// the simulated implementation uses goroutine sleeps with jitter.
type PathProber interface {
	// SendChallenge sends a PATH_CHALLENGE on the given path.
	SendChallenge(pathID PathID, challenge [8]byte) error
	// ReceiveResponse waits for a PATH_RESPONSE matching a challenge.
	// Returns error on timeout.
	ReceiveResponse(pathID PathID, challenge [8]byte, timeout time.Duration) error
}

// Prober manages the continuous probing loop across all paths.
type Prober struct {
	config  DemonConfig
	metrics []*PathMetrics
	backend PathProber

	mu         sync.Mutex
	pending    map[[8]byte]ProbeFrame
	totalProbes atomic.Int64
}

// NewProber creates a prober. If no backend is set, uses SimulatedProber.
func NewProber(cfg DemonConfig, metrics []*PathMetrics) *Prober {
	return &Prober{
		config:  cfg,
		metrics: metrics,
		backend: &SimulatedProber{},
		pending: make(map[[8]byte]ProbeFrame),
	}
}

// SetBackend replaces the probing backend (e.g., with a real QUIC path prober).
func (p *Prober) SetBackend(b PathProber) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backend = b
}

// TotalProbes returns the total number of probes sent.
func (p *Prober) TotalProbes() int64 {
	return p.totalProbes.Load()
}

// Run is the main probing loop. It sends PATH_CHALLENGE frames on each path
// in round-robin, measures RTT, and updates PathMetrics.
func (p *Prober) Run(ctx context.Context) {
	ticker := time.NewTicker(p.config.ProbeInterval)
	defer ticker.Stop()

	pathIdx := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pathID := PathID(pathIdx % len(p.metrics))
			pathIdx++

			go p.probeOnce(pathID)
		}
	}
}

// probeOnce sends a single probe on the given path and records the result.
func (p *Prober) probeOnce(pathID PathID) {
	var challenge [8]byte
	if _, err := rand.Read(challenge[:]); err != nil {
		return
	}

	frame := ProbeFrame{
		PathID:    pathID,
		Challenge: challenge,
		SentAt:    time.Now(),
	}

	p.mu.Lock()
	p.pending[challenge] = frame
	backend := p.backend
	p.mu.Unlock()

	p.totalProbes.Add(1)

	// Send challenge
	if err := backend.SendChallenge(pathID, challenge); err != nil {
		p.metrics[pathID].RecordLoss()
		p.metrics[pathID].SetAlive(false)
		p.removePending(challenge)
		return
	}

	// Wait for response (timeout = 2x smoothed RTT or 1 second)
	timeout := 2 * p.metrics[pathID].SmoothedRTT()
	if timeout < 100*time.Millisecond {
		timeout = 100 * time.Millisecond
	}
	if timeout > time.Second {
		timeout = time.Second
	}

	if err := backend.ReceiveResponse(pathID, challenge, timeout); err != nil {
		p.metrics[pathID].RecordLoss()
		p.removePending(challenge)
		return
	}

	// Compute RTT
	rtt := time.Since(frame.SentAt)
	p.metrics[pathID].RecordRTT(rtt)
	p.metrics[pathID].SetAlive(true)
	p.removePending(challenge)

	if p.config.OnProbe != nil {
		p.config.OnProbe(pathID, rtt)
	}
}

func (p *Prober) removePending(challenge [8]byte) {
	p.mu.Lock()
	delete(p.pending, challenge)
	p.mu.Unlock()
}

// SimulatedProber simulates path probing with configurable latency profiles.
// Each path has a base latency and jitter drawn from a normal distribution.
type SimulatedProber struct {
	mu       sync.RWMutex
	profiles []PathProfile
}

// PathProfile defines a simulated path's characteristics.
type PathProfile struct {
	BaseLatency time.Duration
	Jitter      time.Duration // standard deviation
	LossRate    float64       // [0, 1]
}

// DefaultProfiles returns profiles for N paths with increasing latency.
// Path 0 is the "fast chamber", path N-1 is the "slow chamber".
func DefaultProfiles(n int) []PathProfile {
	profiles := make([]PathProfile, n)
	for i := range profiles {
		profiles[i] = PathProfile{
			BaseLatency: time.Duration(5+i*10) * time.Millisecond,
			Jitter:      time.Duration(1+i*2) * time.Millisecond,
			LossRate:    float64(i) * 0.01, // 0%, 1%, 2%, ...
		}
	}
	return profiles
}

// SetProfiles configures the simulated path characteristics.
func (s *SimulatedProber) SetProfiles(profiles []PathProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles = profiles
}

func (s *SimulatedProber) getProfile(pathID PathID) PathProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if int(pathID) < len(s.profiles) {
		return s.profiles[pathID]
	}
	return PathProfile{BaseLatency: 50 * time.Millisecond, Jitter: 10 * time.Millisecond}
}

// SendChallenge simulates sending a PATH_CHALLENGE.
func (s *SimulatedProber) SendChallenge(pathID PathID, challenge [8]byte) error {
	profile := s.getProfile(pathID)

	// Simulate loss
	if profile.LossRate > 0 {
		var b [4]byte
		rand.Read(b[:])
		roll := float64(binary.LittleEndian.Uint32(b[:])) / float64(^uint32(0))
		if roll < profile.LossRate {
			return fmt.Errorf("simulated loss on path %d", pathID)
		}
	}
	return nil
}

// ReceiveResponse simulates waiting for a PATH_RESPONSE.
func (s *SimulatedProber) ReceiveResponse(pathID PathID, challenge [8]byte, timeout time.Duration) error {
	profile := s.getProfile(pathID)

	// Simulate RTT = base + gaussian jitter
	var jitterBytes [8]byte
	rand.Read(jitterBytes[:])
	// Map to [-1, 1] range (crude but sufficient for simulation)
	jitterFrac := (float64(binary.LittleEndian.Uint64(jitterBytes[:])) / float64(^uint64(0))) * 2 - 1
	jitter := time.Duration(float64(profile.Jitter) * jitterFrac)
	delay := profile.BaseLatency + jitter
	if delay < time.Millisecond {
		delay = time.Millisecond
	}

	if delay > timeout {
		time.Sleep(timeout)
		return fmt.Errorf("probe timeout on path %d", pathID)
	}

	time.Sleep(delay)
	return nil
}
