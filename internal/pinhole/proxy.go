//go:build darwin

package pinhole

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProxyConfig configures the userspace pinhole proxy.
// This replaces pf+tshark with a single sudo-free Go process.
type ProxyConfig struct {
	ListenAddr   string // host:port to listen on (default: 0.0.0.0:3128)
	AllowedPorts []int  // ports the guest may connect to (default: 53,80,443)
	LogDir       string // directory for connection logs
	SessionID    string // session identifier for log files
}

// DefaultProxyConfig returns sensible defaults.
func DefaultProxyConfig() ProxyConfig {
	home, _ := os.UserHomeDir()
	return ProxyConfig{
		ListenAddr:   "0.0.0.0:3128",
		AllowedPorts: []int{53, 80, 443},
		LogDir:       filepath.Join(home, ".boxxy", "logs"),
		SessionID:    fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}

// ConnLog records a single proxied connection.
type ConnLog struct {
	Timestamp   time.Time `json:"timestamp"`
	Method      string    `json:"method"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Allowed     bool      `json:"allowed"`
	BytesSent   int64     `json:"bytes_sent"`
	BytesRecv   int64     `json:"bytes_recv"`
	DurationMs  int64     `json:"duration_ms"`
	Error       string    `json:"error,omitempty"`
}

// ProxyStats summarizes all connections through the proxy.
type ProxyStats struct {
	SessionID    string            `json:"session_id"`
	StartTime    time.Time         `json:"start_time"`
	Duration     time.Duration     `json:"duration"`
	TotalConns   int               `json:"total_connections"`
	AllowedConns int               `json:"allowed_connections"`
	BlockedConns int               `json:"blocked_connections"`
	TotalBytes   int64             `json:"total_bytes"`
	ByPort       map[int]int       `json:"connections_by_port"`
	TopHosts     []string          `json:"top_hosts,omitempty"`
	Compliant    bool              `json:"compliant"`
}

// JSON returns stats as formatted JSON.
func (s *ProxyStats) JSON() string {
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

// Proxy is a sudo-free HTTP CONNECT proxy that enforces port restrictions
// and logs all connection attempts. Replaces pf+tshark.
type Proxy struct {
	config    ProxyConfig
	allowed   map[int]bool
	server    *http.Server
	listener  net.Listener
	startTime time.Time

	mu       sync.Mutex
	logs     []ConnLog
	running  atomic.Bool
}

// NewProxy creates a new pinhole proxy.
func NewProxy(cfg ProxyConfig) *Proxy {
	allowed := make(map[int]bool)
	for _, p := range cfg.AllowedPorts {
		allowed[p] = true
	}
	return &Proxy{
		config:  cfg,
		allowed: allowed,
	}
}

// Start begins listening. Returns the actual listen address (useful when port is 0).
func (p *Proxy) Start() (string, error) {
	if err := os.MkdirAll(p.config.LogDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log dir: %w", err)
	}

	ln, err := net.Listen("tcp", p.config.ListenAddr)
	if err != nil {
		return "", fmt.Errorf("failed to listen: %w", err)
	}
	p.listener = ln
	p.startTime = time.Now()
	p.running.Store(true)

	p.server = &http.Server{
		Handler: http.HandlerFunc(p.handleRequest),
	}

	go p.server.Serve(ln)

	return ln.Addr().String(), nil
}

// Stop shuts down the proxy and writes the connection log.
func (p *Proxy) Stop() (*ProxyStats, error) {
	if !p.running.Load() {
		return nil, fmt.Errorf("proxy not running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.server.Shutdown(ctx)
	p.running.Store(false)

	stats := p.Stats()

	// Write log file
	if err := p.writeLog(); err != nil {
		return stats, fmt.Errorf("stats collected but log write failed: %w", err)
	}

	return stats, nil
}

// ListenAddr returns the actual address the proxy is listening on.
func (p *Proxy) ListenAddr() string {
	if p.listener == nil {
		return ""
	}
	return p.listener.Addr().String()
}

// IsRunning returns whether the proxy is active.
func (p *Proxy) IsRunning() bool {
	return p.running.Load()
}

// Stats returns current proxy statistics.
func (p *Proxy) Stats() *ProxyStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := &ProxyStats{
		SessionID:  p.config.SessionID,
		StartTime:  p.startTime,
		Duration:   time.Since(p.startTime),
		TotalConns: len(p.logs),
		ByPort:     make(map[int]int),
		Compliant:  true,
	}

	hostCount := make(map[string]int)
	for _, l := range p.logs {
		if l.Allowed {
			stats.AllowedConns++
			stats.TotalBytes += l.BytesSent + l.BytesRecv
		} else {
			stats.BlockedConns++
			stats.Compliant = false
		}
		stats.ByPort[l.Port]++
		hostCount[l.Host]++
	}

	// Top hosts by connection count
	type hostEntry struct {
		host  string
		count int
	}
	var hosts []hostEntry
	for h, c := range hostCount {
		hosts = append(hosts, hostEntry{h, c})
	}
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].count > hosts[j].count
	})
	for i, h := range hosts {
		if i >= 10 {
			break
		}
		stats.TopHosts = append(stats.TopHosts, fmt.Sprintf("%s (%d)", h.host, h.count))
	}

	return stats
}

// Logs returns a copy of all connection logs.
func (p *Proxy) Logs() []ConnLog {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]ConnLog, len(p.logs))
	copy(out, p.logs)
	return out
}

func (p *Proxy) addLog(l ConnLog) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.logs = append(p.logs, l)
}

func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

// handleConnect handles HTTPS CONNECT tunneling.
func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	host, portStr, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		portStr = "443"
	}
	port := parsePort(portStr)

	log := ConnLog{
		Timestamp: start,
		Method:    "CONNECT",
		Host:      host,
		Port:      port,
	}

	if !p.allowed[port] {
		log.Allowed = false
		log.DurationMs = time.Since(start).Milliseconds()
		log.Error = fmt.Sprintf("port %d not in allowed list", port)
		p.addLog(log)
		http.Error(w, fmt.Sprintf("port %d blocked by pinhole policy", port), http.StatusForbidden)
		return
	}

	log.Allowed = true

	// Dial the target
	target, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		log.Error = err.Error()
		log.DurationMs = time.Since(start).Milliseconds()
		p.addLog(log)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer target.Close()

	// Hijack the client connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Error = "hijack not supported"
		log.DurationMs = time.Since(start).Milliseconds()
		p.addLog(log)
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}

	client, _, err := hj.Hijack()
	if err != nil {
		log.Error = err.Error()
		log.DurationMs = time.Since(start).Milliseconds()
		p.addLog(log)
		return
	}
	defer client.Close()

	// Send 200 Connection Established
	client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Bidirectional copy with byte counting
	var sent, recv int64
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(target, client)
		atomic.AddInt64(&sent, n)
		target.(*net.TCPConn).CloseWrite()
	}()
	go func() {
		defer wg.Done()
		n, _ := io.Copy(client, target)
		atomic.AddInt64(&recv, n)
		client.(*net.TCPConn).CloseWrite()
	}()
	wg.Wait()

	log.BytesSent = atomic.LoadInt64(&sent)
	log.BytesRecv = atomic.LoadInt64(&recv)
	log.DurationMs = time.Since(start).Milliseconds()
	p.addLog(log)
}

// handleHTTP handles plain HTTP requests (GET, POST, etc).
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	host := r.URL.Hostname()
	portStr := r.URL.Port()
	if portStr == "" {
		portStr = "80"
	}
	port := parsePort(portStr)

	log := ConnLog{
		Timestamp: start,
		Method:    r.Method,
		Host:      host,
		Port:      port,
	}

	if !p.allowed[port] {
		log.Allowed = false
		log.DurationMs = time.Since(start).Milliseconds()
		log.Error = fmt.Sprintf("port %d not in allowed list", port)
		p.addLog(log)
		http.Error(w, fmt.Sprintf("port %d blocked by pinhole policy", port), http.StatusForbidden)
		return
	}

	log.Allowed = true

	// Forward the request
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		log.Error = err.Error()
		log.DurationMs = time.Since(start).Milliseconds()
		p.addLog(log)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	outReq.Header = r.Header.Clone()
	outReq.Header.Del("Proxy-Connection")

	resp, err := http.DefaultTransport.RoundTrip(outReq)
	if err != nil {
		log.Error = err.Error()
		log.DurationMs = time.Since(start).Milliseconds()
		p.addLog(log)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	n, _ := io.Copy(w, resp.Body)
	log.BytesRecv = n
	log.DurationMs = time.Since(start).Milliseconds()
	p.addLog(log)
}

func (p *Proxy) writeLog() error {
	p.mu.Lock()
	logs := make([]ConnLog, len(p.logs))
	copy(logs, p.logs)
	p.mu.Unlock()

	logPath := filepath.Join(p.config.LogDir, fmt.Sprintf("%s.jsonl", p.config.SessionID))
	f, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, l := range logs {
		if err := enc.Encode(l); err != nil {
			return err
		}
	}
	return nil
}

func parsePort(s string) int {
	var port int
	fmt.Sscanf(s, "%d", &port)
	if port == 0 {
		return 80
	}
	return port
}

// ProxyEnvVars returns the environment variables to set on the guest
// so that traffic routes through this proxy.
func ProxyEnvVars(hostIP string, port int) []string {
	addr := fmt.Sprintf("http://%s:%d", hostIP, port)
	return []string{
		fmt.Sprintf("http_proxy=%s", addr),
		fmt.Sprintf("https_proxy=%s", addr),
		fmt.Sprintf("HTTP_PROXY=%s", addr),
		fmt.Sprintf("HTTPS_PROXY=%s", addr),
		// Don't proxy localhost traffic within the guest
		"no_proxy=localhost,127.0.0.1",
		"NO_PROXY=localhost,127.0.0.1",
	}
}

// HostBridgeIP returns the host's IP on the bridge interface.
func HostBridgeIP(bridge string) (string, error) {
	iface, err := net.InterfaceByName(bridge)
	if err != nil {
		return "", fmt.Errorf("interface %s not found: %w", bridge, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			ip := ipnet.IP.String()
			if strings.HasPrefix(ip, "192.168.") {
				return ip, nil
			}
		}
	}
	return "", fmt.Errorf("no IPv4 address found on %s", bridge)
}
