//go:build darwin

// antibullshit is a CatColab-powered MCP server for epistemological verification.
//
// It exposes three tools over stdio JSON-RPC (MCP protocol):
//   - analyze_claim: build a DblModel from text, verify derivation paths
//   - validate_sources: extract and classify sources with witness weights
//   - check_manipulation: detect 10 manipulation patterns with severity
//
// Usage:
//
//	antibullshit                    # start MCP server on stdio
//	antibullshit --framework empirical  # set default framework
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bmorphism/boxxy/internal/antibullshit"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var defaultFramework string

func main() {
	fw := flag.String("framework", "pluralistic", "default epistemological framework")
	flag.Parse()
	defaultFramework = *fw

	fmt.Fprintln(os.Stderr, "[antibullshit] CatColab MCP server on stdio (framework="+defaultFramework+")")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		var req jsonRPCRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			enc.Encode(jsonRPCResponse{
				JSONRPC: "2.0", ID: req.ID,
				Error: &jsonRPCError{Code: -32700, Message: "parse error"},
			})
			continue
		}

		resp := handleRequest(req)
		enc.Encode(resp)
	}
}

func handleRequest(req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities":   map[string]interface{}{"tools": map[string]interface{}{"listChanged": true}},
				"serverInfo":     map[string]interface{}{"name": "antibullshit-catcolab", "version": "0.2.0"},
			},
		}

	case "notifications/initialized", "ping":
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}

	case "tools/list":
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{
				"tools": []interface{}{
					map[string]interface{}{
						"name":        "analyze_claim",
						"description": "Analyze a claim using CatColab double theory — builds DblModel with typed derivation paths, checks sheaf H¹ consistency, GF(3) conservation",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text":      map[string]interface{}{"type": "string", "description": "Claim text to analyze"},
								"framework": map[string]interface{}{"type": "string", "enum": []string{"empirical", "responsible", "harmonic", "pluralistic"}},
							},
							"required": []string{"text"},
						},
					},
					map[string]interface{}{
						"name":        "validate_sources",
						"description": "Extract and classify sources from text, compute witness weights and derivation strengths",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text":      map[string]interface{}{"type": "string", "description": "Text containing claims and sources"},
								"framework": map[string]interface{}{"type": "string", "enum": []string{"empirical", "responsible", "harmonic", "pluralistic"}},
							},
							"required": []string{"text"},
						},
					},
					map[string]interface{}{
						"name":        "check_manipulation",
						"description": "Detect manipulation patterns (urgency, fear, false consensus, appeal to authority, etc.) with severity scoring",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text": map[string]interface{}{"type": "string", "description": "Text to check for manipulation"},
							},
							"required": []string{"text"},
						},
					},
				},
			},
		}

	case "tools/call":
		return handleToolCall(req)

	default:
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &jsonRPCError{Code: -32601, Message: fmt.Sprintf("unknown method: %s", req.Method)},
		}
	}
}

func handleToolCall(req jsonRPCRequest) jsonRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &jsonRPCError{Code: -32602, Message: "invalid params"},
		}
	}

	var args struct {
		Text      string `json:"text"`
		Framework string `json:"framework"`
	}
	json.Unmarshal(params.Arguments, &args)

	if args.Framework == "" {
		args.Framework = defaultFramework
	}

	switch params.Name {
	case "analyze_claim":
		return handleAnalyzeClaim(req.ID, args.Text, args.Framework)
	case "validate_sources":
		return handleValidateSources(req.ID, args.Text, args.Framework)
	case "check_manipulation":
		return handleCheckManipulation(req.ID, args.Text)
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &jsonRPCError{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", params.Name)},
		}
	}
}

func handleAnalyzeClaim(id json.RawMessage, text, framework string) jsonRPCResponse {
	model := antibullshit.AnalyzeWithCatColab(text, framework)

	h1, cocycles := model.SheafConsistency()
	balanced, counts := model.GF3Balance()

	// Build path summaries
	var pathSummaries []map[string]interface{}
	for _, p := range model.Paths {
		pathSummaries = append(pathSummaries, map[string]interface{}{
			"segments":           len(p.Segments),
			"composes":           p.Composes(model.Theory),
			"composite_strength": p.CompositeStrength(),
		})
	}

	// Build cocycle summaries
	var cocycleSummaries []map[string]interface{}
	for _, c := range cocycles {
		cocycleSummaries = append(cocycleSummaries, map[string]interface{}{
			"kind": c.Kind, "severity": c.Severity,
			"claim_a": c.ClaimA, "claim_b": c.ClaimB,
		})
	}

	summary := fmt.Sprintf("CatColab DblTheory analysis (%s framework):\n", framework)
	summary += fmt.Sprintf("  Objects: %d (claims+sources+witnesses)\n", len(model.Objects))
	summary += fmt.Sprintf("  Morphisms: %d (derivation+attestation)\n", len(model.Morphisms))
	summary += fmt.Sprintf("  Paths: %d derivation chains\n", len(model.Paths))
	summary += fmt.Sprintf("  Confidence: %.3f\n", model.Confidence)
	summary += fmt.Sprintf("  Sheaf H¹: %d (%s)\n", h1, map[bool]string{true: "consistent", false: "contradictions"}[h1 == 0])
	summary += fmt.Sprintf("  GF(3): balanced=%v %v\n", balanced, counts)

	return jsonRPCResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": summary},
				{"type": "text", "text": mustJSON(map[string]interface{}{
					"framework":  framework,
					"confidence": model.Confidence,
					"objects":    len(model.Objects),
					"morphisms":  len(model.Morphisms),
					"paths":      pathSummaries,
					"sheaf_h1":   h1,
					"gf3":        map[string]interface{}{"balanced": balanced, "counts": counts},
					"cocycles":   cocycleSummaries,
				})},
			},
		},
	}
}

func handleValidateSources(id json.RawMessage, text, framework string) jsonRPCResponse {
	model := antibullshit.AnalyzeWithCatColab(text, framework)

	var sources []map[string]interface{}
	for _, obj := range model.Objects {
		if obj.Type == antibullshit.ObSource {
			kind := ""
			if obj.Meta != nil {
				kind = obj.Meta["kind"]
			}
			sources = append(sources, map[string]interface{}{
				"id": obj.ID, "label": obj.Label, "kind": kind,
				"trit": obj.Trit.String(), "hash": obj.Hash,
			})
		}
	}

	var morphisms []map[string]interface{}
	for _, mor := range model.Morphisms {
		morphisms = append(morphisms, map[string]interface{}{
			"id": mor.ID, "type": mor.Type, "kind": mor.Kind,
			"strength": mor.Strength, "source": mor.SourceID, "target": mor.TargetID,
		})
	}

	return jsonRPCResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Found %d sources with %d derivation morphisms", len(sources), len(morphisms))},
				{"type": "text", "text": mustJSON(map[string]interface{}{
					"sources": sources, "morphisms": morphisms,
				})},
			},
		},
	}
}

func handleCheckManipulation(id json.RawMessage, text string) jsonRPCResponse {
	patterns := antibullshit.DetectManipulation(text)

	var patternMaps []map[string]interface{}
	for _, p := range patterns {
		patternMaps = append(patternMaps, map[string]interface{}{
			"kind": p.Kind, "evidence": p.Evidence, "severity": p.Severity,
		})
	}

	verdict := "clean"
	if len(patterns) > 0 {
		maxSeverity := 0.0
		for _, p := range patterns {
			if p.Severity > maxSeverity {
				maxSeverity = p.Severity
			}
		}
		if maxSeverity >= 0.7 {
			verdict = "high-risk"
		} else if maxSeverity >= 0.4 {
			verdict = "suspicious"
		} else {
			verdict = "low-risk"
		}
	}

	return jsonRPCResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Manipulation check: %s (%d patterns detected)", verdict, len(patterns))},
				{"type": "text", "text": mustJSON(map[string]interface{}{
					"verdict": verdict, "patterns": patternMaps,
				})},
			},
		},
	}
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
