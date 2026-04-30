//go:build darwin

package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/streams"
	"github.com/bmorphism/boxxy/internal/tile"
	"github.com/bmorphism/boxxy/internal/tropical"
	"github.com/bmorphism/boxxy/internal/vm"
)

// TileREPL is a color-dispatched REPL where each session has a deterministic
// tile identity (seed → SplitMix64 → Color) and all output uses OSC 8
// terminal hyperlinks for inspectable, clickable structure.
type TileREPL struct {
	Seed     uint64
	Identity tile.ColorIdentity
	Env      *lisp.Env
	Lattice  *tile.TileLattice
}

// NewTileREPL creates a REPL bound to a tile seed.
// If seed==0, derives from current time for a fresh session.
func NewTileREPL(seed uint64) *TileREPL {
	if seed == 0 {
		seed = uint64(time.Now().UnixNano())
	}
	ci := tile.NewColorIdentity(seed)
	env := lisp.CreateStandardEnv()

	// Register all namespaces
	vm.RegisterNamespace(env)
	streams.RegisterNamespace(env)
	tropical.RegisterNamespace(env)
	tile.RegisterNamespace(env)

	tr := &TileREPL{
		Seed:     seed,
		Identity: ci,
		Env:      env,
		Lattice:  tile.NewTileLattice(),
	}

	// Register AGM
	RegisterAGM(env)

	// Register tile-REPL-specific builtins
	tr.registerBuiltins()

	return tr
}

func (tr *TileREPL) registerBuiltins() {
	env := tr.Env

	env.Set("help", &lisp.Fn{Name: "help", Func: func(args []lisp.Value) lisp.Value {
		tr.printHelp()
		return lisp.Nil{}
	}})

	env.Set("backends", &lisp.Fn{Name: "backends", Func: func(args []lisp.Value) lisp.Value {
		// Use raw ANSI since we're OSC 8 native now
		tr.printBackends()
		return lisp.Nil{}
	}})

	env.Set("repl", &lisp.Fn{Name: "repl", Func: func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			tr.printBackends()
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
			fmt.Println(ansiBold(239, 68, 68, "repl: expected backend slug"))
			return lisp.Nil{}
		}
		backend := FindBackend(slug)
		if backend == nil {
			fmt.Println(ansiBold(239, 68, 68, fmt.Sprintf("Unknown backend: %s", slug)))
			tr.printBackends()
			return lisp.Nil{}
		}
		if !backend.IsAvailable() {
			fmt.Println(ansiBold(239, 68, 68, fmt.Sprintf("%s not found on PATH", backend.Cmd)))
			return lisp.Nil{}
		}
		fmt.Println(ansiColor(107, 114, 128, fmt.Sprintf("Launching %s...", backend.Name)))
		if err := backend.Exec(); err != nil {
			fmt.Println(ansiBold(239, 68, 68, fmt.Sprintf("Error: %v", err)))
		}
		fmt.Println(ansiColor(107, 114, 128, "Back in boxxy."))
		return lisp.Nil{}
	}})

	// tile/identity — show current tile identity with OSC 8 links
	env.Set("tile/identity", &lisp.Fn{Name: "tile/identity", Func: func(args []lisp.Value) lisp.Value {
		fmt.Println(LinkedTile(tr.Identity))
		return lisp.Nil{}
	}})

	// tile/seed — get or set session seed
	env.Set("tile/seed", &lisp.Fn{Name: "tile/seed", Func: func(args []lisp.Value) lisp.Value {
		if len(args) > 0 {
			tr.Seed = uint64(args[0].(lisp.Int))
			tr.Identity = tile.NewColorIdentity(tr.Seed)
		}
		return lisp.Int(int64(tr.Seed))
	}})

	// tile/wire — show Syrup wire encoding with link
	env.Set("tile/wire", &lisp.Fn{Name: "tile/wire", Func: func(args []lisp.Value) lisp.Value {
		wire := tr.Identity.EncodeSyrupColor()
		url := SyrupURI(tr.Seed)
		fmt.Println(OSC8Link(url, string(wire)))
		return lisp.Nil{}
	}})
}

// Run launches the interactive tile-colored, OSC 8-linked REPL.
func (tr *TileREPL) Run() {
	tr.printBanner()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(tr.prompt())
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Re-render input with linked highlighting (overwrite plain)
		highlighted := LinkedHighlightExpr(line, tr.Seed)
		fmt.Printf("\033[1A%s%s\n", tr.prompt(), highlighted)

		switch line {
		case "(quit)", "(exit)":
			fmt.Println(ansiColor(107, 114, 128, "Goodbye!"))
			return
		case "(help)":
			tr.printHelp()
			continue
		case "(backends)":
			tr.printBackends()
			continue
		}

		result := tr.evalString(line)
		if result != nil {
			if _, ok := result.(lisp.Nil); !ok {
				resultStr := result.String()
				// Result in the tile's own color, linked to tile://eval
				url := fmt.Sprintf("tile://eval/%d", tr.Seed)
				colored := ansiColor(16, 185, 129, resultStr)
				fmt.Println(OSC8Link(url, colored))
			}
		}
	}
}

// prompt renders the tile-colored prompt with OSC 8 link on the tile name.
func (tr *TileREPL) prompt() string {
	ci := tr.Identity
	url := TileURI(tr.Seed)
	colored := fmt.Sprintf("\033[1;38;2;%d;%d;%dm", ci.Color.R, ci.Color.G, ci.Color.B)
	reset := "\033[0m"
	dim := "\033[38;2;107;114;128m"
	return colored + OSC8Link(url, "boxxy") + reset + dim + "=> " + reset
}

func (tr *TileREPL) evalString(s string) lisp.Value {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(ansiBold(239, 68, 68, fmt.Sprintf("Error: %v", r)))
		}
	}()

	reader := lisp.NewReader(strings.NewReader(s))
	obj, err := reader.Read()
	if err != nil {
		fmt.Println(ansiBold(239, 68, 68, fmt.Sprintf("Read error: %v", err)))
		return nil
	}

	return lisp.Eval(obj, tr.Env)
}

func (tr *TileREPL) printBanner() {
	ci := tr.Identity
	c := ci.Color
	hex := ci.HexCode

	// Title in the tile's own color
	titleColor := fmt.Sprintf("\033[1;38;2;%d;%d;%dm", c.R, c.G, c.B)
	dim := "\033[38;2;107;114;128m"
	reset := "\033[0m"

	fmt.Println()
	fmt.Println(titleColor + " ┌─────────────────────────────────────────────────────────┐" + reset)
	fmt.Println(titleColor + " │  " + reset + titleColor + "boxxy" + reset + dim + "  ·  Tile-colored VM REPL with OSC 8 hyperlinks   " + titleColor + "│" + reset)
	fmt.Println(titleColor + " │" + reset + dim + "                                                           " + titleColor + "│" + reset)
	fmt.Print(titleColor + " │" + reset + dim + "  Tile: " + reset)
	fmt.Print(LinkedTile(ci))
	fmt.Print(dim + "  Seed: " + reset)
	fmt.Print(LinkedSeed(tr.Seed))
	padding := 59 - len(hex) - len(fmt.Sprintf("seed:%d", tr.Seed)) - 18
	if padding < 0 {
		padding = 0
	}
	fmt.Print(strings.Repeat(" ", padding))
	fmt.Println(titleColor + "│" + reset)

	fmt.Println(titleColor + " │" + reset + dim + "  Rainbow parens from golden thread φ, linked via OSC 8    " + titleColor + "│" + reset)
	fmt.Println(titleColor + " │" + reset + dim + "  Namespace symbols are clickable: tile/*, vz/*, agm/*     " + titleColor + "│" + reset)
	fmt.Println(titleColor + " └─────────────────────────────────────────────────────────┘" + reset)

	// Demo line with OSC 8 rainbow parens
	demo := `(def vm (tile/color-identity 42))`
	fmt.Println()
	fmt.Print("  ")
	fmt.Println(LinkedHighlightExpr(demo, tr.Seed))
	fmt.Println()

	fmt.Println(dim + "  Type (help) for commands, (quit) to exit" + reset)
	fmt.Println()
}

func (tr *TileREPL) printHelp() {
	dim := "\033[38;2;107;114;128m"
	reset := "\033[0m"

	sections := []struct {
		title string
		cmds  []struct{ cmd, desc string }
	}{
		{
			title: "Tile Identity",
			cmds: []struct{ cmd, desc string }{
				{"(tile/identity)", "Show current tile color + OSC 8 link"},
				{"(tile/seed)", "Get session seed"},
				{"(tile/seed N)", "Set session seed, re-derive color"},
				{"(tile/wire)", "Show Syrup wire encoding of tile color"},
				{"(tile/color-identity N)", "Compute full identity for any seed"},
				{"(tile/rainbow-parens seed expr)", "Colorize parens from seed's palette"},
			},
		},
		{
			title: "Tile Lattice",
			cmds: []struct{ cmd, desc string }{
				{"(tile/lattice-new)", "Create empty tile lattice"},
				{"(tile/lattice-add lattice vm)", "Add tileable VM"},
				{"(tile/lattice-balanced? lattice)", "Check GF(3) balance"},
				{"(tile/lattice-find-balancer lattice)", "Find balancing seed"},
				{"(tile/lattice-wire-colors lattice)", "All tile colors as Syrup"},
			},
		},
		{
			title: "VM Creation",
			cmds: []struct{ cmd, desc string }{
				{"(vz/new-vm-config cpus mem boot plat)", "Create VM config"},
				{"(tile/new-tileable-vm name seed cfg)", "Wrap VM in tile identity"},
				{"(vz/new-vm config)", "Create bare VM"},
				{"(vz/start-vm! vm)", "Start VM"},
				{"(vz/stop-vm! vm)", "Stop VM"},
			},
		},
		{
			title: "Syrup Wire Format",
			cmds: []struct{ cmd, desc string }{
				{"(tile/syrup-encode seed)", "Encode color as Syrup record"},
				{"(tile/syrup-checkpoint seed wid inv)", "Encode checkpoint"},
				{"(tile/message-frame payload)", "4-byte BE length prefix"},
			},
		},
		{
			title: "GF(3) Arithmetic",
			cmds: []struct{ cmd, desc string }{
				{"(tile/balanced-trit seed)", "Compute balanced trit from seed"},
				{"(tile/find-balancer seed-a seed-b)", "Find third balancing seed"},
			},
		},
		{
			title: "AGM Belief Revision",
			cmds: []struct{ cmd, desc string }{
				{"(agm/new-belief-set)", "Create empty belief set"},
				{"(agm/revise K p)", "Add belief, maintain consistency"},
				{"(agm/worlds K)", "Possible worlds"},
			},
		},
		{
			title: "Polyglot REPL Backends",
			cmds: []struct{ cmd, desc string }{
				{"(backends)", "List available backends"},
				{"(repl :joker)", "Launch Joker Clojure"},
				{"(repl :bb)", "Launch Babashka nREPL"},
				{"(repl :emacs)", "Launch Emacs -nw"},
			},
		},
		{
			title: "Utilities",
			cmds: []struct{ cmd, desc string }{
				{"(help)", "Show this help"},
				{"(quit)", "Exit REPL"},
			},
		},
	}

	fmt.Println()
	for _, section := range sections {
		fmt.Println(ansiBold(245, 158, 11, "  "+section.title))
		for _, c := range section.cmds {
			cmd := LinkedHighlightExpr(c.cmd, tr.Seed)
			desc := dim + c.desc + reset
			fmt.Printf("    %-56s %s\n", cmd, desc)
		}
		fmt.Println()
	}
}

func (tr *TileREPL) printBackends() {
	fmt.Println()
	fmt.Println(ansiBold(245, 158, 11, "  Available REPL Backends"))
	fmt.Println()

	dim := "\033[38;2;107;114;128m"
	reset := "\033[0m"

	for _, b := range Backends {
		r, g, bl := hexToRGB(b.Color)

		avail := "  "
		if b.IsAvailable() {
			avail = ansiColor(16, 185, 129, "*") + " "
		} else {
			avail = ansiColor(239, 68, 68, "x") + " "
		}

		slug := ansiBold(r, g, bl, b.Slug)
		desc := dim + b.Description + reset

		sign := "0"
		var sr, sg, sb uint8
		switch b.Trit {
		case 1:
			sign = "+"
			sr, sg, sb = 16, 185, 129
		case 0:
			sign = "0"
			sr, sg, sb = 245, 158, 11
		case -1:
			sign = "-"
			sr, sg, sb = 239, 68, 68
		}
		tritLabel := ansiColor(sr, sg, sb, sign)

		fmt.Printf("  %s[%s] %-20s %s\n", avail, tritLabel, slug, desc)
	}

	fmt.Println()
	fmt.Println(dim + "  * = available on PATH    (repl <slug>) to launch" + reset)
	fmt.Println()
}

func hexToRGB(hex string) (uint8, uint8, uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 255, 255, 255
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// StartTileREPL is the top-level entry point for the tile-colored REPL.
// Pass seed=0 for a fresh session derived from time.
func StartTileREPL(seed uint64) {
	tr := NewTileREPL(seed)
	tr.Run()
}
