//go:build darwin

package pinhole

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// CaptureStats holds summary statistics from a tshark capture.
type CaptureStats struct {
	PcapPath     string        `json:"pcap_path"`
	PacketCount  int           `json:"packet_count"`
	Bytes        int64         `json:"bytes"`
	Duration     time.Duration `json:"duration"`
	TopEndpoints []string      `json:"top_endpoints,omitempty"`
	Summary      string        `json:"summary,omitempty"`
}

// Capture manages a tshark capture process.
type Capture struct {
	cmd       *exec.Cmd
	pcapPath  string
	iface     string
	guestIP   string
	sessionID string
	startTime time.Time
	mu        sync.Mutex
	running   bool
}

// StartCapture spawns tshark to capture traffic for the given guest IP.
// Returns a Capture handle for later stopping.
func StartCapture(iface, guestIP, pcapDir, sessionID string) (*Capture, error) {
	if err := os.MkdirAll(pcapDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pcap dir: %w", err)
	}

	pcapPath := filepath.Join(pcapDir, fmt.Sprintf("%s.pcap", sessionID))
	filter := fmt.Sprintf("host %s", guestIP)

	cmd := exec.Command("tshark",
		"-i", iface,
		"-w", pcapPath,
		"-f", filter,
		"-q", // quiet mode (no packet output to stdout)
	)
	cmd.Stdout = os.Stderr // tshark status goes to stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tshark: %w", err)
	}

	return &Capture{
		cmd:       cmd,
		pcapPath:  pcapPath,
		iface:     iface,
		guestIP:   guestIP,
		sessionID: sessionID,
		startTime: time.Now(),
		running:   true,
	}, nil
}

// StopCapture sends SIGINT to tshark and collects capture statistics.
func (c *Capture) StopCapture() (*CaptureStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil, fmt.Errorf("capture not running")
	}

	// Send SIGINT for graceful shutdown
	if c.cmd.Process != nil {
		c.cmd.Process.Signal(syscall.SIGINT)
	}

	// Wait for process to exit
	c.cmd.Wait()
	c.running = false

	duration := time.Since(c.startTime)

	stats := &CaptureStats{
		PcapPath: c.pcapPath,
		Duration: duration,
	}

	// Get capture summary via tshark -r
	summary, err := captureSummary(c.pcapPath)
	if err == nil {
		stats.Summary = summary
	}

	// Get packet count
	count, bytes, err := captureCount(c.pcapPath)
	if err == nil {
		stats.PacketCount = count
		stats.Bytes = bytes
	}

	// Get top endpoints
	endpoints, err := captureEndpoints(c.pcapPath)
	if err == nil {
		stats.TopEndpoints = endpoints
	}

	return stats, nil
}

// IsRunning returns whether the capture is still active.
func (c *Capture) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// PcapPath returns the path to the pcap file.
func (c *Capture) PcapPath() string {
	return c.pcapPath
}

// JSON returns the capture stats as JSON.
func (s *CaptureStats) JSON() string {
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

func captureSummary(pcapPath string) (string, error) {
	out, err := exec.Command("tshark", "-r", pcapPath, "-q", "-z", "io,stat,0").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func captureCount(pcapPath string) (int, int64, error) {
	// Use capinfos for packet count and byte count
	out, err := exec.Command("capinfos", "-c", "-s", "-M", pcapPath).Output()
	if err != nil {
		// Fallback: count with tshark
		out2, err2 := exec.Command("tshark", "-r", pcapPath, "-T", "fields", "-e", "frame.number").Output()
		if err2 != nil {
			return 0, 0, err2
		}
		lines := strings.Split(strings.TrimSpace(string(out2)), "\n")
		if len(lines) == 1 && lines[0] == "" {
			return 0, 0, nil
		}
		return len(lines), 0, nil
	}

	var count int
	var bytes int64
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Number of packets") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
					count = n
				}
			}
		}
		if strings.Contains(line, "File size") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				if n, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
					bytes = n
				}
			}
		}
	}
	return count, bytes, nil
}

func captureEndpoints(pcapPath string) ([]string, error) {
	out, err := exec.Command("tshark", "-r", pcapPath, "-q", "-z", "endpoints,ip").Output()
	if err != nil {
		return nil, err
	}

	var endpoints []string
	inTable := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Filter:") || line == "" {
			continue
		}
		if strings.Contains(line, "Bytes") && strings.Contains(line, "Packets") {
			inTable = true
			continue
		}
		if inTable && line != "" && !strings.HasPrefix(line, "=") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				endpoints = append(endpoints, fields[0])
			}
		}
	}
	return endpoints, nil
}

// TsharkAvailable checks if tshark is installed.
func TsharkAvailable() bool {
	_, err := exec.LookPath("tshark")
	return err == nil
}

// VerifyPinholeCompliance reads a pcap and checks that only allowed ports were used.
func VerifyPinholeCompliance(pcapPath string, allowedPorts []int) (bool, string, error) {
	// Build a display filter excluding allowed ports
	var parts []string
	for _, p := range allowedPorts {
		parts = append(parts, fmt.Sprintf("tcp.port != %d and udp.port != %d", p, p))
	}
	filter := strings.Join(parts, " and ")

	out, err := exec.Command("tshark", "-r", pcapPath, "-Y", filter, "-T", "fields", "-e", "frame.number").Output()
	if err != nil {
		return false, "", fmt.Errorf("tshark compliance check failed: %w", err)
	}

	lines := strings.TrimSpace(string(out))
	if lines == "" {
		return true, "all traffic within allowed ports", nil
	}

	count := len(strings.Split(lines, "\n"))
	return false, fmt.Sprintf("%d packets on disallowed ports", count), nil
}
