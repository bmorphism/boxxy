//go:build darwin

// ninja is the unified boxxy binary: tape recording + anti-bullshit verification
// + self-evolving daemon + MCP server — all in one.
//
// Usage:
//
//	ninja record [-o file.jsonl]           Record terminal at 1 FPS
//	ninja analyze "claim text"             CatColab DblTheory analysis
//	ninja manipulate "suspicious text"     10-pattern manipulation check
//	ninja interleave file.jsonl            Interleave tape → epistemic model
//	ninja prove file.jsonl                 Prove conservation + composition + sheaf
//	ninja daemon [-interval 10]            Self-evolving recorder daemon
//	ninja mcp                              Start MCP server on stdio
//	ninja serve [-port 7888]               HTTP MCP server
//	ninja repl                             Interactive Joker REPL
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bmorphism/boxxy/internal/antibullshit"
	"github.com/bmorphism/boxxy/internal/gf3"
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
	case "analyze":
		doAnalyze(args)
	case "manipulate", "manip":
		doManipulate(args)
	case "interleave":
		doInterleave(args)
	case "prove":
		doProve(args)
	case "daemon":
		doDaemon(args)
	case "mcp":
		doMCP()
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
	fmt.Fprintf(os.Stderr, `ninja — unified boxxy: tape + anti-bullshit + self-evolution + MCP

Commands:
  record       Record terminal at 1 FPS with causal timestamps
  analyze      CatColab DblTheory epistemological analysis
  manipulate   Detect manipulation patterns (10 detectors)
  interleave   Convert tape JSONL → verified epistemic model
  prove        Prove GF(3) conservation + path composition + sheaf H¹
  daemon       Self-evolving recorder (DGM hot-swap)
  mcp          MCP server on stdio (JSON-RPC)
  repl         Interactive Joker REPL with all namespaces
`)
}

// --- record ---

func doRecord(args []string) {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	out := fs.String("o", fmt.Sprintf("tape-%s.jsonl", hostname()), "output file")
	nodeID := fs.String("node", hostname(), "node identifier")
	fs.Parse(args)

	capFn := tape.PTYCaptureFunc()
	rec := tape.NewRecorder(*nodeID, "ninja", capFn)

	fmt.Fprintf(os.Stderr, "[ninja] recording at 1 FPS → %s\n", *out)
	rec.Start()
	waitInterrupt()
	t := rec.Stop()

	t.SaveJSONL(*out)
	fmt.Fprintf(os.Stderr, "[ninja] %d frames saved\n", t.Len())
}

// --- analyze ---

func doAnalyze(args []string) {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	framework := fs.String("fw", "pluralistic", "framework: empirical|responsible|harmonic|pluralistic")
	fs.Parse(args)

	text := strings.Join(fs.Args(), " ")
	if text == "" {
		text = readStdin()
	}

	model := antibullshit.AnalyzeWithCatColab(text, *framework)
	h1, cocycles := model.SheafConsistency()
	balanced, counts := model.GF3Balance()

	fmt.Printf("CatColab DblTheory analysis (%s):\n", *framework)
	fmt.Printf("  Objects:    %d\n", len(model.Objects))
	fmt.Printf("  Morphisms:  %d\n", len(model.Morphisms))
	fmt.Printf("  Paths:      %d\n", len(model.Paths))
	fmt.Printf("  Confidence: %.3f\n", model.Confidence)
	fmt.Printf("  Sheaf H¹:   %d (%s)\n", h1, ternary(h1 == 0, "consistent", "contradictions"))
	fmt.Printf("  GF(3):      balanced=%v %v\n", balanced, counts)

	if len(cocycles) > 0 {
		fmt.Printf("  Cocycles:\n")
		for _, c := range cocycles {
			fmt.Printf("    %s (severity=%.2f)\n", c.Kind, c.Severity)
		}
	}
}

// --- manipulate ---

func doManipulate(args []string) {
	text := strings.Join(args, " ")
	if text == "" {
		text = readStdin()
	}

	patterns := antibullshit.DetectManipulation(text)

	if len(patterns) == 0 {
		fmt.Println("clean — no manipulation patterns detected")
		return
	}

	maxSev := 0.0
	for _, p := range patterns {
		if p.Severity > maxSev {
			maxSev = p.Severity
		}
	}

	verdict := "low-risk"
	if maxSev >= 0.7 {
		verdict = "HIGH-RISK"
	} else if maxSev >= 0.4 {
		verdict = "suspicious"
	}

	fmt.Printf("%s — %d patterns detected:\n", verdict, len(patterns))
	for _, p := range patterns {
		fmt.Printf("  [%.1f] %s: %q\n", p.Severity, p.Kind, p.Evidence)
	}
}

// --- interleave ---

func doInterleave(args []string) {
	fs := flag.NewFlagSet("interleave", flag.ExitOnError)
	framework := fs.String("fw", "pluralistic", "framework")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: ninja interleave <tape.jsonl>\n")
		os.Exit(1)
	}

	t, err := tape.LoadJSONL(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Convert tape.Frames to antibullshit.TapeFrames
	var frames []antibullshit.TapeFrame
	for i := 0; i < t.Len(); i++ {
		f, ok := t.At(i)
		if !ok {
			continue
		}
		frames = append(frames, antibullshit.TapeFrame{
			SeqNo: f.SeqNo, LamportTS: f.LamportTS, NodeID: f.NodeID,
			Content: f.Content, Width: f.Width, Height: f.Height,
			Trit: f.Trit, WallTime: f.WallTime,
		})
	}

	cfg := antibullshit.DefaultInterleaveConfig()
	cfg.Framework = *framework
	result := antibullshit.InterleaveTapeFrames(frames, cfg)

	fmt.Printf("Interleave: %d frames → %d accepted, %d skipped\n",
		result.TotalFrames, result.AcceptedFrames, result.SkippedFrames)
	fmt.Printf("  Causal chain: %d links\n", result.CausalChainLen)
	fmt.Printf("  Objects:      %d\n", len(result.Model.Objects))
	fmt.Printf("  Morphisms:    %d\n", len(result.Model.Morphisms))
	fmt.Printf("  Paths:        %d\n", len(result.Model.Paths))
	fmt.Printf("  Confidence:   %.3f\n", result.AvgConfidence)
	fmt.Printf("  Sheaf H¹:     %d\n", result.SheafH1)
	fmt.Printf("  GF(3):        %v\n", result.GF3Balanced)
	fmt.Printf("  Manipulation: %d patterns\n", len(result.ManipPatterns))
}

// --- prove ---

func doProve(args []string) {
	fs := flag.NewFlagSet("prove", flag.ExitOnError)
	framework := fs.String("fw", "pluralistic", "framework")
	fs.Parse(args)

	var model *antibullshit.EpistemicModel

	if fs.NArg() > 0 && strings.HasSuffix(fs.Arg(0), ".jsonl") {
		// Prove a tape file
		t, err := tape.LoadJSONL(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		var frames []antibullshit.TapeFrame
		for i := 0; i < t.Len(); i++ {
			f, _ := t.At(i)
			frames = append(frames, antibullshit.TapeFrame{
				SeqNo: f.SeqNo, LamportTS: f.LamportTS, NodeID: f.NodeID,
				Content: f.Content, Trit: f.Trit, WallTime: f.WallTime,
			})
		}
		cfg := antibullshit.DefaultInterleaveConfig()
		cfg.Framework = *framework
		result := antibullshit.InterleaveTapeFrames(frames, cfg)
		model = result.Model
	} else {
		// Prove a text claim
		text := strings.Join(fs.Args(), " ")
		if text == "" {
			text = readStdin()
		}
		model = antibullshit.AnalyzeWithCatColab(text, *framework)
	}

	allPass := true

	// 1. GF(3) conservation
	fmt.Print("  GF(3) conservation: ")
	if err := antibullshit.ProveConservation(model); err != nil {
		fmt.Printf("FAIL — %v\n", err)
		allPass = false
	} else {
		fmt.Println("PASS")
	}

	// 2. Path composition
	fmt.Print("  Path composition:   ")
	pathErrs := antibullshit.ProvePathComposition(model)
	if len(pathErrs) > 0 {
		fmt.Printf("FAIL — %d non-composable paths\n", len(pathErrs))
		allPass = false
	} else {
		fmt.Printf("PASS (%d paths compose)\n", len(model.Paths))
	}

	// 3. Sheaf consistency
	fmt.Print("  Sheaf H¹:           ")
	cocycles := antibullshit.ProveSheafConsistency(model)
	if len(cocycles) > 0 {
		fmt.Printf("FAIL — H¹=%d cocycles\n", len(cocycles))
		for _, c := range cocycles {
			fmt.Printf("    %s (severity=%.2f)\n", c.Kind, c.Severity)
		}
		allPass = false
	} else {
		fmt.Println("PASS (H¹=0)")
	}

	if allPass {
		fmt.Println("\n  All proofs PASS")
	} else {
		fmt.Println("\n  Some proofs FAILED")
		os.Exit(1)
	}
}

// --- daemon ---

func doDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	nodeID := fs.String("node", hostname(), "node identifier")
	interval := fs.Int("interval", 10, "evolve interval seconds")
	out := fs.String("o", fmt.Sprintf("tape-ninja-%s.jsonl", hostname()), "output tape")
	fs.Parse(args)

	capFn := tape.PTYCaptureFunc()
	rec := tape.NewRecorder(*nodeID, "ninja-daemon", capFn)

	cfg := tape.DefaultDaemonConfig()
	cfg.EvolveInterval = time.Duration(*interval) * time.Second

	daemon, err := tape.NewDaemon(rec, capFn, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	daemon.OnEvolve(func(ev tape.DaemonEvent) {
		fmt.Fprintf(os.Stderr, "[dgm] %s: %s\n", ev.Type, ev.Message)
	})

	fmt.Fprintf(os.Stderr, "[ninja] daemon: node=%s interval=%ds\n", *nodeID, *interval)
	rec.Start()
	daemon.Start()
	waitInterrupt()
	daemon.Stop()
	t := rec.Stop()
	t.SaveJSONL(*out)
	fmt.Fprintf(os.Stderr, "[ninja] %d frames → %s\n", t.Len(), *out)
}

// --- mcp ---

func doMCP() {
	// Reuse antibullshit MCP protocol handler
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	enc := json.NewEncoder(os.Stdout)
	framework := "pluralistic"

	fmt.Fprintln(os.Stderr, "[ninja] MCP server on stdio")

	for scanner.Scan() {
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			enc.Encode(map[string]interface{}{
				"jsonrpc": "2.0", "id": req.ID,
				"result": map[string]interface{}{
					"protocolVersion": "2025-03-26",
					"capabilities":   map[string]interface{}{"tools": map[string]interface{}{"listChanged": true}},
					"serverInfo":     map[string]interface{}{"name": "ninja", "version": "1.0.0"},
				},
			})
		case "notifications/initialized", "ping":
			enc.Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": map[string]interface{}{}})
		case "tools/list":
			enc.Encode(map[string]interface{}{
				"jsonrpc": "2.0", "id": req.ID,
				"result": map[string]interface{}{
					"tools": []interface{}{
						tool("analyze_claim", "CatColab DblTheory epistemological analysis", "text", "framework"),
						tool("validate_sources", "Extract and classify sources with witness weights", "text", "framework"),
						tool("check_manipulation", "10-pattern manipulation detection with severity", "text"),
						tool("interleave_frames", "Convert tape frames to verified epistemic model", "frames"),
						tool("prove", "Prove conservation + composition + sheaf", "text", "framework"),
					},
				},
			})
		case "tools/call":
			handleNinjaToolCall(enc, req.ID, req.Params, framework)
		default:
			enc.Encode(map[string]interface{}{
				"jsonrpc": "2.0", "id": req.ID,
				"error": map[string]interface{}{"code": -32601, "message": "unknown method"},
			})
		}
	}
}

func handleNinjaToolCall(enc *json.Encoder, id json.RawMessage, params json.RawMessage, defFramework string) {
	var p struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"arguments"`
	}
	json.Unmarshal(params, &p)

	var args struct {
		Text      string `json:"text"`
		Framework string `json:"framework"`
	}
	json.Unmarshal(p.Args, &args)
	if args.Framework == "" {
		args.Framework = defFramework
	}

	switch p.Name {
	case "analyze_claim":
		model := antibullshit.AnalyzeWithCatColab(args.Text, args.Framework)
		h1, _ := model.SheafConsistency()
		balanced, counts := model.GF3Balance()
		enc.Encode(mcpResult(id, fmt.Sprintf("CatColab: %d objects, %d morphisms, conf=%.3f, H¹=%d, GF3=%v %v",
			len(model.Objects), len(model.Morphisms), model.Confidence, h1, balanced, counts)))

	case "check_manipulation":
		patterns := antibullshit.DetectManipulation(args.Text)
		verdict := "clean"
		if len(patterns) > 0 {
			max := 0.0
			for _, p := range patterns {
				if p.Severity > max {
					max = p.Severity
				}
			}
			if max >= 0.7 {
				verdict = "high-risk"
			} else if max >= 0.4 {
				verdict = "suspicious"
			} else {
				verdict = "low-risk"
			}
		}
		enc.Encode(mcpResult(id, fmt.Sprintf("%s: %d patterns", verdict, len(patterns))))

	case "validate_sources":
		model := antibullshit.AnalyzeWithCatColab(args.Text, args.Framework)
		srcCount := 0
		for _, obj := range model.Objects {
			if obj.Type == antibullshit.ObSource {
				srcCount++
			}
		}
		enc.Encode(mcpResult(id, fmt.Sprintf("%d sources, %d morphisms", srcCount, len(model.Morphisms))))

	case "prove":
		model := antibullshit.AnalyzeWithCatColab(args.Text, args.Framework)
		consErr := antibullshit.ProveConservation(model)
		pathErrs := antibullshit.ProvePathComposition(model)
		cocycles := antibullshit.ProveSheafConsistency(model)
		enc.Encode(mcpResult(id, fmt.Sprintf("conservation=%v paths=%d/%d sheaf=%d",
			consErr == nil, len(model.Paths)-len(pathErrs), len(model.Paths), len(cocycles))))

	default:
		enc.Encode(map[string]interface{}{
			"jsonrpc": "2.0", "id": id,
			"error": map[string]interface{}{"code": -32602, "message": fmt.Sprintf("unknown tool: %s", p.Name)},
		})
	}
}

// --- repl ---

func doREPL() {
	env := lisp.CreateStandardEnv()
	tape.RegisterNamespace(env)
	tape.RegisterEvolveNamespace(env)
	tape.RegisterDaemonNamespace(env)

	fmt.Println("ninja repl — all namespaces loaded (tape/*, vz/*, agm/*)")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("ninja=> ")
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

// --- helpers ---

func tool(name, desc string, params ...string) map[string]interface{} {
	props := make(map[string]interface{})
	required := []string{}
	for _, p := range params {
		props[p] = map[string]interface{}{"type": "string"}
		if p == "text" || p == "frames" {
			required = append(required, p)
		}
	}
	return map[string]interface{}{
		"name": name, "description": desc,
		"inputSchema": map[string]interface{}{
			"type": "object", "properties": props, "required": required,
		},
	}
}

func mcpResult(id json.RawMessage, text string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0", "id": id,
		"result": map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": text}},
		},
	}
}

func readStdin() string {
	b, _ := io.ReadAll(os.Stdin)
	return strings.TrimSpace(string(b))
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

func waitInterrupt() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func ternary(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}

// Ensure gf3 is used (referenced in interleave via TapeFrame)
var _ = gf3.Zero
