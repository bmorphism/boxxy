package demon

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// PathMetrics tracks quality measurements for a single network path.
// The demon reads these to decide which "chamber" each packet enters.
type PathMetrics struct {
	ID PathID

	mu          sync.RWMutex
	rttSamples  []time.Duration
	rttIdx      int
	rttFull     bool
	smoothedRTT time.Duration
	rttVar      time.Duration
	minRTT      time.Duration
	maxRTT      time.Duration
	lastProbe   time.Time
	lossCount   atomic.Int64
	sendCount   atomic.Int64
	recvCount   atomic.Int64
	bytesOut    atomic.Int64
	bytesIn     atomic.Int64
	alive       atomic.Bool
}

const rttWindowSize = 64

// NewPathMetrics creates metrics for a path.
func NewPathMetrics(id PathID) *PathMetrics {
	m := &PathMetrics{
		ID:         id,
		rttSamples: make([]time.Duration, rttWindowSize),
		minRTT:     time.Duration(math.MaxInt64),
	}
	m.alive.Store(true)
	return m
}

// RecordRTT records a probe round-trip time sample.
// Uses exponentially weighted moving average (EWMA) like QUIC's RTT estimator.
func (m *PathMetrics) RecordRTT(rtt time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rttSamples[m.rttIdx] = rtt
	m.rttIdx = (m.rttIdx + 1) % rttWindowSize
	if m.rttIdx == 0 {
		m.rttFull = true
	}
	m.lastProbe = time.Now()

	if rtt < m.minRTT {
		m.minRTT = rtt
	}
	if rtt > m.maxRTT {
		m.maxRTT = rtt
	}

	// EWMA: smoothed_rtt = 7/8 * smoothed_rtt + 1/8 * rtt
	if m.smoothedRTT == 0 {
		m.smoothedRTT = rtt
		m.rttVar = rtt / 2
	} else {
		diff := m.smoothedRTT - rtt
		if diff < 0 {
			diff = -diff
		}
		m.rttVar = (3*m.rttVar + diff) / 4
		m.smoothedRTT = (7*m.smoothedRTT + rtt) / 8
	}
}

// RecordLoss records a detected packet loss on this path.
func (m *PathMetrics) RecordLoss() {
	m.lossCount.Add(1)
}

// RecordSend records an outgoing packet.
func (m *PathMetrics) RecordSend(size int) {
	m.sendCount.Add(1)
	m.bytesOut.Add(int64(size))
}

// RecordRecv records an incoming packet.
func (m *PathMetrics) RecordRecv(size int) {
	m.recvCount.Add(1)
	m.bytesIn.Add(int64(size))
}

// SetAlive marks the path as reachable or unreachable.
func (m *PathMetrics) SetAlive(alive bool) {
	m.alive.Store(alive)
}

// IsAlive returns whether the path is currently reachable.
func (m *PathMetrics) IsAlive() bool {
	return m.alive.Load()
}

// SmoothedRTT returns the EWMA RTT estimate.
func (m *PathMetrics) SmoothedRTT() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.smoothedRTT
}

// RTTVariance returns the RTT variance estimate.
func (m *PathMetrics) RTTVariance() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rttVar
}

// MinRTT returns the minimum observed RTT.
func (m *PathMetrics) MinRTT() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.minRTT == time.Duration(math.MaxInt64) {
		return 0
	}
	return m.minRTT
}

// Jitter returns the absolute RTT variance — the demon's primary sorting signal.
// Low jitter = predictable path = demon prefers this for latency-sensitive traffic.
func (m *PathMetrics) Jitter() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rttVar
}

// LossRate returns the observed loss rate as a fraction [0, 1].
func (m *PathMetrics) LossRate() float64 {
	sent := m.sendCount.Load()
	if sent == 0 {
		return 0
	}
	lost := m.lossCount.Load()
	return float64(lost) / float64(sent)
}

// SendCount returns total packets sent on this path.
func (m *PathMetrics) SendCount() int64 {
	return m.sendCount.Load()
}

// Score computes a composite path quality score.
// Lower is better. The demon uses this to sort packets.
//
// Score = smoothed_rtt + 4*rtt_var + penalty*loss_rate
//
// This mirrors QUIC's PTO calculation: PTO = smoothed_rtt + max(4*rttvar, kGranularity)
func (m *PathMetrics) Score() float64 {
	m.mu.RLock()
	srtt := m.smoothedRTT
	rvar := m.rttVar
	m.mu.RUnlock()

	if srtt == 0 {
		return math.MaxFloat64 // Unknown path — worst score until probed
	}

	loss := m.LossRate()
	lossPenalty := 100.0 * time.Millisecond.Seconds() // 100ms penalty per 100% loss

	score := srtt.Seconds() + 4*rvar.Seconds() + lossPenalty*loss

	if !m.alive.Load() {
		score = math.MaxFloat64
	}

	return score
}

// Stats returns a snapshot of path statistics.
func (m *PathMetrics) Stats() PathStats {
	m.mu.RLock()
	srtt := m.smoothedRTT
	rvar := m.rttVar
	minRTT := m.minRTT
	maxRTT := m.maxRTT
	lastProbe := m.lastProbe
	m.mu.RUnlock()

	if minRTT == time.Duration(math.MaxInt64) {
		minRTT = 0
	}

	return PathStats{
		ID:          m.ID,
		SmoothedRTT: srtt,
		RTTVariance: rvar,
		MinRTT:      minRTT,
		MaxRTT:      maxRTT,
		Jitter:      rvar,
		LossRate:    m.LossRate(),
		SendCount:   m.sendCount.Load(),
		RecvCount:   m.recvCount.Load(),
		BytesOut:    m.bytesOut.Load(),
		BytesIn:     m.bytesIn.Load(),
		Alive:       m.alive.Load(),
		LastProbe:   lastProbe,
		Score:       m.Score(),
	}
}

// PathStats is a JSON-serializable snapshot of path quality.
type PathStats struct {
	ID          PathID        `json:"id"`
	SmoothedRTT time.Duration `json:"smoothed_rtt"`
	RTTVariance time.Duration `json:"rtt_variance"`
	MinRTT      time.Duration `json:"min_rtt"`
	MaxRTT      time.Duration `json:"max_rtt"`
	Jitter      time.Duration `json:"jitter"`
	LossRate    float64       `json:"loss_rate"`
	SendCount   int64         `json:"send_count"`
	RecvCount   int64         `json:"recv_count"`
	BytesOut    int64         `json:"bytes_out"`
	BytesIn     int64         `json:"bytes_in"`
	Alive       bool          `json:"alive"`
	LastProbe   time.Time     `json:"last_probe"`
	Score       float64       `json:"score"`
}
