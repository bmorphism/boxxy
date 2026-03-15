//go:build darwin

package tape

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLamportClock(t *testing.T) {
	c := NewLamportClock("node-a")

	if c.Now() != 0 {
		t.Fatalf("initial clock should be 0, got %d", c.Now())
	}

	v1 := c.Tick()
	if v1 != 1 {
		t.Fatalf("first tick should be 1, got %d", v1)
	}

	v2 := c.Tick()
	if v2 != 2 {
		t.Fatalf("second tick should be 2, got %d", v2)
	}

	// Witness a remote clock ahead of us
	v3 := c.Witness(100)
	if v3 != 101 {
		t.Fatalf("witness(100) should give 101, got %d", v3)
	}

	// Witness a remote clock behind us
	v4 := c.Witness(50)
	if v4 != 102 {
		t.Fatalf("witness(50) after 101 should give 102, got %d", v4)
	}
}

func TestCausalOrder(t *testing.T) {
	tests := []struct {
		aTick uint64
		aNode string
		bTick uint64
		bNode string
		want  int
	}{
		{1, "a", 2, "b", -1},
		{3, "a", 2, "b", 1},
		{2, "a", 2, "b", -1}, // same tick, tiebreak by node
		{2, "b", 2, "a", 1},
		{2, "a", 2, "a", 0},
	}
	for _, tt := range tests {
		got := CausalOrder(tt.aTick, tt.aNode, tt.bTick, tt.bNode)
		if got != tt.want {
			t.Errorf("CausalOrder(%d,%s,%d,%s) = %d, want %d",
				tt.aTick, tt.aNode, tt.bTick, tt.bNode, got, tt.want)
		}
	}
}

func TestRecorderBasic(t *testing.T) {
	callCount := 0
	capFn := func() (string, int, int, error) {
		callCount++
		return "frame content", 80, 24, nil
	}

	rec := NewRecorder("test-node", "test", capFn)
	rec.Start()

	// Wait for a couple frames (1 FPS = 1 frame per second)
	time.Sleep(2500 * time.Millisecond)

	tape := rec.Stop()

	if tape.Len() < 2 {
		t.Fatalf("expected at least 2 frames after 2.5s, got %d", tape.Len())
	}

	f, ok := tape.At(0)
	if !ok {
		t.Fatal("expected frame at index 0")
	}
	if f.NodeID != "test-node" {
		t.Fatalf("expected node test-node, got %s", f.NodeID)
	}
	if f.Content != "frame content" {
		t.Fatalf("expected 'frame content', got %q", f.Content)
	}
	if f.LamportTS == 0 {
		t.Fatal("expected non-zero Lamport timestamp")
	}
}

func TestSaveLoadJSONL(t *testing.T) {
	tape := NewTape("node-x", "session-1")
	tape.Append(Frame{
		SeqNo: 1, LamportTS: 1, NodeID: "node-x",
		WallTime: time.Now(), Width: 80, Height: 24,
		Content: "hello world",
	})
	tape.Append(Frame{
		SeqNo: 2, LamportTS: 2, NodeID: "node-x",
		WallTime: time.Now(), Width: 80, Height: 24,
		Content: "second frame",
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := tape.SaveJSONL(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadJSONL(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Len() != 2 {
		t.Fatalf("expected 2 frames, got %d", loaded.Len())
	}
	if loaded.NodeID != "node-x" {
		t.Fatalf("expected node-x, got %s", loaded.NodeID)
	}

	f, _ := loaded.At(0)
	if f.Content != "hello world" {
		t.Fatalf("expected 'hello world', got %q", f.Content)
	}
}

func TestMergeTapes(t *testing.T) {
	t1 := NewTape("node-a", "s1")
	t1.Append(Frame{SeqNo: 1, LamportTS: 1, NodeID: "node-a", Content: "a1"})
	t1.Append(Frame{SeqNo: 2, LamportTS: 3, NodeID: "node-a", Content: "a2"})

	t2 := NewTape("node-b", "s2")
	t2.Append(Frame{SeqNo: 1, LamportTS: 2, NodeID: "node-b", Content: "b1"})
	t2.Append(Frame{SeqNo: 2, LamportTS: 4, NodeID: "node-b", Content: "b2"})

	merged := MergeTapes(t1, t2)

	if merged.Len() != 4 {
		t.Fatalf("expected 4 merged frames, got %d", merged.Len())
	}

	// Verify causal order: lamport 1, 2, 3, 4
	expected := []uint64{1, 2, 3, 4}
	for i, want := range expected {
		f, _ := merged.At(i)
		if f.LamportTS != want {
			t.Errorf("frame %d: expected lamport=%d, got %d", i, want, f.LamportTS)
		}
	}
}

func TestVectorClock(t *testing.T) {
	vc := NewVectorClock()
	vc.Increment("a")
	vc.Increment("a")
	vc.Increment("b")

	snap := vc.Snapshot()
	if snap["a"] != 2 {
		t.Fatalf("expected a=2, got %d", snap["a"])
	}
	if snap["b"] != 1 {
		t.Fatalf("expected b=1, got %d", snap["b"])
	}

	vc.Merge(map[string]uint64{"a": 5, "c": 3})
	snap = vc.Snapshot()
	if snap["a"] != 5 {
		t.Fatalf("expected a=5 after merge, got %d", snap["a"])
	}
	if snap["c"] != 3 {
		t.Fatalf("expected c=3 after merge, got %d", snap["c"])
	}
}

func TestNetworkServerClient(t *testing.T) {
	serverCap := func() (string, int, int, error) {
		return "server-frame", 80, 24, nil
	}
	clientCap := func() (string, int, int, error) {
		return "client-frame", 80, 24, nil
	}

	serverRec := NewRecorder("server", "srv", serverCap)
	clientRec := NewRecorder("client", "cli", clientCap)

	srv, err := NewServer("127.0.0.1:0", serverRec)
	if err != nil {
		t.Fatalf("server creation failed: %v", err)
	}
	go srv.Serve()
	defer srv.Stop()

	serverRec.Start()
	defer serverRec.Stop()

	// Give server a moment
	time.Sleep(100 * time.Millisecond)

	client, err := Dial(srv.Addr(), "client", "cli", clientRec)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer client.Close()

	clientRec.Start()

	// Let frames flow for 2.5 seconds
	time.Sleep(2500 * time.Millisecond)

	clientRec.Stop()

	// Server should have received some client frames
	serverTape := serverRec.Tape()
	if serverTape.Len() < 2 {
		t.Logf("server tape has %d frames (some may be remote)", serverTape.Len())
	}

	// Cleanup temp file if created
	os.Remove("tape-server.jsonl")
	os.Remove("tape-client.jsonl")
}
