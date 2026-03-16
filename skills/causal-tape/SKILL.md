---
name: causal-tape
description: TUI QuickTime alternative with causal 1-FPS tape recording, Lamport clock convergence, Darwin Godel Machine self-evolving capture strategies, ACSets categorical data model, network tape sharing, and GF(3) conservation
version: 1.0.0
license: MIT
compatibility: macOS darwin arm64/amd64 with Go 1.24+
metadata:
  trit: "0"
  role: Coordinator
  color: "#F59E0B"
  category: tape-recording
  uri: tape://local/world
---

# causal-tape

TUI QuickTime alternative for recording terminal sessions at 1 FPS with
causal convergence across distributed participants.

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    CAUSAL TAPE SYSTEM                    │
├──────────────────────────────────────────────────────────┤
│  ┌─────────────┐    ┌─────────────┐    ┌──────────────┐ │
│  │  Recorder   │───▶│ Lamport     │───▶│  Tape JSONL  │ │
│  │  (1 FPS)    │    │  Clock      │    │  (frames)    │ │
│  └──────┬──────┘    └──────┬──────┘    └──────────────┘ │
│         │                  │                             │
│  ┌──────▼──────┐    ┌──────▼──────┐    ┌──────────────┐ │
│  │  DGM        │    │  Network    │    │  ACSet       │ │
│  │  Archive    │    │  Server     │    │  World       │ │
│  │  (evolve)   │    │  (TCP)      │    │  (schema)    │ │
│  └─────────────┘    └─────────────┘    └──────────────┘ │
└──────────────────────────────────────────────────────────┘
```

## Causal Model

Every participant maintains a Lamport clock. Local captures increment
the clock. Remote frames advance it to `max(local, remote) + 1`.
All tapes converge to the same causal partial order.

## Self-Evolving Capture (DGM)

Capture strategies are agents in a Darwin Godel Machine archive:
1. Sample capture agent (fitness-proportionate)
2. Mutate: adjust interval, jitter, diff threshold, compression
3. Evaluate: measure information density of captured frames
4. If novel and fit, add to archive
5. Prune archive to maintain diversity

## GF(3) Conservation

Frames carry GF(3) trits: `seq % 3`. The tape world verifies
`sum(trits) = 0 (mod 3)` across all frames, maintaining the
conservation law across distributed participants.

## Joker Lisp API

```clojure
;; Record locally at 1 FPS
(def rec (tape/new-recorder "alice" "session"))
(tape/start! rec)
;; ... work ...
(def t (tape/stop! rec))
(tape/save! t "session.jsonl")

;; Record remote via SSH
(def rec (tape/new-recorder "alice" "remote" "bob@server"))
(tape/start! rec)

;; Network sharing
(def addr (tape/serve! ":4444" rec))
(tape/connect! "server:4444" rec)

;; Self-evolving capture
(def archive (tape/new-archive 20))
(tape/evolve! archive rec 10)
(tape/archive-status archive)

;; Merge and play
(def merged (tape/merge t1 t2))
(tape/play! merged 2)
```

## CLI

```bash
tapeqt record -o session.jsonl -node alice
tapeqt record-ssh -node alice bob@server
tapeqt play -speed 2 session.jsonl
tapeqt merge -o merged.jsonl a.jsonl b.jsonl
tapeqt serve -addr :4444 -node alice
tapeqt repl
```
