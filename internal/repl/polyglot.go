//go:build darwin

package repl

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/lipgloss"

	"github.com/bmorphism/boxxy/internal/color"
)

// Backend represents a REPL backend that boxxy can launch.
type Backend struct {
	Name        string   // display name
	Slug        string   // CLI identifier
	Cmd         string   // executable
	Args        []string // arguments
	Env         []string // extra environment variables
	Description string   // one-line description
	Trit        int      // GF(3) trit: +1 creation, 0 ergodic, -1 verification
	Color       string   // Gay MCP hex color
}

// Backends is the registry of all available REPL backends, ordered from
// lightweight to full-fledged (culminating in Emacs).
var Backends = []Backend{
	{
		Name:        "boxxy",
		Slug:        "boxxy",
		Cmd:         "", // internal — no subprocess
		Description: "Built-in Clojure-dialect REPL for Apple Virtualization.framework",
		Trit:        1,
		Color:       "#A855F7",
	},
	{
		Name:        "IPython + Hy",
		Slug:        "ipython-hy",
		Cmd:         "uv",
		Args:        []string{"run", "--with", "ipython", "--with", "discopy", "--with", "pyzx", "--with", "hy", "ipython"},
		Description: "IPython with Hy (Clojure-on-Python), DisCoPy (category theory), PyZX (quantum ZX-calculus)",
		Trit:        0,
		Color:       "#F59E0B",
	},
	{
		Name:        "Hy REPL",
		Slug:        "hy",
		Cmd:         "uv",
		Args:        []string{"run", "--with", "hy", "--with", "discopy", "--with", "pyzx", "hy"},
		Description: "Hy Lisp dialect on Python — S-expressions with full Python interop",
		Trit:        1,
		Color:       "#10B981",
	},
	{
		Name:        "Joker",
		Slug:        "joker",
		Cmd:         "joker",
		Args:        []string{},
		Description: "Clojure interpreter/linter in Go — fast startup, EDN native",
		Trit:        -1,
		Color:       "#2E5FA3",
	},
	{
		Name:        "Babashka",
		Slug:        "bb",
		Cmd:         "bb",
		Args:        []string{"nrepl-server"},
		Description: "Babashka nREPL — Clojure scripting without JVM startup",
		Trit:        0,
		Color:       "#6366F1",
	},
	{
		Name:        "Nushell",
		Slug:        "nu",
		Cmd:         "nu",
		Args:        []string{},
		Description: "Structured data shell — pipelines return tables, not text",
		Trit:        0,
		Color:       "#14B8A6",
	},
	{
		Name:        "Emacs",
		Slug:        "emacs",
		Cmd:         "emacs",
		Args:        []string{"-nw"},
		Description: "GNU Emacs in terminal — the fully-fledged programmable environment",
		Trit:        1,
		Color:       "#EF4444",
	},
}

// FindBackend looks up a backend by slug.
func FindBackend(slug string) *Backend {
	for i := range Backends {
		if Backends[i].Slug == slug {
			return &Backends[i]
		}
	}
	return nil
}

// IsAvailable checks if the backend's command is on PATH.
func (b *Backend) IsAvailable() bool {
	if b.Cmd == "" {
		return true // internal backend
	}
	_, err := exec.LookPath(b.Cmd)
	return err == nil
}

// Launch replaces the current process with the backend's REPL via execvp.
// For internal backends (boxxy), this is a no-op — the caller should
// fall through to the built-in REPL.
func (b *Backend) Launch() error {
	if b.Cmd == "" {
		return nil // internal
	}

	bin, err := exec.LookPath(b.Cmd)
	if err != nil {
		return fmt.Errorf("%s not found on PATH: %w", b.Cmd, err)
	}

	argv := append([]string{b.Cmd}, b.Args...)
	env := append(os.Environ(), b.Env...)

	return syscall.Exec(bin, argv, env)
}

// Exec runs the backend as a subprocess (non-replacing), piping
// stdin/stdout/stderr. Used when we want to return to boxxy after.
func (b *Backend) Exec() error {
	if b.Cmd == "" {
		return nil
	}

	cmd := exec.Command(b.Cmd, b.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), b.Env...)
	return cmd.Run()
}

// PrintBackendList renders the available backends with Gay MCP colors.
func PrintBackendList(theme *color.Theme) {
	fmt.Println()
	fmt.Println(theme.HelpTitle.Render("Available REPL Backends"))
	fmt.Println()

	for _, b := range Backends {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(b.Color))
		avail := "  "
		if b.IsAvailable() {
			avail = theme.Result.Render("*") + " "
		} else {
			avail = theme.Error.Render("x") + " "
		}

		slug := style.Bold(true).Render(b.Slug)
		desc := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(b.Description)

		tritLabel := ""
		switch b.Trit {
		case 1:
			tritLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("+")
		case 0:
			tritLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render("0")
		case -1:
			tritLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("-")
		}

		fmt.Printf("  %s[%s] %-16s %s\n", avail, tritLabel, slug, desc)
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"  * = available on PATH    (repl <slug>) to launch"))
	fmt.Println()
}
