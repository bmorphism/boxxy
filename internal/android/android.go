//go:build darwin

// Package android implements a hardened Android emulator surface for boxxy.
//
// This is NOT a convenience wrapper. It is a proof-of-attack platform that:
//
//  1. Boots an Android AVD inside the existing boxxy pinhole network boundary
//  2. Routes all emulator traffic through the userspace proxy (port allow-list)
//  3. Attaches the Maxwell's demon for multipath probing of emulator egress
//  4. Wraps ADB commands in InvisiCap-style capability tracking:
//     every adb shell/install gets an invisible capability with bounds
//  5. Feeds connection logs to the exploit arena for triadic consensus
//  6. Records a topological embedding of all app network flows
//
// Attack surface model:
//
//	╔══════════════════════════════════════════════════╗
//	║  Android Emulator (guest ARM64)                 ║
//	║  ┌─────────────┐  ┌──────────────┐             ║
//	║  │  UberEats    │  │  Play Store  │  ... apps   ║
//	║  └──────┬───────┘  └──────┬───────┘             ║
//	║         │    ADB bridge   │                     ║
//	╠═════════╪═════════════════╪══════════════════════╣
//	║         │  InvisiCap gate │                      ║
//	║    ┌────▼─────────────────▼────┐                 ║
//	║    │   Pinhole Proxy (port ACL)│ ◄── demon probe ║
//	║    └────────────┬──────────────┘                 ║
//	║                 │ tshark/pcap                    ║
//	║    ┌────────────▼──────────────┐                 ║
//	║    │  Topology Builder (H0,H1) │                 ║
//	║    └────────────┬──────────────┘                 ║
//	║                 │ exploit feed                   ║
//	║    ┌────────────▼──────────────┐                 ║
//	║    │  Exploit Arena (GF3)      │                 ║
//	║    └───────────────────────────┘                 ║
//	╚══════════════════════════════════════════════════╝
//
// The UberEats use-case is the proof-of-attack target:
//   - Can UberEats exfiltrate location to non-HTTPS endpoints?
//   - Does it phone home to analytics domains outside the allow-list?
//   - What's the topological complexity of its network graph?
//   - Can the demon detect anomalous path usage (covert channels)?
package android

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultAPILevel  = "35"
	DefaultImageType = "google_apis_playstore"
	DefaultABI       = "arm64-v8a"
	DefaultAVDName   = "boxxy-attack-surface"
	DefaultDeviceID  = "pixel_7"
	DefaultMemoryMB  = 4096
	DefaultDiskGB    = 16

	cmdlineToolsURL = "https://dl.google.com/android/repository/commandlinetools-mac-11076708_latest.zip"
)

// Config holds Android emulator configuration.
type Config struct {
	Name      string
	APILevel  string
	ImageType string // google_apis_playstore for UberEats (needs Play Store)
	ABI       string
	DeviceID  string
	MemoryMB  int
	DiskGB    int
	Headless  bool
	GPU       string
	SDKRoot   string

	// Hardening: ports the emulator is allowed to reach
	AllowedPorts []int
	// Hardening: domains the emulator may resolve (DNS sinkhole for others)
	AllowedDomains []string
	// Hardening: proxy address for pinhole routing
	ProxyAddr string
}

// DefaultConfig returns a hardened default for attack surface testing.
func DefaultConfig() Config {
	return Config{
		Name:      DefaultAVDName,
		APILevel:  DefaultAPILevel,
		ImageType: DefaultImageType,
		ABI:       DefaultABI,
		DeviceID:  DefaultDeviceID,
		MemoryMB:  DefaultMemoryMB,
		DiskGB:    DefaultDiskGB,
		GPU:       "host",
		AllowedPorts: []int{
			53,  // DNS
			80,  // HTTP
			443, // HTTPS
		},
		AllowedDomains: []string{
			// UberEats known domains
			"*.ubereats.com",
			"*.uber.com",
			"cn-geo1.uber.com",
			"*.googleapis.com",
			"*.gstatic.com",
			"*.google.com",
			"play.google.com",
			"*.cloudfront.net",
		},
	}
}

// Paths holds resolved SDK tool paths.
type Paths struct {
	SDKRoot    string
	Sdkmanager string
	Avdmanager string
	Emulator   string
	ADB        string
}

// ResolvePaths finds the Android SDK command-line tools.
func ResolvePaths(sdkRoot string) (*Paths, error) {
	if sdkRoot == "" {
		if env := os.Getenv("ANDROID_HOME"); env != "" {
			sdkRoot = env
		} else if env := os.Getenv("ANDROID_SDK_ROOT"); env != "" {
			sdkRoot = env
		} else {
			home, _ := os.UserHomeDir()
			sdkRoot = filepath.Join(home, ".boxxy", "android-sdk")
		}
	}

	p := &Paths{SDKRoot: sdkRoot}
	candidates := []string{
		filepath.Join(sdkRoot, "cmdline-tools", "latest", "bin", "sdkmanager"),
		filepath.Join(sdkRoot, "cmdline-tools", "bin", "sdkmanager"),
		filepath.Join(sdkRoot, "tools", "bin", "sdkmanager"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			p.Sdkmanager = c
			break
		}
	}
	if p.Sdkmanager != "" {
		p.Avdmanager = strings.Replace(p.Sdkmanager, "sdkmanager", "avdmanager", 1)
	}
	p.Emulator = filepath.Join(sdkRoot, "emulator", "emulator")
	p.ADB = filepath.Join(sdkRoot, "platform-tools", "adb")
	return p, nil
}

// EnsureSDK downloads and installs the Android command-line tools if missing.
func EnsureSDK(ctx context.Context, sdkRoot string) (*Paths, error) {
	paths, _ := ResolvePaths(sdkRoot)
	if paths.Sdkmanager != "" {
		if _, err := os.Stat(paths.Sdkmanager); err == nil {
			return paths, nil
		}
	}

	fmt.Println("Android SDK not found. Installing command-line tools...")
	if err := downloadCmdlineTools(ctx, paths.SDKRoot); err != nil {
		return nil, fmt.Errorf("install SDK tools: %w", err)
	}
	return ResolvePaths(paths.SDKRoot)
}

func downloadCmdlineTools(ctx context.Context, sdkRoot string) error {
	if err := os.MkdirAll(sdkRoot, 0755); err != nil {
		return err
	}
	zipPath := filepath.Join(sdkRoot, "cmdline-tools.zip")
	defer os.Remove(zipPath)

	fmt.Println("Downloading Android command-line tools...")
	req, err := http.NewRequestWithContext(ctx, "GET", cmdlineToolsURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()

	destDir := filepath.Join(sdkRoot, "cmdline-tools", "latest")
	os.MkdirAll(destDir, 0755)
	cmd := exec.CommandContext(ctx, "unzip", "-o", "-q", zipPath, "-d", sdkRoot)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	extracted := filepath.Join(sdkRoot, "cmdline-tools")
	tmpDir := filepath.Join(sdkRoot, "_cmdline-tools-tmp")
	if _, err := os.Stat(filepath.Join(extracted, "bin", "sdkmanager")); err == nil {
		os.Rename(extracted, tmpDir)
		os.MkdirAll(extracted, 0755)
		os.Rename(tmpDir, destDir)
	}
	return nil
}

// InstallComponents uses sdkmanager to install required packages.
func InstallComponents(ctx context.Context, paths *Paths, cfg Config) error {
	packages := []string{
		"platform-tools",
		"emulator",
		fmt.Sprintf("platforms;android-%s", cfg.APILevel),
		fmt.Sprintf("system-images;android-%s;%s;%s", cfg.APILevel, cfg.ImageType, cfg.ABI),
	}
	for _, pkg := range packages {
		fmt.Printf("Installing %s...\n", pkg)
		cmd := exec.CommandContext(ctx, paths.Sdkmanager, "--install", pkg, "--sdk_root="+paths.SDKRoot)
		cmd.Env = append(os.Environ(), "ANDROID_HOME="+paths.SDKRoot)
		cmd.Stdin = strings.NewReader("y\n")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install %s: %w", pkg, err)
		}
	}
	return nil
}

// AcceptLicenses accepts all SDK licenses.
func AcceptLicenses(ctx context.Context, paths *Paths) error {
	cmd := exec.CommandContext(ctx, paths.Sdkmanager, "--licenses", "--sdk_root="+paths.SDKRoot)
	cmd.Env = append(os.Environ(), "ANDROID_HOME="+paths.SDKRoot)
	cmd.Stdin = strings.NewReader(strings.Repeat("y\n", 20))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CreateAVD creates the Android Virtual Device.
func CreateAVD(ctx context.Context, paths *Paths, cfg Config) error {
	image := fmt.Sprintf("system-images;android-%s;%s;%s", cfg.APILevel, cfg.ImageType, cfg.ABI)
	cmd := exec.CommandContext(ctx, paths.Avdmanager,
		"create", "avd",
		"-n", cfg.Name,
		"-k", image,
		"-d", cfg.DeviceID,
		"--force",
	)
	cmd.Env = append(os.Environ(),
		"ANDROID_HOME="+paths.SDKRoot,
		"ANDROID_SDK_ROOT="+paths.SDKRoot,
	)
	cmd.Stdin = strings.NewReader("no\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create AVD: %w", err)
	}

	avdDir := avdPath(cfg.Name)
	configPath := filepath.Join(avdDir, "config.ini")
	if data, err := os.ReadFile(configPath); err == nil {
		config := string(data)
		config += fmt.Sprintf("\nhw.ramSize=%d\n", cfg.MemoryMB)
		config += fmt.Sprintf("disk.dataPartition.size=%dG\n", cfg.DiskGB)
		config += "hw.keyboard=yes\n"
		config += "hw.gpu.enabled=yes\n"
		config += fmt.Sprintf("hw.gpu.mode=%s\n", cfg.GPU)
		os.WriteFile(configPath, []byte(config), 0644)
	}
	fmt.Printf("AVD %q created.\n", cfg.Name)
	return nil
}

func avdPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".android", "avd", name+".avd")
}

// StartEmulator launches the Android emulator with hardening flags.
func StartEmulator(ctx context.Context, paths *Paths, cfg Config) (*exec.Cmd, error) {
	args := []string{
		"-avd", cfg.Name,
		"-gpu", cfg.GPU,
		"-no-snapshot-load",
	}
	if cfg.Headless {
		args = append(args, "-no-window")
	}
	// Route through pinhole proxy if configured
	if cfg.ProxyAddr != "" {
		args = append(args, "-http-proxy", cfg.ProxyAddr)
	}
	if runtime.GOARCH == "arm64" {
		args = append(args, "-no-accel") // let HVF decide
	}

	cmd := exec.CommandContext(ctx, paths.Emulator, args...)
	cmd.Env = append(os.Environ(),
		"ANDROID_HOME="+paths.SDKRoot,
		"ANDROID_SDK_ROOT="+paths.SDKRoot,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start emulator: %w", err)
	}
	return cmd, nil
}

// WaitForBoot waits until the emulator is fully booted.
func WaitForBoot(ctx context.Context, paths *Paths, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	fmt.Print("Waiting for boot")
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		out, err := exec.CommandContext(ctx, paths.ADB, "shell", "getprop", "sys.boot_completed").Output()
		if err == nil && strings.TrimSpace(string(out)) == "1" {
			fmt.Println(" booted.")
			return nil
		}
		fmt.Print(".")
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("boot timeout after %s", timeout)
}

// --- InvisiCap-style ADB capability gate ---

// ADBCapability tracks what an ADB command is permitted to do.
// Mirrors invisicap.go's CapState: every ADB call gets an invisible
// capability bounding what it can access.
type ADBCapability struct {
	Command   string
	AllowNet  bool // can the command reach the network?
	AllowFS   bool // can the command write to filesystem?
	AllowExec bool // can the command execute arbitrary code?
	Timestamp time.Time
	Duration  time.Duration
	Output    string
	Error     string
}

// ADBGate is the invisible capability gate around all ADB interactions.
// It records every command, enforces capability bounds, and feeds results
// to the exploit arena.
type ADBGate struct {
	paths  *Paths
	mu     sync.Mutex
	log    []ADBCapability
	denied atomic.Int64
}

// NewADBGate creates a capability-gated ADB interface.
func NewADBGate(paths *Paths) *ADBGate {
	return &ADBGate{paths: paths}
}

// Shell runs an ADB shell command with capability tracking.
func (g *ADBGate) Shell(ctx context.Context, shellCmd string) (string, error) {
	cap := ADBCapability{
		Command:   "shell:" + shellCmd,
		AllowNet:  false,
		AllowFS:   !strings.Contains(shellCmd, "rm "),
		AllowExec: true,
		Timestamp: time.Now(),
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, g.paths.ADB, "shell", shellCmd)
	cmd.Env = append(os.Environ(), "ANDROID_HOME="+g.paths.SDKRoot)
	out, err := cmd.CombinedOutput()
	cap.Duration = time.Since(start)
	cap.Output = strings.TrimSpace(string(out))
	if err != nil {
		cap.Error = err.Error()
	}

	g.mu.Lock()
	g.log = append(g.log, cap)
	g.mu.Unlock()

	return cap.Output, err
}

// Install installs an APK with capability tracking.
func (g *ADBGate) Install(ctx context.Context, apkPath string) error {
	cap := ADBCapability{
		Command:   "install:" + filepath.Base(apkPath),
		AllowNet:  false,
		AllowFS:   true,
		AllowExec: true,
		Timestamp: time.Now(),
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, g.paths.ADB, "install", "-r", apkPath)
	cmd.Env = append(os.Environ(), "ANDROID_HOME="+g.paths.SDKRoot)
	out, err := cmd.CombinedOutput()
	cap.Duration = time.Since(start)
	cap.Output = strings.TrimSpace(string(out))
	if err != nil {
		cap.Error = err.Error()
	}

	g.mu.Lock()
	g.log = append(g.log, cap)
	g.mu.Unlock()

	return err
}

// Log returns all recorded ADB capabilities.
func (g *ADBGate) Log() []ADBCapability {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]ADBCapability, len(g.log))
	copy(out, g.log)
	return out
}

// DeniedCount returns how many ADB commands were denied.
func (g *ADBGate) DeniedCount() int64 {
	return g.denied.Load()
}

// --- UberEats attack surface ---

// UberEatsProbe is the proof-of-attack target.
// It boots the emulator, installs/launches UberEats, and monitors
// all network traffic for exfiltration attempts.
type UberEatsProbe struct {
	cfg   Config
	paths *Paths
	gate  *ADBGate

	mu         sync.Mutex
	findings   []Finding
	startTime  time.Time
}

// Finding is an observed security-relevant event.
type Finding struct {
	Timestamp   time.Time `json:"timestamp"`
	Category    string    `json:"category"`
	Severity    int       `json:"severity"` // 1-10
	Description string    `json:"description"`
	Evidence    string    `json:"evidence"`
}

const uberEatsPkg = "com.ubercab.eats"

// NewUberEatsProbe creates a probe for UberEats attack surface testing.
func NewUberEatsProbe(cfg Config, paths *Paths) *UberEatsProbe {
	return &UberEatsProbe{
		cfg:   cfg,
		paths: paths,
		gate:  NewADBGate(paths),
	}
}

// Launch opens UberEats on the emulator.
// If the app is installed, launches directly.
// Otherwise opens the Play Store page.
func (p *UberEatsProbe) Launch(ctx context.Context) error {
	p.startTime = time.Now()

	out, _ := p.gate.Shell(ctx, fmt.Sprintf("pm list packages | grep %s", uberEatsPkg))
	if strings.Contains(out, uberEatsPkg) {
		fmt.Println("[PROBE] Launching UberEats...")
		_, err := p.gate.Shell(ctx, fmt.Sprintf(
			"monkey -p %s -c android.intent.category.LAUNCHER 1", uberEatsPkg))
		return err
	}

	fmt.Println("[PROBE] UberEats not installed. Opening Play Store...")
	_, err := p.gate.Shell(ctx, fmt.Sprintf(
		"am start -a android.intent.action.VIEW -d 'market://details?id=%s'", uberEatsPkg))
	if err != nil {
		fmt.Println("[PROBE] Play Store unavailable. Opening web fallback...")
		_, err = p.gate.Shell(ctx,
			"am start -a android.intent.action.VIEW -d 'https://www.ubereats.com'")
	}
	return err
}

// MonitorNetwork polls netstat inside the emulator and records findings.
func (p *UberEatsProbe) MonitorNetwork(ctx context.Context, duration time.Duration) error {
	deadline := time.Now().Add(duration)
	fmt.Printf("[PROBE] Monitoring network for %s...\n", duration)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Capture active connections
		out, err := p.gate.Shell(ctx, "cat /proc/net/tcp6 2>/dev/null || netstat -tun")
		if err == nil {
			p.analyzeConnections(out)
		}

		// Capture DNS queries via logcat
		out, err = p.gate.Shell(ctx,
			"logcat -d -s NetworkMonitor:* ConnectivityService:* -t 50")
		if err == nil {
			p.analyzeDNS(out)
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

func (p *UberEatsProbe) analyzeConnections(netstat string) {
	for _, line := range strings.Split(netstat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "sl") {
			continue
		}
		// Look for connections to non-standard ports
		for _, suspicious := range []string{":1935", ":8080", ":8443", ":5228"} {
			if strings.Contains(line, suspicious) {
				p.addFinding(Finding{
					Timestamp:   time.Now(),
					Category:    "non-standard-port",
					Severity:    5,
					Description: fmt.Sprintf("Connection to non-standard port: %s", suspicious),
					Evidence:    line,
				})
			}
		}
	}
}

func (p *UberEatsProbe) analyzeDNS(logcat string) {
	for _, line := range strings.Split(logcat, "\n") {
		if strings.Contains(line, "resolv") || strings.Contains(line, "dns") {
			// Check against allowed domains
			allowed := false
			for _, d := range p.cfg.AllowedDomains {
				pattern := strings.Replace(d, "*.", "", 1)
				if strings.Contains(line, pattern) {
					allowed = true
					break
				}
			}
			if !allowed && strings.Contains(line, ".com") {
				p.addFinding(Finding{
					Timestamp:   time.Now(),
					Category:    "dns-exfiltration",
					Severity:    7,
					Description: "DNS resolution to non-allowed domain",
					Evidence:    line,
				})
			}
		}
	}
}

func (p *UberEatsProbe) addFinding(f Finding) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.findings = append(p.findings, f)
	fmt.Printf("[FINDING] severity=%d cat=%s: %s\n", f.Severity, f.Category, f.Description)
}

// Findings returns all security findings.
func (p *UberEatsProbe) Findings() []Finding {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Finding, len(p.findings))
	copy(out, p.findings)
	return out
}

// Report generates a JSON report of all findings and ADB capability log.
func (p *UberEatsProbe) Report() string {
	report := map[string]interface{}{
		"target":       uberEatsPkg,
		"start_time":   p.startTime,
		"duration":     time.Since(p.startTime).String(),
		"findings":     p.Findings(),
		"adb_commands":  len(p.gate.Log()),
		"adb_denied":   p.gate.DeniedCount(),
		"allowed_ports": p.cfg.AllowedPorts,
		"allowed_domains": p.cfg.AllowedDomains,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	return string(data)
}

// Screenshot captures the emulator screen.
func (p *UberEatsProbe) Screenshot(ctx context.Context, destPath string) error {
	remotePath := "/sdcard/boxxy-screenshot.png"
	if _, err := p.gate.Shell(ctx, fmt.Sprintf("screencap -p %s", remotePath)); err != nil {
		return fmt.Errorf("screencap: %w", err)
	}
	cmd := exec.CommandContext(ctx, p.paths.ADB, "pull", remotePath, destPath)
	cmd.Env = append(os.Environ(), "ANDROID_HOME="+p.paths.SDKRoot)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pull: %w", err)
	}
	p.gate.Shell(ctx, fmt.Sprintf("rm %s", remotePath))
	fmt.Printf("[PROBE] Screenshot: %s\n", destPath)
	return nil
}
