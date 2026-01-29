// boxxy serve — WebSocket proxy for browser access
//
// Exposes the boxxy Lisp evaluator over WebSocket so that
// squint/SCI running in a browser can call vz/ functions
// against a local boxxy process.
//
// Protocol:
//   Client sends:  {"id": "uuid", "expr": "(vz/vm-state vm)"}
//   Server sends:  {"id": "uuid", "result": "\"running\""}
//
// This makes the browser↔local shape identical to CLI:
//   browser → WebSocket → Go eval → Virtualization.framework
//   CLI     → stdin     → Go eval → Virtualization.framework
//
// Usage:
//   boxxy serve --port 7888
//   boxxy serve --port 7888 --cors-origin http://localhost:3000

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/vm"
)

type wsMessage struct {
	ID   string `json:"id"`
	Expr string `json:"expr"`
}

type wsResponse struct {
	ID     string `json:"id"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	port := flag.Int("port", 7888, "WebSocket listen port")
	corsOrigin := flag.String("cors-origin", "*", "CORS allowed origin")
	flag.Parse()

	// Create shared Lisp environment with vz namespace
	env := lisp.NewEnv(nil)
	lisp.RegisterStdlib(env)
	vm.RegisterNamespace(env)
	var evalMu sync.Mutex

	// HTTP handler for WebSocket upgrade
	http.HandleFunc("/vz", func(w http.ResponseWriter, r *http.Request) {
		// CORS headers for browser access
		w.Header().Set("Access-Control-Allow-Origin", *corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}

		// For simplicity, use HTTP POST with JSON body
		// (WebSocket upgrade would use gorilla/websocket in production)
		if r.Method == "POST" {
			var msg wsMessage
			if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
				json.NewEncoder(w).Encode(wsResponse{
					ID:    msg.ID,
					Error: fmt.Sprintf("parse error: %v", err),
				})
				return
			}

			// Evaluate the expression
			evalMu.Lock()
			result, evalErr := safeEval(msg.Expr, env)
			evalMu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			if evalErr != "" {
				json.NewEncoder(w).Encode(wsResponse{
					ID:    msg.ID,
					Error: evalErr,
				})
			} else {
				json.NewEncoder(w).Encode(wsResponse{
					ID:     msg.ID,
					Result: result,
				})
			}
			return
		}

		http.Error(w, "use POST with {\"id\":\"...\",\"expr\":\"(vz/...)\"}", 400)
	})

	// Health endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"provider": "boxxy",
			"color":    "#0BC68E",
			"trit":     0,
		})
	})

	// Provider EDN endpoint
	http.HandleFunc("/provider.edn", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/edn")
		w.Header().Set("Access-Control-Allow-Origin", *corsOrigin)
		http.ServeFile(w, r, "std/provider.edn")
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[boxxy serve] listening on %s (CORS: %s)", addr, *corsOrigin)
	log.Printf("[boxxy serve] POST /vz with {\"id\":\"uuid\",\"expr\":\"(vz/...)\"}")
	log.Printf("[boxxy serve] GET  /health")
	log.Printf("[boxxy serve] GET  /provider.edn")
	log.Fatal(http.ListenAndServe(addr, nil))
}

// safeEval evaluates a Lisp expression string, catching panics.
func safeEval(exprStr string, env *lisp.Env) (result string, errStr string) {
	defer func() {
		if r := recover(); r != nil {
			errStr = fmt.Sprintf("%v", r)
		}
	}()

	reader := lisp.NewReader(strings.NewReader(exprStr))
	exprs := reader.ReadAll()

	var lastVal lisp.Value = lisp.Nil{}
	for _, expr := range exprs {
		lastVal = lisp.Eval(expr, env)
	}

	return fmt.Sprintf("%v", lastVal), ""
}
