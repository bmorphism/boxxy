//go:build darwin

package runner

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/tape"
	"github.com/bmorphism/boxxy/internal/vm"
)

type RunConfig struct {
	BootMode       string // efi, linux, macos
	Kernel         string
	Initrd         string
	Cmdline        string
	ISO            string
	Disk           string
	Memory         int
	CPUs           int
	NVRAM          string
	DisableNetwork bool
	EnableRosetta  bool
	RosettaTag     string
}

func Run(args []string) error {
	cfg, err := parseRunArgs(args)
	if err != nil {
		return err
	}
	return runVM(cfg)
}

func runVM(cfg *RunConfig) error {
	fmt.Printf("Starting VM: mode=%s cpus=%d memory=%dGB\n",
		cfg.BootMode, cfg.CPUs, cfg.Memory)

	vmInstance, err := vm.CreateVM(vm.Config{
		BootMode:       cfg.BootMode,
		Kernel:         cfg.Kernel,
		Initrd:         cfg.Initrd,
		Cmdline:        cfg.Cmdline,
		ISO:            cfg.ISO,
		Disk:           cfg.Disk,
		Memory:         cfg.Memory,
		CPUs:           cfg.CPUs,
		NVRAM:          cfg.NVRAM,
		DisableNetwork: cfg.DisableNetwork,
		EnableRosetta:  cfg.EnableRosetta,
		RosettaTag:     cfg.RosettaTag,
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

func parseRunArgs(args []string) (*RunConfig, error) {
	cfg := &RunConfig{
		Memory: 4,
		CPUs:   2,
		NVRAM:  "boxxy-nvram",
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	efi := fs.Bool("efi", false, "Use EFI boot")
	linux := fs.Bool("linux", false, "Use Linux boot")
	macos := fs.Bool("macos", false, "Use macOS boot")
	guix := fs.Bool("guix", false, "Use Guix (auto-select EFI or Linux boot)")
	fs.StringVar(&cfg.Kernel, "kernel", "", "Linux kernel path")
	fs.StringVar(&cfg.Initrd, "initrd", "", "Linux initrd path")
	fs.StringVar(&cfg.Cmdline, "cmdline", "console=hvc0", "Linux kernel cmdline")
	fs.StringVar(&cfg.ISO, "iso", "", "ISO image path")
	fs.StringVar(&cfg.Disk, "disk", "", "Disk image path")
	fs.IntVar(&cfg.Memory, "memory", 4, "Memory in GB")
	fs.IntVar(&cfg.CPUs, "cpus", 2, "CPU count")
	fs.StringVar(&cfg.NVRAM, "nvram", "boxxy-nvram", "EFI NVRAM path")
	fs.BoolVar(&cfg.DisableNetwork, "hardened", false, "Disable networking for stronger sandboxing")
	fs.BoolVar(&cfg.EnableRosetta, "rosetta", false, "Enable Rosetta for Linux x86_64 binaries (Apple Silicon)")
	fs.StringVar(&cfg.RosettaTag, "rosetta-tag", "rosetta", "VirtioFS tag for Rosetta directory share")
	guixArch := fs.String("guix-arch", "aarch64", "Guix architecture: aarch64 or x86_64 (x86_64 requires --rosetta)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	switch {
	case *guix:
		if *efi {
			cfg.BootMode = "efi"
		} else if *linux {
			cfg.BootMode = "linux"
		} else if cfg.Kernel != "" || cfg.Initrd != "" {
			cfg.BootMode = "linux"
		} else if cfg.ISO != "" {
			cfg.BootMode = "efi"
		} else {
			return nil, fmt.Errorf("guix mode requires --iso or --kernel/--initrd")
		}
	case *efi:
		cfg.BootMode = "efi"
	case *linux:
		cfg.BootMode = "linux"
	case *macos:
		cfg.BootMode = "macos"
	default:
		return nil, fmt.Errorf("must specify --efi, --linux, --macos, or --guix")
	}

	if *guix {
		switch strings.ToLower(*guixArch) {
		case "aarch64", "arm64":
			// default
		case "x86_64", "amd64":
			cfg.EnableRosetta = true
		default:
			return nil, fmt.Errorf("unsupported guix architecture: %s", *guixArch)
		}
	}

	return cfg, nil
}

func RunScript(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	// Initialize environment with all namespaces
	env := lisp.CreateStandardEnv()
	vm.RegisterNamespace(env)
	tape.RegisterNamespace(env)
	tape.RegisterEvolveNamespace(env)
	tape.RegisterDaemonNamespace(env)

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
