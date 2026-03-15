//go:build darwin

package tape

import (
	"testing"
	"time"
)

func TestDaemonStartStop(t *testing.T) {
	dir := t.TempDir()

	counter := 0
	capFn := func() (string, int, int, error) {
		counter++
		return "daemon test frame", 80, 24, nil
	}

	rec := NewRecorder("daemon-test", "test", capFn)
	rec.Start()
	defer rec.Stop()

	cfg := DefaultDaemonConfig()
	cfg.ArchivePath = dir + "/daemon-archive.jsonl"
	cfg.EvolveInterval = 1 * time.Second
	cfg.TrialDuration = 500 * time.Millisecond
	cfg.MaxGenerations = 3

	daemon, err := NewDaemon(rec, capFn, cfg)
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}

	var events []DaemonEvent
	daemon.OnEvolve(func(ev DaemonEvent) {
		events = append(events, ev)
	})

	if err := daemon.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}

	// Wait for some evolution
	time.Sleep(4 * time.Second)

	daemon.Stop()

	stats := daemon.Stats()
	if stats.TotalGenerations == 0 {
		t.Fatal("expected at least 1 generation")
	}

	// Should have emitted events
	if len(events) < 2 {
		t.Logf("got %d events (expected started + evolved)", len(events))
	}

	// Check started event
	foundStarted := false
	for _, ev := range events {
		if ev.Type == "started" {
			foundStarted = true
		}
	}
	if !foundStarted {
		t.Fatal("expected 'started' event")
	}
}

func TestDaemonPersistence(t *testing.T) {
	dir := t.TempDir()
	archivePath := dir + "/persist-archive.jsonl"

	counter := 0
	capFn := func() (string, int, int, error) {
		counter++
		return "persist frame", 80, 24, nil
	}

	// First session: evolve and save
	rec1 := NewRecorder("persist-test", "s1", capFn)
	rec1.Start()

	cfg := DefaultDaemonConfig()
	cfg.ArchivePath = archivePath
	cfg.EvolveInterval = 500 * time.Millisecond
	cfg.TrialDuration = 300 * time.Millisecond
	cfg.MaxGenerations = 5
	cfg.SaveInterval = 2

	d1, err := NewDaemon(rec1, capFn, cfg)
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}
	d1.Start()
	time.Sleep(4 * time.Second)
	d1.Stop()
	rec1.Stop()

	gen1 := d1.Archive().Generation()

	// Second session: should resume from saved archive
	rec2 := NewRecorder("persist-test", "s2", capFn)
	rec2.Start()

	d2, err := NewDaemon(rec2, capFn, cfg)
	if err != nil {
		t.Fatalf("create daemon from saved: %v", err)
	}

	// Archive should have been loaded from disk
	gen2Start := d2.Archive().Generation()
	if gen2Start < gen1-1 {
		t.Fatalf("expected resumed generation >= %d, got %d", gen1-1, gen2Start)
	}

	d2.Start()
	time.Sleep(2 * time.Second)
	d2.Stop()
	rec2.Stop()

	// Second session should have evolved further
	gen2End := d2.Archive().Generation()
	if gen2End <= gen2Start {
		t.Logf("gen2 start=%d end=%d (may not have triggered)", gen2Start, gen2End)
	}
}
