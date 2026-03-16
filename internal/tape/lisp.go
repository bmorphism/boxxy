//go:build darwin

package tape

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bmorphism/boxxy/internal/lisp"
)

// RegisterNamespace registers tape/* functions in the Lisp environment.
func RegisterNamespace(env *lisp.Env) {
	env.Set("tape/new-recorder", &lisp.Fn{Name: "tape/new-recorder", Func: newRecorderLisp})
	env.Set("tape/start!", &lisp.Fn{Name: "tape/start!", Func: startLisp})
	env.Set("tape/stop!", &lisp.Fn{Name: "tape/stop!", Func: stopLisp})
	env.Set("tape/save!", &lisp.Fn{Name: "tape/save!", Func: saveLisp})
	env.Set("tape/load", &lisp.Fn{Name: "tape/load", Func: loadLisp})
	env.Set("tape/play!", &lisp.Fn{Name: "tape/play!", Func: playLisp})
	env.Set("tape/merge", &lisp.Fn{Name: "tape/merge", Func: mergeLisp})
	env.Set("tape/info", &lisp.Fn{Name: "tape/info", Func: infoLisp})
	env.Set("tape/serve!", &lisp.Fn{Name: "tape/serve!", Func: serveLisp})
	env.Set("tape/connect!", &lisp.Fn{Name: "tape/connect!", Func: connectLisp})
	env.Set("tape/peers", &lisp.Fn{Name: "tape/peers", Func: peersLisp})
}

// ptyCaptureFunc returns a CaptureFunc that reads the terminal via `tput` and stty.
func ptyCaptureFunc() CaptureFunc {
	return func() (string, int, int, error) {
		// Get terminal size
		w, h := 80, 24
		if cols, err := exec.Command("tput", "cols").Output(); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(cols)), "%d", &w)
		}
		if rows, err := exec.Command("tput", "lines").Output(); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(rows)), "%d", &h)
		}

		// Capture via ANSI DSR (device status report) or screen dump
		// For now, use a simple shell-based capture
		out, err := exec.Command("sh", "-c",
			"script -q /dev/null sh -c 'clear && cat /dev/stdin' < /dev/null 2>/dev/null || echo '[terminal capture unavailable]'",
		).Output()
		if err != nil {
			return "[capture error]", w, h, nil
		}
		return string(out), w, h, nil
	}
}

// sshCaptureFunc returns a CaptureFunc that captures a remote terminal via SSH.
func sshCaptureFunc(host string) CaptureFunc {
	return func() (string, int, int, error) {
		out, err := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=2",
			host, "TERM=dumb script -q /dev/null sh -c 'tput cols; tput lines; cat /dev/null'",
		).Output()
		if err != nil {
			return fmt.Sprintf("[ssh %s: %v]", host, err), 80, 24, nil
		}

		lines := strings.SplitN(string(out), "\n", 3)
		w, h := 80, 24
		if len(lines) >= 2 {
			fmt.Sscanf(lines[0], "%d", &w)
			fmt.Sscanf(lines[1], "%d", &h)
		}
		content := ""
		if len(lines) >= 3 {
			content = lines[2]
		}
		return content, w, h, nil
	}
}

func newRecorderLisp(args []lisp.Value) lisp.Value {
	nodeID := "local"
	label := "session"

	if len(args) > 0 {
		nodeID = extractString(args[0])
	}
	if len(args) > 1 {
		label = extractString(args[1])
	}

	var capFn CaptureFunc

	// If a third arg is provided, treat it as an SSH host for remote recording
	if len(args) > 2 {
		host := extractString(args[2])
		capFn = sshCaptureFunc(host)
	} else {
		capFn = ptyCaptureFunc()
	}

	rec := NewRecorder(nodeID, label, capFn)
	return &lisp.ExternalValue{Value: rec, Type: "TapeRecorder"}
}

func startLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/start! requires a recorder")
	}
	rec := args[0].(*lisp.ExternalValue).Value.(*Recorder)
	if err := rec.Start(); err != nil {
		panic(fmt.Sprintf("tape/start!: %v", err))
	}
	return lisp.Bool(true)
}

func stopLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/stop! requires a recorder")
	}
	rec := args[0].(*lisp.ExternalValue).Value.(*Recorder)
	tape := rec.Stop()
	return &lisp.ExternalValue{Value: tape, Type: "Tape"}
}

func saveLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("tape/save! requires a tape and path")
	}
	tape := args[0].(*lisp.ExternalValue).Value.(*Tape)
	path := extractString(args[1])
	if err := tape.SaveJSONL(path); err != nil {
		panic(fmt.Sprintf("tape/save!: %v", err))
	}
	return lisp.String(path)
}

func loadLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/load requires a path")
	}
	path := extractString(args[0])
	tape, err := LoadJSONL(path)
	if err != nil {
		panic(fmt.Sprintf("tape/load: %v", err))
	}
	return &lisp.ExternalValue{Value: tape, Type: "Tape"}
}

func playLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/play! requires a tape")
	}
	tape := args[0].(*lisp.ExternalValue).Value.(*Tape)

	speed := 1.0
	if len(args) > 1 {
		switch v := args[1].(type) {
		case lisp.Int:
			speed = float64(v)
		case lisp.Float:
			speed = float64(v)
		}
	}

	player := NewPlayer(tape, os.Stdout)
	player.SetSpeed(speed)
	stop := make(chan struct{})
	player.Play(stop)
	return lisp.Int(int64(tape.Len()))
}

func mergeLisp(args []lisp.Value) lisp.Value {
	var tapes []*Tape
	for _, a := range args {
		t := a.(*lisp.ExternalValue).Value.(*Tape)
		tapes = append(tapes, t)
	}
	merged := MergeTapes(tapes...)
	return &lisp.ExternalValue{Value: merged, Type: "Tape"}
}

func infoLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/info requires a tape or recorder")
	}

	ext := args[0].(*lisp.ExternalValue)
	switch v := ext.Value.(type) {
	case *Tape:
		return lisp.HashMap{
			lisp.Keyword("node"):     lisp.String(v.NodeID),
			lisp.Keyword("label"):    lisp.String(v.Label),
			lisp.Keyword("frames"):   lisp.Int(int64(v.Len())),
			lisp.Keyword("duration"): lisp.String(v.Duration().String()),
		}
	case *Recorder:
		t := v.Tape()
		return lisp.HashMap{
			lisp.Keyword("node"):     lisp.String(t.NodeID),
			lisp.Keyword("label"):    lisp.String(t.Label),
			lisp.Keyword("frames"):   lisp.Int(int64(t.Len())),
			lisp.Keyword("lamport"):  lisp.Int(int64(v.clock.Now())),
			lisp.Keyword("duration"): lisp.String(t.Duration().String()),
		}
	default:
		panic("tape/info: expected tape or recorder")
	}
}

// --- Network ---

var activeServer *Server

func serveLisp(args []lisp.Value) lisp.Value {
	addr := ":0"
	if len(args) > 0 {
		addr = extractString(args[0])
	}

	if len(args) < 2 {
		panic("tape/serve! requires an address and a recorder")
	}
	rec := args[1].(*lisp.ExternalValue).Value.(*Recorder)

	srv, err := NewServer(addr, rec)
	if err != nil {
		panic(fmt.Sprintf("tape/serve!: %v", err))
	}

	activeServer = srv
	go srv.Serve()

	return lisp.String(srv.Addr())
}

func connectLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("tape/connect! requires server address and recorder")
	}
	addr := extractString(args[0])
	rec := args[1].(*lisp.ExternalValue).Value.(*Recorder)

	nodeID := rec.tape.NodeID
	label := rec.tape.Label

	client, err := Dial(addr, nodeID, label, rec)
	if err != nil {
		panic(fmt.Sprintf("tape/connect!: %v", err))
	}

	return &lisp.ExternalValue{Value: client, Type: "TapeClient"}
}

func peersLisp(args []lisp.Value) lisp.Value {
	if activeServer == nil {
		return lisp.Int(0)
	}
	return lisp.Int(int64(activeServer.PeerCount()))
}

func extractString(v lisp.Value) string {
	switch s := v.(type) {
	case lisp.String:
		return string(s)
	case lisp.Symbol:
		return string(s)
	case lisp.Keyword:
		return string(s)
	default:
		return fmt.Sprintf("%v", s)
	}
}
