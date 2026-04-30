//go:build darwin && arm64

package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// === Morphcloud backend tests ===

func TestMorphcloudLifecycle_Init(t *testing.T) {
	mc := NewMorphcloudLifecycle("test-mc", 4, 8)
	if mc.Name != "test-mc" {
		t.Errorf("expected name test-mc, got %s", mc.Name)
	}
	if mc.CPUs != 4 {
		t.Errorf("expected 4 CPUs, got %d", mc.CPUs)
	}
	if mc.MemoryGB != 8 {
		t.Errorf("expected 8 GB, got %d", mc.MemoryGB)
	}
}

func TestMorphcloudLifecycle_Summary(t *testing.T) {
	mc := NewMorphcloudLifecycle("test-mc-sum", 2, 4)
	mc.SnapshotID = "snap-42"
	s := mc.Summary()
	if !strings.Contains(s, "test-mc-sum") {
		t.Errorf("summary should contain name, got: %s", s)
	}
	if !strings.Contains(s, "snap-42") {
		t.Errorf("summary should contain snapshot, got: %s", s)
	}
}

func TestMorphcloudLifecycle_StateString(t *testing.T) {
	mc := NewMorphcloudLifecycle("test-mc-state", 1, 1)
	if mc.StateString() != "none" {
		t.Errorf("expected state none, got %s", mc.StateString())
	}
}

// === Vers backend tests ===

func TestVersVM_Init(t *testing.T) {
	v := NewVersVM("test-vers-vm")
	if v.Name != "test-vers-vm" {
		t.Errorf("expected name test-vers-vm, got %s", v.Name)
	}
	if v.VMID != "" {
		t.Errorf("expected empty VMID, got %s", v.VMID)
	}
	if v.Branch != "main" {
		t.Errorf("expected branch main, got %s", v.Branch)
	}
}

func TestVersVM_Summary(t *testing.T) {
	v := NewVersVM("my-vers")
	v.VMID = "vm-42"
	v.Branch = "feature"
	s := v.Summary()
	if !strings.Contains(s, "my-vers") {
		t.Errorf("summary should contain name, got: %s", s)
	}
	if !strings.Contains(s, "vm-42") {
		t.Errorf("summary should contain VM ID, got: %s", s)
	}
	if !strings.Contains(s, "feature") {
		t.Errorf("summary should contain branch, got: %s", s)
	}
}

func TestVersVM_StateString(t *testing.T) {
	v := NewVersVM("test-vers-state")
	if v.StateString() != "none" {
		t.Errorf("expected state none, got %s", v.StateString())
	}
}

// === Bridge backend tests ===

func TestBridge_Init(t *testing.T) {
	b := NewBridge(BridgeSignal, "signal-bridge")
	if b.Config.Name != "signal-bridge" {
		t.Errorf("expected signal-bridge, got %s", b.Config.Name)
	}
	if b.Config.Type != BridgeSignal {
		t.Errorf("expected BridgeSignal, got %v", b.Config.Type)
	}
}

func TestBridge_Configure(t *testing.T) {
	tmp := t.TempDir()
	b := NewBridge(BridgeTelegram, "tg-test")
	b.BaseDir = filepath.Join(tmp, "tg-test")

	if err := b.Configure("https://matrix.example.com"); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if b.Config.Homeserver != "https://matrix.example.com" {
		t.Errorf("expected homeserver, got %s", b.Config.Homeserver)
	}
	if b.Config.BotUsername != "telegrambot" {
		t.Errorf("expected telegrambot, got %s", b.Config.BotUsername)
	}
	// Config file should exist
	if _, err := os.Stat(filepath.Join(b.BaseDir, "bridge.json")); err != nil {
		t.Errorf("bridge.json should exist: %v", err)
	}
}

func TestBridge_SaveLoadConfig(t *testing.T) {
	tmp := t.TempDir()
	b := NewBridge(BridgeBluesky, "bsky-test")
	b.BaseDir = filepath.Join(tmp, "bsky-test")
	os.MkdirAll(b.BaseDir, 0755)

	b.Config.Homeserver = "https://matrix.bsky.test"
	b.Config.ConfigPath = "/tmp/bsky.yaml"
	if err := b.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	b2 := NewBridge(BridgeBluesky, "bsky-test")
	b2.BaseDir = filepath.Join(tmp, "bsky-test")
	if err := b2.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if b2.Config.Homeserver != "https://matrix.bsky.test" {
		t.Errorf("expected homeserver, got %s", b2.Config.Homeserver)
	}
	if b2.Config.ConfigPath != "/tmp/bsky.yaml" {
		t.Errorf("expected config path, got %s", b2.Config.ConfigPath)
	}
}

func TestBridgeTypes(t *testing.T) {
	types := ListBridgeTypes()
	if len(types) != 7 {
		t.Fatalf("expected 7 bridge types, got %d", len(types))
	}
	expected := []BridgeType{
		BridgeSignal, BridgeWhatsApp, BridgeTelegram,
		BridgeBluesky, BridgeIRC, BridgeSlack, BridgeDiscord,
	}
	for i, bt := range types {
		if bt != expected[i] {
			t.Errorf("bridge type %d: expected %s, got %s", i, expected[i], bt)
		}
	}
}

func TestBridge_Summary(t *testing.T) {
	b := NewBridge(BridgeTelegram, "telegram-1")
	s := b.Summary()
	if !strings.Contains(s, "telegram-1") {
		t.Errorf("summary should contain name, got: %s", s)
	}
	if !strings.Contains(s, "telegram") {
		t.Errorf("summary should contain type, got: %s", s)
	}
}

func TestBridge_StateString(t *testing.T) {
	b := NewBridge(BridgeIRC, "irc-test")
	if b.StateString() != "not configured" {
		t.Errorf("expected state 'not configured', got %s", b.StateString())
	}
}
