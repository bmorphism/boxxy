//go:build darwin && integration

package pinhole

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCaptureLoopbackIntegration runs a real tshark capture on lo0,
// generates some HTTP traffic, stops capture, and verifies stats.
//
// Run with: go test -tags integration -v ./internal/pinhole/ -run TestCaptureLoopback
//
// Requires: tshark installed with BPF capture permissions (sudo or ChmodBPF).
func TestCaptureLoopbackIntegration(t *testing.T) {
	if !TsharkAvailable() {
		t.Skip("tshark not available")
	}
	if !canCapture() {
		t.Skip("no BPF capture permissions (need sudo or ChmodBPF)")
	}

	pcapDir := t.TempDir()
	sessionID := "loopback-integration-test"

	// Start a local HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	// Start capture on loopback for 127.0.0.1
	cap, err := StartCapture("lo0", "127.0.0.1", pcapDir, sessionID)
	if err != nil {
		t.Fatalf("StartCapture: %v", err)
	}

	if !cap.IsRunning() {
		t.Fatal("expected capture to be running")
	}

	// Give tshark time to initialize
	time.Sleep(500 * time.Millisecond)

	// Generate some traffic
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 5; i++ {
		resp, err := client.Get("http://127.0.0.1:" + itoa(port) + "/ping")
		if err != nil {
			t.Fatalf("HTTP request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	// Let packets settle
	time.Sleep(500 * time.Millisecond)

	// Stop capture
	stats, err := cap.StopCapture()
	if err != nil {
		t.Fatalf("StopCapture: %v", err)
	}

	if !cap.IsRunning() == true {
		// Should not be running after stop
	}
	if cap.IsRunning() {
		t.Error("capture should not be running after stop")
	}

	// Verify pcap file exists
	pcapPath := filepath.Join(pcapDir, sessionID+".pcap")
	info, err := os.Stat(pcapPath)
	if err != nil {
		t.Fatalf("pcap file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("pcap file is empty")
	}

	// Verify stats
	t.Logf("Capture stats: packets=%d bytes=%d duration=%v",
		stats.PacketCount, stats.Bytes, stats.Duration)
	t.Logf("Endpoints: %v", stats.TopEndpoints)
	t.Logf("Summary:\n%s", stats.Summary)

	if stats.PacketCount == 0 {
		t.Error("expected at least some packets captured")
	}
	if stats.Duration < 500*time.Millisecond {
		t.Errorf("duration too short: %v", stats.Duration)
	}
	if stats.PcapPath != pcapPath {
		t.Errorf("PcapPath = %q, want %q", stats.PcapPath, pcapPath)
	}

	// JSON output should be valid
	j := stats.JSON()
	if j == "" {
		t.Error("JSON() returned empty")
	}
	t.Logf("JSON:\n%s", j)
}

// TestDetectGuestIPIntegration tests real ARP/DHCP parsing against the live system.
//
// Run with: go test -tags integration -v ./internal/pinhole/ -run TestDetectGuestIP
func TestDetectGuestIPIntegration(t *testing.T) {
	// This may or may not find an IP depending on whether a VM is running.
	// We just verify it doesn't panic or hang.
	ip, err := DetectGuestIP("bridge100")
	if err != nil {
		t.Logf("no guest IP found (expected if no VM running): %v", err)
	} else {
		t.Logf("detected guest IP: %s", ip)
	}
}

// TestVerifyComplianceIntegration captures loopback traffic and verifies compliance check.
//
// Run with: go test -tags integration -v ./internal/pinhole/ -run TestVerifyCompliance
func TestVerifyComplianceIntegration(t *testing.T) {
	if !TsharkAvailable() {
		t.Skip("tshark not available")
	}
	if !canCapture() {
		t.Skip("no BPF capture permissions (need sudo or ChmodBPF)")
	}

	pcapDir := t.TempDir()
	sessionID := "compliance-test"

	// Start a server on a known port
	listener, err := net.Listen("tcp", "127.0.0.1:18080")
	if err != nil {
		t.Skip("port 18080 not available")
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	// Capture on loopback
	cap, err := StartCapture("lo0", "127.0.0.1", pcapDir, sessionID)
	if err != nil {
		t.Fatalf("StartCapture: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Hit port 18080 (not in allowed list)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:18080/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	time.Sleep(500 * time.Millisecond)
	cap.StopCapture()

	pcapPath := filepath.Join(pcapDir, sessionID+".pcap")

	// Check compliance against ports 53/80/443 — traffic on 18080 should be flagged
	compliant, msg, err := VerifyPinholeCompliance(pcapPath, []int{53, 80, 443})
	if err != nil {
		t.Fatalf("compliance check failed: %v", err)
	}

	t.Logf("compliant=%v msg=%s", compliant, msg)

	if compliant {
		t.Error("expected non-compliant (traffic on port 18080 should be flagged)")
	}
}

// canCapture checks if the current user can capture packets on lo0.
func canCapture() bool {
	// Try a short capture — if BPF access fails, tshark exits quickly with an error.
	cmd := exec.Command("tshark", "-i", "lo0", "-a", "duration:0", "-w", os.DevNull, "-q")
	err := cmd.Run()
	return err == nil
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// TestDetectGuestIPFromLeases verifies that real DHCP leases file can be parsed.
func TestDetectGuestIPFromLeases(t *testing.T) {
	if _, err := os.Stat("/var/db/dhcpd_leases"); err != nil {
		t.Skip("no dhcpd_leases file")
	}

	// parseDHCPLeases hardcodes the path, so we test it directly
	ip, err := parseDHCPLeases("bridge100")
	if err != nil {
		t.Logf("no 192.168.64.x lease found: %v (expected if no recent VMs)", err)
	} else {
		t.Logf("found lease IP: %s", ip)
		if !strings.HasPrefix(ip, "192.168.64.") {
			t.Errorf("unexpected IP prefix: %s", ip)
		}
	}
}

// TestGenerateAndWriteRulesIntegration writes real rules to /tmp and validates content.
func TestGenerateAndWriteRulesIntegration(t *testing.T) {
	cfg := PinholeConfig{
		Bridge:    "bridge100",
		GuestIP:   "192.168.64.17",
		SessionID: fmt.Sprintf("integration-test-%d", time.Now().UnixNano()),
		Anchor:    "com.boxxy.zerobrew",
		NATSPort:  true,
		AllowedPorts: []int{8080},
	}

	rules := generateRules(cfg)
	path := confPath(cfg.SessionID)
	defer os.Remove(path)

	if err := os.WriteFile(path, []byte(rules), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Read it back
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	got := string(content)

	// Verify all expected rules
	checks := []struct {
		name, needle string
	}{
		{"DNS", "port 53"},
		{"HTTPS", "port 443"},
		{"HTTP", "port 80"},
		{"NATS", "port 4222"},
		{"extra 8080", "port 8080"},
		{"block", "block on bridge100 from 192.168.64.17"},
		{"guest IP", "192.168.64.17"},
		{"bridge", "bridge100"},
	}
	for _, c := range checks {
		if !strings.Contains(got, c.needle) {
			t.Errorf("rules missing %s (%q)", c.name, c.needle)
		}
	}

	// Verify the file is in the expected /tmp location
	if !strings.HasPrefix(path, "/tmp/boxxy-pinhole-") {
		t.Errorf("conf path not in /tmp: %s", path)
	}

	t.Logf("rules written to %s (%d bytes)", path, len(content))
}

// TestPfctlHelperScript verifies the helper script exists and is executable.
func TestPfctlHelperScript(t *testing.T) {
	// Find the script relative to the project root
	root := findProjectRoot(t)
	candidates := []string{
		filepath.Join(root, "scripts", "boxxy-pfctl-helper.sh"),
		"../../../scripts/boxxy-pfctl-helper.sh",
		"scripts/boxxy-pfctl-helper.sh",
	}

	var found string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			found = c
			break
		}
	}
	if found == "" {
		t.Skip("boxxy-pfctl-helper.sh not found relative to test")
	}

	info, err := os.Stat(found)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Must be executable
	if info.Mode()&0111 == 0 {
		t.Errorf("helper script not executable: %s (mode %v)", found, info.Mode())
	}

	// Read and verify it has our safety checks
	content, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "com.boxxy.") {
		t.Error("helper script missing anchor validation")
	}
	if !strings.Contains(s, "/tmp/boxxy-pinhole-") {
		t.Error("helper script missing conf path validation")
	}
	if !strings.Contains(s, "set -euo pipefail") {
		t.Error("helper script missing strict mode")
	}
}

// TestBoxxyPinholeCLI runs the built boxxy binary's pinhole subcommand.
func TestBoxxyPinholeCLI(t *testing.T) {
	// Build boxxy to a temp location
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "boxxy")

	cmd := exec.Command("go", "build", "-o", binary, "./cmd/boxxy/")
	cmd.Dir = findProjectRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Test: no args shows usage
	out, err := exec.Command(binary, "pinhole").CombinedOutput()
	if err == nil {
		t.Error("expected nonzero exit for pinhole with no args")
	}
	if !strings.Contains(string(out), "activate") {
		t.Error("pinhole usage should mention 'activate'")
	}

	// Test: unknown subcommand
	out, err = exec.Command(binary, "pinhole", "badcmd").CombinedOutput()
	if err == nil {
		t.Error("expected nonzero exit for unknown subcommand")
	}
	if !strings.Contains(string(out), "unknown pinhole subcommand") {
		t.Errorf("unexpected output: %s", out)
	}

	// Test: capture without session ID
	out, err = exec.Command(binary, "pinhole", "capture").CombinedOutput()
	if err == nil {
		t.Error("expected nonzero exit for capture without session ID")
	}
	if !strings.Contains(string(out), "session ID required") {
		t.Errorf("unexpected output: %s", out)
	}

	t.Logf("CLI smoke tests passed")
}

// TestActivateDeactivateWithMockHelper tests the full Activate→Deactivate lifecycle
// using a mock helper script that records calls instead of running pfctl.
func TestActivateDeactivateWithMockHelper(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock helper that logs calls to a file instead of running pfctl
	logFile := filepath.Join(tmpDir, "calls.log")
	mockHelper := filepath.Join(tmpDir, "mock-pfctl-helper.sh")
	mockScript := fmt.Sprintf(`#!/bin/bash
set -euo pipefail
echo "$@" >> %s
`, logFile)
	if err := os.WriteFile(mockHelper, []byte(mockScript), 0755); err != nil {
		t.Fatalf("WriteFile mock: %v", err)
	}

	cfg := PinholeConfig{
		Bridge:    "bridge100",
		GuestIP:   "192.168.64.99",
		Anchor:    "com.boxxy.zerobrew",
		SessionID: fmt.Sprintf("mock-test-%d", time.Now().UnixNano()),
		PcapDir:   tmpDir,
	}

	// Activate — should write rules and call mock helper
	err := activateWithHelper(cfg, mockHelper)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// Verify rules file was written
	rulesPath := confPath(cfg.SessionID)
	defer os.Remove(rulesPath)

	rulesContent, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("rules file not found: %v", err)
	}
	if !strings.Contains(string(rulesContent), "192.168.64.99") {
		t.Error("rules missing guest IP")
	}

	// Verify mock helper was called with activate
	logContent, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("log file not found: %v", err)
	}
	if !strings.Contains(string(logContent), "activate") {
		t.Errorf("mock helper not called with activate: %s", logContent)
	}
	if !strings.Contains(string(logContent), "com.boxxy.zerobrew") {
		t.Errorf("mock helper not called with anchor: %s", logContent)
	}

	// Deactivate
	err = deactivateWithHelper(cfg, mockHelper)
	if err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	// Verify deactivate was logged
	logContent, err = os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("log file re-read: %v", err)
	}
	if !strings.Contains(string(logContent), "deactivate") {
		t.Errorf("mock helper not called with deactivate: %s", logContent)
	}

	// Verify conf file was cleaned up
	if _, err := os.Stat(rulesPath); err == nil {
		t.Error("rules file should have been cleaned up after deactivate")
	}

	t.Logf("mock helper calls:\n%s", logContent)
}

// activateWithHelper is Activate without sudo — calls the helper directly.
func activateWithHelper(cfg PinholeConfig, helperPath string) error {
	if cfg.GuestIP == "" {
		return fmt.Errorf("guest IP required")
	}
	if cfg.SessionID == "" {
		cfg.SessionID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if err := os.MkdirAll(cfg.PcapDir, 0755); err != nil {
		return fmt.Errorf("failed to create pcap dir: %w", err)
	}
	rules := generateRules(cfg)
	path := confPath(cfg.SessionID)
	if err := os.WriteFile(path, []byte(rules), 0644); err != nil {
		return fmt.Errorf("failed to write pf rules: %w", err)
	}
	// Call helper directly (no sudo) — works with mock helper
	cmd := exec.Command(helperPath, "activate", path, cfg.Anchor)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(path)
		return fmt.Errorf("failed to activate pinhole: %w", err)
	}
	return nil
}

// deactivateWithHelper is Deactivate without sudo.
func deactivateWithHelper(cfg PinholeConfig, helperPath string) error {
	cmd := exec.Command(helperPath, "deactivate", cfg.Anchor)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to deactivate pinhole: %w", err)
	}
	if cfg.SessionID != "" {
		os.Remove(confPath(cfg.SessionID))
	}
	return nil
}

// TestZbScriptHelp verifies the zb wrapper's --help output.
func TestZbScriptHelp(t *testing.T) {
	root := findProjectRoot(t)
	zbPath := filepath.Join(root, "scripts", "zb")

	info, err := os.Stat(zbPath)
	if err != nil {
		t.Fatalf("zb not found: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatalf("zb not executable: mode %v", info.Mode())
	}

	out, _ := exec.Command("bash", zbPath, "--help").CombinedOutput()
	output := string(out)

	checks := []string{
		"zero-effort zerobrew",
		"zb install vim",
		"zb search python",
		"DNS/HTTP/HTTPS",
	}
	for _, needle := range checks {
		if !strings.Contains(output, needle) {
			t.Errorf("zb --help missing %q", needle)
		}
	}
	t.Logf("zb --help output:\n%s", output)
}

// TestExampleJokeFileExists verifies the zerobrew-pinhole.joke file is valid.
func TestExampleJokeFileExists(t *testing.T) {
	root := findProjectRoot(t)
	jokePath := filepath.Join(root, "examples", "zerobrew-pinhole.joke")

	content, err := os.ReadFile(jokePath)
	if err != nil {
		t.Fatalf("example .joke file not found: %v", err)
	}

	s := string(content)
	checks := []struct {
		name, needle string
	}{
		{"disk creation", "create-disk-image"},
		{"NAT networking", "new-nat-network"},
		{"boot loader", "new-efi-boot-loader"},
		{"VM config", "new-vm-config"},
		{"start VM", "start-vm!"},
		{"wait for shutdown", "wait-for-shutdown"},
		{"4 CPUs", "4 4"},
		{"PID file", "vm.pid"},
	}
	for _, c := range checks {
		if !strings.Contains(s, c.needle) {
			t.Errorf("joke file missing %s (%q)", c.name, c.needle)
		}
	}
}

// TestConfigYAMLExists verifies the zerobrew-joker-config.yaml.
func TestConfigYAMLExists(t *testing.T) {
	root := findProjectRoot(t)
	yamlPath := filepath.Join(root, "duck", "zerobrew-joker-config.yaml")

	content, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("config yaml not found: %v", err)
	}

	s := string(content)
	checks := []string{
		"pinhole:",
		"bridge: bridge100",
		"pf_anchor: com.boxxy.zerobrew",
		"allowed_ports:",
		"pcap_dir:",
	}
	for _, needle := range checks {
		if !strings.Contains(s, needle) {
			t.Errorf("config missing %q", needle)
		}
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from test file to find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}
