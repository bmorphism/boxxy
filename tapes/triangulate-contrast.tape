# boxxy Triangulation Contrast
# Different seed, different trits, different behavioral trace.
# Used for embedding comparison with tile-self-awareness.
# Run: vhs tapes/triangulate-contrast.tape

Output tapes/gifs/triangulate-contrast.gif
Output tapes/gifs/triangulate-contrast.mp4

Set FontSize 15
Set FontFamily "Berkeley Mono"
Set Width 1500
Set Height 1000
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms
Set Padding 20

# Launch with a DIFFERENT seed — 42 (purple, trit -1)
Type "boxxy repl --seed 42"
Enter
Sleep 3s

Type "(tile/identity)"
Enter
Sleep 2s

Type "(tile/color-identity 42)"
Enter
Sleep 2s

# Same fib definition — structurally identical program
Type "(def fib (fn [n] (loop [a 0 b 1 i n] (if (= i 0) a (recur b (+ a b) (- i 1))))))"
Enter
Sleep 2s

Type "(fib 10)"
Enter
Sleep 1.5s

# Trace with different seed base
Type "(tile/trace 42 20)"
Enter
Sleep 2.5s

# Triangulate with different triple
Type "(tile/triangulate 42 100 999)"
Enter
Sleep 3s

# Back to basics
Type "(tile/wire)"
Enter
Sleep 2s

Type "(quit)"
Enter
Sleep 1s
