//go:build darwin && arm64

package vm

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// MacOSState represents lifecycle state
type MacOSState int

const (
	MacOSStateNone      MacOSState = iota // no VM
	MacOSStateCreated                     // IPSW + hardware model saved, not installed
	MacOSStateInstalled                   // ready to boot
	MacOSStateRunning                     // VM running
)

// MacOSLifecycle manages create → install → boot → ssh → stop
type MacOSLifecycle struct {
	Name     string
	CPUs     int
	MemoryGB int
	DiskGB   int
	IPSWPath string // optional local IPSW
	Paths    MacOSVMPaths
	instance *VMInstance
}

func NewMacOSLifecycle(name string, cpus, memGB, diskGB int) *MacOSLifecycle {
	return &MacOSLifecycle{
		Name:     name,
		CPUs:     cpus,
		MemoryGB: memGB,
		DiskGB:   diskGB,
		Paths:    DefaultMacOSPaths(name),
	}
}

func (m *MacOSLifecycle) installedMarkerPath() string {
	return filepath.Join(m.Paths.BaseDir, ".installed")
}

// State checks filesystem to determine current lifecycle state
func (m *MacOSLifecycle) State() MacOSState {
	if _, err := os.Stat(m.Paths.HardwareModel); os.IsNotExist(err) {
		return MacOSStateNone
	}
	if _, err := os.Stat(m.installedMarkerPath()); os.IsNotExist(err) {
		return MacOSStateCreated
	}
	return MacOSStateInstalled
}

func (m *MacOSLifecycle) StateString() string {
	switch m.State() {
	case MacOSStateNone:
		return "none"
	case MacOSStateCreated:
		return "created (needs install)"
	case MacOSStateInstalled:
		return "installed (ready to boot)"
	case MacOSStateRunning:
		return "running"
	default:
		return "unknown"
	}
}

// Create downloads IPSW and creates the VM
func (m *MacOSLifecycle) Create(ctx context.Context) error {
	if m.State() >= MacOSStateCreated {
		return fmt.Errorf("VM %q already created (%s)", m.Name, m.StateString())
	}

	// Link local IPSW if provided
	if m.IPSWPath != "" {
		if err := os.MkdirAll(m.Paths.BaseDir, 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
		if err := os.Symlink(m.IPSWPath, m.Paths.RestoreImage); err != nil {
			return fmt.Errorf("link IPSW: %w", err)
		}
		fmt.Printf("Using IPSW: %s\n", m.IPSWPath)
	}

	cfg := MacOSVMConfig{
		Name:     m.Name,
		CPUs:     m.CPUs,
		MemoryGB: m.MemoryGB,
		DiskGB:   m.DiskGB,
		Paths:    m.Paths,
	}

	var instance *VMInstance
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		instance, err = CreateMacOSVM(ctx, cfg)
		if err == nil {
			break
		}
		if attempt < 3 {
			fmt.Printf("Attempt %d failed: %v\nRetrying...\n", attempt, err)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("failed after 3 attempts: %w\n\nTip: boxxy macos up --ipsw /path/to/restore.ipsw", err)
	}
	m.instance = instance
	return nil
}

// Install runs the macOS installer
func (m *MacOSLifecycle) Install(ctx context.Context) error {
	if m.instance == nil {
		cfg := MacOSVMConfig{
			Name: m.Name, CPUs: m.CPUs, MemoryGB: m.MemoryGB, DiskGB: m.DiskGB, Paths: m.Paths,
		}
		instance, err := CreateMacOSVM(ctx, cfg)
		if err != nil {
			return fmt.Errorf("create for install: %w", err)
		}
		m.instance = instance
	}
	if err := InstallMacOS(ctx, m.instance.VM, m.Paths.RestoreImage); err != nil {
		return err
	}
	return os.WriteFile(m.installedMarkerPath(), []byte("ok\n"), 0644)
}

// Boot starts the VM
func (m *MacOSLifecycle) Boot() error {
	if m.State() < MacOSStateInstalled {
		return fmt.Errorf("VM %q not installed (%s)", m.Name, m.StateString())
	}
	if m.instance == nil {
		cfg := MacOSVMConfig{
			Name: m.Name, CPUs: m.CPUs, MemoryGB: m.MemoryGB, DiskGB: m.DiskGB, Paths: m.Paths,
		}
		instance, err := LoadMacOSVM(cfg)
		if err != nil {
			return fmt.Errorf("load VM: %w", err)
		}
		m.instance = instance
	}
	RegisterVM(m.Name, m.instance)
	return StartVM(m.instance)
}

func (m *MacOSLifecycle) Instance() *VMInstance {
	return m.instance
}

// Summary returns a status string
func (m *MacOSLifecycle) Summary() string {
	s := fmt.Sprintf("VM: %s\nState: %s\nConfig: %d CPU, %d GB RAM, %d GB disk\nPath: %s\n",
		m.Name, m.StateString(), m.CPUs, m.MemoryGB, m.DiskGB, m.Paths.BaseDir)
	if info, err := os.Stat(m.Paths.RestoreImage); err == nil {
		s += fmt.Sprintf("IPSW: %.1f GB\n", float64(info.Size())/1e9)
	}
	if info, err := os.Stat(m.Paths.DiskImage); err == nil {
		s += fmt.Sprintf("Disk: %.1f GB\n", float64(info.Size())/1e9)
	}
	return s
}

// WaitForSSH polls until SSH is reachable
func WaitForSSH(ip string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "22"), 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("SSH not reachable on %s:22 after %s", ip, timeout)
}

// RunSSH executes an SSH command on the guest
func RunSSH(ip, user string, args []string) (int, error) {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "LogLevel=ERROR",
		fmt.Sprintf("%s@%s", user, ip),
	}
	sshArgs = append(sshArgs, args...)

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

// WritePIDFile writes current PID to ~/.boxxy/vm.pid
func WritePIDFile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".boxxy")
	os.MkdirAll(dir, 0755)
	return os.WriteFile(filepath.Join(dir, "vm.pid"), []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

func RemovePIDFile() {
	home, _ := os.UserHomeDir()
	os.Remove(filepath.Join(home, ".boxxy", "vm.pid"))
}
