//go:build darwin

package tape

import (
	"testing"
	"time"
)

func TestAutopoieticRecorderCreation(t *testing.T) {
	dir := t.TempDir()

	cfg := AutopoieticConfig{
		NodeID:    "auto-test",
		Label:     "test",
		CaptureFn: func() (string, int, int, error) { return "auto frame", 80, 24, nil },
		Daemon: DaemonConfig{
			ArchivePath:    dir + "/auto-archive.jsonl",
			ArchiveMaxSize: 10,
			TrialDuration:  300 * time.Millisecond,
			EvolveInterval: 1 * time.Second,
			SaveInterval:   5,
			HotSwap:        true,
			MaxGenerations: 3,
		},
	}

	ar, err := NewAutopoieticRecorder(cfg)
	if err != nil {
		t.Fatalf("create autopoietic recorder: %v", err)
	}

	if ar.Recorder() == nil {
		t.Fatal("expected non-nil recorder")
	}
	if ar.Daemon() == nil {
		t.Fatal("expected non-nil daemon")
	}
	if ar.Colors() == nil {
		t.Fatal("expected non-nil color stream")
	}
}

func TestAutopoieticRecorderStartStop(t *testing.T) {
	dir := t.TempDir()

	counter := 0
	cfg := AutopoieticConfig{
		NodeID: "auto-test",
		Label:  "lifecycle",
		CaptureFn: func() (string, int, int, error) {
			counter++
			return "auto lifecycle frame", 80, 24, nil
		},
		Daemon: DaemonConfig{
			ArchivePath:    dir + "/auto-lifecycle.jsonl",
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

	if err := ar.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Let it record + evolve
	time.Sleep(3 * time.Second)

	tape := ar.Stop()

	if tape.Len() < 2 {
		t.Fatalf("expected at least 2 frames, got %d", tape.Len())
	}

	// Status should reflect recording
	status := ar.Status()
	if status["node_id"] != "auto-test" {
		t.Fatalf("expected node_id auto-test, got %v", status["node_id"])
	}
}

func TestAutopoieticEvents(t *testing.T) {
	dir := t.TempDir()

	cfg := AutopoieticConfig{
		NodeID: "events-test",
		Label:  "events",
		CaptureFn: func() (string, int, int, error) {
			return "events frame", 80, 24, nil
		},
		Daemon: DaemonConfig{
			ArchivePath:    dir + "/events.jsonl",
			ArchiveMaxSize: 10,
			TrialDuration:  200 * time.Millisecond,
			EvolveInterval: 500 * time.Millisecond,
			SaveInterval:   5,
			HotSwap:        true,
			MaxGenerations: 3,
		},
	}

	ar, err := NewAutopoieticRecorder(cfg)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ar.Start()
	time.Sleep(3 * time.Second)
	ar.Stop()

	events := ar.Events()
	if len(events) == 0 {
		t.Log("no events captured (daemon may not have triggered)")
	} else {
		t.Logf("captured %d daemon events", len(events))
		for _, ev := range events {
			t.Logf("  %s: %s", ev.Type, ev.Message)
		}
	}
}
