# boxxy Rainbow Parentheses Demo
# Records the REPL showing Gay MCP golden-thread rainbow parens
# Run: vhs tapes/rainbow-parens.tape

Output tapes/rainbow-parens.gif
Output tapes/rainbow-parens.mp4

Set FontSize 18
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set Padding 20

# Launch boxxy REPL
Type "boxxy repl"
Enter
Sleep 2s

# Simple expression
Type "(+ 1 2 3)"
Enter
Sleep 1.5s

# Nested — shows rainbow depth coloring
Type "(def vm (vz/new-vm-config 4 8 (vz/new-efi-boot-loader (vz/new-efi-variable-store \"/tmp/nvram\" true)) (vz/new-generic-platform)))"
Enter
Sleep 2s

# Deeper nesting
Type "(let [net (vz/new-nat-network) nic (vz/new-virtio-network net)] (vz/add-network-devices vm [nic]))"
Enter
Sleep 2s

# Show help with syntax highlighting
Type "(help)"
Enter
Sleep 3s

# Exit
Type "(quit)"
Enter
Sleep 1s
