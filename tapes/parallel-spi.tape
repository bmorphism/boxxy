# boxxy Parallel SPI — Strong Parallelism Invariant Demo
# Demonstrates embarrassingly parallel tile verification via goroutine
# value-copy closures + GF(3) conservation join.
# SPI: same seed → same color regardless of goroutine scheduling.
# Run: vhs tapes/parallel-spi.tape

Output tapes/gifs/parallel-spi.gif
Output tapes/gifs/parallel-spi.mp4

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
Type "(tile/color-identity 1069)"
Enter
Sleep 2s

# Phase 2: Parallel map — 6 seeds → colors in parallel goroutines
Type "(tile/parallel-map [1069 42 7 256 1337 2025])"
Enter
Sleep 3s

# Phase 3: Find a balanced triad from seed space
Type "(tile/find-triad 0 10000)"
Enter
Sleep 2.5s

# Phase 4: Parallel verify — 3 balanced seeds, move lifecycle ops
Type "(tile/parallel-verify [3 0 1] [:mint :transfer :burn])"
Enter
Sleep 4s

# Phase 5: Verify conservation on larger set
Type "(tile/parallel-verify [1069 42 7] [:move-to :borrow-global :move-from])"
Enter
Sleep 3s

# Phase 6: Single verifier for comparison
Type "(def v (tile/verifier-new 1069))"
Enter
Sleep 1s

Type "(tile/verifier-op v :mint)"
Enter
Sleep 1s

Type "(tile/verifier-op v :transfer)"
Enter
Sleep 1s

Type "(tile/verifier-op v :burn)"
Enter
Sleep 1s

Type "(tile/verifier-status v)"
Enter
Sleep 2.5s

Type "(tile/verifier-report v)"
Enter
Sleep 2s

# Exit
Type "(quit)"
Enter
Sleep 1s
