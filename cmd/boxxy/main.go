//go:build darwin

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bmorphism/boxxy/internal/repl"
	"github.com/bmorphism/boxxy/internal/runner"
	"github.com/bmorphism/boxxy/internal/skill"
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

	case "skill":
		runSkill(args)

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

func runSkill(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `usage: boxxy skill <subcommand> <path|dir> [options]

Subcommands:
  render    Render SKILL.md with GF(3) trit colors (default)
  to-prompt Convert SKILL.md to agent-consumable prompt format
  validate  Check SKILL.md against agentskills.io spec

Options:
  --width N, -w N    Terminal width (default: 100)
  --summary, -s      Show GF(3) triad summary only
  --full             Include full body in to-prompt batch mode`)
		os.Exit(1)
	}

	// Check for subcommand
	subcmd := "render"
	startIdx := 0
	switch args[0] {
	case "to-prompt", "validate", "render":
		subcmd = args[0]
		startIdx = 1
	default:
		// No subcommand — treat first arg as path (default to render)
	}

	width := 100
	summary := false
	full := false
	var paths []string

	for i := startIdx; i < len(args); i++ {
		switch args[i] {
		case "--width", "-w":
			if i+1 < len(args) {
				i++
				w, err := strconv.Atoi(args[i])
				if err == nil && w > 0 {
					width = w
				}
			}
		case "--summary", "-s":
			summary = true
		case "--full":
			full = true
		default:
			paths = append(paths, args[i])
		}
	}

	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "error: no path specified")
		os.Exit(1)
	}

	switch subcmd {
	case "render":
		runSkillRender(paths, width, summary)
	case "to-prompt":
		runSkillToPrompt(paths, full)
	case "validate":
		runSkillValidate(paths)
	}
}

func runSkillRender(paths []string, width int, summary bool) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if info.IsDir() {
			skills, err := skill.LoadSkillDir(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			if summary || len(skills) > 1 {
				fmt.Print(skill.RenderTriadSummary(skills))
			}
			if !summary {
				for _, s := range skills {
					rendered, err := s.Render(width)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error rendering %s: %v\n", s.Name, err)
						continue
					}
					fmt.Print(rendered)
				}
			}
		} else {
			s, err := skill.ParseSkillFile(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			rendered, err := s.Render(width)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(rendered)
		}
	}
}

func runSkillToPrompt(paths []string, full bool) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if info.IsDir() {
			skills, err := skill.LoadSkillDir(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(skill.BatchToPrompt(skills, full))
		} else {
			s, err := skill.ParseSkillFile(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(s.ToPrompt())
		}
	}
}

func runSkillValidate(paths []string) {
	totalValid := 0
	totalInvalid := 0

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if info.IsDir() {
			valid, invalid, results, err := skill.ValidateDir(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			totalValid += valid
			totalInvalid += invalid
			for _, r := range results {
				fmt.Fprintf(os.Stderr, "  %s\n", r)
			}
		} else {
			s, err := skill.ParseSkillFile(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			errs := s.Validate()
			if len(errs) == 0 {
				totalValid++
				fmt.Printf("%s: valid (%s %s)\n", s.Name, s.Role, s.HexColor)
			} else {
				totalInvalid++
				fmt.Fprintf(os.Stderr, "%s: INVALID\n", s.Name)
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
			}
		}
	}

	fmt.Printf("\n%d valid, %d invalid\n", totalValid, totalInvalid)
	if totalInvalid > 0 {
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`boxxy - Clojure SCI for Apple Virtualization.framework

Usage:
  boxxy repl                     Start interactive REPL
  boxxy run [options]            Run a VM with options
  boxxy skill [render] <path>     Render SKILL.md with GF(3) colors
  boxxy skill to-prompt <path>   Convert to agent-consumable prompt
  boxxy skill validate <path>    Check against agentskills.io spec
  boxxy <script.joke>            Run a Joker script
  boxxy version                  Show version
  boxxy help                     Show this help

Run Options:
  --efi                          Use EFI boot (for HaikuOS, FreeBSD, etc)
  --linux                        Use Linux direct boot
  --macos                        Use macOS boot (ARM64 only)
  --guix                         Use Guix (auto-select EFI or Linux boot)
  --kernel <path>                Linux kernel path
  --initrd <path>                Linux initrd path
  --iso <path>                   ISO image path
  --disk <path>                  Disk image path
  --memory <GB>                  Memory in GB (default: 4)
  --cpus <N>                     CPU count (default: 2)
  --nvram <path>                 EFI variable store path
  --hardened                     Disable networking for stronger sandboxing
  --rosetta                      Enable Rosetta for Linux x86_64 binaries (Apple Silicon)
  --rosetta-tag <tag>            VirtioFS tag for Rosetta directory share (default: rosetta)
  --guix-arch <arch>             Guix arch: aarch64 or x86_64 (x86_64 implies --rosetta)

Examples:
  boxxy repl
  boxxy run --efi --iso haiku.iso --disk haiku.img
  boxxy run --linux --kernel vmlinuz --initrd initrd --disk root.img
  boxxy run --guix --iso guix.iso --disk guix.img
  boxxy run --guix --kernel vmlinuz --initrd initrd --disk guix.img
  boxxy examples/haiku-vm.joke
`)
}
