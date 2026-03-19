//go:build darwin && arm64

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/bmorphism/boxxy/internal/vm"
)

type haikuOptions struct {
	ISO         string
	Disk        string
	NVRAM       string
	CPUs        int
	MemoryGB    int
	DiskGB      int
	Width       int
	Height      int
	WindowTitle string
	Hardened    bool
	NoCreateDisk bool
}

func runHaiku(args []string) {
	mode := "gui"
	rest := args

	if len(args) > 0 {
		switch args[0] {
		case "gui", "headless", "joke", "help", "-h", "--help":
			mode = args[0]
			rest = args[1:]
		}
	}

	if mode == "help" || mode == "-h" || mode == "--help" {
		printHaikuUsage("gui")
		return
	}

	opts := defaultHaikuOptions()
	fs := newHaikuFlagSet(mode, &opts)
	if err := fs.Parse(rest); err != nil {
		os.Exit(2)
	}

	switch mode {
	case "gui":
		must(runHaikuVM(opts, true))
	case "headless":
		must(runHaikuVM(opts, false))
	case "joke":
		if opts.ISO == "" {
			fmt.Fprintln(os.Stderr, "error: --iso is required to render a Haiku joke script")
			os.Exit(1)
		}
		fmt.Print(renderHaikuJoke(opts))
	default:
		fmt.Fprintf(os.Stderr, "unknown haiku subcommand: %s\n\n", mode)
		printHaikuUsage("gui")
		os.Exit(1)
	}
}

func defaultHaikuOptions() haikuOptions {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".boxxy", "haiku")
	return haikuOptions{
		ISO:          defaultHaikuISO(home),
		Disk:         filepath.Join(baseDir, "haiku.img"),
		NVRAM:        filepath.Join(baseDir, "haiku.nvram"),
		CPUs:         4,
		MemoryGB:     8,
		DiskGB:       32,
		Width:        1920,
		Height:       1200,
		WindowTitle:  "HaikuOS - boxxy",
	}
}

func defaultHaikuISO(home string) string {
	candidates := []string{
		filepath.Join(home, "Downloads", "haiku", "haiku-r1beta5-x86_64-anyboot.iso"),
		filepath.Join(home, "Downloads", "haiku-r1beta5-x86_64-anyboot.iso"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func newHaikuFlagSet(mode string, opts *haikuOptions) *flag.FlagSet {
	fs := flag.NewFlagSet("haiku "+mode, flag.ContinueOnError)
	fs.StringVar(&opts.ISO, "iso", opts.ISO, "Haiku anyboot ISO path")
	fs.StringVar(&opts.Disk, "disk", opts.Disk, "Persistent VM disk image path")
	fs.StringVar(&opts.NVRAM, "nvram", opts.NVRAM, "EFI variable store path")
	fs.IntVar(&opts.CPUs, "cpus", opts.CPUs, "CPU count")
	fs.IntVar(&opts.MemoryGB, "memory", opts.MemoryGB, "Memory in GB")
	fs.IntVar(&opts.DiskGB, "disk-size", opts.DiskGB, "Disk size in GB when creating a new disk")
	fs.BoolVar(&opts.Hardened, "hardened", false, "Disable networking for stronger sandboxing")
	fs.BoolVar(&opts.NoCreateDisk, "no-create-disk", false, "Require an existing disk image instead of creating one")

	if mode == "gui" || mode == "joke" {
		fs.IntVar(&opts.Width, "width", opts.Width, "Window width in pixels")
		fs.IntVar(&opts.Height, "height", opts.Height, "Window height in pixels")
		fs.StringVar(&opts.WindowTitle, "title", opts.WindowTitle, "GUI window title")
	}

	fs.Usage = func() {
		printHaikuUsage(mode)
	}
	return fs
}

func printHaikuUsage(mode string) {
	fmt.Fprintf(os.Stderr, `boxxy haiku — first-class HaikuOS guest on Apple Virtualization.framework

Usage:
  boxxy haiku gui [options]       Boot HaikuOS with a graphics window
  boxxy haiku headless [options]  Boot HaikuOS without graphics
  boxxy haiku joke [options]      Print the equivalent canonical .joke script

Default:
  boxxy haiku                     Same as "boxxy haiku gui"

Common options:
  --iso PATH         Haiku anyboot ISO path
  --disk PATH        Persistent disk image path
  --nvram PATH       EFI variable store path
  --cpus N           CPU count (default: 4)
  --memory GB        Memory in GB (default: 8)
  --disk-size GB     Disk size when creating a new disk (default: 32)
  --hardened         Disable networking
  --no-create-disk   Require disk image to already exist

GUI-only options:
  --width PX         Window width (default: 1920)
  --height PX        Window height (default: 1200)
  --title STRING     Window title

Examples:
  boxxy haiku --iso ~/Downloads/haiku/haiku-r1beta5-x86_64-anyboot.iso
  boxxy haiku headless --iso ~/Downloads/haiku/haiku-r1beta5-x86_64-anyboot.iso
  boxxy haiku joke --iso ~/Downloads/haiku/haiku-r1beta5-x86_64-anyboot.iso > haiku-vm.joke
`)

	if mode != "gui" {
		fmt.Fprintf(os.Stderr, "\nCurrent subcommand: %s\n", mode)
	}
}

func runHaikuVM(opts haikuOptions, gui bool) error {
	if opts.ISO == "" {
		return fmt.Errorf("missing --iso; download a Haiku anyboot ISO first")
	}
	if opts.CPUs <= 0 {
		return fmt.Errorf("--cpus must be > 0")
	}
	if opts.MemoryGB <= 0 {
		return fmt.Errorf("--memory must be > 0")
	}
	if gui && (opts.Width <= 0 || opts.Height <= 0) {
		return fmt.Errorf("--width and --height must be > 0")
	}

	if err := os.MkdirAll(filepath.Dir(opts.NVRAM), 0755); err != nil {
		return fmt.Errorf("prepare nvram directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(opts.Disk), 0755); err != nil {
		return fmt.Errorf("prepare disk directory: %w", err)
	}

	if _, err := os.Stat(opts.Disk); err != nil {
		if os.IsNotExist(err) {
			if opts.NoCreateDisk {
				return fmt.Errorf("disk image does not exist: %s", opts.Disk)
			}
			if err := vz.CreateDiskImage(opts.Disk, int64(opts.DiskGB)*1024*1024*1024); err != nil {
				return fmt.Errorf("create disk image: %w", err)
			}
		} else {
			return fmt.Errorf("stat disk image: %w", err)
		}
	}

	cfg := vm.Config{
		BootMode:       "efi",
		ISO:            opts.ISO,
		Disk:           opts.Disk,
		NVRAM:          opts.NVRAM,
		Memory:         opts.MemoryGB,
		CPUs:           opts.CPUs,
		DisableNetwork: opts.Hardened,
		Graphics:       gui,
		Width:          opts.Width,
		Height:         opts.Height,
		Keyboard:       gui,
		Pointer:        gui,
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

	if gui {
		fmt.Printf("Starting HaikuOS VM with GUI: %s\n", opts.WindowTitle)
		fmt.Printf("  ISO:    %s\n", opts.ISO)
		fmt.Printf("  Disk:   %s\n", opts.Disk)
		fmt.Printf("  NVRAM:  %s\n", opts.NVRAM)
		fmt.Printf("  CPUs:   %d\n", opts.CPUs)
		fmt.Printf("  Memory: %d GB\n", opts.MemoryGB)
		fmt.Printf("  Window: %dx%d\n", opts.Width, opts.Height)
		fmt.Println()
		fmt.Println("Close the window or press Ctrl+C to stop.")
		instance.VM.StartGraphicApplication(
			float64(opts.Width),
			float64(opts.Height),
			vz.WithWindowTitle(opts.WindowTitle),
			vz.WithController(true),
		)
		stopGraphicVM(instance)
		return nil
	}

	fmt.Printf("Starting HaikuOS VM headless\n  ISO:    %s\n  Disk:   %s\n  NVRAM:  %s\n\n", opts.ISO, opts.Disk, opts.NVRAM)
	fmt.Println("VM running. Press Ctrl+C to stop.")
	vm.WaitForShutdown(instance)
	return nil
}

func stopGraphicVM(instance *vm.VMInstance) {
	for i := 1; instance.VM.CanRequestStop(); i++ {
		result, err := instance.VM.RequestStop()
		log.Printf("Sent stop request(%d): %t, %v", i, result, err)
		time.Sleep(3 * time.Second)
		if i > 3 {
			log.Println("Force stopping VM...")
			if err := vm.StopVM(instance); err != nil {
				log.Println("Stop error:", err)
			}
			break
		}
	}
}

func renderHaikuJoke(opts haikuOptions) string {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env boxxy\n")
	b.WriteString(";; haiku-vm.joke - generated by `boxxy haiku joke`\n\n")
	fmt.Fprintf(&b, "(def iso-path %q)\n", opts.ISO)
	fmt.Fprintf(&b, "(def disk-path %q)\n", opts.Disk)
	fmt.Fprintf(&b, "(def nvram-path %q)\n", opts.NVRAM)
	fmt.Fprintf(&b, "(def cpus %d)\n", opts.CPUs)
	fmt.Fprintf(&b, "(def memory-gb %d)\n", opts.MemoryGB)
	b.WriteString("\n")
	b.WriteString("(def store (vz/new-efi-variable-store nvram-path true))\n")
	b.WriteString("(def boot (vz/new-efi-boot-loader store))\n")
	b.WriteString("(def platform (vz/new-generic-platform))\n")
	b.WriteString("(def vm-config (vz/new-vm-config cpus memory-gb boot platform))\n")
	b.WriteString("\n")
	b.WriteString("(def iso-att (vz/new-disk-attachment iso-path true))\n")
	b.WriteString("(def iso-dev (vz/new-usb-mass-storage iso-att))\n")
	fmt.Fprintf(&b, "(vz/create-disk-image disk-path %d)\n", opts.DiskGB)
	b.WriteString("(def disk-att (vz/new-disk-attachment disk-path false))\n")
	b.WriteString("(def disk-dev (vz/new-virtio-block-device disk-att))\n")
	b.WriteString("(vz/add-storage-devices vm-config [iso-dev disk-dev])\n")
	b.WriteString("\n")
	if opts.Hardened {
		b.WriteString(";; Hardened mode requested: omit network device\n")
	} else {
		b.WriteString("(def nat (vz/new-nat-network))\n")
		b.WriteString("(def net (vz/new-virtio-network nat))\n")
		b.WriteString("(vz/add-network-devices vm-config [net])\n")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "(def graphics (vz/new-virtio-graphics-device %d %d))\n", opts.Width, opts.Height)
	b.WriteString("(vz/add-graphics-devices vm-config [graphics])\n")
	b.WriteString("\n")
	b.WriteString("(def valid (vz/validate-config vm-config))\n")
	b.WriteString("(if (= valid true)\n")
	b.WriteString("  (do\n")
	b.WriteString("    (def vm (vz/new-vm vm-config))\n")
	b.WriteString("    (vz/start-vm! vm)\n")
	fmt.Fprintf(&b, "    (vz/start-graphic-app! vm %d %d %q))\n", opts.Width, opts.Height, opts.WindowTitle)
	b.WriteString("  (println \"Configuration invalid:\" valid))\n")
	return b.String()
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
