//go:build darwin

package main

import (
	"fmt"
	"os"

	"github.com/bmorphism/boxxy/internal/repl"
	"github.com/bmorphism/boxxy/internal/runner"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "repl":
		repl.Start()

	case "run":
		if err := runner.Run(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "version", "-v", "--version":
		fmt.Printf("boxxy %s\n", version)

	case "help", "-h", "--help":
		printUsage()

	default:
		// Assume it's a script file
		if err := runner.RunScript(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Print(`boxxy - Clojure SCI for Apple Virtualization.framework

Usage:
  boxxy repl                     Start interactive REPL
  boxxy run [options]            Run a VM with options
  boxxy <script.joke>            Run a Joker script
  boxxy version                  Show version
  boxxy help                     Show this help

Run Options:
  --efi                          Use EFI boot (for HaikuOS, FreeBSD, etc)
  --linux                        Use Linux direct boot
  --macos                        Use macOS boot (ARM64 only)
  --kernel <path>                Linux kernel path
  --initrd <path>                Linux initrd path
  --iso <path>                   ISO image path
  --disk <path>                  Disk image path
  --memory <GB>                  Memory in GB (default: 4)
  --cpus <N>                     CPU count (default: 2)
  --nvram <path>                 EFI variable store path

Examples:
  boxxy repl
  boxxy run --efi --iso haiku.iso --disk haiku.img
  boxxy run --linux --kernel vmlinuz --initrd initrd --disk root.img
  boxxy examples/haiku-vm.joke
`)
}
