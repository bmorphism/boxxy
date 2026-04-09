# nanoclj-zig miniKanren Demo
# Relational programming: run forward, backward, sideways
# Run: vhs tapes/nanoclj-zig-kanren.tape

Output tapes/nanoclj-zig-kanren.gif
Output tapes/nanoclj-zig-kanren.mp4

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

# --- Unification: the core primitive ---
Type ";; miniKanren: relations, not functions"
Enter
Sleep 1s

Type "(run* [q] (== q 42))"
Enter
Sleep 1.5s

Type "(run* [q] (== q [:hello :world]))"
Enter
Sleep 1.5s

# --- Forward: compute append ---
Type ";; Forward: what is (append [1 2] [3 4])?"
Enter
Sleep 1s

Type "(run* [q] (appendo '(1 2) '(3 4) q))"
Enter
Sleep 2s

# --- Backward: invert append ---
Type ";; Backward: what lists append to (1 2 3 4)?"
Enter
Sleep 1s

Type "(run 5 [x y] (appendo x y '(1 2 3 4)))"
Enter
Sleep 3s

# --- Membership ---
Type "(run* [q] (membero q '(:red :blue :purple)))"
Enter
Sleep 2s

# --- Fresh variables + conjunction ---
Type "(run* [q] (fresh [x y] (== x :zig) (== y :clojure) (== q [x y])))"
Enter
Sleep 2s

# --- Type inference as search (SPJ-style) ---
Type ";; Type inference via unification"
Enter
Sleep 1s

Type "(run* [result] (fresh [a b1 b2 c] (== [:Bool :-> :String] [b1 :-> c]) (== [:Int :-> :Bool] [a :-> b2]) (== b1 b2) (== result [a :-> c])))"
Enter
Sleep 3s

# --- conde: disjunction ---
Type "(run* [q] (conde [(== q :red)] [(== q :blue)] [(== q :purple)]))"
Enter
Sleep 2s

# --- GF(3) conservation as a relation ---
Type ";; GF(3): trit sum = 0 mod 3"
Enter
Sleep 1s

Type "(run* [c] (fresh [a b] (== a 1) (== b -1) (== c (+ a b))))"
Enter
Sleep 2s

Sleep 2s
