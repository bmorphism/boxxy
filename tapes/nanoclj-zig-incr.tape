# nanoclj-zig Incremental Compilation Demo
# Three tiers: red (bytecode), blue (tree-walk), green (incr AOT)
# Run: vhs tapes/nanoclj-zig-incr.tape

Output tapes/nanoclj-zig-incr.gif
Output tapes/nanoclj-zig-incr.mp4

Set FontSize 18
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms
Set Padding 20

Type "cd ~/i/nanoclj-zig && zig build run"
Enter
Sleep 2s

# --- Tier 1: Blue (tree-walk) ---
Type ";; BLUE tier: tree-walk interpreter"
Enter
Sleep 1s

Type "(defn square [x] (* x x))"
Enter
Sleep 1s

Type "(square 7)"
Enter
Sleep 1.5s

Type "(map square (range 1 11))"
Enter
Sleep 2s

# --- Tier 2: Red (bytecode VM) ---
Type ";; RED tier: 22-opcode bytecode VM"
Enter
Sleep 1s

Type "(bc (square 7))"
Enter
Sleep 1.5s

Type "(bench (reduce + (map square (range 1000))))"
Enter
Sleep 3s

# --- Tier 3: Green (incremental AOT) ---
Type ";; GREEN tier: Clojure -> Zig transpilation"
Enter
Sleep 1s

Type "(incr (defn cube [x] (* x (* x x))))"
Enter
Sleep 2s

Type "(incr (defn add3 [a b c] (+ a (+ b c))))"
Enter
Sleep 2s

# List registered functions
Type "(incr-list)"
Enter
Sleep 2s

# Show the generated Zig source
Type ";; Generated .incr/ncz_cube.zig:"
Enter
Sleep 1s

# Exit REPL, show the file
Ctrl+D
Sleep 1s

Type "cat .incr/ncz_cube.zig"
Enter
Sleep 3s

# Show add3
Type "cat .incr/ncz_add3.zig"
Enter
Sleep 3s

Sleep 2s
