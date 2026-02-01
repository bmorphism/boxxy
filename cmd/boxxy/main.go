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

	case "detect-strategy":
		runDetectStrategy(args)

	case "list-skills":
		runListSkills(args)

	case "check-balance":
		runCheckBalance(args)

	case "generate-sideref":
		runGenerateSideref(args)

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

// detectVersionBump determines semantic version increment based on changes
func detectVersionBump(siderefChanges, skillChanges, coreChanges bool) string {
	// MAJOR: OCAPN Sideref token format changes (cryptographic breaking changes)
	// MINOR: New skills or capability additions
	// PATCH: Bug fixes and validation improvements
	if siderefChanges {
		return "major"
	}
	if skillChanges || coreChanges {
		return "minor"
	}
	return "patch"
}

// runDetectStrategy analyzes capability changes and determines version strategy
func runDetectStrategy(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `usage: boxxy detect-strategy [options]

Analyzes changes and determines semantic version increment strategy.

Options:
  --sideref       Include Sideref cryptographic changes (MAJOR)
  --skills        Include skill additions (MINOR)
  --core          Include core capability changes (MINOR)
  --patches       Include patch/fix changes (PATCH)

Output format: JSON with detected bump level and justification`)
		os.Exit(1)
	}

	siderefChanges := false
	skillChanges := false
	coreChanges := false

	for _, arg := range args {
		switch arg {
		case "--sideref":
			siderefChanges = true
		case "--skills":
			skillChanges = true
		case "--core":
			coreChanges = true
		}
	}

	bump := detectVersionBump(siderefChanges, skillChanges, coreChanges)

	fmt.Printf(`{
  "strategy": "%s",
  "sideref_changes": %v,
  "skill_changes": %v,
  "core_changes": %v,
  "description": "%s"
}
`, bump, siderefChanges, skillChanges, coreChanges, getVersionDescription(bump))
}

// getVersionDescription returns human-readable explanation for version bump
func getVersionDescription(bump string) string {
	switch bump {
	case "major":
		return "Breaking changes to Sideref token format or cryptographic protocol"
	case "minor":
		return "New skill capabilities or core feature additions (backward compatible)"
	case "patch":
		return "Bug fixes and validation improvements (no behavior changes)"
	default:
		return "Unknown version strategy"
	}
}

// runListSkills lists all registered skills with their GF(3) trit values
func runListSkills(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `usage: boxxy list-skills <path> [options]

Lists all registered skills with GF(3) trit classification.

Options:
  --trit N        Filter by trit value (0, 1, or 2)
  --json          Output as JSON
  --csv           Output as CSV

Example:
  boxxy list-skills skills/ --trit 1
  boxxy list-skills skills/ --json`)
		os.Exit(1)
	}

	path := args[0]
	filterTrit := -1
	outputFormat := "table"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--trit":
			if i+1 < len(args) {
				if t, err := strconv.Atoi(args[i+1]); err == nil && t >= 0 && t <= 2 {
					filterTrit = t
					i++
				}
			}
		case "--json":
			outputFormat = "json"
		case "--csv":
			outputFormat = "csv"
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot access %s: %v\n", path, err)
		os.Exit(1)
	}

	var skills []*skill.Skill
	if info.IsDir() {
		skills, err = skill.LoadSkillDir(path)
	} else {
		s, err := skill.ParseSkillFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		skills = []*skill.Skill{s}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Filter by trit if requested
	if filterTrit >= 0 {
		var filtered []*skill.Skill
		for _, s := range skills {
			if int(s.Trit) == filterTrit {
				filtered = append(filtered, s)
			}
		}
		skills = filtered
	}

	// Output in requested format
	switch outputFormat {
	case "json":
		fmt.Printf(`{"skills": [
`)
		for i, s := range skills {
			if i > 0 {
				fmt.Printf(",\n")
			}
			fmt.Printf(`  {"name": %q, "trit": %d, "role": %q}`, s.Name, s.Trit, s.Role)
		}
		fmt.Printf(`
]}
`)
	case "csv":
		fmt.Println("name,trit,role")
		for _, s := range skills {
			fmt.Printf("%s,%d,%s\n", s.Name, s.Trit, s.Role)
		}
	default: // table
		fmt.Println("Name                          Trit  Role")
		fmt.Println("────────────────────────────  ────  ──────────────")
		for _, s := range skills {
			fmt.Printf("%-30s%-5d%s\n", s.Name, s.Trit, s.Role)
		}
	}

	fmt.Printf("\nTotal: %d skills\n", len(skills))
}

// runCheckBalance validates GF(3) triadic balance
func runCheckBalance(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `usage: boxxy check-balance <path>

Validates that GF(3) trit sum equals 0 (mod 3).
Returns exit code 0 if balanced, 1 if unbalanced.

Example:
  boxxy check-balance skills/`)
		os.Exit(1)
	}

	path := args[0]
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot access %s: %v\n", path, err)
		os.Exit(1)
	}

	var skills []*skill.Skill
	if info.IsDir() {
		skills, err = skill.LoadSkillDir(path)
	} else {
		s, err := skill.ParseSkillFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		skills = []*skill.Skill{s}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Calculate trit sum
	tritSum := 0
	tritCounts := [3]int{0, 0, 0}
	for _, s := range skills {
		tritSum += int(s.Trit)
		tritCounts[s.Trit]++
	}

	balanced := tritSum%3 == 0

	fmt.Printf("GF(3) Balance Check\n")
	fmt.Printf("─────────────────────\n")
	fmt.Printf("Total skills: %d\n", len(skills))
	fmt.Printf("Trit 0 (Coordinator): %d\n", tritCounts[0])
	fmt.Printf("Trit 1 (Generator):   %d\n", tritCounts[1])
	fmt.Printf("Trit 2 (Verifier):    %d\n", tritCounts[2])
	fmt.Printf("Sum of trits: %d\n", tritSum)
	fmt.Printf("Sum mod 3: %d\n", tritSum%3)

	if balanced {
		fmt.Printf("\n✓ Balanced: System is in GF(3) equilibrium\n")
		os.Exit(0)
	} else {
		fmt.Printf("\n✗ Unbalanced: Sum of trits is not ≡ 0 (mod 3)\n")
		os.Exit(1)
	}
}

// runGenerateSideref creates OCAPN Sideref tokens for skills
func runGenerateSideref(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, `usage: boxxy generate-sideref <skill-name> <device-secret-hex>

Generates an unforgeable OCAPN Sideref capability token.

Arguments:
  skill-name           Canonical skill name (e.g., glucose-monitor)
  device-secret-hex    16-byte device secret as hex string (32 hex chars)

Example:
  boxxy generate-sideref glucose-monitor 0102030405060708090a0b0c0d0e0f10

Output: Sideref token details including HMAC-SHA256 verification token`)
		os.Exit(1)
	}

	skillName := args[0]
	secretHex := args[1]

	// Parse device secret from hex
	if len(secretHex) != 32 {
		fmt.Fprintf(os.Stderr, "error: device secret must be exactly 32 hex characters (16 bytes), got %d\n", len(secretHex))
		os.Exit(1)
	}

	secret := [16]byte{}
	for i := 0; i < 16; i++ {
		var b byte
		_, err := fmt.Sscanf(secretHex[i*2:i*2+2], "%02x", &b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid hex character in device secret at position %d\n", i*2)
			os.Exit(1)
		}
		secret[i] = b
	}

	// Generate Sideref token
	token := skill.NewSiderefToken(skillName, secret)

	// Output token details
	fmt.Printf(`Sideref Token Generated
──────────────────────────

Skill Name:    %s
Device Secret: %s
Token Version: %d
Expires At:    %d (0 = never)
HMAC Token:    %x
`, token.SkillName, secretHex, token.TokenVersion, token.ExpiresAt, token.Token[:])

	// Show compact format for BLE advertisement
	fmt.Printf("\nCompact Format (BLE Wire Format):\n")
	compact := token.MarshalSideref()
	fmt.Printf("Hex: %x\n", compact)

	// Show string representation
	fmt.Printf("\nString Representation:\n")
	fmt.Printf("%s\n", token.String())
}

func printUsage() {
	fmt.Print(`boxxy - Clojure SCI for Apple Virtualization.framework

Usage:
  boxxy repl                           Start interactive REPL
  boxxy run [options]                  Run a VM with options
  boxxy skill [render] <path>          Render SKILL.md with GF(3) colors
  boxxy skill to-prompt <path>         Convert to agent-consumable prompt
  boxxy skill validate <path>          Check against agentskills.io spec
  boxxy detect-strategy [options]      Detect semantic version increment (Phase 2)
  boxxy list-skills <path> [options]   List skills with GF(3) trits (Phase 2)
  boxxy check-balance <path>           Validate GF(3) equilibrium (Phase 2)
  boxxy generate-sideref <name> <hex>  Create OCAPN Sideref token (Phase 1)
  boxxy <script.joke>                  Run a Joker script
  boxxy version                        Show version
  boxxy help                           Show this help

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
