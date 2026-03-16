//go:build darwin

package tape

import (
	"testing"
	"time"
)

func TestGossipStateHeartbeat(t *testing.T) {
	gs := NewGossipState("node-a")
	msg := gs.LocalHeartbeat(42)

	if msg.NodeID != "node-a" {
		t.Fatalf("expected node-a, got %s", msg.NodeID)
	}
	if msg.FrameCount != 42 {
		t.Fatalf("expected 42 frames, got %d", msg.FrameCount)
	}
	if msg.Lamport == 0 {
		t.Fatal("expected non-zero Lamport in heartbeat")
	}
	if msg.Type != "heartbeat" {
		t.Fatalf("expected heartbeat type, got %s", msg.Type)
	}
}

func TestGossipReceive(t *testing.T) {
	gs := NewGossipState("node-a")

	msg := GossipMessage{
		Type:       "heartbeat",
		NodeID:     "node-b",
		VClock:     map[string]uint64{"node-b": 5},
		Lamport:    10,
		Trit:       1,
		Timestamp:  time.Now(),
		FrameCount: 100,
	}

	changed := gs.ReceiveGossip(msg)
	if !changed {
		t.Fatal("expected state change from new peer")
	}

	peers := gs.PeerList()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].NodeID != "node-b" {
		t.Fatalf("expected node-b, got %s", peers[0].NodeID)
	}
	if peers[0].FrameCount != 100 {
		t.Fatalf("expected 100 frames, got %d", peers[0].FrameCount)
	}
}

func TestGossipIgnoresSelf(t *testing.T) {
	gs := NewGossipState("node-a")

	msg := GossipMessage{
		Type:   "heartbeat",
		NodeID: "node-a",
	}
	changed := gs.ReceiveGossip(msg)
	if changed {
		t.Fatal("should not change state for self-messages")
	}
}

func TestGossipGF3PeerBalance(t *testing.T) {
	gs := NewGossipState("node-a")

	// Add peers with trits 0, 1, 2
	gs.ReceiveGossip(GossipMessage{NodeID: "b", Lamport: 1, Trit: 1, Timestamp: time.Now()})
	gs.ReceiveGossip(GossipMessage{NodeID: "c", Lamport: 1, Trit: 2, Timestamp: time.Now()})

	balanced, counts := gs.GF3PeerBalance()
	t.Logf("GF(3) peer balance: balanced=%v counts=%v", balanced, counts)

	// Self + b + c = 3 peers. Whether balanced depends on self's current trit.
	if counts["coordinator"]+counts["generator"]+counts["verifier"] != 3 {
		t.Fatalf("expected 3 total trits, got %v", counts)
	}
}

func TestGossipConvergenceStatus(t *testing.T) {
	gs := NewGossipState("test-node")
	gs.ReceiveGossip(GossipMessage{
		NodeID: "peer-1", Lamport: 5, Trit: 1,
		FrameCount: 50, Timestamp: time.Now(),
		VClock: map[string]uint64{"peer-1": 5},
	})

	status := gs.ConvergenceStatus()
	if status["node_id"] != "test-node" {
		t.Fatalf("expected test-node, got %v", status["node_id"])
	}
	if status["peers"].(int) != 1 {
		t.Fatalf("expected 1 peer, got %v", status["peers"])
	}
}

func TestGossipRunnerLifecycle(t *testing.T) {
	capFn := func() (string, int, int, error) {
		return "gossip runner frame", 80, 24, nil
	}

	rec := NewRecorder("gossip-runner-test", "test", capFn)
	rec.Start()
	defer rec.Stop()

	srv, err := NewServer("127.0.0.1:0", rec)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	go srv.Serve()
	defer srv.Stop()

	gs := NewGossipState("gossip-runner-test")
	gr := NewGossipRunner(gs, rec, srv, 500*time.Millisecond)
	gr.Start()

	// Let gossip run a few cycles
	time.Sleep(2 * time.Second)

	gr.Stop()

	// Verify the gossip state was updated
	lamport := gs.lamport.Now()
	if lamport == 0 {
		t.Fatal("expected non-zero Lamport after gossip runner cycles")
	}
	t.Logf("gossip runner: lamport=%d after 2s", lamport)
}

func TestGossipMergeRequest(t *testing.T) {
	gs := NewGossipState("requester")
	msg := gs.MergeRequest()

	if msg.Type != "merge-request" {
		t.Fatalf("expected merge-request, got %s", msg.Type)
	}
	if msg.NodeID != "requester" {
		t.Fatalf("expected requester, got %s", msg.NodeID)
	}
}
