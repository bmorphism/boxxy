//go:build darwin

package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bmorphism/boxxy/internal/color"
	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/streams"
	"github.com/bmorphism/boxxy/internal/tropical"
	"github.com/bmorphism/boxxy/internal/vm"
)

// Start launches the interactive boxxy REPL with full syntax highlighting,
// rainbow parentheses, polyglot backend switching, and AGM belief revision.
func Start() {
	theme := color.DefaultTheme()

	printBanner(theme)

	// Create environment with standard functions
	env := lisp.CreateStandardEnv()

	// Register vz namespace
	vm.RegisterNamespace(env)

	// Register streams namespace (macOS event consumption)
	streams.RegisterNamespace(env)

	// Register tropical semiring namespace (max-plus DP)
	tropical.RegisterNamespace(env)

	// Register AGM belief revision namespace
	RegisterAGM(env)

	// REPL-specific commands
	env.Set("help", &lisp.Fn{Name: "help", Func: func(args []lisp.Value) lisp.Value {
		printColorHelp(theme)
		return lisp.Nil{}
	}})

	env.Set("backends", &lisp.Fn{Name: "backends", Func: func(args []lisp.Value) lisp.Value {
		PrintBackendList(theme)
		return lisp.Nil{}
	}})

	env.Set("repl", &lisp.Fn{Name: "repl", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			PrintBackendList(theme)
			return lisp.Nil{}
		}
		slug := ""
		switch v := args[0].(type) {
		case lisp.String:
			slug = string(v)
		case lisp.Symbol:
			slug = string(v)
		case lisp.Keyword:
			slug = string(v)
		default:
			fmt.Println(theme.Error.Render("repl: expected backend slug"))
			return lisp.Nil{}
		}
		backend := FindBackend(slug)
		if backend == nil {
			fmt.Println(theme.Error.Render(fmt.Sprintf("Unknown backend: %s", slug)))
			PrintBackendList(theme)
			return lisp.Nil{}
		}
		if !backend.IsAvailable() {
			fmt.Println(theme.Error.Render(fmt.Sprintf("%s not found on PATH", backend.Cmd)))
			return lisp.Nil{}
		}
		fmt.Println(theme.BannerDim.Render(fmt.Sprintf("Launching %s...", backend.Name)))
		if err := backend.Exec(); err != nil {
			fmt.Println(theme.Error.Render(fmt.Sprintf("Error: %v", err)))
		}
		fmt.Println(theme.BannerDim.Render("Back in boxxy."))
		return lisp.Nil{}
	}})

	scanner := bufio.NewScanner(os.Stdin)
	prompt := theme.Prompt.Render("boxxy") + theme.BannerDim.Render("=> ") 

	for {
		fmt.Print(prompt)
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Echo the input with syntax highlighting
		highlighted := theme.HighlightExpr(line)
		// Move cursor up and reprint with colors (overwrite the plain input)
		fmt.Printf("\033[1A%s%s\n", prompt, highlighted)

		// Handle special commands
		switch line {
		case "(quit)", "(exit)":
			fmt.Println(theme.BannerDim.Render("Goodbye!"))
			return
		case "(help)":
			printColorHelp(theme)
			continue
		case "(backends)":
			PrintBackendList(theme)
			continue
		}

		// Evaluate expression
		result := evalString(env, line)
		if result != nil {
			if _, ok := result.(lisp.Nil); !ok {
				// Highlight the result too
				resultStr := result.String()
				fmt.Println(theme.Result.Render(resultStr))
			}
		}
	}
}

func evalString(env *lisp.Env, s string) lisp.Value {
	defer func() {
		if r := recover(); r != nil {
			theme := color.DefaultTheme()
			fmt.Println(theme.Error.Render(fmt.Sprintf("Error: %v", r)))
		}
	}()

	reader := lisp.NewReader(strings.NewReader(s))
	obj, err := reader.Read()
	if err != nil {
		theme := color.DefaultTheme()
		fmt.Println(theme.Error.Render(fmt.Sprintf("Read error: %v", err)))
		return nil
	}

	return lisp.Eval(obj, env)
}

func printBanner(theme *color.Theme) {
	// Box drawing banner with Gay MCP purple
	banner := `
 ┌─────────────────────────────────────────────────────────┐
 │  boxxy  ·  Clojure SCI for Apple Virtualization.framework │
 │                                                           │
 │  Rainbow parens derived from Gay MCP golden thread φ      │
 │  AGM belief revision for possible-worlds reasoning        │
 │  Polyglot backends: (backends) to list, (repl <slug>)     │
 └─────────────────────────────────────────────────────────┘`

	lines := strings.Split(banner, "\n")
	for i, line := range lines {
		if i == 0 && line == "" {
			continue
		}
		if i <= 2 {
			fmt.Println(theme.Banner.Render(line))
		} else {
			fmt.Println(theme.BannerDim.Render(line))
		}
	}

	// Show a quick rainbow parens demo
	demo := `(def vm (vz/new-vm (vz/new-vm-config 4 8 boot platform)))`
	fmt.Println()
	fmt.Print("  ")
	fmt.Println(theme.HighlightExpr(demo))
	fmt.Println()

	fmt.Println(theme.BannerDim.Render("  Type (help) for commands, (quit) to exit"))
	fmt.Println()
}

func printColorHelp(theme *color.Theme) {
	sections := []struct {
		title string
		cmds  []struct{ cmd, desc string }
	}{
		{
			title: "VM Creation",
			cmds: []struct{ cmd, desc string }{
				{"(vz/new-efi-variable-store path create?)", "Create EFI NVRAM"},
				{"(vz/new-efi-boot-loader store)", "EFI boot (HaikuOS, FreeBSD)"},
				{"(vz/new-linux-boot-loader kernel initrd cmdline)", "Linux boot"},
				{"(vz/new-macos-boot-loader)", "macOS boot"},
				{"(vz/new-generic-platform)", "Generic platform config"},
				{"(vz/new-disk-attachment path read-only?)", "Disk/ISO attachment"},
				{"(vz/new-virtio-block-device att)", "Virtio disk"},
				{"(vz/new-usb-mass-storage att)", "USB storage (ISOs)"},
				{"(vz/new-nat-network)", "NAT network"},
				{"(vz/new-virtio-network att)", "Virtio network"},
				{"(vz/new-vm-config cpus mem-gb boot platform)", "Create config"},
				{"(vz/add-storage-devices config devices)", "Add storage"},
				{"(vz/add-network-devices config devices)", "Add network"},
				{"(vz/validate-config config)", "Validate"},
				{"(vz/new-vm config)", "Create VM"},
			},
		},
		{
			title: "VM Control",
			cmds: []struct{ cmd, desc string }{
				{"(vz/start-vm! vm)", "Start VM"},
				{"(vz/stop-vm! vm)", "Stop VM"},
				{"(vz/pause-vm! vm)", "Pause VM"},
				{"(vz/resume-vm! vm)", "Resume VM"},
				{"(vz/vm-state vm)", "Get state"},
			},
		},
		{
			title: "AGM Belief Revision (Possible Worlds)",
			cmds: []struct{ cmd, desc string }{
				{"(agm/new-belief-set)", "Create empty belief set K"},
				{"(agm/expand K p)", "K + p — add belief (no consistency check)"},
				{"(agm/revise K p)", "K * p — add belief, maintain consistency"},
				{"(agm/contract K p)", "K - p — remove belief"},
				{"(agm/beliefs K)", "List all beliefs in K"},
				{"(agm/entails? K p)", "Does K entail p?"},
				{"(agm/consistent? K)", "Is K consistent?"},
				{"(agm/worlds K)", "Possible worlds compatible with K"},
			},
		},
		{
			title: "Polyglot REPL Backends",
			cmds: []struct{ cmd, desc string }{
				{"(backends)", "List all available REPL backends"},
				{"(repl :ipython-hy)", "Launch IPython + Hy + DisCoPy + PyZX"},
				{"(repl :hy)", "Launch Hy Lisp on Python"},
				{"(repl :joker)", "Launch Joker Clojure"},
				{"(repl :bb)", "Launch Babashka nREPL"},
				{"(repl :nu)", "Launch Nushell"},
				{"(repl :emacs)", "Launch Emacs -nw"},
			},
		},
		{
			title: "Utilities",
			cmds: []struct{ cmd, desc string }{
				{"(vz/create-disk-image path size-gb)", "Create disk image"},
				{"(help)", "Show this help"},
				{"(quit)", "Exit REPL"},
			},
		},
	}

	fmt.Println()
	for _, section := range sections {
		fmt.Println(theme.HelpTitle.Render("  " + section.title))
		for _, c := range section.cmds {
			cmd := theme.HighlightExpr(c.cmd)
			desc := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(c.desc)
			fmt.Printf("    %-52s %s\n", cmd, desc)
		}
		fmt.Println()
	}
}
