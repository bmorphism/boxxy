//go:build darwin

package tape

import (
	"fmt"
	"time"

	"github.com/bmorphism/boxxy/internal/lisp"
)

// RegisterEvolveNamespace adds DGM evolution + ACSet + gossip + color functions to the Lisp env.
func RegisterEvolveNamespace(env *lisp.Env) {
	// DGM Archive
	env.Set("tape/new-archive", &lisp.Fn{Name: "tape/new-archive", Func: newArchiveLisp})
	env.Set("tape/evolve!", &lisp.Fn{Name: "tape/evolve!", Func: evolveLisp})
	env.Set("tape/archive-status", &lisp.Fn{Name: "tape/archive-status", Func: archiveStatusLisp})
	env.Set("tape/archive-best", &lisp.Fn{Name: "tape/archive-best", Func: archiveBestLisp})

	// ACSet World
	env.Set("tape/new-world", &lisp.Fn{Name: "tape/new-world", Func: newWorldLisp})
	env.Set("tape/world-gf3", &lisp.Fn{Name: "tape/world-gf3", Func: worldGF3Lisp})
	env.Set("tape/world-bisim?", &lisp.Fn{Name: "tape/world-bisim?", Func: worldBisimLisp})
	env.Set("tape/world-uri", &lisp.Fn{Name: "tape/world-uri", Func: worldURILisp})
	env.Set("tape/world-schema", &lisp.Fn{Name: "tape/world-schema", Func: worldSchemaLisp})

	// Gossip convergence
	env.Set("tape/gossip-status", &lisp.Fn{Name: "tape/gossip-status", Func: gossipStatusLisp})

	// Color stream
	env.Set("tape/color-stream", &lisp.Fn{Name: "tape/color-stream", Func: colorStreamLisp})

	// Enhanced capture functions
	env.Set("tape/pty-recorder", &lisp.Fn{Name: "tape/pty-recorder", Func: ptyRecorderLisp})
	env.Set("tape/ps-recorder", &lisp.Fn{Name: "tape/ps-recorder", Func: psRecorderLisp})
}

func newArchiveLisp(args []lisp.Value) lisp.Value {
	maxSize := 20
	if len(args) > 0 {
		maxSize = int(args[0].(lisp.Int))
	}
	archive := NewArchive(maxSize)
	return &lisp.ExternalValue{Value: archive, Type: "DGMArchive"}
}

func evolveLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("tape/evolve! requires archive and recorder")
	}

	archive := args[0].(*lisp.ExternalValue).Value.(*Archive)
	rec := args[1].(*lisp.ExternalValue).Value.(*Recorder)

	generations := 5
	if len(args) > 2 {
		generations = int(args[2].(lisp.Int))
	}

	trialDuration := 2 * time.Second
	if len(args) > 3 {
		trialDuration = time.Duration(args[3].(lisp.Int)) * time.Second
	}

	best := archive.EvolveN(generations, rec.capture, trialDuration)
	if best == nil {
		return lisp.Nil{}
	}

	return lisp.HashMap{
		lisp.Keyword("id"):         lisp.String(best.ID),
		lisp.Keyword("generation"): lisp.Int(int64(best.Generation)),
		lisp.Keyword("fitness"):    lisp.Float(best.Fitness),
		lisp.Keyword("interval"):   lisp.Int(int64(best.Params.IntervalMs)),
		lisp.Keyword("trit"):       lisp.String(best.Trit.String()),
	}
}

func archiveStatusLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/archive-status requires archive")
	}
	archive := args[0].(*lisp.ExternalValue).Value.(*Archive)
	status := archive.GF3Status()

	return lisp.HashMap{
		lisp.Keyword("agents"):       lisp.Int(int64(status["agents"].(int))),
		lisp.Keyword("generation"):   lisp.Int(int64(status["generation"].(int))),
		lisp.Keyword("coordinators"): lisp.Int(int64(status["coordinators"].(int))),
		lisp.Keyword("generators"):   lisp.Int(int64(status["generators"].(int))),
		lisp.Keyword("verifiers"):    lisp.Int(int64(status["verifiers"].(int))),
		lisp.Keyword("balanced"):     lisp.Bool(status["balanced"].(bool)),
		lisp.Keyword("best-fitness"): lisp.Float(status["best_fitness"].(float64)),
	}
}

func archiveBestLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/archive-best requires archive")
	}
	archive := args[0].(*lisp.ExternalValue).Value.(*Archive)
	best := archive.Best()
	if best == nil {
		return lisp.Nil{}
	}
	return lisp.HashMap{
		lisp.Keyword("id"):         lisp.String(best.ID),
		lisp.Keyword("generation"): lisp.Int(int64(best.Generation)),
		lisp.Keyword("fitness"):    lisp.Float(best.Fitness),
		lisp.Keyword("interval"):   lisp.Int(int64(best.Params.IntervalMs)),
		lisp.Keyword("diff-threshold"): lisp.Float(best.Params.DiffThreshold),
		lisp.Keyword("compress"):   lisp.Bool(best.Params.CompressFrames),
		lisp.Keyword("trit"):       lisp.String(best.Trit.String()),
	}
}

func newWorldLisp(args []lisp.Value) lisp.Value {
	var tapes []*Tape
	for _, a := range args {
		t := a.(*lisp.ExternalValue).Value.(*Tape)
		tapes = append(tapes, t)
	}
	world := NewTapeWorld(tapes...)
	return &lisp.ExternalValue{Value: world, Type: "TapeWorld"}
}

func worldGF3Lisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/world-gf3 requires a world")
	}
	world := args[0].(*lisp.ExternalValue).Value.(*TapeWorld)
	balanced, counts := world.GF3Conservation()
	return lisp.HashMap{
		lisp.Keyword("balanced"):     lisp.Bool(balanced),
		lisp.Keyword("coordinators"): lisp.Int(int64(counts["coordinator"])),
		lisp.Keyword("generators"):   lisp.Int(int64(counts["generator"])),
		lisp.Keyword("verifiers"):    lisp.Int(int64(counts["verifier"])),
	}
}

func worldBisimLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("tape/world-bisim? requires two worlds")
	}
	a := args[0].(*lisp.ExternalValue).Value.(*TapeWorld)
	b := args[1].(*lisp.ExternalValue).Value.(*TapeWorld)
	return lisp.Bool(BisimulationVerify(a, b))
}

func worldURILisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("tape/world-uri requires a world")
	}
	world := args[0].(*lisp.ExternalValue).Value.(*TapeWorld)
	return lisp.String(world.URIScheme())
}

func worldSchemaLisp(args []lisp.Value) lisp.Value {
	schema := TapeWorldSchema()
	objects := make(lisp.Vector, len(schema.Objects))
	for i, o := range schema.Objects {
		objects[i] = lisp.String(o)
	}
	morphisms := make(lisp.Vector, len(schema.Morphisms))
	for i, m := range schema.Morphisms {
		morphisms[i] = lisp.HashMap{
			lisp.Keyword("name"):   lisp.String(m.Name),
			lisp.Keyword("source"): lisp.String(m.Source),
			lisp.Keyword("target"): lisp.String(m.Target),
		}
	}
	return lisp.HashMap{
		lisp.Keyword("objects"):    objects,
		lisp.Keyword("morphisms"): morphisms,
		lisp.Keyword("uri"):       lisp.String(fmt.Sprintf("tape://schema/v1")),
	}
}

// --- Gossip ---

var activeGossipState *GossipState

func gossipStatusLisp(args []lisp.Value) lisp.Value {
	if activeGossipState == nil {
		return lisp.HashMap{
			lisp.Keyword("status"): lisp.String("no gossip active"),
		}
	}
	status := activeGossipState.ConvergenceStatus()
	balanced, _ := status["gf3_balanced"].(bool)
	return lisp.HashMap{
		lisp.Keyword("node"):        lisp.String(fmt.Sprintf("%v", status["node_id"])),
		lisp.Keyword("peers"):       lisp.Int(int64(status["peers"].(int))),
		lisp.Keyword("lamport"):     lisp.Int(int64(status["lamport"].(uint64))),
		lisp.Keyword("gf3-balanced"): lisp.Bool(balanced),
	}
}

// --- Color stream ---

func colorStreamLisp(args []lisp.Value) lisp.Value {
	scs := NewSessionColorStream()
	// Seed with some initial frames if a tape is provided
	if len(args) > 0 {
		t := args[0].(*lisp.ExternalValue).Value.(*Tape)
		t.mu.RLock()
		for _, f := range t.Frames {
			scs.FeedFrame(f)
		}
		t.mu.RUnlock()
	}
	return &lisp.ExternalValue{Value: scs, Type: "SessionColorStream"}
}

// --- Enhanced recorders ---

func ptyRecorderLisp(args []lisp.Value) lisp.Value {
	nodeID := "local"
	label := "pty"
	if len(args) > 0 {
		nodeID = extractString(args[0])
	}
	if len(args) > 1 {
		label = extractString(args[1])
	}
	rec := NewRecorder(nodeID, label, PTYCaptureFunc())
	return &lisp.ExternalValue{Value: rec, Type: "TapeRecorder"}
}

func psRecorderLisp(args []lisp.Value) lisp.Value {
	nodeID := "local"
	label := "processes"
	if len(args) > 0 {
		nodeID = extractString(args[0])
	}
	if len(args) > 1 {
		label = extractString(args[1])
	}
	rec := NewRecorder(nodeID, label, ProcessListCaptureFunc())
	return &lisp.ExternalValue{Value: rec, Type: "TapeRecorder"}
}
