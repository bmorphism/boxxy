//go:build darwin

package runner

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/vm"
)

type RunConfig struct {
	BootMode string // efi, linux, macos
	Kernel   string
	Initrd   string
	Cmdline  string
	ISO      string
	Disk     string
	Memory   int
	CPUs     int
	NVRAM    string
}

func Run(args []string) error {
	cfg := &RunConfig{
		Memory: 4,
		CPUs:   2,
		NVRAM:  "boxxy-nvram",
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	efi := fs.Bool("efi", false, "Use EFI boot")
	linux := fs.Bool("linux", false, "Use Linux boot")
	macos := fs.Bool("macos", false, "Use macOS boot")
	fs.StringVar(&cfg.Kernel, "kernel", "", "Linux kernel path")
	fs.StringVar(&cfg.Initrd, "initrd", "", "Linux initrd path")
	fs.StringVar(&cfg.Cmdline, "cmdline", "console=hvc0", "Linux kernel cmdline")
	fs.StringVar(&cfg.ISO, "iso", "", "ISO image path")
	fs.StringVar(&cfg.Disk, "disk", "", "Disk image path")
	fs.IntVar(&cfg.Memory, "memory", 4, "Memory in GB")
	fs.IntVar(&cfg.CPUs, "cpus", 2, "CPU count")
	fs.StringVar(&cfg.NVRAM, "nvram", "boxxy-nvram", "EFI NVRAM path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Determine boot mode
	switch {
	case *efi:
		cfg.BootMode = "efi"
	case *linux:
		cfg.BootMode = "linux"
	case *macos:
		cfg.BootMode = "macos"
	default:
		return fmt.Errorf("must specify --efi, --linux, or --macos")
	}

	return runVM(cfg)
}

func runVM(cfg *RunConfig) error {
	fmt.Printf("Starting VM: mode=%s cpus=%d memory=%dGB\n", 
		cfg.BootMode, cfg.CPUs, cfg.Memory)

	vmInstance, err := vm.CreateVM(vm.Config{
		BootMode: cfg.BootMode,
		Kernel:   cfg.Kernel,
		Initrd:   cfg.Initrd,
		Cmdline:  cfg.Cmdline,
		ISO:      cfg.ISO,
		Disk:     cfg.Disk,
		Memory:   cfg.Memory,
		CPUs:     cfg.CPUs,
		NVRAM:    cfg.NVRAM,
	})
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	fmt.Println("Starting VM... (Ctrl+C to stop)")
	if err := vm.StartVM(vmInstance); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	// Wait for interrupt
	vm.WaitForShutdown(vmInstance)
	return nil
}

func RunScript(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	// Initialize environment
	env := lisp.CreateStandardEnv()
	vm.RegisterNamespace(env)

	reader := lisp.NewReader(strings.NewReader(string(content)))
	exprs, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	for _, expr := range exprs {
		lisp.Eval(expr, env)
	}

	return nil
}
