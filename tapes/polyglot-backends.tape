# boxxy Polyglot REPL Backends Demo
# Shows launching various REPLs from boxxy: IPython+Hy, Joker, Emacs
# Run: vhs tapes/polyglot-backends.tape

Output tapes/polyglot-backends.gif
Output tapes/polyglot-backends.mp4

Set FontSize 18
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set Padding 20

Type "boxxy repl"
Enter
Sleep 2s

# List available backends
Type "(backends)"
Enter
Sleep 2.5s

# Launch IPython with Hy, DisCoPy, PyZX
Type ";; Launch IPython + Hy (Clojure-on-Python) + category theory"
Enter
Sleep 1s

Type "(repl :ipython-hy)"
Enter
Sleep 5s

# Inside IPython
Type "import hy"
Enter
Sleep 1s

Type "# Hy is a Lisp that compiles to Python AST"
Enter
Sleep 1s

Type "exit()"
Enter
Sleep 2s

# Back in boxxy
Type ";; Back in boxxy — try Joker next"
Enter
Sleep 1s

Type "(repl :joker)"
Enter
Sleep 3s

# Inside Joker
Type "(println \"Hello from Joker!\")"
Enter
Sleep 1s

# Ctrl+D to exit Joker
Ctrl+D
Sleep 2s

Type "(quit)"
Enter
Sleep 1s
