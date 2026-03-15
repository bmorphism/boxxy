//go:build darwin

package tape

import (
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-archive.jsonl")

	// Create archive with some evolved agents
	a := NewArchive(20)
	counter := 0
	capFn := func() (string, int, int, error) {
		counter++
		return "persist test frame", 80, 24, nil
	}
	a.EvolveN(3, capFn, 500*time.Millisecond)

	originalCount := a.Count()
	originalGen := a.Generation()

	// Save
	if err := a.SaveArchive(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Load
	loaded, err := LoadArchive(path, 20)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Count() != originalCount {
		t.Fatalf("expected %d agents, got %d", originalCount, loaded.Count())
	}
	if loaded.Generation() != originalGen {
		t.Fatalf("expected generation %d, got %d", originalGen, loaded.Generation())
	}

	// Best agent should be preserved
	origBest := a.Best()
	loadedBest := loaded.Best()
	if origBest.ID != loadedBest.ID {
		t.Fatalf("best agent ID mismatch: %s vs %s", origBest.ID, loadedBest.ID)
	}
}

func TestLoadArchiveFileNotExist(t *testing.T) {
	a, err := LoadArchive("/nonexistent/path/archive.jsonl", 10)
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if a.Count() != 1 {
		t.Fatalf("expected 1 seed agent, got %d", a.Count())
	}
}

func TestAutoSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "autosave.jsonl")

	a := NewArchive(10)
	counter := 0
	capFn := func() (string, int, int, error) {
		counter++
		return "autosave frame", 80, 24, nil
	}

	// Evolve to generation 5
	a.EvolveN(5, capFn, 200*time.Millisecond)

	// AutoSave with interval=5 should trigger at gen 5
	saved := a.AutoSave(path, 5)
	if !saved {
		t.Fatal("expected AutoSave to trigger at generation 5")
	}

	// Verify the file was created
	loaded, err := LoadArchive(path, 10)
	if err != nil {
		t.Fatalf("failed to load autosaved archive: %v", err)
	}
	if loaded.Count() < 1 {
		t.Fatal("autosaved archive should have agents")
	}
}
