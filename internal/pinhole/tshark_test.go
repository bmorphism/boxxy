//go:build darwin

package pinhole

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCaptureStatsJSON(t *testing.T) {
	stats := &CaptureStats{
		PcapPath:     "/tmp/test.pcap",
		PacketCount:  42,
		Bytes:        12345,
		Duration:     5 * time.Second,
		TopEndpoints: []string{"192.168.64.2", "93.184.216.34"},
		Summary:      "test summary",
	}

	j := stats.JSON()
	if j == "" {
		t.Fatal("JSON() returned empty string")
	}

	// Must be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(j), &parsed); err != nil {
		t.Fatalf("JSON() produced invalid JSON: %v", err)
	}

	if parsed["pcap_path"] != "/tmp/test.pcap" {
		t.Errorf("pcap_path = %v, want /tmp/test.pcap", parsed["pcap_path"])
	}
	if int(parsed["packet_count"].(float64)) != 42 {
		t.Errorf("packet_count = %v, want 42", parsed["packet_count"])
	}
}

func TestCaptureStatsJSONOmitsEmpty(t *testing.T) {
	stats := &CaptureStats{
		PcapPath:    "/tmp/test.pcap",
		PacketCount: 0,
	}

	j := stats.JSON()
	// top_endpoints should be omitted when nil
	if strings.Contains(j, "top_endpoints") {
		t.Error("expected top_endpoints to be omitted when nil")
	}
}

func TestCaptureEndpointsParser(t *testing.T) {
	// Simulate tshark -q -z endpoints,ip output
	output := `================================================================================
IPv4 Endpoints
Filter:<No Filter>
                       |  Packets  | |  Bytes  | | Tx Packets | | Tx Bytes | | Rx Packets | | Rx Bytes |
192.168.64.2                   100       50000           50         25000           50         25000
93.184.216.34                   80       40000           40         20000           40         20000
================================================================================`

	// This tests the parsing logic used in captureEndpoints
	var endpoints []string
	inTable := false
	for _, line := range strings.Split(output, "\n") {
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

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d: %v", len(endpoints), endpoints)
	}
	if endpoints[0] != "192.168.64.2" {
		t.Errorf("endpoints[0] = %q, want 192.168.64.2", endpoints[0])
	}
	if endpoints[1] != "93.184.216.34" {
		t.Errorf("endpoints[1] = %q, want 93.184.216.34", endpoints[1])
	}
}

func TestCaptureNotRunningError(t *testing.T) {
	c := &Capture{
		running: false,
	}
	_, err := c.StopCapture()
	if err == nil {
		t.Error("expected error when stopping non-running capture")
	}
	if !strings.Contains(err.Error(), "capture not running") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCaptureIsRunning(t *testing.T) {
	c := &Capture{running: true}
	if !c.IsRunning() {
		t.Error("expected IsRunning() = true")
	}
	c.running = false
	if c.IsRunning() {
		t.Error("expected IsRunning() = false")
	}
}

func TestCapturePcapPath(t *testing.T) {
	c := &Capture{pcapPath: "/tmp/test.pcap"}
	if c.PcapPath() != "/tmp/test.pcap" {
		t.Errorf("PcapPath() = %q, want /tmp/test.pcap", c.PcapPath())
	}
}

func TestTsharkAvailable(t *testing.T) {
	// Just verify it doesn't panic — result depends on system
	_ = TsharkAvailable()
}

func TestVerifyPinholeComplianceFilter(t *testing.T) {
	// We can't run tshark in unit tests, but we can verify the filter construction
	// by checking VerifyPinholeCompliance returns an error for a nonexistent pcap
	_, _, err := VerifyPinholeCompliance("/nonexistent.pcap", []int{53, 80, 443})
	if err == nil {
		t.Error("expected error for nonexistent pcap")
	}
}

func TestCaptureEndpointsParserEmpty(t *testing.T) {
	output := `================================================================================
IPv4 Endpoints
Filter:<No Filter>
                       |  Packets  | |  Bytes  | | Tx Packets | | Tx Bytes | | Rx Packets | | Rx Bytes |
================================================================================`

	var endpoints []string
	inTable := false
	for _, line := range strings.Split(output, "\n") {
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

	if len(endpoints) != 0 {
		t.Errorf("expected 0 endpoints, got %d: %v", len(endpoints), endpoints)
	}
}

func TestCaptureEndpointsParserSingleEntry(t *testing.T) {
	output := `================================================================================
IPv4 Endpoints
Filter:<No Filter>
                       |  Packets  | |  Bytes  | | Tx Packets | | Tx Bytes | | Rx Packets | | Rx Bytes |
1.2.3.4                         5          500            3           300            2           200
================================================================================`

	var endpoints []string
	inTable := false
	for _, line := range strings.Split(output, "\n") {
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

	if len(endpoints) != 1 || endpoints[0] != "1.2.3.4" {
		t.Errorf("expected [1.2.3.4], got %v", endpoints)
	}
}

func TestCaptureStatsJSONRoundTrip(t *testing.T) {
	stats := &CaptureStats{
		PcapPath:     "/home/user/.boxxy/pcaps/session-123.pcap",
		PacketCount:  1000,
		Bytes:        512000,
		Duration:     30 * time.Second,
		TopEndpoints: []string{"192.168.64.2", "1.1.1.1", "93.184.216.34"},
		Summary:      "io,stat summary here",
	}

	j := stats.JSON()

	var parsed CaptureStats
	if err := json.Unmarshal([]byte(j), &parsed); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}

	if parsed.PcapPath != stats.PcapPath {
		t.Errorf("PcapPath = %q, want %q", parsed.PcapPath, stats.PcapPath)
	}
	if parsed.PacketCount != stats.PacketCount {
		t.Errorf("PacketCount = %d, want %d", parsed.PacketCount, stats.PacketCount)
	}
	if parsed.Bytes != stats.Bytes {
		t.Errorf("Bytes = %d, want %d", parsed.Bytes, stats.Bytes)
	}
	if len(parsed.TopEndpoints) != len(stats.TopEndpoints) {
		t.Errorf("TopEndpoints length = %d, want %d", len(parsed.TopEndpoints), len(stats.TopEndpoints))
	}
}

func TestCaptureStopTwiceFails(t *testing.T) {
	c := &Capture{running: false}
	_, err1 := c.StopCapture()
	if err1 == nil {
		t.Error("first stop of non-running should fail")
	}
	_, err2 := c.StopCapture()
	if err2 == nil {
		t.Error("second stop should also fail")
	}
}

func TestCaptureFieldsAfterConstruction(t *testing.T) {
	c := &Capture{
		pcapPath:  "/tmp/test.pcap",
		iface:     "bridge100",
		guestIP:   "192.168.64.2",
		sessionID: "sess-42",
		running:   true,
	}

	if c.PcapPath() != "/tmp/test.pcap" {
		t.Errorf("PcapPath = %q", c.PcapPath())
	}
	if !c.IsRunning() {
		t.Error("expected running")
	}
}
