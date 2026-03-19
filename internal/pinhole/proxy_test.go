//go:build darwin

package pinhole

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProxyAllowsHTTPOnPort80(t *testing.T) {
	// Start a target HTTP server
	target := startTestHTTPServer(t, "hello from target")
	defer target.Close()

	// Start proxy allowing port 80 and the target's port
	targetPort := portFromAddr(t, target.Listener.Addr().String())
	proxy := NewProxy(ProxyConfig{
		ListenAddr:   "127.0.0.1:0",
		AllowedPorts: []int{80, targetPort},
		LogDir:       t.TempDir(),
		SessionID:    "test-allow-http",
	})
	addr, err := proxy.Start()
	if err != nil {
		t.Fatalf("proxy start: %v", err)
	}
	defer proxy.Stop()

	// Make request through proxy
	client := proxyClient(t, addr)
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", targetPort))
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "hello from target") {
		t.Errorf("body = %q, want 'hello from target'", body)
	}

	// Check logs
	logs := proxy.Logs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	if !logs[0].Allowed {
		t.Error("expected connection to be allowed")
	}
	if logs[0].Port != targetPort {
		t.Errorf("logged port = %d, want %d", logs[0].Port, targetPort)
	}
}

func TestProxyBlocksDisallowedPort(t *testing.T) {
	// Start a target on a port NOT in the allowed list
	target := startTestHTTPServer(t, "should not reach")
	defer target.Close()
	targetPort := portFromAddr(t, target.Listener.Addr().String())

	// Proxy only allows port 80
	proxy := NewProxy(ProxyConfig{
		ListenAddr:   "127.0.0.1:0",
		AllowedPorts: []int{80},
		LogDir:       t.TempDir(),
		SessionID:    "test-block",
	})
	addr, err := proxy.Start()
	if err != nil {
		t.Fatalf("proxy start: %v", err)
	}
	defer proxy.Stop()

	client := proxyClient(t, addr)
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", targetPort))
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 Forbidden", resp.StatusCode)
	}

	logs := proxy.Logs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Allowed {
		t.Error("expected connection to be blocked")
	}
	if logs[0].Error == "" {
		t.Error("expected error message in log for blocked connection")
	}
}

func TestProxyCONNECTAllowed(t *testing.T) {
	// Start a TLS-ish TCP server (just echoes)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	targetPort := portFromAddr(t, ln.Addr().String())

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("tunnel-ok"))
			conn.Close()
		}
	}()

	proxy := NewProxy(ProxyConfig{
		ListenAddr:   "127.0.0.1:0",
		AllowedPorts: []int{targetPort},
		LogDir:       t.TempDir(),
		SessionID:    "test-connect",
	})
	addr, err := proxy.Start()
	if err != nil {
		t.Fatalf("proxy start: %v", err)
	}
	defer proxy.Stop()

	// Manual CONNECT
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT 127.0.0.1:%d HTTP/1.1\r\nHost: 127.0.0.1:%d\r\n\r\n", targetPort, targetPort)

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	response := string(buf[:n])

	if !strings.Contains(response, "200 Connection Established") {
		t.Errorf("CONNECT response = %q, want 200", response)
	}

	// Read tunnel data
	n, _ = conn.Read(buf)
	if !strings.Contains(string(buf[:n]), "tunnel-ok") {
		t.Errorf("tunnel data = %q, want 'tunnel-ok'", buf[:n])
	}
}

func TestProxyCONNECTBlocked(t *testing.T) {
	proxy := NewProxy(ProxyConfig{
		ListenAddr:   "127.0.0.1:0",
		AllowedPorts: []int{443},
		LogDir:       t.TempDir(),
		SessionID:    "test-connect-block",
	})
	addr, err := proxy.Start()
	if err != nil {
		t.Fatalf("proxy start: %v", err)
	}
	defer proxy.Stop()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// CONNECT to port 8080 (not allowed)
	fmt.Fprintf(conn, "CONNECT evil.com:8080 HTTP/1.1\r\nHost: evil.com:8080\r\n\r\n")

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	response := string(buf[:n])

	if !strings.Contains(response, "403") {
		t.Errorf("blocked CONNECT response = %q, want 403", response)
	}

	logs := proxy.Logs()
	blocked := false
	for _, l := range logs {
		if !l.Allowed && l.Port == 8080 {
			blocked = true
		}
	}
	if !blocked {
		t.Error("expected blocked log entry for port 8080")
	}
}

func TestProxyStats(t *testing.T) {
	target := startTestHTTPServer(t, "stats-test")
	defer target.Close()
	targetPort := portFromAddr(t, target.Listener.Addr().String())

	proxy := NewProxy(ProxyConfig{
		ListenAddr:   "127.0.0.1:0",
		AllowedPorts: []int{targetPort},
		LogDir:       t.TempDir(),
		SessionID:    "test-stats",
	})
	addr, err := proxy.Start()
	if err != nil {
		t.Fatalf("proxy start: %v", err)
	}

	client := proxyClient(t, addr)

	// 3 allowed requests
	for i := 0; i < 3; i++ {
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", targetPort))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	// 1 blocked request
	resp, _ := client.Get("http://127.0.0.1:9999/")
	if resp != nil {
		resp.Body.Close()
	}

	stats, err := proxy.Stop()
	if err != nil {
		t.Fatalf("proxy stop: %v", err)
	}

	if stats.TotalConns != 4 {
		t.Errorf("TotalConns = %d, want 4", stats.TotalConns)
	}
	if stats.AllowedConns != 3 {
		t.Errorf("AllowedConns = %d, want 3", stats.AllowedConns)
	}
	if stats.BlockedConns != 1 {
		t.Errorf("BlockedConns = %d, want 1", stats.BlockedConns)
	}
	if stats.Compliant {
		t.Error("expected non-compliant (had blocked connection)")
	}
	if stats.ByPort[targetPort] != 3 {
		t.Errorf("ByPort[%d] = %d, want 3", targetPort, stats.ByPort[targetPort])
	}

	j := stats.JSON()
	if j == "" {
		t.Error("JSON() empty")
	}
	t.Logf("stats JSON:\n%s", j)
}

func TestProxyStopWritesLog(t *testing.T) {
	logDir := t.TempDir()
	proxy := NewProxy(ProxyConfig{
		ListenAddr:   "127.0.0.1:0",
		AllowedPorts: []int{80},
		LogDir:       logDir,
		SessionID:    "test-log-write",
	})
	_, err := proxy.Start()
	if err != nil {
		t.Fatalf("proxy start: %v", err)
	}

	_, err = proxy.Stop()
	if err != nil {
		t.Fatalf("proxy stop: %v", err)
	}

	// Log file should exist
	logPath := fmt.Sprintf("%s/test-log-write.jsonl", logDir)
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file not written: %v", err)
	}
}

func TestProxyDoubleStopFails(t *testing.T) {
	proxy := NewProxy(ProxyConfig{
		ListenAddr:   "127.0.0.1:0",
		AllowedPorts: []int{80},
		LogDir:       t.TempDir(),
		SessionID:    "test-double-stop",
	})
	_, err := proxy.Start()
	if err != nil {
		t.Fatal(err)
	}

	_, err = proxy.Stop()
	if err != nil {
		t.Fatal(err)
	}

	_, err = proxy.Stop()
	if err == nil {
		t.Error("second stop should fail")
	}
}

func TestProxyEnvVars(t *testing.T) {
	vars := ProxyEnvVars("192.168.64.1", 3128)

	found := map[string]bool{}
	for _, v := range vars {
		if strings.HasPrefix(v, "http_proxy=") {
			if !strings.Contains(v, "192.168.64.1:3128") {
				t.Errorf("http_proxy = %q", v)
			}
			found["http_proxy"] = true
		}
		if strings.HasPrefix(v, "https_proxy=") {
			if !strings.Contains(v, "192.168.64.1:3128") {
				t.Errorf("https_proxy = %q", v)
			}
			found["https_proxy"] = true
		}
		if strings.HasPrefix(v, "no_proxy=") {
			found["no_proxy"] = true
		}
	}
	for _, key := range []string{"http_proxy", "https_proxy", "no_proxy"} {
		if !found[key] {
			t.Errorf("missing %s in env vars", key)
		}
	}
}

func TestProxyDefaultConfig(t *testing.T) {
	cfg := DefaultProxyConfig()
	if cfg.ListenAddr != "0.0.0.0:3128" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if len(cfg.AllowedPorts) != 3 {
		t.Errorf("AllowedPorts = %v, want [53,80,443]", cfg.AllowedPorts)
	}
}

func TestHostBridgeIP(t *testing.T) {
	// May or may not find an IP — just verify no panic
	ip, err := HostBridgeIP("bridge100")
	if err != nil {
		t.Logf("no bridge100 IP (expected if no VM): %v", err)
	} else {
		t.Logf("bridge100 IP: %s", ip)
		if !strings.HasPrefix(ip, "192.168.") {
			t.Errorf("unexpected IP: %s", ip)
		}
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"80", 80},
		{"443", 443},
		{"8080", 8080},
		{"", 80},
		{"abc", 80},
	}
	for _, tt := range tests {
		got := parsePort(tt.in)
		if got != tt.want {
			t.Errorf("parsePort(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// --- helpers ---

type testServer struct {
	*http.Server
	Listener net.Listener
}

func (s *testServer) Close() {
	s.Server.Close()
}

func startTestHTTPServer(t *testing.T, response string) *testServer {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return &testServer{Server: srv, Listener: ln}
}

func portFromAddr(t *testing.T, addr string) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

func proxyClient(t *testing.T, proxyAddr string) *http.Client {
	t.Helper()
	proxyURL, _ := url.Parse("http://" + proxyAddr)
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}
