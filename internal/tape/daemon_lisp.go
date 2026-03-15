//go:build darwin

package tape

import (
	"fmt"
	"time"

	"github.com/bmorphism/boxxy/internal/lisp"
)

// RegisterDaemonNamespace adds self-evolving daemon + persistence + sheaf functions.
func RegisterDaemonNamespace(env *lisp.Env) {
	// Archive persistence
	env.Set("tape/save-archive!", &lisp.Fn{Name: "tape/save-archive!", Func: saveArchiveLisp})
	env.Set("tape/load-archive", &lisp.Fn{Name: "tape/load-archive", Func: loadArchiveLisp})

	// Self-evolving daemon
	env.Set("tape/new-daemon", &lisp.Fn{Name: "tape/new-daemon", Func: newDaemonLisp})
	env.Set("tape/daemon-start!", &lisp.Fn{Name: "tape/daemon-start!", Func: daemonStartLisp})
	env.Set("tape/daemon-stop!", &lisp.Fn{Name: "tape/daemon-stop!", Func: daemonStopLisp})
	env.Set("tape/daemon-status", &lisp.Fn{Name: "tape/daemon-status", Func: daemonStatusLisp})

	// Autopoietic recorder
	env.Set("tape/autopoietic-recorder", &lisp.Fn{Name: "tape/autopoietic-recorder", Func: autopoieticRecorderLisp})
	env.Set("tape/autopoietic-start!", &lisp.Fn{Name: "tape/autopoietic-start!", Func: autopoieticStartLisp})
	env.Set("tape/autopoietic-stop!", &lisp.Fn{Name: "tape/autopoietic-stop!", Func: autopoieticStopLisp})
	env.Set("tape/autopoietic-status", &lisp.Fn{Name: "tape/autopoietic-status", Func: autopoieticStatusLisp})

	// Sheaf consistency
	env.Set("tape/sheaf-check", &lisp.Fn{Name: "tape/sheaf-check", Func: sheafCheckLisp})
}

// --- Archive persistence ---

func saveArchiveLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/save-archive! requires archive")
	}
	archive := args[0].(*lisp.ExternalValue).Value.(*Archive)

	path := DefaultArchivePath()
	if len(args) > 1 {
		path = extractString(args[1])
	}

	if err := archive.SaveArchive(path); err != nil {
		panic(fmt.Sprintf("tape/save-archive!: %v", err))
	}

	return lisp.HashMap{
		lisp.Keyword("path"):   lisp.String(path),
		lisp.Keyword("agents"): lisp.Int(int64(archive.Count())),
		lisp.Keyword("gen"):    lisp.Int(int64(archive.Generation())),
	}
}

func loadArchiveLisp(args []lisp.Value) lisp.Value {
	path := DefaultArchivePath()
	if len(args) > 0 {
		path = extractString(args[0])
	}

	maxSize := 30
	if len(args) > 1 {
		maxSize = int(args[1].(lisp.Int))
	}

	archive, err := LoadArchive(path, maxSize)
	if err != nil {
		panic(fmt.Sprintf("tape/load-archive: %v", err))
	}

	return &lisp.ExternalValue{Value: archive, Type: "DGMArchive"}
}

// --- Daemon ---

func newDaemonLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/new-daemon requires a recorder")
	}

	rec := args[0].(*lisp.ExternalValue).Value.(*Recorder)
	cfg := DefaultDaemonConfig()

	if len(args) > 1 {
		// Optional config overrides via hashmap
		if hm, ok := args[1].(lisp.HashMap); ok {
			if v, exists := hm[lisp.Keyword("interval")]; exists {
				cfg.EvolveInterval = time.Duration(v.(lisp.Int)) * time.Second
			}
			if v, exists := hm[lisp.Keyword("trial")]; exists {
				cfg.TrialDuration = time.Duration(v.(lisp.Int)) * time.Second
			}
			if v, exists := hm[lisp.Keyword("max-gen")]; exists {
				cfg.MaxGenerations = int(v.(lisp.Int))
			}
			if v, exists := hm[lisp.Keyword("archive-path")]; exists {
				cfg.ArchivePath = extractString(v)
			}
		}
	}

	daemon, err := NewDaemon(rec, rec.capture, cfg)
	if err != nil {
		panic(fmt.Sprintf("tape/new-daemon: %v", err))
	}

	return &lisp.ExternalValue{Value: daemon, Type: "DGMDaemon"}
}

func daemonStartLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/daemon-start! requires a daemon")
	}
	daemon := args[0].(*lisp.ExternalValue).Value.(*Daemon)
	if err := daemon.Start(); err != nil {
		panic(fmt.Sprintf("tape/daemon-start!: %v", err))
	}
	return lisp.Bool(true)
}

func daemonStopLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/daemon-stop! requires a daemon")
	}
	daemon := args[0].(*lisp.ExternalValue).Value.(*Daemon)
	daemon.Stop()
	stats := daemon.Stats()
	return lisp.HashMap{
		lisp.Keyword("generations"):  lisp.Int(int64(stats.TotalGenerations)),
		lisp.Keyword("hot-swaps"):    lisp.Int(int64(stats.HotSwaps)),
		lisp.Keyword("saves"):        lisp.Int(int64(stats.TotalSaves)),
		lisp.Keyword("best-fitness"): lisp.Float(stats.BestFitness),
		lisp.Keyword("best-agent"):   lisp.String(stats.BestAgentID),
	}
}

func daemonStatusLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/daemon-status requires a daemon")
	}
	daemon := args[0].(*lisp.ExternalValue).Value.(*Daemon)
	stats := daemon.Stats()
	return lisp.HashMap{
		lisp.Keyword("generations"):  lisp.Int(int64(stats.TotalGenerations)),
		lisp.Keyword("hot-swaps"):    lisp.Int(int64(stats.HotSwaps)),
		lisp.Keyword("saves"):        lisp.Int(int64(stats.TotalSaves)),
		lisp.Keyword("best-fitness"): lisp.Float(stats.BestFitness),
		lisp.Keyword("best-agent"):   lisp.String(stats.BestAgentID),
		lisp.Keyword("uptime"):       lisp.String(stats.Uptime.String()),
	}
}

// --- Autopoietic recorder ---

func autopoieticRecorderLisp(args []lisp.Value) lisp.Value {
	nodeID := "local"
	label := "autopoietic"

	if len(args) > 0 {
		nodeID = extractString(args[0])
	}
	if len(args) > 1 {
		label = extractString(args[1])
	}

	cfg := AutopoieticConfig{
		NodeID:    nodeID,
		Label:     label,
		CaptureFn: PTYCaptureFunc(),
		Daemon:    DefaultDaemonConfig(),
	}

	if len(args) > 2 {
		// SSH host
		host := extractString(args[2])
		cfg.CaptureFn = SSHCaptureFunc(host)
	}

	ar, err := NewAutopoieticRecorder(cfg)
	if err != nil {
		panic(fmt.Sprintf("tape/autopoietic-recorder: %v", err))
	}

	return &lisp.ExternalValue{Value: ar, Type: "AutopoieticRecorder"}
}

func autopoieticStartLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/autopoietic-start! requires an autopoietic recorder")
	}
	ar := args[0].(*lisp.ExternalValue).Value.(*AutopoieticRecorder)
	if err := ar.Start(); err != nil {
		panic(fmt.Sprintf("tape/autopoietic-start!: %v", err))
	}
	return lisp.Bool(true)
}

func autopoieticStopLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/autopoietic-stop! requires an autopoietic recorder")
	}
	ar := args[0].(*lisp.ExternalValue).Value.(*AutopoieticRecorder)
	tape := ar.Stop()
	return &lisp.ExternalValue{Value: tape, Type: "Tape"}
}

func autopoieticStatusLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/autopoietic-status requires an autopoietic recorder")
	}
	ar := args[0].(*lisp.ExternalValue).Value.(*AutopoieticRecorder)
	status := ar.Status()

	// Convert to Lisp HashMap
	dStatus := status["daemon"].(map[string]interface{})

	return lisp.HashMap{
		lisp.Keyword("node"):        lisp.String(fmt.Sprintf("%v", status["node_id"])),
		lisp.Keyword("frames"):      lisp.Int(int64(status["frames"].(int))),
		lisp.Keyword("duration"):    lisp.String(status["duration"].(string)),
		lisp.Keyword("color-seed"):  lisp.Int(int64(status["color_seed"].(uint64))),
		lisp.Keyword("generations"): lisp.Int(int64(dStatus["generations"].(int))),
		lisp.Keyword("hot-swaps"):   lisp.Int(int64(dStatus["hot_swaps"].(int))),
		lisp.Keyword("best-fitness"): lisp.Float(dStatus["best_fitness"].(float64)),
	}
}

// --- Sheaf consistency ---

func sheafCheckLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/sheaf-check requires a world")
	}
	world := args[0].(*lisp.ExternalValue).Value.(*TapeWorld)
	result := CheckSheafConsistency(world)

	cocycleVec := make(lisp.Vector, len(result.Cocycles))
	for i, c := range result.Cocycles {
		cocycleVec[i] = lisp.HashMap{
			lisp.Keyword("a"):        lisp.String(c.SectionA),
			lisp.Keyword("b"):        lisp.String(c.SectionB),
			lisp.Keyword("lamport"):  lisp.Int(int64(c.LamportAt)),
			lisp.Keyword("kind"):     lisp.String(c.Kind),
			lisp.Keyword("severity"): lisp.Int(int64(c.Severity)),
		}
	}

	return lisp.HashMap{
		lisp.Keyword("consistent"):   lisp.Bool(result.Consistent),
		lisp.Keyword("h1-dim"):       lisp.Int(int64(result.H1Dim)),
		lisp.Keyword("sections"):     lisp.Int(int64(result.Sections)),
		lisp.Keyword("cocycles"):     cocycleVec,
		lisp.Keyword("gf3-balanced"): lisp.Bool(result.GF3Balanced),
		lisp.Keyword("coverage"):     lisp.Float(result.Coverage),
	}
}
