//go:build darwin

package pinhole

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmorphism/boxxy/internal/demon"
)

// QUICProxyConfig configures the QUIC multipath proxy.
// This extends the existing pinhole Proxy with multipath awareness
// and demon-driven path selection.
type QUICProxyConfig struct {
	// ListenAddr for downstream clients (Minecraft players).
	ListenAddr string
	// UpstreamAddr is Hyperion's proxy address.
	UpstreamAddr string
	// NumPaths is the number of simulated/real network paths.
	NumPaths int
	// DemonConfig configures the Maxwell's demon.
	DemonConfig demon.DemonConfig
	// CaptureConfig for pcap capture (optional).
	CaptureConfig *PinholeConfig
	// ArenaConfig for in-game spectacle (optional).
	ArenaConfig *demon.ArenaConfig
}

// DefaultQUICProxyConfig returns sensible defaults.
func DefaultQUICProxyConfig() QUICProxyConfig {
	return QUICProxyConfig{
		ListenAddr:   "0.0.0.0:25566",
		UpstreamAddr: "127.0.0.1:25565",
		NumPaths:     4,
		DemonConfig:  demon.DefaultConfig(4),
	}
}

// QUICProxy is the Maxwell's demon-driven multipath proxy.
// It sits between Minecraft clients and Hyperion, routing traffic
// through the demon for optimal path selection while capturing
// pcaps for topological analysis.
type QUICProxy struct {
	config    QUICProxyConfig
	demon     *demon.Demon
	bridge    *demon.Bridge
	topology  *demon.TopologyBuilder
	spectacle *demon.Spectacle
	capture   *Capture

	listener net.Listener
	mu       sync.Mutex
	running  atomic.Bool

	// Stats
	clientConns atomic.Int64
	startTime   time.Time
}

// NewQUICProxy creates a new QUIC multipath proxy.
func NewQUICProxy(cfg QUICProxyConfig) *QUICProxy {
	d := demon.New(cfg.DemonConfig)
	topo := demon.NewTopologyBuilder()

	var spec *demon.Spectacle
	if cfg.ArenaConfig != nil {
		spec = demon.NewSpectacle(*cfg.ArenaConfig, d)
	}

	return &QUICProxy{
		config:    cfg,
		demon:     d,
		topology:  topo,
		spectacle: spec,
	}
}

// Start begins the proxy, demon, optional pcap capture, and bridge.
func (q *QUICProxy) Start() error {
	q.startTime = time.Now()

	// Start the demon's probing loop
	if err := q.demon.Start(nil); err != nil {
		return fmt.Errorf("demon start: %w", err)
	}

	// Start pcap capture if configured
	if q.config.CaptureConfig != nil && TsharkAvailable() {
		cfg := q.config.CaptureConfig
		cap, err := StartCapture(cfg.Bridge, cfg.GuestIP, cfg.PcapDir, cfg.SessionID)
		if err != nil {
			// Non-fatal: proxy works without capture
			fmt.Printf("warning: pcap capture failed: %v\n", err)
		} else {
			q.capture = cap
		}
	}

	// Start the Hyperion bridge
	bridgeCfg := demon.BridgeConfig{
		ListenAddr:   q.config.ListenAddr,
		UpstreamAddr: q.config.UpstreamAddr,
		Demon:        q.demon,
		Topology:     q.topology,
		Spectacle:    q.spectacle,
	}
	q.bridge = demon.NewBridge(bridgeCfg)
	if err := q.bridge.Start(); err != nil {
		return fmt.Errorf("bridge start: %w", err)
	}

	q.running.Store(true)
	return nil
}

// Stop shuts down everything.
func (q *QUICProxy) Stop() (*QUICProxyStats, error) {
	q.running.Store(false)

	var captureStats *CaptureStats
	if q.capture != nil {
		var err error
		captureStats, err = q.capture.StopCapture()
		if err != nil {
			fmt.Printf("warning: capture stop: %v\n", err)
		}
	}

	q.demon.Stop()
	q.bridge.Stop()

	// Build final topology
	sc := q.topology.Build()

	stats := &QUICProxyStats{
		Duration:     time.Since(q.startTime),
		DemonStats:   q.demon.Stats(),
		BridgeStats:  q.bridge.Stats(),
		Topology:     sc,
		CaptureStats: captureStats,
	}

	return stats, nil
}

// Demon returns the demon instance for external monitoring.
func (q *QUICProxy) Demon() *demon.Demon {
	return q.demon
}

// Topology returns the topology builder.
func (q *QUICProxy) Topology() *demon.TopologyBuilder {
	return q.topology
}

// Spectacle returns the spectacle renderer.
func (q *QUICProxy) Spectacle() *demon.Spectacle {
	return q.spectacle
}

// QUICProxyStats is the full report after stopping the proxy.
type QUICProxyStats struct {
	Duration     time.Duration              `json:"duration"`
	DemonStats   demon.DemonStats           `json:"demon"`
	BridgeStats  map[string]int64           `json:"bridge"`
	Topology     *demon.SimplicialComplex   `json:"topology"`
	CaptureStats *CaptureStats              `json:"capture,omitempty"`
}

// JSON returns stats as formatted JSON.
func (s *QUICProxyStats) JSON() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}
