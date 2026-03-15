//go:build darwin

package tape

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
	"github.com/bmorphism/boxxy/internal/vtterm"
)

// ColoredFrame extends Frame with Gay MCP deterministic color data.
type ColoredFrame struct {
	Frame
	HexColor string   `json:"hex_color"`
	Seed     uint64   `json:"seed"`
	Role     string   `json:"role"`
}

// SessionColorStream binds a vtterm EntropyCollector to a Recorder,
// producing deterministic Gay MCP colors per session. Each frame
// feeds terminal damage entropy into the collector, which seeds
// the color stream via golden angle spiral.
type SessionColorStream struct {
	entropy *vtterm.EntropyCollector
	stream  *vtterm.ColorStream
	seeded  bool
}

// NewSessionColorStream creates a color stream that will be seeded
// by terminal interaction entropy.
func NewSessionColorStream() *SessionColorStream {
	return &SessionColorStream{
		entropy: vtterm.NewEntropyCollector(),
	}
}

// FeedFrame records a frame's damage into the entropy pool.
func (scs *SessionColorStream) FeedFrame(f Frame) {
	// Record damage from frame dimensions as a RectDamage
	scs.entropy.RecordDamage(vtterm.RectDamage(image.Rect(0, 0, f.Width, f.Height)))

	// Record content length as cursor position entropy
	scs.entropy.RecordCursor(len(f.Content)%f.Width, len(f.Content)/f.Width)

	// If we have enough entropy, seed the color stream
	if !scs.seeded && scs.entropy.Len() >= 3 {
		seed := scs.entropy.Seed()
		scs.stream = vtterm.NewColorStream(seed)
		scs.seeded = true
	}
}

// ColorFor returns the deterministic color for a frame index.
func (scs *SessionColorStream) ColorFor(index int) string {
	if scs.stream == nil {
		// Fallback to GF(3) palette before entropy is collected
		return vtterm.HexColor(vtterm.GF3Palette[index%3])
	}
	c := scs.stream.ColorAt(index)
	return vtterm.HexColor(c)
}

// Seed returns the entropy-derived seed, or 0 if not yet seeded.
func (scs *SessionColorStream) Seed() uint64 {
	if scs.stream == nil {
		return 0
	}
	return scs.stream.Seed()
}

// Trit returns the GF(3) trit for the current stream position.
func (scs *SessionColorStream) Trit() gf3.Elem {
	if scs.stream == nil {
		return gf3.Zero
	}
	return gf3.Elem(scs.stream.Index() % 3)
}

// PTYCaptureFunc returns a CaptureFunc that reads the real terminal
// screen content using macOS screencapture of the terminal window.
// Falls back to environment info + process listing for headless contexts.
func PTYCaptureFunc() CaptureFunc {
	return func() (string, int, int, error) {
		w, h := termSize()

		// Try to get real terminal content via shell
		// Use `screen -X hardcopy` approach or read from /dev/tty
		content, err := captureTerminalContent(w, h)
		if err != nil {
			// Fallback: capture environment state
			content = captureEnvironmentState(w, h)
		}

		return content, w, h, nil
	}
}

// SSHCaptureFunc returns a CaptureFunc that captures a remote host's
// terminal via SSH, suitable for recording others using systems over network.
func SSHCaptureFunc(host string, command ...string) CaptureFunc {
	remoteCmd := "TERM=dumb; tput cols 2>/dev/null; tput lines 2>/dev/null; ps aux --sort=-%cpu 2>/dev/null | head -15 || ps aux | head -15"
	if len(command) > 0 {
		remoteCmd = strings.Join(command, " ")
	}

	return func() (string, int, int, error) {
		out, err := exec.Command("ssh",
			"-o", "BatchMode=yes",
			"-o", "ConnectTimeout=3",
			"-o", "StrictHostKeyChecking=no",
			host, remoteCmd,
		).Output()
		if err != nil {
			return fmt.Sprintf("[ssh %s @ %s: %v]",
				host, time.Now().Format("15:04:05"), err), 80, 24, nil
		}

		lines := strings.SplitN(string(out), "\n", 3)
		w, h := 80, 24
		if len(lines) >= 2 {
			fmt.Sscanf(strings.TrimSpace(lines[0]), "%d", &w)
			fmt.Sscanf(strings.TrimSpace(lines[1]), "%d", &h)
		}
		content := ""
		if len(lines) >= 3 {
			content = lines[2]
		}
		return content, w, h, nil
	}
}

// ProcessListCaptureFunc captures the current process table --
// useful for recording what others are running on shared systems.
func ProcessListCaptureFunc() CaptureFunc {
	return func() (string, int, int, error) {
		w, h := termSize()

		out, err := exec.Command("ps", "aux", "--sort=-%cpu").Output()
		if err != nil {
			// macOS ps doesn't support --sort, use different form
			out, err = exec.Command("ps", "aux").Output()
			if err != nil {
				return "[ps failed]", w, h, nil
			}
		}

		lines := strings.Split(string(out), "\n")
		maxLines := h - 2
		if maxLines > len(lines) {
			maxLines = len(lines)
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("=== Process Snapshot @ %s ===\n", time.Now().Format("15:04:05")))
		for i := 0; i < maxLines; i++ {
			line := lines[i]
			if len(line) > w {
				line = line[:w]
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}

		return b.String(), w, h, nil
	}
}

// --- helpers ---

func termSize() (int, int) {
	w, h := 80, 24
	if cols, err := exec.Command("tput", "cols").Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(cols)), "%d", &w)
	}
	if rows, err := exec.Command("tput", "lines").Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(rows)), "%d", &h)
	}
	return w, h
}

func captureTerminalContent(w, h int) (string, error) {
	// Method 1: Try tmux capture-pane if inside tmux
	if os.Getenv("TMUX") != "" {
		out, err := exec.Command("tmux", "capture-pane", "-p", "-S", "-24").Output()
		if err == nil && len(out) > 0 {
			return string(out), nil
		}
	}

	// Method 2: Read from /dev/tty with ANSI DSR
	// This is limited; real implementation would use a pty wrapper
	return "", fmt.Errorf("no direct capture method available")
}

func captureEnvironmentState(w, h int) string {
	var b strings.Builder
	ts := time.Now().Format("2006-01-02 15:04:05")
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	b.WriteString(fmt.Sprintf("╔═══ tapeqt capture @ %s ═══╗\n", ts))
	b.WriteString(fmt.Sprintf("║ host: %-*s║\n", w-10, hostname))
	b.WriteString(fmt.Sprintf("║ cwd:  %-*s║\n", w-10, cwd))
	b.WriteString(fmt.Sprintf("║ term: %dx%d%-*s║\n", w, h, w-22, ""))

	// Top processes
	if out, err := exec.Command("ps", "axo", "pid,pcpu,pmem,comm").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		maxProcs := h - 6
		if maxProcs > len(lines) {
			maxProcs = len(lines)
		}
		b.WriteString("╠═══ processes ═══════════════════════╣\n")
		for i := 0; i < maxProcs && i < len(lines); i++ {
			line := lines[i]
			if len(line) > w-4 {
				line = line[:w-4]
			}
			b.WriteString(fmt.Sprintf("║ %-*s║\n", w-4, line))
		}
	}
	b.WriteString("╚═════════════════════════════════════╝\n")
	return b.String()
}
