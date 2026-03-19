package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Code-Hex/vz/v3"
	"github.com/bmorphism/boxxy/internal/vm"
)

type sel4Options struct {
	Mode        string
	Kernel      string
	Disk        string
	NVRAM       string
	Cmdline     string
	CPUs        int
	MemoryGB    int
	Hardened    bool
	GUI         bool
	Width       int
	Height      int
	WindowTitle string
}

func runSel4(args []string) {
	mode := "direct"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		mode = args[0]
		args = args[1:]
	}

	opts := defaultSel4Options(mode)
	fs := newSel4FlagSet(mode, &opts)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	switch mode {
	case "direct", "efi":
		if err := runSel4VM(opts); err != nil {
			fmt.Fprintf(os.Stderr, "boxxy sel4: %v\n", err)
			os.Exit(1)
		}
	case "joke":
		fmt.Print(renderSel4Joke(opts))
	default:
		fmt.Fprintf(os.Stderr, "unknown sel4 subcommand: %s\n\n", mode)
		printSel4Usage()
		os.Exit(2)
	}
}

func defaultSel4Options(mode string) sel4Options {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".boxxy", "sel4")

	return sel4Options{
		Mode:        mode,
		Kernel:      firstExistingSel4(filepath.Join(home, "projects", "xmonad-sel4", "seL4", "build", "root-task.elf"), filepath.Join(home, "projects", "seL4", "build", "root-task.elf")),
		Disk:        firstExistingSel4(filepath.Join(home, "projects", "xmonad-sel4", "seL4", "build", "sel4-arm64.img"), filepath.Join(home, "projects", "seL4", "build", "sel4-arm64.img")),
		NVRAM:       filepath.Join(baseDir, "sel4.nvram"),
		Cmdline:     "console=hvc0",
		CPUs:        4,
		MemoryGB:    4,
		Hardened:    false,
		GUI:         false,
		Width:       1440,
		Height:      900,
		WindowTitle: "seL4 - boxxy",
	}
}

func newSel4FlagSet(mode string, opts *sel4Options) *flag.FlagSet {
	fs := flag.NewFlagSet("sel4 "+mode, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&opts.Kernel, "kernel", opts.Kernel, "Path to seL4 kernel/root task ELF")
	fs.StringVar(&opts.Disk, "disk", opts.Disk, "Path to seL4 disk image")
	fs.StringVar(&opts.NVRAM, "nvram", opts.NVRAM, "Path to EFI variable store")
	fs.StringVar(&opts.Cmdline, "cmdline", opts.Cmdline, "Kernel command line for direct boot")
	fs.IntVar(&opts.CPUs, "cpus", opts.CPUs, "CPU count")
	fs.IntVar(&opts.MemoryGB, "memory", opts.MemoryGB, "Memory in GB")
	fs.BoolVar(&opts.Hardened, "hardened", opts.Hardened, "Disable networking")
	fs.BoolVar(&opts.GUI, "gui", opts.GUI, "Attach a graphics window")
	fs.IntVar(&opts.Width, "width", opts.Width, "Graphics width")
	fs.IntVar(&opts.Height, "height", opts.Height, "Graphics height")
	fs.StringVar(&opts.WindowTitle, "title", opts.WindowTitle, "Window title when --gui is enabled")
	fs.Usage = printSel4Usage

	return fs
}

func printSel4Usage() {
	fmt.Fprintf(os.Stderr, `boxxy sel4 — first-class seL4 guest on Apple Virtualization.framework

Usage:
  boxxy sel4 direct [options]   Boot seL4 via direct kernel loading
  boxxy sel4 efi [options]      Boot seL4 via EFI and disk image
  boxxy sel4 joke [options]     Print the equivalent canonical .joke script

Defaults:
  boxxy sel4                    Same as "boxxy sel4 direct"

Options:
  --kernel PATH   seL4 kernel/root-task ELF
  --disk PATH     seL4 disk image
  --nvram PATH    EFI variable store path
  --cmdline STR   Direct-boot kernel command line
  --cpus N        CPU count
  --memory N      Memory in GB
  --gui           Attach a graphics window
  --width N       Graphics width
  --height N      Graphics height
  --title STR     Window title
  --hardened      Disable networking

Examples:
  boxxy sel4
  boxxy sel4 direct --kernel ~/projects/xmonad-sel4/seL4/build/root-task.elf
  boxxy sel4 efi --disk ~/projects/xmonad-sel4/seL4/build/sel4-arm64.img
  boxxy sel4 joke --kernel ~/projects/xmonad-sel4/seL4/build/root-task.elf > sel4-vm.joke
`)
}

func runSel4VM(opts sel4Options) error {
	if opts.CPUs <= 0 {
		return fmt.Errorf("--cpus must be > 0")
	}
	if opts.MemoryGB <= 0 {
		return fmt.Errorf("--memory must be > 0")
	}
	if opts.GUI && (opts.Width <= 0 || opts.Height <= 0) {
		return fmt.Errorf("--width and --height must be > 0 when --gui is enabled")
	}

	cfg := vm.Config{
		CPUs:           opts.CPUs,
		Memory:         opts.MemoryGB,
		DisableNetwork: opts.Hardened,
		Graphics:       opts.GUI,
		Width:          opts.Width,
		Height:         opts.Height,
		Keyboard:       opts.GUI,
		Pointer:        opts.GUI,
	}

	switch opts.Mode {
	case "direct":
		if opts.Kernel == "" {
			return fmt.Errorf("missing --kernel; point to a seL4 root-task.elf")
		}
		cfg.BootMode = "linux"
		cfg.Kernel = opts.Kernel
		cfg.Cmdline = opts.Cmdline
		if opts.Disk != "" {
			if _, err := os.Stat(opts.Disk); err == nil {
				cfg.Disk = opts.Disk
			}
		}
	case "efi":
		if opts.Disk == "" {
			return fmt.Errorf("missing --disk; EFI boot requires a seL4 disk image")
		}
		if _, err := os.Stat(opts.Disk); err != nil {
			return fmt.Errorf("disk image does not exist: %s", opts.Disk)
		}
		if err := os.MkdirAll(filepath.Dir(opts.NVRAM), 0755); err != nil {
			return fmt.Errorf("prepare nvram directory: %w", err)
		}
		cfg.BootMode = "efi"
		cfg.Disk = opts.Disk
		cfg.NVRAM = opts.NVRAM
	default:
		return fmt.Errorf("unsupported sel4 mode: %s", opts.Mode)
	}

	instance, err := vm.CreateVM(cfg)
	if err != nil {
		return err
	}

	if err := vm.WritePIDFile(); err == nil {
		defer vm.RemovePIDFile()
	}

	if err := vm.StartVM(instance); err != nil {
		return err
	}

	if opts.GUI {
		fmt.Printf("Starting seL4 VM with GUI: %s\n", opts.WindowTitle)
		if opts.Mode == "direct" {
			fmt.Printf("  Kernel: %s\n", opts.Kernel)
		} else {
			fmt.Printf("  Disk:   %s\n", opts.Disk)
			fmt.Printf("  NVRAM:  %s\n", opts.NVRAM)
		}
		fmt.Printf("  CPUs:   %d\n", opts.CPUs)
		fmt.Printf("  Memory: %d GB\n", opts.MemoryGB)
		fmt.Printf("  Window: %dx%d\n", opts.Width, opts.Height)
		fmt.Println()
		instance.VM.StartGraphicApplication(
			float64(opts.Width),
			float64(opts.Height),
			vz.WithWindowTitle(opts.WindowTitle),
			vz.WithController(true),
		)
		stopGraphicVM(instance)
		return nil
	}

	if opts.Mode == "direct" {
		fmt.Printf("Starting seL4 VM via direct kernel boot\n  Kernel: %s\n", opts.Kernel)
		if cfg.Disk != "" {
			fmt.Printf("  Disk:   %s\n", cfg.Disk)
		}
		fmt.Println()
	} else {
		fmt.Printf("Starting seL4 VM via EFI boot\n  Disk:   %s\n  NVRAM:  %s\n\n", opts.Disk, opts.NVRAM)
	}
	fmt.Println("VM running. Press Ctrl+C to stop.")
	vm.WaitForShutdown(instance)
	return nil
}

func renderSel4Joke(opts sel4Options) string {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env boxxy\n")
	b.WriteString(";; sel4-vm.joke - generated by `boxxy sel4 joke`\n\n")
	fmt.Fprintf(&b, "(def boot-method %s)\n", sel4Keyword(opts.Mode))
	if opts.Kernel != "" {
		fmt.Fprintf(&b, "(def kernel-path %q)\n", opts.Kernel)
	}
	if opts.Disk != "" {
		fmt.Fprintf(&b, "(def disk-path %q)\n", opts.Disk)
	}
	fmt.Fprintf(&b, "(def nvram-path %q)\n", opts.NVRAM)
	fmt.Fprintf(&b, "(def vm-cpus %d)\n", opts.CPUs)
	fmt.Fprintf(&b, "(def vm-memory-gb %d)\n", opts.MemoryGB)
	fmt.Fprintf(&b, "(def kernel-cmdline %q)\n", opts.Cmdline)
	b.WriteString("\n")
	if opts.Mode == "efi" {
		b.WriteString("(def store (vz/new-efi-variable-store nvram-path true))\n")
		b.WriteString("(def boot (vz/new-efi-boot-loader store))\n")
	} else {
		b.WriteString("(def boot (vz/new-linux-boot-loader kernel-path nil kernel-cmdline))\n")
	}
	b.WriteString("(def platform (vz/new-generic-platform))\n")
	b.WriteString("(def vm-config (vz/new-vm-config vm-cpus vm-memory-gb boot platform))\n")
	b.WriteString("\n")
	if opts.Disk != "" {
		b.WriteString("(def disk-att (vz/new-disk-attachment disk-path false))\n")
		b.WriteString("(def disk-dev (vz/new-virtio-block-device disk-att))\n")
		b.WriteString("(vz/add-storage-devices vm-config [disk-dev])\n")
	}
	if opts.Hardened {
		b.WriteString(";; Hardened mode requested: omit network device\n")
	} else {
		b.WriteString("(def nat (vz/new-nat-network))\n")
		b.WriteString("(def net (vz/new-virtio-network-device nat))\n")
		b.WriteString("(vz/add-network-devices vm-config [net])\n")
	}
	return b.String()
}

func firstExistingSel4(paths ...string) string {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}

func sel4Keyword(mode string) string {
	switch mode {
	case "efi":
		return ":efi"
	default:
		return ":direct-kernel"
	}
}
