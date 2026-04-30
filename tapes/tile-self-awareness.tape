# boxxy Tile Self-Awareness + Triangulation Loop
# The REPL inspects its own color identity, performs TCO recursion,
# and triangulates 3 seeds to check triangle inequality on behavioral traces.
# REBEL bridge: identical memo patterns => d=0 => IR-equivalent compositional primitives.
# Run: vhs tapes/tile-self-awareness.tape

Output tapes/gifs/tile-self-awareness.gif
Output tapes/gifs/tile-self-awareness.mp4

Set FontSize 15
Set FontFamily "Berkeley Mono"
Set Width 1500
Set Height 1000
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms
Set Padding 20

# Launch tile REPL with seed 1069
Type "boxxy repl --seed 1069"
Enter
Sleep 3s

# Phase 1: Who am I?
Type "(tile/identity)"
Enter
Sleep 2s

# Phase 2: Full self-portrait
Type "(tile/color-identity 1069)"
Enter
Sleep 2s

# Phase 3: TCO — loop/recur fibonacci (no stack growth)
Type "(def fib (fn [n] (loop [a 0 b 1 i n] (if (= i 0) a (recur b (+ a b) (- i 1))))))"
Enter
Sleep 2s

Type "(fib 10)"
Enter
Sleep 1.5s

# Phase 4: Behavioral trace — 20 steps of trit-memoization
Type "(tile/trace 1069 20)"
Enter
Sleep 2.5s

# Phase 5: Triangulation — 3 seeds, check triangle inequality
Type "(tile/triangulate 1069 42 7)"
Enter
Sleep 3s

# Phase 6: Metamorphosis — switch seed, watch prompt color shift
Type "(tile/seed 42)"
Enter
Sleep 2s

Type "(tile/identity)"
Enter
Sleep 2s

# Phase 7: Syrup wire — how the new identity looks on the wire
Type "(tile/wire)"
Enter
Sleep 2s

# Phase 8: Return to origin — the loop closes
Type "(tile/seed 1069)"
Enter
Sleep 1.5s

Type "(tile/identity)"
Enter
Sleep 2s

# Exit
Type "(quit)"
Enter
Sleep 1s
