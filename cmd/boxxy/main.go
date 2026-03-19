//go:build darwin && arm64

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bmorphism/boxxy/internal/android"
	"github.com/bmorphism/boxxy/internal/vm"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "macos":
		runMacOS(os.Args[2:])
	case "android":
		runAndroid(os.Args[2:])
	case "haiku":
		runHaiku(os.Args[2:])
	case "sel4":
		runSel4(os.Args[2:])
	case "run":
		runVM(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("boxxy %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runMacOS(args []string) {
	name := "default"
	cpus := 4
	memGB := 4
	diskGB := 64
	sshUser := "bob"
	ipswPath := ""

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `usage: boxxy macos <subcommand> [options]

Subcommands:
  create       Download IPSW and create VM
  boot         Boot an installed macOS VM
  up           Create + install + boot + wait for SSH
  ssh [cmd]    SSH into the running guest
  status       Show VM state
  stop         Stop the running VM

Options:
  --name NAME      VM name (default: default)
  --cpus N         CPU count (default: 4)
  --memory N       Memory in GB (default: 4)
  --disk N         Disk in GB (default: 64)
  --user NAME      SSH user (default: bob)
  --ipsw PATH      Use local IPSW file`)
		os.Exit(1)
	}

	subcmd := args[0]
	args = args[1:]

	var sshCmd []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "--cpus":
			if i+1 < len(args) {
				i++
				cpus, _ = strconv.Atoi(args[i])
			}
		case "--memory":
			if i+1 < len(args) {
				i++
				memGB, _ = strconv.Atoi(args[i])
			}
		case "--disk":
			if i+1 < len(args) {
				i++
				diskGB, _ = strconv.Atoi(args[i])
			}
		case "--user":
			if i+1 < len(args) {
				i++
				sshUser = args[i]
			}
		case "--ipsw":
			if i+1 < len(args) {
				i++
				abs, err := filepath.Abs(args[i])
				if err == nil {
					ipswPath = abs
				} else {
					ipswPath = args[i]
				}
			}
		case "--":
			sshCmd = args[i+1:]
			i = len(args)
		default:
			sshCmd = append(sshCmd, args[i])
		}
	}

	lc := vm.NewMacOSLifecycle(name, cpus, memGB, diskGB)
	if ipswPath != "" {
		lc.IPSWPath = ipswPath
	}

	switch subcmd {
	case "create":
		fmt.Printf("Creating macOS VM %q (%d CPU, %d GB RAM, %d GB disk)...\n", name, cpus, memGB, diskGB)
		if err := lc.Create(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("VM created. Run 'boxxy macos up' to install and boot.")

	case "boot":
		fmt.Printf("Booting macOS VM %q...\n", name)
		if err := lc.Boot(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		vm.WritePIDFile()
		defer vm.RemovePIDFile()
		fmt.Println("VM running. Press Ctrl+C to stop.")
		vm.WaitForShutdown(lc.Instance())

	case "up":
		ctx := context.Background()
		state := lc.State()

		if state == vm.MacOSStateNone {
			fmt.Printf("Creating macOS VM %q (%d CPU, %d GB RAM, %d GB disk)...\n", name, cpus, memGB, diskGB)
			if err := lc.Create(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			state = lc.State()
		}

		if state == vm.MacOSStateCreated {
			fmt.Println("Installing macOS (this will take a while)...")
			if err := lc.Install(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Printf("Booting %q...\n", name)
		if err := lc.Boot(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		vm.WritePIDFile()
		defer vm.RemovePIDFile()
		fmt.Println("VM running. Press Ctrl+C to stop.")
		vm.WaitForShutdown(lc.Instance())

	case "ssh":
		// TODO: auto-detect guest IP via ARP on bridge interface
		guestIP := "192.168.64.2" // default VZ NAT guest IP
		exitCode, err := vm.RunSSH(guestIP, sshUser, sshCmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(exitCode)

	case "status":
		fmt.Print(lc.Summary())

	case "stop":
		home, _ := os.UserHomeDir()
		data, err := os.ReadFile(filepath.Join(home, ".boxxy", "vm.pid"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "no running VM (no PID file)")
			os.Exit(1)
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid <= 0 {
			fmt.Fprintln(os.Stderr, "invalid PID")
			os.Exit(1)
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "process %d not found\n", pid)
			os.Exit(1)
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "signal failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Sent stop to VM process %d\n", pid)

	default:
		fmt.Fprintf(os.Stderr, "unknown macos subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func runVM(args []string) {
	cfg := vm.Config{
		BootMode: "efi",
		Memory:   4,
		CPUs:     2,
		Cmdline:  "console=hvc0",
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--efi":
			cfg.BootMode = "efi"
		case "--linux":
			cfg.BootMode = "linux"
		case "--macos":
			cfg.BootMode = "macos"
		case "--kernel":
			if i+1 < len(args) {
				i++
				cfg.Kernel = args[i]
			}
		case "--initrd":
			if i+1 < len(args) {
				i++
				cfg.Initrd = args[i]
			}
		case "--iso":
			if i+1 < len(args) {
				i++
				cfg.ISO = args[i]
			}
		case "--disk":
			if i+1 < len(args) {
				i++
				cfg.Disk = args[i]
			}
		case "--memory":
			if i+1 < len(args) {
				i++
				cfg.Memory, _ = strconv.Atoi(args[i])
			}
		case "--cpus":
			if i+1 < len(args) {
				i++
				cfg.CPUs, _ = strconv.Atoi(args[i])
			}
		case "--nvram":
			if i+1 < len(args) {
				i++
				cfg.NVRAM = args[i]
			}
		case "--hardened":
			cfg.DisableNetwork = true
		case "--rosetta":
			cfg.EnableRosetta = true
		case "--rosetta-tag":
			if i+1 < len(args) {
				i++
				cfg.RosettaTag = args[i]
			}
		}
	}

	instance, err := vm.CreateVM(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := vm.StartVM(instance); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("VM running. Press Ctrl+C to stop.")
	vm.WaitForShutdown(instance)
}

func runAndroid(args []string) {
	cfg := android.DefaultConfig()
	var monitorDuration string
	var screenshotPath string

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `usage: boxxy android <subcommand> [options]

Subcommands:
  setup          Download SDK + system image + create AVD
  boot           Boot the hardened emulator (pinhole proxy routed)
  probe          Launch UberEats attack surface probe
  monitor        Monitor emulator network for exfiltration (default: 60s)
  screenshot     Capture emulator screen
  report         Print probe findings as JSON
  destroy        Delete the AVD

Options:
  --name NAME          AVD name (default: boxxy-attack-surface)
  --api LEVEL          Android API level (default: 35)
  --proxy ADDR         Route through pinhole proxy (host:port)
  --headless           No window (for CI)
  --duration SECS      Monitor duration (default: 60)
  --output PATH        Screenshot output path`)
		os.Exit(1)
	}

	subcmd := args[0]
	args = args[1:]

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				i++
				cfg.Name = args[i]
			}
		case "--api":
			if i+1 < len(args) {
				i++
				cfg.APILevel = args[i]
			}
		case "--proxy":
			if i+1 < len(args) {
				i++
				cfg.ProxyAddr = args[i]
			}
		case "--headless":
			cfg.Headless = true
		case "--duration":
			if i+1 < len(args) {
				i++
				monitorDuration = args[i]
			}
		case "--output":
			if i+1 < len(args) {
				i++
				screenshotPath = args[i]
			}
		}
	}
	ctx := context.Background()

	switch subcmd {
	case "setup":
		fmt.Printf("[BOXXY] Setting up Android attack surface %q (API %s, %s)...\n",
			cfg.Name, cfg.APILevel, cfg.ABI)
		paths, err := android.EnsureSDK(ctx, cfg.SDKRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := android.AcceptLicenses(ctx, paths); err != nil {
			fmt.Fprintf(os.Stderr, "warning: license acceptance: %v\n", err)
		}
		if err := android.InstallComponents(ctx, paths, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := android.CreateAVD(ctx, paths, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[BOXXY] Android attack surface ready.")
		fmt.Println("  Next: boxxy android boot")

	case "boot":
		paths, err := android.ResolvePaths(cfg.SDKRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if cfg.ProxyAddr != "" {
			fmt.Printf("[BOXXY] Routing emulator through pinhole proxy: %s\n", cfg.ProxyAddr)
			fmt.Printf("[BOXXY] Allowed ports: %v\n", cfg.AllowedPorts)
		}
		fmt.Printf("[BOXXY] Booting %q...\n", cfg.Name)
		emuCmd, err := android.StartEmulator(ctx, paths, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[BOXXY] Emulator PID: %d\n", emuCmd.Process.Pid)
		if err := android.WaitForBoot(ctx, paths, 3*time.Minute); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[BOXXY] Emulator booted. Press Ctrl+C to stop.")
		// Wait for emulator process
		emuCmd.Wait()

	case "probe":
		paths, err := android.ResolvePaths(cfg.SDKRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		probe := android.NewUberEatsProbe(cfg, paths)
		if err := probe.Launch(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "error launching probe: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[BOXXY] UberEats probe launched. Run 'boxxy android monitor' to watch traffic.")

	case "monitor":
		dur := 60
		if monitorDuration != "" {
			dur, _ = strconv.Atoi(monitorDuration)
		}
		paths, err := android.ResolvePaths(cfg.SDKRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		probe := android.NewUberEatsProbe(cfg, paths)
		if err := probe.MonitorNetwork(ctx, time.Duration(dur)*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(probe.Report())

	case "screenshot":
		if screenshotPath == "" {
			screenshotPath = fmt.Sprintf("boxxy-android-%d.png", time.Now().Unix())
		}
		paths, err := android.ResolvePaths(cfg.SDKRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		probe := android.NewUberEatsProbe(cfg, paths)
		if err := probe.Screenshot(ctx, screenshotPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "report":
		paths, err := android.ResolvePaths(cfg.SDKRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		probe := android.NewUberEatsProbe(cfg, paths)
		fmt.Println(probe.Report())

	case "destroy":
		paths, err := android.ResolvePaths(cfg.SDKRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		cmd := exec.CommandContext(ctx, paths.Avdmanager, "delete", "avd", "-n", cfg.Name)
		cmd.Env = append(os.Environ(), "ANDROID_HOME="+paths.SDKRoot)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[BOXXY] AVD %q destroyed.\n", cfg.Name)

	default:
		fmt.Fprintf(os.Stderr, "unknown android subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`boxxy — proof-of-attack VM platform via Apple Virtualization.framework

Usage:
  boxxy macos up                       Create + install + boot macOS VM
  boxxy macos ssh [cmd]                SSH into running guest
  boxxy macos status                   Show VM state
  boxxy macos stop                     Stop VM
  boxxy android setup                  Download SDK + create hardened AVD
  boxxy android boot [--proxy ADDR]    Boot emulator through pinhole proxy
  boxxy android probe                  Launch UberEats attack surface probe
  boxxy android monitor [--duration S] Monitor emulator network exfiltration
  boxxy android screenshot             Capture emulator screen
  boxxy android report                 Print findings as JSON
  boxxy android destroy                Delete the AVD
  boxxy haiku [gui|headless|joke]      Run HaikuOS with first-class defaults
  boxxy sel4 [direct|efi|joke]         Run seL4 with first-class defaults
  boxxy run [options]                  Run a VM directly

Run Options:
  --efi / --linux / --macos            Boot mode
  --kernel PATH   --initrd PATH        Linux boot files
  --iso PATH      --disk PATH          Storage images
  --memory GB     --cpus N             Resources
  --nvram PATH                         EFI variable store
  --hardened                           Disable networking
  --rosetta                            Rosetta for x86_64 on ARM

Examples:
  boxxy macos up
  boxxy macos up --ipsw ~/restore.ipsw
  boxxy macos ssh -- ls /tmp
  boxxy android setup
  boxxy android boot --proxy 0.0.0.0:3128
  boxxy android probe
  boxxy android monitor --duration 120
  boxxy haiku --iso ~/Downloads/haiku/haiku-r1beta5-x86_64-anyboot.iso
  boxxy sel4 direct --kernel ~/projects/xmonad-sel4/seL4/build/root-task.elf
  boxxy run --efi --iso haiku.iso --disk haiku.img
  boxxy run --linux --kernel vmlinuz --initrd initrd --disk root.img
`)
}
