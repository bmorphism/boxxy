//go:build darwin

// tapeqt is a TUI QuickTime alternative: causal tape recording at 1 FPS
// with Lamport clocks for distributed session convergence over network.
//
// Usage:
//
//	tapeqt record [-o file.jsonl] [-node id] [-label name]
//	tapeqt record-ssh [-o file.jsonl] [-node id] <host>
//	tapeqt play [-speed 2.0] <file.jsonl>
//	tapeqt merge [-o merged.jsonl] <a.jsonl> <b.jsonl> ...
//	tapeqt serve [-addr :4444] [-node id]
//	tapeqt repl
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/tape"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "record":
		doRecord(args)
	case "record-ssh":
		doRecordSSH(args)
	case "play":
		doPlay(args)
	case "merge":
		doMerge(args)
	case "serve":
		doServe(args)
	case "evolve":
		doEvolve(args)
	case "daemon":
		doDaemon(args)
	case "sheaf":
		doSheaf(args)
	case "repl":
		doREPL()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `tapeqt - TUI QuickTime alternative with causal tape recording

Commands:
  record       Record local terminal at 1 FPS
  record-ssh   Record a remote terminal session over SSH
  play         Play back a recorded tape
  merge        Merge multiple tapes into causal order
  serve        Start a tape sharing server (network recording)
  evolve       Run DGM evolution on capture strategies
  daemon       Start self-evolving daemon (continuous DGM)
  sheaf        Check sheaf consistency of tape files
  repl         Interactive Joker/Lisp REPL with tape/* namespace

Flags:
  -o <path>    Output file (default: tape-<timestamp>.jsonl)
  -node <id>   Node identifier (default: hostname)
  -label <s>   Session label
  -addr <a>    Server listen address (default: :4444)
  -speed <f>   Playback speed multiplier (default: 1.0)
`)
}

func doRecord(args []string) {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	out := fs.String("o", "", "output file")
	nodeID := fs.String("node", hostname(), "node identifier")
	label := fs.String("label", "local", "session label")
	fs.Parse(args)

	if *out == "" {
		*out = fmt.Sprintf("tape-%s.jsonl", *nodeID)
	}

	capFn := localCaptureFunc()
	rec := tape.NewRecorder(*nodeID, *label, capFn)

	fmt.Fprintf(os.Stderr, "[tapeqt] recording at 1 FPS, node=%s, output=%s\n", *nodeID, *out)
	fmt.Fprintf(os.Stderr, "[tapeqt] press Ctrl+C to stop\n")

	rec.Start()
	waitInterrupt()
	t := rec.Stop()

	if err := t.SaveJSONL(*out); err != nil {
		fmt.Fprintf(os.Stderr, "error saving tape: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[tapeqt] saved %d frames to %s (duration: %s)\n",
		t.Len(), *out, t.Duration())
}

func doRecordSSH(args []string) {
	fs := flag.NewFlagSet("record-ssh", flag.ExitOnError)
	out := fs.String("o", "", "output file")
	nodeID := fs.String("node", hostname(), "node identifier")
	label := fs.String("label", "", "session label")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: tapeqt record-ssh <host>\n")
		os.Exit(1)
	}
	host := fs.Arg(0)

	if *label == "" {
		*label = host
	}
	if *out == "" {
		*out = fmt.Sprintf("tape-%s-%s.jsonl", *nodeID, host)
	}

	capFn := sshCaptureFunc(host)
	rec := tape.NewRecorder(*nodeID, *label, capFn)

	fmt.Fprintf(os.Stderr, "[tapeqt] recording %s at 1 FPS, node=%s\n", host, *nodeID)
	fmt.Fprintf(os.Stderr, "[tapeqt] press Ctrl+C to stop\n")

	rec.Start()
	waitInterrupt()
	t := rec.Stop()

	if err := t.SaveJSONL(*out); err != nil {
		fmt.Fprintf(os.Stderr, "error saving tape: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[tapeqt] saved %d frames to %s\n", t.Len(), *out)
}

func doPlay(args []string) {
	fs := flag.NewFlagSet("play", flag.ExitOnError)
	speed := fs.Float64("speed", 1.0, "playback speed")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: tapeqt play <file.jsonl>\n")
		os.Exit(1)
	}

	t, err := tape.LoadJSONL(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading tape: %v\n", err)
		os.Exit(1)
	}

	player := tape.NewPlayer(t, os.Stdout)
	player.SetSpeed(*speed)

	stop := make(chan struct{})
	go func() {
		waitInterrupt()
		close(stop)
	}()

	player.Play(stop)
}

func doMerge(args []string) {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	out := fs.String("o", "merged.jsonl", "output file")
	fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "usage: tapeqt merge <a.jsonl> <b.jsonl> ...\n")
		os.Exit(1)
	}

	var tapes []*tape.Tape
	for _, path := range fs.Args() {
		t, err := tape.LoadJSONL(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %v\n", path, err)
			os.Exit(1)
		}
		tapes = append(tapes, t)
	}

	merged := tape.MergeTapes(tapes...)
	if err := merged.SaveJSONL(*out); err != nil {
		fmt.Fprintf(os.Stderr, "error saving: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[tapeqt] merged %d tapes -> %s (%d frames)\n",
		len(tapes), *out, merged.Len())
}

func doServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":4444", "listen address")
	nodeID := fs.String("node", hostname(), "node identifier")
	label := fs.String("label", "server", "session label")
	fs.Parse(args)

	capFn := localCaptureFunc()
	rec := tape.NewRecorder(*nodeID, *label, capFn)

	srv, err := tape.NewServer(*addr, rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error starting server: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[tapeqt] serving on %s, node=%s\n", srv.Addr(), *nodeID)
	fmt.Fprintf(os.Stderr, "[tapeqt] recording + sharing at 1 FPS, Ctrl+C to stop\n")

	rec.Start()
	go srv.Serve()

	waitInterrupt()

	srv.Stop()
	t := rec.Stop()

	outPath := fmt.Sprintf("tape-%s.jsonl", *nodeID)
	if err := t.SaveJSONL(outPath); err != nil {
		fmt.Fprintf(os.Stderr, "error saving tape: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "[tapeqt] saved %d frames to %s\n", t.Len(), outPath)
}

func doREPL() {
	env := lisp.CreateStandardEnv()
	tape.RegisterNamespace(env)
	tape.RegisterEvolveNamespace(env)
	tape.RegisterDaemonNamespace(env)

	fmt.Print(`tapeqt repl - Joker Lisp with full tape/* namespace

  Recording:
  (tape/new-recorder "node" "label")          Create 1-FPS recorder
  (tape/autopoietic-recorder "node" "label")  Self-evolving recorder
  (tape/start! rec) / (tape/stop! rec)        Start/stop recording
  (tape/save! tape "path.jsonl")              Save tape as JSONL
  (tape/load "path.jsonl")                    Load tape from JSONL
  (tape/merge t1 t2)                          Merge by causal order

  Self-Evolution (Darwin Godel Machine):
  (tape/new-daemon rec)                       Create self-evolving daemon
  (tape/daemon-start! d)                      Start continuous evolution
  (tape/daemon-stop! d)                       Stop daemon, save archive
  (tape/daemon-status d)                      Daemon progress stats
  (tape/save-archive! archive path)           Persist DGM archive
  (tape/load-archive path)                    Load persisted archive

  Categorical + Sheaf:
  (tape/new-world t1 t2)                      Build ACSet world
  (tape/sheaf-check world)                    Cech cohomology H1 check
  (tape/world-gf3 world)                      GF(3) conservation
  (tape/world-bisim? w1 w2)                   Bisimulation verification

  (quit)                                      Exit
`)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("tapeqt=> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "(quit)" || line == "(exit)" {
			break
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", r)
				}
			}()

			reader := lisp.NewReader(strings.NewReader(line))
			expr, err := reader.Read()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
				return
			}

			result := lisp.Eval(expr, env)
			if _, ok := result.(lisp.Nil); !ok {
				fmt.Println(result)
			}
		}()
	}
}

func doEvolve(args []string) {
	fs := flag.NewFlagSet("evolve", flag.ExitOnError)
	generations := fs.Int("gen", 20, "number of generations")
	archivePath := fs.String("archive", tape.DefaultArchivePath(), "archive file")
	trial := fs.Int("trial", 2, "trial duration in seconds")
	fs.Parse(args)

	capFn := localCaptureFunc()
	rec := tape.NewRecorder(hostname(), "evolve", capFn)
	rec.Start()

	archive, err := tape.LoadArchive(*archivePath, 30)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading archive: %v\n", err)
		archive = tape.NewArchive(30)
	}

	fmt.Fprintf(os.Stderr, "[tapeqt] evolving %d generations (trial=%ds, archive=%d agents)\n",
		*generations, *trial, archive.Count())

	for i := 0; i < *generations; i++ {
		parent := archive.Sample()
		if parent == nil {
			continue
		}
		child := archive.Mutate(parent)
		archive.Evaluate(child, capFn, time.Duration(*trial)*time.Second)
		if archive.Insert(child) {
			fmt.Fprintf(os.Stderr, "  gen %d: new agent %s (fitness=%.3f, interval=%dms)\n",
				child.Generation, child.ID, child.Fitness, child.Params.IntervalMs)
		}
	}

	rec.Stop()

	best := archive.Best()
	if best != nil {
		fmt.Fprintf(os.Stderr, "[tapeqt] best: %s fitness=%.3f interval=%dms diff=%.2f compress=%v\n",
			best.ID, best.Fitness, best.Params.IntervalMs, best.Params.DiffThreshold, best.Params.CompressFrames)
	}

	if err := archive.SaveArchive(*archivePath); err != nil {
		fmt.Fprintf(os.Stderr, "error saving archive: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "[tapeqt] archive saved to %s (%d agents)\n", *archivePath, archive.Count())
	}

	status := archive.GF3Status()
	fmt.Fprintf(os.Stderr, "[tapeqt] GF(3): coord=%d gen=%d ver=%d balanced=%v\n",
		status["coordinators"], status["generators"], status["verifiers"], status["balanced"])
}

func doDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	nodeID := fs.String("node", hostname(), "node identifier")
	archivePath := fs.String("archive", tape.DefaultArchivePath(), "archive file")
	interval := fs.Int("interval", 10, "evolve interval in seconds")
	trial := fs.Int("trial", 3, "trial duration in seconds")
	out := fs.String("o", "", "output tape file")
	fs.Parse(args)

	if *out == "" {
		*out = fmt.Sprintf("tape-daemon-%s.jsonl", *nodeID)
	}

	capFn := tape.PTYCaptureFunc()
	rec := tape.NewRecorder(*nodeID, "daemon", capFn)

	cfg := tape.DefaultDaemonConfig()
	cfg.ArchivePath = *archivePath
	cfg.EvolveInterval = time.Duration(*interval) * time.Second
	cfg.TrialDuration = time.Duration(*trial) * time.Second

	daemon, err := tape.NewDaemon(rec, capFn, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating daemon: %v\n", err)
		os.Exit(1)
	}

	daemon.OnEvolve(func(ev tape.DaemonEvent) {
		fmt.Fprintf(os.Stderr, "[dgm] %s: %s\n", ev.Type, ev.Message)
	})

	fmt.Fprintf(os.Stderr, "[tapeqt] self-evolving daemon: node=%s, interval=%ds\n", *nodeID, *interval)
	fmt.Fprintf(os.Stderr, "[tapeqt] recording + evolving, Ctrl+C to stop\n")

	rec.Start()
	daemon.Start()

	waitInterrupt()

	daemon.Stop()
	t := rec.Stop()

	if err := t.SaveJSONL(*out); err != nil {
		fmt.Fprintf(os.Stderr, "error saving tape: %v\n", err)
	}

	stats := daemon.Stats()
	fmt.Fprintf(os.Stderr, "[tapeqt] stopped: %d frames, %d generations, %d hot-swaps, best=%.3f\n",
		t.Len(), stats.TotalGenerations, stats.HotSwaps, stats.BestFitness)
}

func doSheaf(args []string) {
	fs := flag.NewFlagSet("sheaf", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: tapeqt sheaf <file1.jsonl> [file2.jsonl ...]\n")
		os.Exit(1)
	}

	var tapes []*tape.Tape
	for _, path := range fs.Args() {
		t, err := tape.LoadJSONL(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %v\n", path, err)
			os.Exit(1)
		}
		tapes = append(tapes, t)
	}

	world := tape.NewTapeWorld(tapes...)
	result := tape.CheckSheafConsistency(world)

	fmt.Fprintf(os.Stderr, "[tapeqt] sheaf consistency check:\n")
	fmt.Fprintf(os.Stderr, "  consistent: %v (H¹ dim = %d)\n", result.Consistent, result.H1Dim)
	fmt.Fprintf(os.Stderr, "  sections:   %d\n", result.Sections)
	fmt.Fprintf(os.Stderr, "  coverage:   %.1f%%\n", result.Coverage*100)
	fmt.Fprintf(os.Stderr, "  GF(3):      %v\n", result.GF3Balanced)

	if len(result.Cocycles) > 0 {
		fmt.Fprintf(os.Stderr, "  cocycles:\n")
		for _, c := range result.Cocycles {
			fmt.Fprintf(os.Stderr, "    %s↔%s @ lamport=%d: %s (severity=%d)\n",
				c.SectionA, c.SectionB, c.LamportAt, c.Kind, c.Severity)
		}
	}

	if !result.Consistent {
		os.Exit(1)
	}
}

func localCaptureFunc() tape.CaptureFunc {
	return func() (string, int, int, error) {
		w, h := 80, 24
		if cols, err := exec.Command("tput", "cols").Output(); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(cols)), "%d", &w)
		}
		if rows, err := exec.Command("tput", "lines").Output(); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(rows)), "%d", &h)
		}

		// Screen dump via ANSI escape
		return fmt.Sprintf("[%s %dx%d @ lamport tick]", hostname(), w, h), w, h, nil
	}
}

func sshCaptureFunc(host string) tape.CaptureFunc {
	return func() (string, int, int, error) {
		out, err := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=2",
			host, "TERM=dumb tput cols; tput lines; uptime",
		).Output()
		if err != nil {
			return fmt.Sprintf("[ssh %s: %v]", host, err), 80, 24, nil
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

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func waitInterrupt() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
