//go:build darwin && arm64

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BridgeType enumerates supported mautrix bridge protocols
type BridgeType string

const (
	BridgeSignal   BridgeType = "signal"
	BridgeWhatsApp BridgeType = "whatsapp"
	BridgeTelegram BridgeType = "telegram"
	BridgeBluesky  BridgeType = "bluesky"
	BridgeIRC      BridgeType = "irc"
	BridgeSlack    BridgeType = "slack"
	BridgeDiscord  BridgeType = "discord"
)

// BridgeState represents the bridge lifecycle
type BridgeState int

const (
	BridgeStateNone       BridgeState = iota // not configured
	BridgeStateConfigured                    // config exists, not running
	BridgeStateRunning                       // bridge active
	BridgeStateConnected                     // bridge + logged in
)

// BridgeConfig holds configuration for a mautrix bridge
type BridgeConfig struct {
	Type         BridgeType `json:"type"`
	Name         string     `json:"name"`
	Homeserver   string     `json:"homeserver"`
	AppserviceID string     `json:"appservice_id,omitempty"`
	BotUsername  string     `json:"bot_username,omitempty"`
	ConfigPath   string     `json:"config_path,omitempty"`
	DBPath       string     `json:"db_path,omitempty"`
}

// Bridge manages a mautrix bridge instance
type Bridge struct {
	Config    BridgeConfig
	BaseDir   string
	state     BridgeState
	pid       int
	bbctlPath string
}

func NewBridge(bridgeType BridgeType, name string) *Bridge {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".boxxy", "bridges", name)

	bbctlPath, _ := exec.LookPath("bbctl")
	if bbctlPath == "" {
		bbctlPath = "bbctl" // fallback
	}

	return &Bridge{
		Config: BridgeConfig{
			Type: bridgeType,
			Name: name,
		},
		BaseDir:   baseDir,
		bbctlPath: bbctlPath,
	}
}

// State returns current bridge state
func (b *Bridge) State() BridgeState {
	return b.state
}

func (b *Bridge) StateString() string {
	switch b.state {
	case BridgeStateNone:
		return "not configured"
	case BridgeStateConfigured:
		return "configured"
	case BridgeStateRunning:
		return "running"
	case BridgeStateConnected:
		return "connected"
	default:
		return "unknown"
	}
}

// Configure creates the bridge config directory and registration
func (b *Bridge) Configure(homeserver string) error {
	if err := os.MkdirAll(b.BaseDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	b.Config.Homeserver = homeserver
	b.Config.ConfigPath = filepath.Join(b.BaseDir, "config.yaml")
	b.Config.DBPath = filepath.Join(b.BaseDir, "bridge.db")
	b.Config.BotUsername = fmt.Sprintf("%sbot", b.Config.Type)

	if err := b.SaveConfig(); err != nil {
		return err
	}
	b.state = BridgeStateConfigured
	return nil
}

// Start launches the bridge using bbctl or direct binary
func (b *Bridge) Start(ctx context.Context) error {
	if b.state < BridgeStateConfigured {
		return fmt.Errorf("bridge %q not configured", b.Config.Name)
	}

	// Try bbctl first (Beeper Bridge Manager)
	if b.hasBbctl() {
		return b.startViaBbctl(ctx)
	}

	// Fallback: direct mautrix binary
	return b.startDirect(ctx)
}

func (b *Bridge) hasBbctl() bool {
	_, err := exec.LookPath("bbctl")
	return err == nil
}

func (b *Bridge) startViaBbctl(ctx context.Context) error {
	bridgeName := fmt.Sprintf("mautrix-%s", b.Config.Type)
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, b.bbctlPath, "run", bridgeName)
	cmd.Dir = b.BaseDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("bbctl run %s: %w", bridgeName, err)
	}
	b.pid = cmd.Process.Pid
	b.state = BridgeStateRunning
	return nil
}

func (b *Bridge) startDirect(ctx context.Context) error {
	binaryName := fmt.Sprintf("mautrix-%s", b.Config.Type)
	binPath, err := exec.LookPath(binaryName)
	if err != nil {
		return fmt.Errorf("%s not found: %w", binaryName, err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, binPath, "-c", b.Config.ConfigPath)
	cmd.Dir = b.BaseDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", binaryName, err)
	}
	b.pid = cmd.Process.Pid
	b.state = BridgeStateRunning
	return nil
}

// Stop terminates the bridge process
func (b *Bridge) Stop() error {
	if b.pid == 0 {
		return fmt.Errorf("bridge not running")
	}
	proc, err := os.FindProcess(b.pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		return proc.Kill()
	}
	b.state = BridgeStateConfigured
	b.pid = 0
	return nil
}

// Login performs bridge-specific login (e.g., Signal QR code, WhatsApp pairing)
func (b *Bridge) Login(ctx context.Context, credentials string) (string, error) {
	if b.state < BridgeStateRunning {
		return "", fmt.Errorf("bridge not running")
	}

	// Protocol-specific login via bbctl
	if b.hasBbctl() {
		cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		bridgeName := fmt.Sprintf("mautrix-%s", b.Config.Type)
		cmd := exec.CommandContext(cmdCtx, b.bbctlPath, "login", bridgeName, credentials)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("login: %w", err)
		}
		b.state = BridgeStateConnected
		return strings.TrimSpace(string(out)), nil
	}

	return "", fmt.Errorf("login requires bbctl")
}

// ListBridgeTypes returns all supported bridge types
func ListBridgeTypes() []BridgeType {
	return []BridgeType{
		BridgeSignal, BridgeWhatsApp, BridgeTelegram,
		BridgeBluesky, BridgeIRC, BridgeSlack, BridgeDiscord,
	}
}

// Summary returns a status string
func (b *Bridge) Summary() string {
	s := fmt.Sprintf("Bridge: %s (%s)\nState: %s\nPath: %s\n",
		b.Config.Name, b.Config.Type, b.StateString(), b.BaseDir)
	if b.Config.Homeserver != "" {
		s += fmt.Sprintf("Homeserver: %s\n", b.Config.Homeserver)
	}
	if b.pid != 0 {
		s += fmt.Sprintf("PID: %d\n", b.pid)
	}
	return s
}

// SaveConfig persists bridge config to disk
func (b *Bridge) SaveConfig() error {
	data, err := json.MarshalIndent(b.Config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.BaseDir, "bridge.json"), data, 0644)
}

// LoadConfig restores bridge config from disk
func (b *Bridge) LoadConfig() error {
	data, err := os.ReadFile(filepath.Join(b.BaseDir, "bridge.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &b.Config); err != nil {
		return err
	}
	b.state = BridgeStateConfigured
	return nil
}

// BridgeManager manages multiple bridges
type BridgeManager struct {
	Bridges map[string]*Bridge
	BaseDir string
}

func NewBridgeManager() *BridgeManager {
	home, _ := os.UserHomeDir()
	return &BridgeManager{
		Bridges: make(map[string]*Bridge),
		BaseDir: filepath.Join(home, ".boxxy", "bridges"),
	}
}

// Add registers a new bridge
func (bm *BridgeManager) Add(bridgeType BridgeType, name string) *Bridge {
	b := NewBridge(bridgeType, name)
	bm.Bridges[name] = b
	return b
}

// Get retrieves a bridge by name
func (bm *BridgeManager) Get(name string) (*Bridge, bool) {
	b, ok := bm.Bridges[name]
	return b, ok
}

// List returns all registered bridges
func (bm *BridgeManager) List() []*Bridge {
	var out []*Bridge
	for _, b := range bm.Bridges {
		out = append(out, b)
	}
	return out
}

// LoadAll discovers bridges from disk
func (bm *BridgeManager) LoadAll() error {
	entries, err := os.ReadDir(bm.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b := NewBridge("", e.Name())
		if err := b.LoadConfig(); err == nil {
			bm.Bridges[e.Name()] = b
		}
	}
	return nil
}
