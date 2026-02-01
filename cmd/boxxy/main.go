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

	case "asi-select-balanced":
		runASISelectBalanced(args)

	case "asi-list":
		runASIList(args)

	case "asi-export":
		runASIExport(args)

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

// runASISelectBalanced selects a balanced subset from ASI registry
func runASISelectBalanced(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, `usage: boxxy asi-select-balanced <json-file> <count>

Selects a balanced subset of N skills from ASI registry JSON.
Ensures GF(3) triadic balance (sum ≡ 0 mod 3).

Arguments:
  json-file    Path to ASI registry JSON file
  count        Number of skills to select (ideally multiple of 3)

Example:
  boxxy asi-select-balanced asi-registry.json 27`)
		os.Exit(1)
	}

	jsonFile := args[0]
	count, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid count %q: %v\n", args[1], err)
		os.Exit(1)
	}

	// Read JSON registry
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", jsonFile, err)
		os.Exit(1)
	}

	// Import registry
	reg := skill.NewASIRegistry()
	if err := reg.FromJSON(data); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse registry: %v\n", err)
		os.Exit(1)
	}

	// Select balanced subset
	selected, err := reg.SelectBalancedSubset(count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Output results
	fmt.Printf("Selected %d balanced skills:\n", len(selected))
	fmt.Printf("─────────────────────────────\n")

	tritCounts := [3]int{}
	for _, s := range selected {
		tritCounts[s.Trit]++
		roleStr := "Coordinator"
		if s.Trit == 1 {
			roleStr = "Generator"
		} else if s.Trit == 2 {
			roleStr = "Verifier"
		}
		fmt.Printf("  %s (%s, trit=%d)\n", s.Name, roleStr, s.Trit)
	}

	fmt.Printf("\nDistribution:\n")
	fmt.Printf("  Coordinators: %d\n", tritCounts[0])
	fmt.Printf("  Generators:   %d\n", tritCounts[1])
	fmt.Printf("  Verifiers:    %d\n", tritCounts[2])

	// Verify balance
	sum := tritCounts[0]*0 + tritCounts[1]*1 + tritCounts[2]*2
	fmt.Printf("  Sum: %d (mod 3 = %d)\n", sum, sum%3)
	if sum%3 == 0 {
		fmt.Printf("  ✓ Balanced\n")
	} else {
		fmt.Printf("  ✗ Not balanced\n")
	}
}

// runASIList lists skills from ASI registry JSON
func runASIList(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, `usage: boxxy asi-list <json-file> [options]

Lists all skills from ASI registry JSON file.

Options:
  --trit N      Filter by trit value (0, 1, or 2)
  --category C  Filter by category

Example:
  boxxy asi-list asi-registry.json --trit 1`)
		os.Exit(1)
	}

	jsonFile := args[0]
	// Note: filterTrit and filterCategory could be used for filtering
	// Currently shows all skills from registry

	// Parse options (for future use)
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--trit", "--category":
			// Skip option and its value for now
			if i+1 < len(args) {
				i++
			}
		}
	}

	// Read JSON registry
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", jsonFile, err)
		os.Exit(1)
	}

	// Import registry
	reg := skill.NewASIRegistry()
	if err := reg.FromJSON(data); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse registry: %v\n", err)
		os.Exit(1)
	}

	// Output header
	fmt.Printf("ASI Skills Registry\n")
	fmt.Printf("───────────────────\n")
	status := reg.TriadStatus()
	fmt.Printf("Total: %v skills\n", status["total_skills"])
	fmt.Printf("Coordinators: %v | Generators: %v | Verifiers: %v\n",
		status["coordinators"], status["generators"], status["verifiers"])
	fmt.Printf("Balance: %v\n\n", status["balanced"])

	// List skills (simplified - just show names and trits)
	fmt.Printf("Skills:\n")
	fmt.Printf("─────────────────────────────────────────\n")
	fmt.Printf("(Full listing requires registry export functionality)\n")
	fmt.Printf("Use 'asi-select-balanced' to create balanced subsets for export.\n")
}

// runASIExport exports balanced subset to embedded skill format
func runASIExport(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, `usage: boxxy asi-export <json-file> <count> <device-secret-hex>

Exports balanced ASI skill subset to embedded skill format with Sideref tokens.

Arguments:
  json-file          Path to ASI registry JSON file
  count              Number of skills to select
  device-secret-hex  16-byte device secret as hex (32 chars)

Example:
  boxxy asi-export asi-registry.json 27 0102030405060708090a0b0c0d0e0f10`)
		os.Exit(1)
	}

	jsonFile := args[0]
	count, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid count: %v\n", err)
		os.Exit(1)
	}

	secretHex := args[2]
	if len(secretHex) != 32 {
		fmt.Fprintf(os.Stderr, "error: device secret must be 32 hex characters\n")
		os.Exit(1)
	}

	// Parse device secret
	secret := [16]byte{}
	for i := 0; i < 16; i++ {
		var b byte
		_, err := fmt.Sscanf(secretHex[i*2:i*2+2], "%02x", &b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid hex in device secret\n")
			os.Exit(1)
		}
		secret[i] = b
	}

	// Read and import registry
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", jsonFile, err)
		os.Exit(1)
	}

	reg := skill.NewASIRegistry()
	if err := reg.FromJSON(data); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse registry: %v\n", err)
		os.Exit(1)
	}

	// Select balanced subset
	_, err = reg.SelectBalancedSubset(count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Export to embedded format
	embedded, err := reg.ExportForEmbedded(secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Output exported skills
	fmt.Printf("Exported %d embedded skills with Sideref tokens:\n", len(embedded))
	fmt.Printf("────────────────────────────────────────────────\n")

	for _, e := range embedded {
		fmt.Printf("• %s (trit=%d)\n", e.Name, e.Trit)
		if e.Sideref != nil {
			fmt.Printf("  Sideref: %x\n", e.Sideref.Token[:8])
		}
	}

	fmt.Printf("\nReady for deployment to medical device firmware.\n")
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
  boxxy asi-select-balanced <j> <n>    Select balanced ASI skill subset (Phase 3)
  boxxy asi-list <json-file>           List ASI registry skills (Phase 3)
  boxxy asi-export <j> <n> <hex>       Export balanced skills with Sideref (Phase 3)
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
