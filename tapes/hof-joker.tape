# boxxy HOF in Joker REPL Demo
# Beta reductions, Church numerals, GF(3) balanced composition
# Run: vhs tapes/hof-joker.tape

Output tapes/hof-joker.gif
Output tapes/hof-joker.mp4

Set FontSize 18
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set Padding 20

# Launch boxxy REPL
Type "boxxy repl"
Enter
Sleep 2s

# Drop into Joker backend
Type "(repl :joker)"
Enter
Sleep 3s

# --- Curried addition: beta reduction ---
Type ";; β-reduction: ((λx.λy.+ x y) 3) 4"
Enter
Sleep 1s

Type "(def curry (fn [x] (fn [y] (+ x y))))"
Enter
Sleep 1s

Type "((curry 3) 4)"
Enter
Sleep 1.5s

# --- Composition ---
Type ";; compose: λf.λg.λx. f(g(x))"
Enter
Sleep 1s

Type "(def compose (fn [f] (fn [g] (fn [x] (f (g x))))))"
Enter
Sleep 1s

Type "(((compose inc) (fn [x] (* x 2))) 5)"
Enter
Sleep 1.5s

# --- Map as HOF ---
Type "(def hof-map (fn [f] (fn [s] (map f s))))"
Enter
Sleep 1s

Type "((hof-map (fn [x] (* x 2))) [1 2 3])"
Enter
Sleep 1.5s

# --- Fold ---
Type "(((  (fn [f] (fn [z] (fn [s] (reduce f z s))))  +) 0) [1 2 3])"
Enter
Sleep 1.5s

# --- Church numerals ---
Type ";; Church numerals"
Enter
Sleep 0.5s

Type "(def C0 (fn [f] (fn [x] x)))"
Enter
Sleep 0.5s

Type "(def C2 (fn [f] (fn [x] (f (f x)))))"
Enter
Sleep 0.5s

Type "(def succ (fn [n] (fn [f] (fn [x] (f ((n f) x))))))"
Enter
Sleep 0.5s

Type "(def to-int (fn [n] ((n inc) 0)))"
Enter
Sleep 0.5s

Type "(to-int C0)"
Enter
Sleep 1s

Type "(to-int C2)"
Enter
Sleep 1s

Type "(to-int (succ C2))"
Enter
Sleep 1.5s

# --- Z combinator (applicative-order Y): factorial ---
Type ";; Z combinator → factorial"
Enter
Sleep 0.5s

Type "(def Z (fn [f] ((fn [x] (f (fn [v] ((x x) v)))) (fn [x] (f (fn [v] ((x x) v)))))))"
Enter
Sleep 1s

Type "((Z (fn [f] (fn [n] (if (<= n 1) 1 (* n (f (- n 1))))))) 5)"
Enter
Sleep 2s

# --- GF(3) balanced triad ---
Type ";; GF(3) balanced triad: 1 + 0 + 2 ≡ 0 (mod 3)"
Enter
Sleep 1s

Type "(zero? (mod (+ 1 0 2) 3))"
Enter
Sleep 1.5s

# Exit Joker, run full file
Ctrl+D
Sleep 1s

Type "joker hof.joke"
Enter
Sleep 3s

Sleep 2s
