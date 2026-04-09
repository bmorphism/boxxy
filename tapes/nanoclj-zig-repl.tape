# nanoclj-zig REPL Demo
# NaN-boxed Clojure in Zig: 0ms startup, fuel-bounded eval, 22 red opcodes
# Run: vhs tapes/nanoclj-zig-repl.tape

Output tapes/nanoclj-zig-repl.gif
Output tapes/nanoclj-zig-repl.mp4

Set FontSize 18
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms
Set Padding 20

# Build + run — sub-second incremental AOT
Type "cd ~/i/nanoclj-zig && zig build run"
Enter
Sleep 2s

# Arithmetic — all red (AOT), no eval needed
Type "(+ 1 2)"
Enter
Sleep 1s

Type "(* 6 7)"
Enter
Sleep 1s

# Data structures — NaN-boxed, 64 bits each
Type "(def xs [1 2 3 4 5])"
Enter
Sleep 1s

Type "(map (fn* [x] (* x x)) xs)"
Enter
Sleep 1.5s

# Keywords and maps
Type "(def m {:name \"nanoclj\" :lang :zig :loc 32000})"
Enter
Sleep 1s

Type "(:lang m)"
Enter
Sleep 1.5s

# Higher-order: reduce
Type "(reduce + 0 (range 1 101))"
Enter
Sleep 1.5s

# Rationals — exact, GCD-normalized
Type "(/ 22 7)"
Enter
Sleep 1s

Type "(+ (/ 1 3) (/ 1 6))"
Enter
Sleep 1.5s

# Lazy sequences
Type "(take 10 (iterate (fn* [x] (* x 2)) 1))"
Enter
Sleep 1.5s

# Pattern matching
Type "(let [p [3 4]] (let [[x y] p] (+ (* x x) (* y y))))"
Enter
Sleep 1.5s

# Atoms — mutable state
Type "(def counter (atom 0))"
Enter
Sleep 1s

Type "(dotimes [_ 5] (swap! counter inc))"
Enter
Sleep 1s

Type "@counter"
Enter
Sleep 1.5s

# Bytecode compilation — switch to red tier
Type "(bc (* (+ 1 2) (- 10 3)))"
Enter
Sleep 2s

# Fuel-bounded eval — always terminates
Type "(bench (reduce + (range 10000)))"
Enter
Sleep 3s

Sleep 2s
