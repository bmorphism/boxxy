# nanoclj-zig NaN-Boxing Demo
# Every value is 64 bits. IEEE 754 double, or quiet NaN with tag+payload.
# Run: vhs tapes/nanoclj-zig-nanbox.tape

Output tapes/nanoclj-zig-nanbox.gif
Output tapes/nanoclj-zig-nanbox.mp4

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

# --- Every value fits in 64 bits ---
Type ";; NaN-boxing: 64 bits per value, no object headers"
Enter
Sleep 1s

# Floats: native IEEE 754
Type "3.14159"
Enter
Sleep 1s

Type "(type 3.14)"
Enter
Sleep 1s

# Integers: tag=2, inline i48 — never touches the heap
Type "42"
Enter
Sleep 1s

Type "(type 42)"
Enter
Sleep 1s

# Max inline integer: 2^47 - 1
Type "140737488355327"
Enter
Sleep 1.5s

# Nil: tag=0, payload=0
Type "nil"
Enter
Sleep 1s

# Booleans: tag=1, payload=0|1
Type "true"
Enter
Sleep 0.5s

Type "false"
Enter
Sleep 1s

# Symbols: tag=3, interned at read time
Type "(quote hello)"
Enter
Sleep 1s

# Keywords: tag=4, interned
Type ":world"
Enter
Sleep 1s

# Strings: tag=5, interned
Type "\"nanoclj-zig\""
Enter
Sleep 1s

# --- The payoff: contiguous memory ---
Type ";; A vector of 5 ints = contiguous 40 bytes of u64"
Enter
Sleep 1s

Type "[1 2 3 4 5]"
Enter
Sleep 1s

# Mix types freely — still 64 bits each
Type "[42 3.14 :keyword nil true \"string\"]"
Enter
Sleep 1.5s

# --- Truthiness ---
Type ";; Truthiness: only nil and false are falsy"
Enter
Sleep 1s

Type "(if 0 :truthy :falsy)"
Enter
Sleep 1s

Type "(if \"\" :truthy :falsy)"
Enter
Sleep 1s

Type "(if nil :truthy :falsy)"
Enter
Sleep 1s

Type "(if false :truthy :falsy)"
Enter
Sleep 1.5s

# --- Numeric coercion ---
Type ";; Seamless: int -> float -> rational"
Enter
Sleep 1s

Type "(+ 1 0.5)"
Enter
Sleep 1s

Type "(/ 1 3)"
Enter
Sleep 1s

Type "(+ (/ 1 3) 0.5)"
Enter
Sleep 1.5s

Sleep 2s
