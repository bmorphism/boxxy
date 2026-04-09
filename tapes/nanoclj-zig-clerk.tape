# nanoclj-zig Clerk Demo
# Double-wrapped rendering: nREPL -> Babashka bridge -> browser
# "The terminal is a rendering surface, not a text grid"
# Run: vhs tapes/nanoclj-zig-clerk.tape

Output tapes/nanoclj-zig-clerk.gif
Output tapes/nanoclj-zig-clerk.mp4

Set FontSize 16
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms
Set Padding 20

# --- Terminal 1: start nanoclj-zig with nREPL ---
Type "cd ~/i/nanoclj-zig && zig build run -- --nrepl &"
Enter
Sleep 3s

# --- Terminal 2: start clerk bridge ---
Type "bb clerk/serve.clj"
Enter
Sleep 2s

# Show what's happening
Type "# nREPL on port 7888 (TCP/bencode)"
Enter
Sleep 1s

Type "# Clerk on http://localhost:8484 (HTTP/SSE)"
Enter
Sleep 1s

Type "# Each eval carries: x-color-r/g/b, x-trit-phase, x-fuel-remaining"
Enter
Sleep 2s

# --- Eval via curl to show the protocol ---
Type "curl -s -X POST http://localhost:8484/eval -d '(+ 1 2 3)'"
Enter
Sleep 2s

Type "curl -s -X POST http://localhost:8484/eval -d '(map (fn* [x] (* x x)) [1 2 3 4 5])'"
Enter
Sleep 2s

Type "curl -s -X POST http://localhost:8484/eval -d '(run* [q] (appendo (quote (1 2)) (quote (3 4)) q))'"
Enter
Sleep 3s

# Show the color metadata
Type "curl -s -X POST http://localhost:8484/eval -d '(def colors {:red 1 :blue -1 :purple 0})'"
Enter
Sleep 2s

Type "curl -s -X POST http://localhost:8484/eval -d '(reduce + (vals colors))'"
Enter
Sleep 2s

# The punchline: browser renders colored cells
Type "# Open http://localhost:8484 — each result has:"
Enter
Sleep 1s

Type "#   border-color from session RGB (SplitMix64 from seed)"
Enter
Sleep 1s

Type "#   tier badge: red/blue/purple from trit-phase"
Enter
Sleep 1s

Type "#   fuel remaining, trit-ticks, eval count"
Enter
Sleep 2s

Type "open http://localhost:8484"
Enter
Sleep 4s

Sleep 2s
