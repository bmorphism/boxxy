//go:build darwin

// persist.go implements cross-session persistence for the DGM archive.
// Archives are saved as JSONL (one agent per line) so that evolution
// can continue across process restarts. This is the memory component
// of the Darwin Gödel Machine: agents discovered in one session seed
// the next session's archive.
package tape

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ArchiveHeader is the first line of a persisted archive.
type ArchiveHeader struct {
	Type       string    `json:"type"`
	Version    int       `json:"version"`
	MaxSize    int       `json:"max_size"`
	Generation int       `json:"generation"`
	AgentCount int       `json:"agent_count"`
	SavedAt    time.Time `json:"saved_at"`
	NodeID     string    `json:"node_id,omitempty"`
}

// SaveArchive persists the DGM archive to a JSONL file.
func (a *Archive) SaveArchive(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	// Write to temp file first for atomicity
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	a.mu.RLock()
	header := ArchiveHeader{
		Type:       "dgm-archive",
		Version:    1,
		MaxSize:    a.maxSize,
		Generation: a.generation,
		AgentCount: len(a.agents),
		SavedAt:    time.Now(),
	}
	agents := make([]*CaptureAgent, len(a.agents))
	copy(agents, a.agents)
	a.mu.RUnlock()

	if err := enc.Encode(header); err != nil {
		return fmt.Errorf("write archive header: %w", err)
	}

	for _, agent := range agents {
		if err := enc.Encode(agent); err != nil {
			return fmt.Errorf("write agent %s: %w", agent.ID, err)
		}
	}

	if err := f.Close(); err != nil {
		return err
	}

	// Atomic rename
	return os.Rename(tmp, path)
}

// LoadArchive restores a DGM archive from a JSONL file.
// If the file doesn't exist, returns a fresh archive.
func LoadArchive(path string, fallbackMaxSize int) (*Archive, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return NewArchive(fallbackMaxSize), nil
	}
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)

	var header ArchiveHeader
	if err := dec.Decode(&header); err != nil {
		return nil, fmt.Errorf("read archive header: %w", err)
	}

	if header.Type != "dgm-archive" {
		return nil, fmt.Errorf("not a DGM archive: type=%s", header.Type)
	}

	maxSize := header.MaxSize
	if maxSize <= 0 {
		maxSize = fallbackMaxSize
	}

	archive := &Archive{
		agents:          make([]*CaptureAgent, 0, header.AgentCount),
		maxSize:         maxSize,
		generation:      header.Generation,
		diversityThresh: 0.3,
	}

	for {
		var agent CaptureAgent
		if err := dec.Decode(&agent); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("read agent: %w", err)
		}
		archive.agents = append(archive.agents, &agent)
		if agent.Fitness > archive.bestFitness {
			archive.bestFitness = agent.Fitness
		}
	}

	// If no agents were loaded, seed with default
	if len(archive.agents) == 0 {
		archive.agents = append(archive.agents, &CaptureAgent{
			ID:      agentID("seed", 0),
			Fitness: 0.1,
			Params:  DefaultCaptureParams(),
		})
	}

	return archive, nil
}

// DefaultArchivePath returns the platform-standard path for the DGM archive.
func DefaultArchivePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".boxxy", "dgm-archive.jsonl")
}

// AutoSave saves the archive if enough generations have passed since last save.
// Returns true if saved.
func (a *Archive) AutoSave(path string, saveInterval int) bool {
	a.mu.RLock()
	gen := a.generation
	a.mu.RUnlock()

	if gen > 0 && gen%saveInterval == 0 {
		return a.SaveArchive(path) == nil
	}
	return false
}
