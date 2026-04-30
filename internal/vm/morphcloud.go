//go:build darwin && arm64

package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const morphcloudAPI = "https://cloud.morph.so/api"

// MorphcloudState represents lifecycle state for a Morphcloud instance
type MorphcloudState int

const (
	MorphcloudStateNone    MorphcloudState = iota // no instance
	MorphcloudStateStarted                        // instance running
	MorphcloudStateStopped                        // instance stopped (snapshot available)
)

// MorphcloudInstance represents a running Morphcloud VM
type MorphcloudInstance struct {
	InstanceID string `json:"instance_id"`
	SnapshotID string `json:"snapshot_id"`
	Status     string `json:"status"`
	IP         string `json:"ip,omitempty"`
}

// MorphcloudSnapshot represents a saved snapshot
type MorphcloudSnapshot struct {
	SnapshotID string `json:"snapshot_id"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// MorphcloudLifecycle manages create → start → exec → snapshot → stop
type MorphcloudLifecycle struct {
	Name       string
	APIKey     string
	SnapshotID string // base snapshot to launch from
	CPUs       int
	MemoryGB   int
	instance   *MorphcloudInstance
	client     *http.Client
}

func NewMorphcloudLifecycle(name string, cpus, memGB int) *MorphcloudLifecycle {
	apiKey := os.Getenv("MORPHCLOUD_API_KEY")
	return &MorphcloudLifecycle{
		Name:     name,
		APIKey:   apiKey,
		CPUs:     cpus,
		MemoryGB: memGB,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (m *MorphcloudLifecycle) apiRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, morphcloudAPI+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API %s %s: %d %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// State returns the current lifecycle state
func (m *MorphcloudLifecycle) State() MorphcloudState {
	if m.instance == nil {
		return MorphcloudStateNone
	}
	if m.instance.Status == "running" {
		return MorphcloudStateStarted
	}
	return MorphcloudStateStopped
}

func (m *MorphcloudLifecycle) StateString() string {
	switch m.State() {
	case MorphcloudStateNone:
		return "none"
	case MorphcloudStateStarted:
		return "running"
	case MorphcloudStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Start launches an instance from a snapshot
func (m *MorphcloudLifecycle) Start(ctx context.Context) error {
	if m.APIKey == "" {
		return fmt.Errorf("MORPHCLOUD_API_KEY not set")
	}
	if m.SnapshotID == "" {
		return fmt.Errorf("no snapshot_id specified")
	}

	payload := map[string]interface{}{
		"snapshot_id": m.SnapshotID,
	}

	data, err := m.apiRequest(ctx, "POST", "/instance", payload)
	if err != nil {
		return fmt.Errorf("start instance: %w", err)
	}

	var inst MorphcloudInstance
	if err := json.Unmarshal(data, &inst); err != nil {
		return fmt.Errorf("parse instance: %w", err)
	}
	m.instance = &inst
	return nil
}

// Exec runs a command inside the instance
func (m *MorphcloudLifecycle) Exec(ctx context.Context, command string) (string, error) {
	if m.instance == nil {
		return "", fmt.Errorf("no running instance")
	}

	payload := map[string]interface{}{
		"command": command,
	}

	path := fmt.Sprintf("/instance/%s/exec", m.instance.InstanceID)
	data, err := m.apiRequest(ctx, "POST", path, payload)
	if err != nil {
		return "", fmt.Errorf("exec: %w", err)
	}

	var result struct {
		Output   string `json:"output"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return string(data), nil
	}
	if result.ExitCode != 0 {
		return result.Output, fmt.Errorf("exit code %d", result.ExitCode)
	}
	return result.Output, nil
}

// Snapshot captures the current instance state
func (m *MorphcloudLifecycle) Snapshot(ctx context.Context) (*MorphcloudSnapshot, error) {
	if m.instance == nil {
		return nil, fmt.Errorf("no running instance")
	}

	path := fmt.Sprintf("/instance/%s/snapshot", m.instance.InstanceID)
	data, err := m.apiRequest(ctx, "POST", path, nil)
	if err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}

	var snap MorphcloudSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}
	return &snap, nil
}

// Stop terminates the instance
func (m *MorphcloudLifecycle) Stop(ctx context.Context) error {
	if m.instance == nil {
		return fmt.Errorf("no running instance")
	}

	path := fmt.Sprintf("/instance/%s", m.instance.InstanceID)
	_, err := m.apiRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	m.instance = nil
	return nil
}

// Instance returns the current instance
func (m *MorphcloudLifecycle) Instance() *MorphcloudInstance {
	return m.instance
}

// Summary returns a status string
func (m *MorphcloudLifecycle) Summary() string {
	s := fmt.Sprintf("Morphcloud VM: %s\nState: %s\nConfig: %d CPU, %d GB RAM\n",
		m.Name, m.StateString(), m.CPUs, m.MemoryGB)
	if m.SnapshotID != "" {
		s += fmt.Sprintf("Base Snapshot: %s\n", m.SnapshotID)
	}
	if m.instance != nil {
		s += fmt.Sprintf("Instance: %s\n", m.instance.InstanceID)
		if m.instance.IP != "" {
			s += fmt.Sprintf("IP: %s\n", m.instance.IP)
		}
	}
	return s
}

// SaveMetadata persists instance info to ~/.boxxy/morphcloud/<name>.json
func (m *MorphcloudLifecycle) SaveMetadata() error {
	if m.instance == nil {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".boxxy", "morphcloud")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.instance, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, m.Name+".json"), data, 0644)
}

// LoadMetadata restores instance info from disk
func (m *MorphcloudLifecycle) LoadMetadata() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".boxxy", "morphcloud", m.Name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var inst MorphcloudInstance
	if err := json.Unmarshal(data, &inst); err != nil {
		return err
	}
	m.instance = &inst
	return nil
}
