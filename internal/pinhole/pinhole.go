//go:build darwin

package pinhole

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PinholeConfig holds configuration for the pf-based network pinhole.
type PinholeConfig struct {
	Bridge       string   // bridge interface (e.g. bridge100)
	GuestIP      string   // guest IP address (e.g. 192.168.64.2)
	AllowedPorts []int    // additional ports beyond DNS/HTTP/HTTPS
	PcapDir      string   // directory for pcap files
	Anchor       string   // pf anchor name (e.g. com.boxxy.zerobrew)
	SessionID    string   // unique session identifier
	NATSPort     bool     // whether to allow NATS port 4222
}

// DefaultConfig returns a PinholeConfig with sensible defaults.
func DefaultConfig() PinholeConfig {
	home, _ := os.UserHomeDir()
	return PinholeConfig{
		Bridge:   "bridge100",
		Anchor:   "com.boxxy.zerobrew",
		PcapDir:  filepath.Join(home, ".boxxy", "pcaps"),
		NATSPort: false,
	}
}

// DetectGuestIP attempts to find the guest IP on the given bridge interface
// by parsing arp -a and /var/db/dhcpd_leases.
func DetectGuestIP(bridge string) (string, error) {
	// Try arp -a first
	out, err := exec.Command("arp", "-a", "-i", bridge).Output()
	if err == nil {
		ip := parseARPOutput(string(out))
		if ip != "" {
			return ip, nil
		}
	}

	// Fall back to dhcpd_leases
	ip, err := parseDHCPLeases(bridge)
	if err == nil && ip != "" {
		return ip, nil
	}

	return "", fmt.Errorf("no guest IP found on %s", bridge)
}

// DetectGuestIPRetry polls for the guest IP with retries.
func DetectGuestIPRetry(bridge string, maxRetries int, interval time.Duration) (string, error) {
	for i := 0; i < maxRetries; i++ {
		ip, err := DetectGuestIP(bridge)
		if err == nil {
			return ip, nil
		}
		if i < maxRetries-1 {
			time.Sleep(interval)
		}
	}
	return "", fmt.Errorf("guest IP not found on %s after %d retries", bridge, maxRetries)
}

func parseARPOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// arp -a format: ? (192.168.64.2) at aa:bb:cc:dd:ee:ff on bridge100 ...
		if strings.Contains(line, "(") && strings.Contains(line, ")") {
			start := strings.Index(line, "(")
			end := strings.Index(line, ")")
			if start >= 0 && end > start {
				ip := line[start+1 : end]
				if strings.HasPrefix(ip, "192.168.64.") {
					return ip
				}
			}
		}
	}
	return ""
}

func parseDHCPLeases(bridge string) (string, error) {
	f, err := os.Open("/var/db/dhcpd_leases")
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var currentIP string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ip_address=") {
			currentIP = strings.TrimPrefix(line, "ip_address=")
		}
		if strings.HasPrefix(line, "}") {
			if strings.HasPrefix(currentIP, "192.168.64.") {
				return currentIP, nil
			}
			currentIP = ""
		}
	}
	return "", fmt.Errorf("no lease found for bridge %s", bridge)
}

// generateRules creates the pf anchor rules content.
func generateRules(cfg PinholeConfig) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# boxxy pinhole rules for session %s\n", cfg.SessionID))
	b.WriteString(fmt.Sprintf("# Generated at %s\n\n", time.Now().Format(time.RFC3339)))

	// DNS
	b.WriteString(fmt.Sprintf("pass on %s proto { tcp udp } from %s to any port 53\n", cfg.Bridge, cfg.GuestIP))
	// HTTPS
	b.WriteString(fmt.Sprintf("pass on %s proto tcp from %s to any port 443\n", cfg.Bridge, cfg.GuestIP))
	// HTTP
	b.WriteString(fmt.Sprintf("pass on %s proto tcp from %s to any port 80\n", cfg.Bridge, cfg.GuestIP))

	// NATS
	if cfg.NATSPort {
		b.WriteString(fmt.Sprintf("pass on %s proto tcp from %s to any port 4222\n", cfg.Bridge, cfg.GuestIP))
	}

	// Additional ports
	for _, port := range cfg.AllowedPorts {
		b.WriteString(fmt.Sprintf("pass on %s proto tcp from %s to any port %d\n", cfg.Bridge, cfg.GuestIP, port))
	}

	// Block everything else from guest
	b.WriteString(fmt.Sprintf("block on %s from %s to any\n", cfg.Bridge, cfg.GuestIP))

	return b.String()
}

// confPath returns the path for the session's pf config file.
func confPath(sessionID string) string {
	return fmt.Sprintf("/tmp/boxxy-pinhole-%s.conf", sessionID)
}

// Activate writes pf rules and loads them via the pfctl helper script.
// The helperPath should point to boxxy-pfctl-helper.sh.
func Activate(cfg PinholeConfig, helperPath string) error {
	if cfg.GuestIP == "" {
		return fmt.Errorf("guest IP required")
	}
	if cfg.SessionID == "" {
		cfg.SessionID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Ensure pcap directory exists
	if err := os.MkdirAll(cfg.PcapDir, 0755); err != nil {
		return fmt.Errorf("failed to create pcap dir: %w", err)
	}

	// Write rules file
	rules := generateRules(cfg)
	path := confPath(cfg.SessionID)
	if err := os.WriteFile(path, []byte(rules), 0644); err != nil {
		return fmt.Errorf("failed to write pf rules: %w", err)
	}

	// Load via helper (requires sudo)
	cmd := exec.Command("sudo", helperPath, "activate", path, cfg.Anchor)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(path)
		return fmt.Errorf("failed to activate pinhole: %w", err)
	}

	return nil
}

// Deactivate flushes the anchor rules and cleans up temp files.
func Deactivate(cfg PinholeConfig, helperPath string) error {
	cmd := exec.Command("sudo", helperPath, "deactivate", cfg.Anchor)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to deactivate pinhole: %w", err)
	}

	// Clean up conf file if session ID is known
	if cfg.SessionID != "" {
		os.Remove(confPath(cfg.SessionID))
	}

	return nil
}

// IsActive checks whether the anchor has active rules.
func IsActive(anchor string) (bool, error) {
	out, err := exec.Command("sudo", "pfctl", "-a", anchor, "-sr").Output()
	if err != nil {
		return false, err
	}
	lines := strings.TrimSpace(string(out))
	return lines != "", nil
}

// Status returns the current anchor rules as a string.
func Status(anchor string) (string, error) {
	out, err := exec.Command("sudo", "pfctl", "-a", anchor, "-sr").Output()
	if err != nil {
		return "", fmt.Errorf("failed to query anchor %s: %w", anchor, err)
	}
	return string(out), nil
}
