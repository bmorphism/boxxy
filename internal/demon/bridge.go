package demon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// HyperionBridge sits between the Hyperion game server's proxy and the
// network, intercepting egress packets and routing them through the demon.
//
// Hyperion's proxy protocol:
//   Server -> Proxy: rkyv-encoded ServerToProxyMessage (length-prefixed u64)
//     - BroadcastLocal { center: ChunkPosition, exclude: u64, order: u32, data: &[u8] }
//     - Unicast { stream: u64, order: u32, data: &[u8] }
//     - Flush
//     - UpdatePlayerChunkPositions
//   Proxy -> Server: rkyv-encoded ProxyToServerMessage
//     - PlayerConnect
//     - PlayerDisconnect
//     - PlayerPackets
//
// The bridge intercepts at the Proxy -> Client boundary, where TCP sends
// are replaced with multipath QUIC sends routed through the demon.

// MessageType identifies Hyperion proxy message types.
type MessageType int

const (
	MsgBroadcastGlobal MessageType = iota
	MsgBroadcastLocal
	MsgUnicast
	MsgFlush
	MsgUpdatePositions
	MsgUnknown
)

// ProxyMessage is a decoded message from Hyperion's proxy egress.
type ProxyMessage struct {
	Type      MessageType
	Raw       []byte
	StreamID  uint64
	Order     uint32
	CenterX   int16
	CenterZ   int16
	Exclude   uint64
	Timestamp time.Time
}

// BridgeConfig configures the Hyperion bridge.
type BridgeConfig struct {
	// ListenAddr is where the bridge accepts connections from Hyperion's proxy.
	ListenAddr string
	// UpstreamAddr is Hyperion's game server address (for ingress forwarding).
	UpstreamAddr string
	// Demon is the Maxwell's demon instance for path selection.
	Demon *Demon
	// Topology records flow data for the topological embedding.
	Topology *TopologyBuilder
	// Spectacle renders in-game visualization.
	Spectacle *Spectacle
	// OnMessage is called for each intercepted message (for logging/analysis).
	OnMessage func(msg *ProxyMessage, pathID PathID)
}

// Bridge manages the connection between Hyperion's proxy and the demon.
type Bridge struct {
	config    BridgeConfig
	listener  net.Listener
	demon     *Demon
	topology  *TopologyBuilder
	spectacle *Spectacle

	mu         sync.Mutex
	clients    map[uint64]*clientConn
	nextID     atomic.Uint64
	running    atomic.Bool

	stats BridgeStats
}

// BridgeStats tracks bridge throughput.
type BridgeStats struct {
	MessagesIntercepted atomic.Int64
	BytesRouted         atomic.Int64
	BroadcastLocal      atomic.Int64
	BroadcastGlobal     atomic.Int64
	Unicast             atomic.Int64
	Flushes             atomic.Int64
	TickCount           atomic.Int64
}

type clientConn struct {
	id   uint64
	conn net.Conn
}

// NewBridge creates a Hyperion bridge.
func NewBridge(cfg BridgeConfig) *Bridge {
	return &Bridge{
		config:    cfg,
		demon:     cfg.Demon,
		topology:  cfg.Topology,
		spectacle: cfg.Spectacle,
		clients:   make(map[uint64]*clientConn),
	}
}

// Start begins accepting connections from Hyperion's proxy.
func (b *Bridge) Start() error {
	ln, err := net.Listen("tcp", b.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("bridge listen: %w", err)
	}
	b.listener = ln
	b.running.Store(true)

	go b.acceptLoop()
	return nil
}

// Stop shuts down the bridge.
func (b *Bridge) Stop() error {
	b.running.Store(false)
	if b.listener != nil {
		b.listener.Close()
	}
	b.mu.Lock()
	for _, c := range b.clients {
		c.conn.Close()
	}
	b.mu.Unlock()
	return nil
}

func (b *Bridge) acceptLoop() {
	for b.running.Load() {
		conn, err := b.listener.Accept()
		if err != nil {
			if b.running.Load() {
				continue
			}
			return
		}

		id := b.nextID.Add(1)
		client := &clientConn{id: id, conn: conn}

		b.mu.Lock()
		b.clients[id] = client
		b.mu.Unlock()

		go b.handleClient(client)
	}
}

// handleClient processes the stream of rkyv-encoded messages from Hyperion's proxy.
// Each message is length-prefixed with a u64 (little-endian).
func (b *Bridge) handleClient(client *clientConn) {
	defer func() {
		b.mu.Lock()
		delete(b.clients, client.id)
		b.mu.Unlock()
		client.conn.Close()
	}()

	for b.running.Load() {
		// Read length prefix (u64 LE)
		var lenBuf [8]byte
		if _, err := io.ReadFull(client.conn, lenBuf[:]); err != nil {
			return
		}
		msgLen := binary.LittleEndian.Uint64(lenBuf[:])
		if msgLen > 64*1024*1024 { // 64MB sanity limit
			return
		}

		// Read message body
		body := make([]byte, msgLen)
		if _, err := io.ReadFull(client.conn, body); err != nil {
			return
		}

		msg := b.decodeMessage(body)
		b.stats.MessagesIntercepted.Add(1)
		b.stats.BytesRouted.Add(int64(len(body)))

		// Route through the demon
		b.routeMessage(msg, client)
	}
}

// decodeMessage performs lightweight parsing of the rkyv-encoded message.
// We don't need full rkyv deserialization — just enough to identify the
// message type and extract routing-relevant fields.
func (b *Bridge) decodeMessage(raw []byte) *ProxyMessage {
	msg := &ProxyMessage{
		Raw:       raw,
		Timestamp: time.Now(),
		Type:      MsgUnknown,
	}

	if len(raw) < 4 {
		return msg
	}

	// rkyv uses a discriminant tag at the end of the archived enum.
	// For Hyperion's ServerToProxyMessage, the tag indicates the variant.
	// This is a simplified heuristic — real implementation would use
	// the rkyv archive layout.
	tag := binary.LittleEndian.Uint32(raw[len(raw)-4:])

	switch tag {
	case 0:
		msg.Type = MsgBroadcastGlobal
		b.stats.BroadcastGlobal.Add(1)
	case 1:
		msg.Type = MsgBroadcastLocal
		b.stats.BroadcastLocal.Add(1)
		// Extract center position if present
		if len(raw) >= 12 {
			msg.CenterX = int16(binary.LittleEndian.Uint16(raw[0:2]))
			msg.CenterZ = int16(binary.LittleEndian.Uint16(raw[2:4]))
		}
	case 2:
		msg.Type = MsgUnicast
		b.stats.Unicast.Add(1)
		if len(raw) >= 16 {
			msg.StreamID = binary.LittleEndian.Uint64(raw[0:8])
			msg.Order = binary.LittleEndian.Uint32(raw[8:12])
		}
	case 3:
		msg.Type = MsgFlush
		b.stats.Flushes.Add(1)
		b.stats.TickCount.Add(1)
	case 4:
		msg.Type = MsgUpdatePositions
	}

	return msg
}

// routeMessage sends the message through the demon for path selection.
func (b *Bridge) routeMessage(msg *ProxyMessage, client *clientConn) {
	pkt := &Packet{
		Data:      msg.Raw,
		StreamID:  msg.StreamID,
		Order:     msg.Order,
		Timestamp: msg.Timestamp,
		Size:      len(msg.Raw),
	}

	// The demon sorts the packet onto a path
	pathID := b.demon.Sort(pkt)

	// Record flow for topological embedding
	if b.topology != nil {
		b.topology.AddFlow(FlowRecord{
			Timestamp: msg.Timestamp,
			SrcAddr:   b.config.ListenAddr,
			DstAddr:   client.conn.RemoteAddr().String(),
			PathID:    pathID,
			RTT:       b.demon.metrics[pathID].SmoothedRTT(),
			Size:      len(msg.Raw),
		})
	}

	// Callback
	if b.config.OnMessage != nil {
		b.config.OnMessage(msg, pathID)
	}

	// In the full implementation, this is where we'd send the packet
	// on the selected QUIC path. For now, we forward on the TCP connection
	// (the demon's decisions are recorded for analysis).
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(msg.Raw)))
	client.conn.Write(lenBuf[:])
	client.conn.Write(msg.Raw)
}

// Stats returns bridge statistics.
func (b *Bridge) Stats() map[string]int64 {
	return map[string]int64{
		"messages_intercepted": b.stats.MessagesIntercepted.Load(),
		"bytes_routed":         b.stats.BytesRouted.Load(),
		"broadcast_local":      b.stats.BroadcastLocal.Load(),
		"broadcast_global":     b.stats.BroadcastGlobal.Load(),
		"unicast":              b.stats.Unicast.Load(),
		"flushes":              b.stats.Flushes.Load(),
		"ticks":                b.stats.TickCount.Load(),
	}
}

// StatsJSON returns bridge statistics as JSON.
func (b *Bridge) StatsJSON() string {
	data, _ := json.MarshalIndent(b.Stats(), "", "  ")
	return string(data)
}
