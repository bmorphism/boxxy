# boxxy Worlds Demo — Polyglot REPLs × AGM Belief Revision × Possible Worlds
# The full pipeline: Gay MCP colors → rainbow parens → AGM → soft-serve branches
# Run: vhs tapes/worlds.tape

Output tapes/worlds.gif
Output tapes/worlds.mp4

Set FontSize 18
Set FontFamily "Berkeley Mono"
Set Width 1500
Set Height 1000
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms
Set Padding 20

# Launch boxxy
Type "boxxy repl"
Enter
Sleep 2s

# Show the polyglot backend registry
Type "(backends)"
Enter
Sleep 3s

# Create a belief set — our "world model" of available REPLs
Type ";; AGM possible worlds: what REPLs exist in our universe?"
Enter
Sleep 1s

Type "(def W (agm/new-belief-set))"
Enter
Sleep 1s

# Expand with beliefs about available backends
Type "(agm/expand W 'boxxy-available)"
Enter
Sleep 0.8s

Type "(agm/expand W 'ipython-available)"
Enter
Sleep 0.8s

Type "(agm/expand W 'hy-is-lisp)"
Enter
Sleep 0.8s

Type "(agm/expand W 'joker-is-clojure)"
Enter
Sleep 0.8s

Type "(agm/expand W 'emacs-is-environment)"
Enter
Sleep 1s

# Check our beliefs
Type "(agm/beliefs W)"
Enter
Sleep 2s

# All consistent?
Type "(agm/consistent? W)"
Enter
Sleep 1.5s

# Now revise: discover that Hy is not just Lisp, it's Clojure-flavored
Type ";; Revision: Hy compiles to Python AST — it's not pure Lisp"
Enter
Sleep 1.5s

Type "(agm/revise W 'not-hy-is-lisp)"
Enter
Sleep 1s

# The old belief is gone
Type "(agm/entails? W 'hy-is-lisp)"
Enter
Sleep 1.5s

# Show possible worlds — each world is a branch in Grove's spheres
Type ";; Possible worlds (Grove spheres = soft-serve branches)"
Enter
Sleep 1s

Type "(agm/worlds W)"
Enter
Sleep 3s

# Show the formal correspondence
Type "(agm/grove-spheres)"
Enter
Sleep 4s

# Now actually launch a backend to demonstrate polyglot
Type ";; Let's visit a possible world: IPython + Hy + DisCoPy + PyZX"
Enter
Sleep 1.5s

Type "(repl :ipython-hy)"
Enter
Sleep 5s

# Inside IPython — demonstrate the Hy/Python bridge
Type "import hy"
Enter
Sleep 1s

Type "# Hy: Clojure-on-Python with category theory (DisCoPy) + quantum (PyZX)"
Enter
Sleep 1.5s

Type "print('Hello from a possible world!')"
Enter
Sleep 1.5s

Type "exit()"
Enter
Sleep 2s

# Back in boxxy
Type ";; Returned from possible world. Contract that belief."
Enter
Sleep 1s

Type "(agm/contract W 'ipython-available)"
Enter
Sleep 1s

Type "(agm/beliefs W)"
Enter
Sleep 2s

# Final: the VM world
Type ";; The actual world: spin up a VM in our belief set"
Enter
Sleep 1s

Type "(agm/expand W 'vm-can-boot)"
Enter
Sleep 0.8s

Type "(agm/expand W 'disk-attached)"
Enter
Sleep 0.8s

Type "(agm/expand W 'network-connected)"
Enter
Sleep 1s

Type "(agm/beliefs W)"
Enter
Sleep 2.5s

Type "(agm/consistent? W)"
Enter
Sleep 1.5s

Type ";; Each belief = a commit. Each world = a branch. boxxy = soft-serve for VMs."
Enter
Sleep 3s

Type "(quit)"
Enter
Sleep 1s
