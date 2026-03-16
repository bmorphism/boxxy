//go:build darwin

// gossip.go implements vector clock gossip for the tape network protocol.
// Peers periodically exchange their vector clocks to build a complete
// causal picture across the network. This is the convergence mechanism:
// even if direct frame delivery is lossy, gossip ensures all participants
// eventually agree on the causal partial order.
package tape

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// GossipMessage is exchanged between peers for causal convergence.
type GossipMessage struct {
	Type      string            `json:"type"`       // "vclock", "heartbeat", "merge-request"
	NodeID    string            `json:"node_id"`
	VClock    map[string]uint64 `json:"vclock"`
	Lamport   uint64            `json:"lamport"`
	Trit      gf3.Elem          `json:"trit"`
	Timestamp time.Time         `json:"timestamp"`
	FrameCount int              `json:"frame_count"`
}

// GossipState tracks the full causal state visible to this node.
type GossipState struct {
	mu          sync.RWMutex
	localID     string
	peers       map[string]*PeerState
	vclock      *VectorClock
	lamport     *LamportClock
	onConverge  func(GossipMessage) // callback when new convergence info arrives
}

// PeerState tracks what we know about a remote peer.
type PeerState struct {
	NodeID     string
	LastVClock map[string]uint64
	LastLamport uint64
	LastSeen   time.Time
	FrameCount int
	Trit       gf3.Elem
	Alive      bool
}

// NewGossipState creates gossip state for a node.
func NewGossipState(nodeID string) *GossipState {
	return &GossipState{
		localID: nodeID,
		peers:   make(map[string]*PeerState),
		vclock:  NewVectorClock(),
		lamport: NewLamportClock(nodeID),
	}
}

// OnConverge registers a callback for convergence events.
func (gs *GossipState) OnConverge(fn func(GossipMessage)) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.onConverge = fn
}

// LocalHeartbeat creates a gossip message for the local node's current state.
func (gs *GossipState) LocalHeartbeat(frameCount int) GossipMessage {
	ts := gs.lamport.Tick()
	gs.vclock.Increment(gs.localID)

	return GossipMessage{
		Type:       "heartbeat",
		NodeID:     gs.localID,
		VClock:     gs.vclock.Snapshot(),
		Lamport:    ts,
		Trit:       gf3.Elem(ts % 3),
		Timestamp:  time.Now(),
		FrameCount: frameCount,
	}
}

// ReceiveGossip processes an incoming gossip message and updates local state.
// Returns true if the message caused a state change (new information).
func (gs *GossipState) ReceiveGossip(msg GossipMessage) bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if msg.NodeID == gs.localID {
		return false // ignore self
	}

	// Advance clocks
	gs.lamport.Witness(msg.Lamport)
	gs.vclock.Merge(msg.VClock)

	// Update peer state
	peer, exists := gs.peers[msg.NodeID]
	if !exists {
		peer = &PeerState{
			NodeID: msg.NodeID,
			Alive:  true,
		}
		gs.peers[msg.NodeID] = peer
	}

	changed := false

	// Check if this is new information
	if msg.Lamport > peer.LastLamport {
		changed = true
		peer.LastLamport = msg.Lamport
		peer.LastVClock = msg.VClock
		peer.FrameCount = msg.FrameCount
		peer.Trit = msg.Trit
	}

	peer.LastSeen = time.Now()
	peer.Alive = true

	if changed && gs.onConverge != nil {
		gs.onConverge(msg)
	}

	return changed
}

// MergeRequest creates a gossip message requesting tape merge from a peer.
func (gs *GossipState) MergeRequest() GossipMessage {
	ts := gs.lamport.Tick()
	return GossipMessage{
		Type:      "merge-request",
		NodeID:    gs.localID,
		VClock:    gs.vclock.Snapshot(),
		Lamport:   ts,
		Trit:      gf3.Elem(ts % 3),
		Timestamp: time.Now(),
	}
}

// PeerList returns the current peer states.
func (gs *GossipState) PeerList() []PeerState {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	var peers []PeerState
	for _, p := range gs.peers {
		peers = append(peers, *p)
	}
	return peers
}

// AlivePeerCount returns the number of peers seen in the last interval.
func (gs *GossipState) AlivePeerCount(timeout time.Duration) int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	count := 0
	cutoff := time.Now().Add(-timeout)
	for _, p := range gs.peers {
		if p.LastSeen.After(cutoff) {
			count++
		}
	}
	return count
}

// GF3PeerBalance checks if the connected peers form a GF(3)-balanced set.
func (gs *GossipState) GF3PeerBalance() (bool, map[string]int) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	counts := map[string]int{
		"coordinator": 0,
		"generator":   0,
		"verifier":    0,
	}

	var trits []gf3.Elem
	// Include self
	selfTrit := gf3.Elem(gs.lamport.Now() % 3)
	trits = append(trits, selfTrit)

	for _, p := range gs.peers {
		if p.Alive {
			trits = append(trits, p.Trit)
		}
	}

	for _, t := range trits {
		switch t {
		case gf3.Zero:
			counts["coordinator"]++
		case gf3.One:
			counts["generator"]++
		case gf3.Two:
			counts["verifier"]++
		}
	}

	return gf3.IsBalanced(trits), counts
}

// ConvergenceStatus returns the overall convergence state of the gossip network.
func (gs *GossipState) ConvergenceStatus() map[string]interface{} {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	balanced, counts := gs.GF3PeerBalance()

	totalFrames := 0
	for _, p := range gs.peers {
		totalFrames += p.FrameCount
	}

	return map[string]interface{}{
		"node_id":      gs.localID,
		"peers":        len(gs.peers),
		"alive_peers":  gs.AlivePeerCount(30 * time.Second),
		"lamport":      gs.lamport.Now(),
		"total_frames": totalFrames,
		"gf3_balanced": balanced,
		"gf3_counts":   counts,
	}
}

// ToJSON serializes the gossip state.
func (gs *GossipState) ToJSON() ([]byte, error) {
	return json.Marshal(gs.ConvergenceStatus())
}

// --- Gossip Protocol Runner ---

// GossipRunner periodically broadcasts heartbeats and processes incoming gossip.
type GossipRunner struct {
	state    *GossipState
	recorder *Recorder
	server   *Server
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewGossipRunner creates a gossip protocol runner.
func NewGossipRunner(state *GossipState, recorder *Recorder, server *Server, interval time.Duration) *GossipRunner {
	return &GossipRunner{
		state:    state,
		recorder: recorder,
		server:   server,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the gossip protocol.
func (gr *GossipRunner) Start() {
	go gr.loop()
}

// Stop halts the gossip protocol.
func (gr *GossipRunner) Stop() {
	close(gr.stop)
	<-gr.done
}

func (gr *GossipRunner) loop() {
	defer close(gr.done)

	ticker := time.NewTicker(gr.interval)
	defer ticker.Stop()

	for {
		select {
		case <-gr.stop:
			return
		case <-ticker.C:
			frameCount := gr.recorder.Tape().Len()
			msg := gr.state.LocalHeartbeat(frameCount)

			// Broadcast heartbeat as a gossip frame
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			// Send as a meta-frame to all peers
			gossipFrame := Frame{
				NodeID:    gr.state.localID,
				LamportTS: msg.Lamport,
				Content:   string(data),
				Trit:      msg.Trit,
				WallTime:  time.Now(),
				Meta:      map[string]string{"type": "gossip"},
			}
			if gr.server != nil {
				gr.server.Broadcast(gossipFrame)
			}
		}
	}
}

// Status returns convergence status as formatted string.
func (gr *GossipRunner) Status() string {
	status := gr.state.ConvergenceStatus()
	data, _ := json.MarshalIndent(status, "", "  ")
	return fmt.Sprintf("gossip convergence:\n%s", string(data))
}
