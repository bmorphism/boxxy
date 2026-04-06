//go:build darwin && arm64

package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// WindowsState represents Windows VM lifecycle state
type WindowsState int

const (
	WindowsStateNone      WindowsState = iota // no VM directory
	WindowsStateCreated                       // disk + NVRAM created, needs ISO install
	WindowsStateInstalled                     // installed, ready to boot
	WindowsStateRunning                       // VM running
)

// WindowsVMPaths holds all filesystem paths for a Windows VM
type WindowsVMPaths struct {
	BaseDir   string
	DiskImage string
	NVRAM     string
	ISOPath   string // optional, for install
}

func DefaultWindowsPaths(name string) WindowsVMPaths {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".boxxy", "windows", name)
	return WindowsVMPaths{
		BaseDir:   base,
		DiskImage: filepath.Join(base, "disk.img"),
		NVRAM:     filepath.Join(base, "nvram.bin"),
	}
}

// WindowsLifecycle manages create → install → boot → rdp → stop
type WindowsLifecycle struct {
	Name     string
	CPUs     int
	MemoryGB int
	DiskGB   int
	ISOPath  string // Windows ARM64 ISO
	Graphics bool
	Paths    WindowsVMPaths
	instance *VMInstance
}

func NewWindowsLifecycle(name string, cpus, memGB, diskGB int) *WindowsLifecycle {
	return &WindowsLifecycle{
		Name:     name,
		CPUs:     cpus,
		MemoryGB: memGB,
		DiskGB:   diskGB,
		Graphics: true,
		Paths:    DefaultWindowsPaths(name),
	}
}

func (w *WindowsLifecycle) installedMarkerPath() string {
	return filepath.Join(w.Paths.BaseDir, ".installed")
}

// State checks filesystem to determine current lifecycle state
func (w *WindowsLifecycle) State() WindowsState {
	if _, err := os.Stat(w.Paths.DiskImage); os.IsNotExist(err) {
		return WindowsStateNone
	}
	if _, err := os.Stat(w.installedMarkerPath()); os.IsNotExist(err) {
		return WindowsStateCreated
	}
	return WindowsStateInstalled
}

func (w *WindowsLifecycle) StateString() string {
	switch w.State() {
	case WindowsStateNone:
		return "none"
	case WindowsStateCreated:
		return "created (needs install from ISO)"
	case WindowsStateInstalled:
		return "installed (ready to boot)"
	case WindowsStateRunning:
		return "running"
	default:
		return "unknown"
	}
}

// Create creates the disk image and NVRAM for a Windows VM
func (w *WindowsLifecycle) Create(_ context.Context) error {
	if w.State() >= WindowsStateCreated {
		return fmt.Errorf("VM %q already created (%s)", w.Name, w.StateString())
	}

	if err := os.MkdirAll(w.Paths.BaseDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// Create sparse disk image
	f, err := os.Create(w.Paths.DiskImage)
	if err != nil {
		return fmt.Errorf("create disk: %w", err)
	}
	if err := f.Truncate(int64(w.DiskGB) * 1024 * 1024 * 1024); err != nil {
		f.Close()
		return fmt.Errorf("truncate disk: %w", err)
	}
	f.Close()
	fmt.Printf("Created %d GB disk at %s\n", w.DiskGB, w.Paths.DiskImage)

	// NVRAM will be created by EFI boot loader on first boot
	return nil
}

// Boot starts the Windows VM via EFI
func (w *WindowsLifecycle) Boot() error {
	if w.State() < WindowsStateCreated {
		return fmt.Errorf("VM %q not created (%s)", w.Name, w.StateString())
	}

	cfg := Config{
		BootMode: "efi",
		CPUs:     w.CPUs,
		Memory:   w.MemoryGB,
		Disk:     w.Paths.DiskImage,
		NVRAM:    w.Paths.NVRAM,
		Graphics: w.Graphics,
		Width:    1920,
		Height:   1080,
		Keyboard: true,
		Pointer:  true,
	}

	// Attach ISO if provided (for installation)
	if w.ISOPath != "" {
		cfg.ISO = w.ISOPath
	} else if w.Paths.ISOPath != "" {
		cfg.ISO = w.Paths.ISOPath
	}

	instance, err := CreateVM(cfg)
	if err != nil {
		return fmt.Errorf("create VM: %w", err)
	}
	w.instance = instance
	RegisterVM(w.Name, instance)
	return StartVM(instance)
}

// MarkInstalled marks the VM as installed (call after Windows setup completes)
func (w *WindowsLifecycle) MarkInstalled() error {
	return os.WriteFile(w.installedMarkerPath(), []byte("ok\n"), 0644)
}

func (w *WindowsLifecycle) Instance() *VMInstance {
	return w.instance
}

// Summary returns a status string
func (w *WindowsLifecycle) Summary() string {
	s := fmt.Sprintf("VM: %s (Windows ARM64)\nState: %s\nConfig: %d CPU, %d GB RAM, %d GB disk\nPath: %s\n",
		w.Name, w.StateString(), w.CPUs, w.MemoryGB, w.DiskGB, w.Paths.BaseDir)
	if info, err := os.Stat(w.Paths.DiskImage); err == nil {
		s += fmt.Sprintf("Disk: %.1f GB (%.1f GB allocated)\n",
			float64(w.DiskGB), float64(info.Size())/1e9)
	}
	if w.ISOPath != "" {
		s += fmt.Sprintf("ISO: %s\n", w.ISOPath)
	}
	return s
}

// RDP opens Microsoft Remote Desktop to the guest
func (w *WindowsLifecycle) RDP(ip string) error {
	// Use macOS `open` to launch RDP connection
	cmd := exec.Command("open", fmt.Sprintf("rdp://full%%20address=s:%s", ip))
	return cmd.Start()
}
