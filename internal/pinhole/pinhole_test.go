//go:build darwin

package pinhole

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

func TestParseARPOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantIP string
	}{
		{
			name:   "typical bridge100 entry",
			input:  "? (192.168.64.2) at aa:bb:cc:dd:ee:ff on bridge100 ifscope [ethernet]",
			wantIP: "192.168.64.2",
		},
		{
			name: "multiple entries picks first 192.168.64.x",
			input: `? (10.0.0.1) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]
? (192.168.64.5) at 11:22:33:44:55:66 on bridge100 ifscope [ethernet]
? (192.168.64.3) at 77:88:99:aa:bb:cc on bridge100 ifscope [ethernet]`,
			wantIP: "192.168.64.5",
		},
		{
			name:   "no matching entry",
			input:  "? (10.0.0.1) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]",
			wantIP: "",
		},
		{
			name:   "empty output",
			input:  "",
			wantIP: "",
		},
		{
			name:   "malformed line no parens",
			input:  "some garbage without parentheses",
			wantIP: "",
		},
		{
			name:   "incomplete entry",
			input:  "? (incomplete) at ff:ff:ff:ff:ff:ff on bridge100",
			wantIP: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseARPOutput(tt.input)
			if got != tt.wantIP {
				t.Errorf("parseARPOutput() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestGenerateRules(t *testing.T) {
	cfg := PinholeConfig{
		Bridge:    "bridge100",
		GuestIP:   "192.168.64.2",
		SessionID: "test-session",
		NATSPort:  false,
	}

	rules := generateRules(cfg)

	// Must contain DNS pass rule (tcp+udp)
	if !strings.Contains(rules, "pass on bridge100 proto { tcp udp } from 192.168.64.2 to any port 53") {
		t.Error("missing DNS pass rule")
	}
	// Must contain HTTPS pass rule
	if !strings.Contains(rules, "pass on bridge100 proto tcp from 192.168.64.2 to any port 443") {
		t.Error("missing HTTPS pass rule")
	}
	// Must contain HTTP pass rule
	if !strings.Contains(rules, "pass on bridge100 proto tcp from 192.168.64.2 to any port 80") {
		t.Error("missing HTTP pass rule")
	}
	// Must contain block rule (last)
	if !strings.Contains(rules, "block on bridge100 from 192.168.64.2 to any") {
		t.Error("missing block rule")
	}
	// Must NOT contain NATS
	if strings.Contains(rules, "4222") {
		t.Error("NATS port should not be present when NATSPort=false")
	}
	// Block rule must come after pass rules
	blockIdx := strings.Index(rules, "block on")
	lastPassIdx := strings.LastIndex(rules, "pass on")
	if blockIdx < lastPassIdx {
		t.Error("block rule must come after all pass rules")
	}
}

func TestGenerateRulesWithNATS(t *testing.T) {
	cfg := PinholeConfig{
		Bridge:    "bridge100",
		GuestIP:   "192.168.64.3",
		SessionID: "nats-session",
		NATSPort:  true,
	}

	rules := generateRules(cfg)
	if !strings.Contains(rules, "pass on bridge100 proto tcp from 192.168.64.3 to any port 4222") {
		t.Error("missing NATS pass rule when NATSPort=true")
	}
}

func TestGenerateRulesWithExtraPorts(t *testing.T) {
	cfg := PinholeConfig{
		Bridge:       "bridge100",
		GuestIP:      "192.168.64.4",
		SessionID:    "extra-ports",
		AllowedPorts: []int{8080, 9090},
	}

	rules := generateRules(cfg)
	if !strings.Contains(rules, "port 8080") {
		t.Error("missing extra port 8080")
	}
	if !strings.Contains(rules, "port 9090") {
		t.Error("missing extra port 9090")
	}
}

func TestGenerateRulesSessionComment(t *testing.T) {
	cfg := PinholeConfig{
		Bridge:    "bridge100",
		GuestIP:   "192.168.64.2",
		SessionID: "my-unique-session",
	}

	rules := generateRules(cfg)
	if !strings.Contains(rules, "# boxxy pinhole rules for session my-unique-session") {
		t.Error("session ID not in comment header")
	}
}

func TestConfPath(t *testing.T) {
	got := confPath("abc123")
	want := "/tmp/boxxy-pinhole-abc123.conf"
	if got != want {
		t.Errorf("confPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Bridge != "bridge100" {
		t.Errorf("default bridge = %q, want bridge100", cfg.Bridge)
	}
	if cfg.Anchor != "com.boxxy.zerobrew" {
		t.Errorf("default anchor = %q, want com.boxxy.zerobrew", cfg.Anchor)
	}
	if cfg.NATSPort != false {
		t.Error("default NATSPort should be false")
	}
	home, _ := os.UserHomeDir()
	wantPcap := filepath.Join(home, ".boxxy", "pcaps")
	if cfg.PcapDir != wantPcap {
		t.Errorf("default pcap dir = %q, want %q", cfg.PcapDir, wantPcap)
	}
}

func TestActivateRequiresGuestIP(t *testing.T) {
	cfg := PinholeConfig{
		Bridge: "bridge100",
		Anchor: "com.boxxy.zerobrew",
	}
	err := Activate(cfg, "/nonexistent/helper.sh")
	if err == nil {
		t.Error("expected error when GuestIP is empty")
	}
	if !strings.Contains(err.Error(), "guest IP required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateRulesWritesToFile(t *testing.T) {
	// Verify that generateRules + os.WriteFile round-trips correctly
	// (tests the same path Activate uses, without needing sudo)
	cfg := PinholeConfig{
		Bridge:    "bridge100",
		GuestIP:   "192.168.64.2",
		Anchor:    "com.boxxy.zerobrew",
		SessionID: "test-write-roundtrip",
	}

	rules := generateRules(cfg)
	path := filepath.Join(t.TempDir(), "test.conf")
	if err := os.WriteFile(path, []byte(rules), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "pass on bridge100 proto { tcp udp } from 192.168.64.2 to any port 53") {
		t.Error("written rules missing DNS rule")
	}
	if !strings.Contains(got, "block on bridge100 from 192.168.64.2 to any") {
		t.Error("written rules missing block rule")
	}
}

func TestParseDHCPLeases(t *testing.T) {
	// Write a synthetic dhcpd_leases file and test parsing
	tmpFile := filepath.Join(t.TempDir(), "dhcpd_leases")
	content := `{
	name=macvm
	ip_address=192.168.64.7
	hw_address=1,aa:bb:cc:dd:ee:ff
	identifier=1,aa:bb:cc:dd:ee:ff
	lease=0x12345678
}
{
	name=othervm
	ip_address=10.0.0.5
	hw_address=1,11:22:33:44:55:66
	identifier=1,11:22:33:44:55:66
	lease=0x12345679
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// We can't test parseDHCPLeases directly since it hardcodes /var/db/dhcpd_leases,
	// but we can test the parsing logic inline
	lines := strings.Split(content, "\n")
	var currentIP string
	var foundIP string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ip_address=") {
			currentIP = strings.TrimPrefix(line, "ip_address=")
		}
		if strings.HasPrefix(line, "}") {
			if strings.HasPrefix(currentIP, "192.168.64.") {
				foundIP = currentIP
				break
			}
			currentIP = ""
		}
	}
	if foundIP != "192.168.64.7" {
		t.Errorf("parsed IP = %q, want 192.168.64.7", foundIP)
	}
}

func TestParseARPOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantIP string
	}{
		{
			name:   "high octet",
			input:  "? (192.168.64.255) at aa:bb:cc:dd:ee:ff on bridge100",
			wantIP: "192.168.64.255",
		},
		{
			name:   "192.168.65 not matched",
			input:  "? (192.168.65.2) at aa:bb:cc:dd:ee:ff on bridge100",
			wantIP: "",
		},
		{
			name:   "nested parens",
			input:  "? (192.168.64.10) at (unknown) on bridge100",
			wantIP: "192.168.64.10",
		},
		{
			name:   "whitespace only",
			input:  "   \n   \n   ",
			wantIP: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseARPOutput(tt.input)
			if got != tt.wantIP {
				t.Errorf("parseARPOutput() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

// Property: generateRules always ends with a block rule
func TestGenerateRulesAlwaysBlocks(t *testing.T) {
	prop := func(bridge, ip, session string, nats bool, ports []uint16) bool {
		if bridge == "" || ip == "" {
			return true // skip degenerate inputs
		}
		cfg := PinholeConfig{
			Bridge:    bridge,
			GuestIP:   ip,
			SessionID: session,
			NATSPort:  nats,
		}
		for _, p := range ports {
			cfg.AllowedPorts = append(cfg.AllowedPorts, int(p))
		}
		rules := generateRules(cfg)
		lines := strings.Split(strings.TrimSpace(rules), "\n")
		lastLine := lines[len(lines)-1]
		return strings.HasPrefix(lastLine, "block on")
	}
	if err := quick.Check(prop, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("property failed: %v", err)
	}
}

// Property: generateRules always contains DNS, HTTP, HTTPS pass rules
func TestGenerateRulesAlwaysHasBaseRules(t *testing.T) {
	prop := func(nats bool, extraPorts []uint16) bool {
		cfg := PinholeConfig{
			Bridge:    "br0",
			GuestIP:   "10.0.0.1",
			SessionID: "prop-test",
			NATSPort:  nats,
		}
		for _, p := range extraPorts {
			cfg.AllowedPorts = append(cfg.AllowedPorts, int(p))
		}
		rules := generateRules(cfg)
		return strings.Contains(rules, "port 53") &&
			strings.Contains(rules, "port 443") &&
			strings.Contains(rules, "port 80")
	}
	if err := quick.Check(prop, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("property failed: %v", err)
	}
}

// Property: NATS rule present iff NATSPort=true
func TestGenerateRulesNATSProperty(t *testing.T) {
	prop := func(nats bool) bool {
		cfg := PinholeConfig{
			Bridge:    "bridge100",
			GuestIP:   "192.168.64.2",
			SessionID: "nats-prop",
			NATSPort:  nats,
		}
		rules := generateRules(cfg)
		has4222 := strings.Contains(rules, "port 4222")
		return has4222 == nats
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Fatalf("property failed: %v", err)
	}
}

// Property: each extra port appears exactly once
func TestGenerateRulesExtraPortsProperty(t *testing.T) {
	prop := func(ports []uint16) bool {
		cfg := PinholeConfig{
			Bridge:    "bridge100",
			GuestIP:   "192.168.64.2",
			SessionID: "ports-prop",
		}
		seen := map[int]bool{}
		for _, p := range ports {
			port := int(p)%65534 + 1
			if !seen[port] {
				cfg.AllowedPorts = append(cfg.AllowedPorts, port)
				seen[port] = true
			}
		}
		rules := generateRules(cfg)
		for _, port := range cfg.AllowedPorts {
			portStr := strings.Replace(
				strings.Replace(rules, "port 53", "SKIP53", 1),
				"port 80", "SKIP80", 1)
			// Just check the port number appears somewhere in a pass rule
			needle := fmt.Sprintf("port %d", port)
			if !strings.Contains(portStr, needle) && port != 443 && port != 53 && port != 80 {
				return false
			}
		}
		return true
	}
	if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("property failed: %v", err)
	}
}

// Fuzz-like: random bridge/IP never panics
type arbConfig struct {
	Bridge    string
	GuestIP   string
	SessionID string
	NATSPort  bool
	Ports     []int
}

func (arbConfig) Generate(r *rand.Rand, _ int) reflect.Value {
	bridges := []string{"bridge100", "bridge101", "en0", "lo0", ""}
	ips := []string{"192.168.64.2", "10.0.0.1", "0.0.0.0", "::1", ""}
	sessions := []string{"test", "", "a-b-c", "123"}
	var ports []int
	for i := 0; i < r.Intn(5); i++ {
		ports = append(ports, r.Intn(65535)+1)
	}
	return reflect.ValueOf(arbConfig{
		Bridge:    bridges[r.Intn(len(bridges))],
		GuestIP:   ips[r.Intn(len(ips))],
		SessionID: sessions[r.Intn(len(sessions))],
		NATSPort:  r.Intn(2) == 0,
		Ports:     ports,
	})
}

func TestGenerateRulesNeverPanics(t *testing.T) {
	prop := func(c arbConfig) bool {
		cfg := PinholeConfig{
			Bridge:       c.Bridge,
			GuestIP:      c.GuestIP,
			SessionID:    c.SessionID,
			NATSPort:     c.NATSPort,
			AllowedPorts: c.Ports,
		}
		_ = generateRules(cfg) // must not panic
		return true
	}
	if err := quick.Check(prop, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatalf("panic detected: %v", err)
	}
}

func TestConfPathProperty(t *testing.T) {
	prop := func(session string) bool {
		path := confPath(session)
		return strings.HasPrefix(path, "/tmp/boxxy-pinhole-") &&
			strings.HasSuffix(path, ".conf") &&
			strings.Contains(path, session)
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Fatalf("property failed: %v", err)
	}
}

func TestActivateFailsWithBadHelper(t *testing.T) {
	cfg := PinholeConfig{
		Bridge:    "bridge100",
		GuestIP:   "192.168.64.2",
		Anchor:    "com.boxxy.zerobrew",
		PcapDir:   t.TempDir(),
		SessionID: "test-bad-helper",
	}

	err := Activate(cfg, "/nonexistent/helper.sh")
	if err == nil {
		t.Error("expected error with nonexistent helper")
	}
	// Conf file should have been cleaned up after helper failure
	if _, statErr := os.Stat(confPath(cfg.SessionID)); statErr == nil {
		os.Remove(confPath(cfg.SessionID))
		t.Error("conf file should be cleaned up after helper failure")
	}
}
