//go:build darwin

package tape

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmorphism/boxxy/internal/lisp"
)

// TestLispNamespaceIntegration exercises the full tape/* Lisp API
// by registering all namespaces and evaluating expressions.
func TestLispNamespaceIntegration(t *testing.T) {
	env := lisp.CreateStandardEnv()
	RegisterNamespace(env)
	RegisterEvolveNamespace(env)
	RegisterDaemonNamespace(env)

	eval := func(expr string) lisp.Value {
		t.Helper()
		reader := lisp.NewReader(strings.NewReader(expr))
		obj, err := reader.Read()
		if err != nil {
			t.Fatalf("read %q: %v", expr, err)
		}
		return lisp.Eval(obj, env)
	}

	dir := t.TempDir()

	// --- Basic recording (use def to bind in Lisp env) ---
	eval(`(def rec (tape/new-recorder "test-node" "integration"))`)
	eval(`(tape/start! rec)`)
	time.Sleep(2500 * time.Millisecond)
	eval(`(def tape1 (tape/stop! rec))`)

	info := eval(`(tape/info tape1)`)
	if info == nil {
		t.Fatal("info returned nil")
	}
	hm := info.(lisp.HashMap)
	frames := hm[lisp.Keyword("frames")].(lisp.Int)
	if frames < 2 {
		t.Fatalf("expected at least 2 frames, got %d", frames)
	}

	// --- Save and load ---
	path := filepath.Join(dir, "integration.jsonl")
	env.Set("_path", lisp.String(path))
	eval(`(def path _path)`)
	eval(`(tape/save! tape1 path)`)
	eval(`(def tape2 (tape/load path))`)

	info2 := eval(`(tape/info tape2)`)
	frames2 := info2.(lisp.HashMap)[lisp.Keyword("frames")].(lisp.Int)
	if frames2 != frames {
		t.Fatalf("loaded %d frames, expected %d", frames2, frames)
	}

	// --- Merge ---
	eval(`(def merged (tape/merge tape1 tape2))`)
	mergedInfo := eval(`(tape/info merged)`)
	mergedFrames := mergedInfo.(lisp.HashMap)[lisp.Keyword("frames")].(lisp.Int)
	if mergedFrames != frames*2 {
		t.Fatalf("merged should have %d frames, got %d", frames*2, mergedFrames)
	}

	// --- DGM evolution ---
	eval(`(def archive (tape/new-archive 10))`)
	eval(`(def rec2 (tape/new-recorder "evolve-node" "trial"))`)
	best := eval(`(tape/evolve! archive rec2 3)`)
	if _, ok := best.(lisp.Nil); ok {
		t.Fatal("evolve returned nil")
	}

	status := eval(`(tape/archive-status archive)`)
	agents := status.(lisp.HashMap)[lisp.Keyword("agents")].(lisp.Int)
	if agents < 1 {
		t.Fatal("expected at least 1 agent in archive")
	}

	bestAgent := eval(`(tape/archive-best archive)`)
	bestID := bestAgent.(lisp.HashMap)[lisp.Keyword("id")].(lisp.String)
	if bestID == "" {
		t.Fatal("expected non-empty best agent ID")
	}

	// --- ACSet world ---
	eval(`(def world (tape/new-world tape1))`)

	gf3 := eval(`(tape/world-gf3 world)`)
	if gf3 == nil {
		t.Fatal("world-gf3 returned nil")
	}

	uri := eval(`(tape/world-uri world)`)
	if !strings.Contains(string(uri.(lisp.String)), "tape://") {
		t.Fatalf("expected tape:// URI, got %s", uri)
	}

	schema := eval(`(tape/world-schema)`)
	objects := schema.(lisp.HashMap)[lisp.Keyword("objects")].(lisp.Vector)
	if len(objects) != 4 {
		t.Fatalf("expected 4 schema objects, got %d", len(objects))
	}

	// --- Bisimulation ---
	eval(`(def world2 (tape/new-world tape1))`)
	bisim := eval(`(tape/world-bisim? world world2)`)
	if bisim != lisp.Bool(true) {
		t.Fatal("identical worlds should be bisimilar")
	}

	// --- Sheaf consistency ---
	sheaf := eval(`(tape/sheaf-check world)`)
	consistent := sheaf.(lisp.HashMap)[lisp.Keyword("consistent")].(lisp.Bool)
	if !bool(consistent) {
		t.Fatal("single-node world should be sheaf-consistent")
	}

	// --- Archive persistence ---
	archivePath := filepath.Join(dir, "archive.jsonl")
	env.Set("_apath", lisp.String(archivePath))
	eval(`(def apath _apath)`)
	eval(`(tape/save-archive! archive apath)`)
	loaded2 := eval(`(tape/load-archive apath)`)
	if loaded2 == nil {
		t.Fatal("load-archive returned nil")
	}

	// --- Color stream ---
	cs := eval(`(tape/color-stream tape1)`)
	if cs == nil {
		t.Fatal("color-stream returned nil")
	}

	// --- PTY and PS recorders ---
	ptyRec := eval(`(tape/pty-recorder "pty-node" "test")`)
	if ptyRec == nil {
		t.Fatal("pty-recorder returned nil")
	}
	psRec := eval(`(tape/ps-recorder "ps-node" "test")`)
	if psRec == nil {
		t.Fatal("ps-recorder returned nil")
	}

	// --- Gossip status (no active state) ---
	gossipStatus := eval(`(tape/gossip-status)`)
	gs := gossipStatus.(lisp.HashMap)
	if gs[lisp.Keyword("status")] != lisp.String("no gossip active") {
		t.Logf("gossip status: %v", gs)
	}

	t.Logf("integration: %d frames recorded, %d agents evolved, sheaf consistent, bisimilar worlds", frames, agents)
}

// TestPlayerPlayback verifies tape playback at various speeds.
func TestPlayerPlayback(t *testing.T) {
	tape := NewTape("player-test", "playback")
	for i := uint64(1); i <= 3; i++ {
		tape.Append(Frame{
			SeqNo: i, LamportTS: i, NodeID: "player-test",
			Content: strings.Repeat("x", 10),
			Width: 80, Height: 24,
			WallTime: time.Now(),
		})
	}

	var buf bytes.Buffer
	player := NewPlayer(tape, &buf)
	player.SetSpeed(100.0) // very fast

	stop := make(chan struct{})
	err := player.Play(stop)
	if err != nil {
		t.Fatalf("play error: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected non-empty playback output")
	}
	// Should contain frame content and status bar
	if !strings.Contains(output, "player-test") {
		t.Fatalf("expected player-test in output, got: %s", output[:min(200, len(output))])
	}
	if !strings.Contains(output, "frame 3/3") {
		t.Fatalf("expected 'frame 3/3' in output")
	}
}

// TestPlayerStop verifies early stop via channel.
func TestPlayerStop(t *testing.T) {
	tape := NewTape("stop-test", "stop")
	for i := uint64(1); i <= 10; i++ {
		tape.Append(Frame{
			SeqNo: i, LamportTS: i, NodeID: "stop-test",
			Content: "frame", Width: 80, Height: 24,
			WallTime: time.Now(),
		})
	}

	var buf bytes.Buffer
	player := NewPlayer(tape, &buf)
	player.SetSpeed(1.0) // normal speed = 1 FPS

	stop := make(chan struct{})
	go func() {
		time.Sleep(500 * time.Millisecond)
		close(stop)
	}()

	player.Play(stop)

	// Should have stopped early (10 frames at 1 FPS would take 10s)
	output := buf.String()
	if strings.Contains(output, "frame 10/10") {
		t.Fatal("should have stopped before playing all 10 frames")
	}
}

// TestTapeLast verifies the Last() method.
func TestTapeLast(t *testing.T) {
	tape := NewTape("last-test", "test")

	_, ok := tape.Last()
	if ok {
		t.Fatal("Last on empty tape should return false")
	}

	tape.Append(Frame{SeqNo: 1, Content: "first"})
	tape.Append(Frame{SeqNo: 2, Content: "second"})

	f, ok := tape.Last()
	if !ok {
		t.Fatal("Last should return true on non-empty tape")
	}
	if f.Content != "second" {
		t.Fatalf("expected 'second', got %q", f.Content)
	}
}

// TestGossipStateToJSON verifies JSON serialization.
func TestGossipStateToJSON(t *testing.T) {
	gs := NewGossipState("json-test")
	gs.ReceiveGossip(GossipMessage{
		NodeID: "peer", Lamport: 5, Trit: 1,
		FrameCount: 10, Timestamp: time.Now(),
		VClock: map[string]uint64{"peer": 5},
	})

	data, err := gs.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}
	if !strings.Contains(string(data), "json-test") {
		t.Fatalf("expected node_id in JSON: %s", string(data))
	}
}

// TestVectorClockMarshalJSON verifies vector clock serialization.
func TestVectorClockMarshalJSON(t *testing.T) {
	vc := NewVectorClock()
	vc.Increment("a")
	vc.Increment("b")

	data, err := vc.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if !strings.Contains(string(data), `"a"`) {
		t.Fatalf("expected 'a' in JSON: %s", string(data))
	}
}

// TestAutopoieticStartWithServer verifies gossip integration.
func TestAutopoieticStartWithServer(t *testing.T) {
	dir := t.TempDir()
	capFn := func() (string, int, int, error) { return "server frame", 80, 24, nil }

	rec := NewRecorder("server-test", "test", capFn)
	srv, err := NewServer("127.0.0.1:0", rec)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	go srv.Serve()
	defer srv.Stop()

	cfg := AutopoieticConfig{
		NodeID:     "auto-srv-test",
		Label:      "gossip-test",
		CaptureFn:  capFn,
		GossipNode: true,
		Daemon: DaemonConfig{
			ArchivePath:    filepath.Join(dir, "srv-archive.jsonl"),
			ArchiveMaxSize: 10,
			TrialDuration:  300 * time.Millisecond,
			EvolveInterval: 1 * time.Second,
			SaveInterval:   5,
			HotSwap:        true,
			MaxGenerations: 2,
		},
	}

	ar, err := NewAutopoieticRecorder(cfg)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if ar.Gossip() == nil {
		t.Fatal("expected gossip state when GossipNode=true")
	}

	if err := ar.StartWithServer(srv); err != nil {
		t.Fatalf("start with server: %v", err)
	}

	time.Sleep(3 * time.Second)
	tape := ar.Stop()

	if tape.Len() < 2 {
		t.Fatalf("expected at least 2 frames, got %d", tape.Len())
	}
}

// TestWaitForEvolution verifies the blocking wait.
func TestWaitForEvolution(t *testing.T) {
	dir := t.TempDir()
	capFn := func() (string, int, int, error) { return "wait frame", 80, 24, nil }

	cfg := AutopoieticConfig{
		NodeID:    "wait-test",
		Label:     "wait",
		CaptureFn: capFn,
		Daemon: DaemonConfig{
			ArchivePath:    filepath.Join(dir, "wait-archive.jsonl"),
			ArchiveMaxSize: 10,
			TrialDuration:  200 * time.Millisecond,
			EvolveInterval: 500 * time.Millisecond,
			SaveInterval:   5,
			HotSwap:        true,
			MaxGenerations: 5,
		},
	}

	ar, err := NewAutopoieticRecorder(cfg)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ar.Start()
	defer ar.Stop()

	// Should complete within 5 seconds
	if !ar.WaitForEvolution(1, 5*time.Second) {
		t.Fatal("timed out waiting for 1 generation")
	}
}

// TestGossipRunnerStatus verifies the status string output.
func TestGossipRunnerStatus(t *testing.T) {
	capFn := func() (string, int, int, error) { return "status frame", 80, 24, nil }
	rec := NewRecorder("status-test", "test", capFn)
	gs := NewGossipState("status-test")
	gr := NewGossipRunner(gs, rec, nil, time.Second)

	status := gr.Status()
	if !strings.Contains(status, "gossip convergence") {
		t.Fatalf("expected 'gossip convergence' in status: %s", status)
	}
	if !strings.Contains(status, "status-test") {
		t.Fatalf("expected node_id in status: %s", status)
	}
}

// TestNetworkPeerCount verifies peer count tracking.
func TestNetworkPeerCount(t *testing.T) {
	capFn := func() (string, int, int, error) { return "peer frame", 80, 24, nil }
	rec := NewRecorder("peer-test", "test", capFn)

	srv, err := NewServer("127.0.0.1:0", rec)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	go srv.Serve()
	defer srv.Stop()

	if srv.PeerCount() != 0 {
		t.Fatalf("expected 0 peers initially, got %d", srv.PeerCount())
	}

	// Connect a client
	clientRec := NewRecorder("client", "test", capFn)
	client, err := Dial(srv.Addr(), "client", "test", clientRec)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	time.Sleep(200 * time.Millisecond)

	if srv.PeerCount() != 1 {
		t.Fatalf("expected 1 peer after connect, got %d", srv.PeerCount())
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
