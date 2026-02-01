# boxxy AGM Belief Revision Demo
# Shows possible-worlds reasoning via Alchourrón-Gärdenfors-Makinson
# Run: vhs tapes/agm-belief-revision.tape

Output tapes/agm-belief-revision.gif
Output tapes/agm-belief-revision.mp4

Set FontSize 18
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set Padding 20

Type "boxxy repl"
Enter
Sleep 2s

# Create a belief set
Type "(def K (agm/new-belief-set))"
Enter
Sleep 1s

# Expand: add beliefs (like git commit)
Type "(agm/expand K 'vm-running)"
Enter
Sleep 1s

Type "(agm/expand K 'network-connected)"
Enter
Sleep 1s

Type "(agm/expand K 'disk-attached)"
Enter
Sleep 1s

# Check beliefs
Type "(agm/beliefs K)"
Enter
Sleep 1.5s

# Check entailment
Type "(agm/entails? K 'vm-running)"
Enter
Sleep 1s

# Check consistency
Type "(agm/consistent? K)"
Enter
Sleep 1s

# Revise: add contradicting belief (like merge with conflicts)
Type ";; Revision: the VM crashed. Revise our beliefs."
Enter
Sleep 1s

Type "(agm/revise K 'not-vm-running)"
Enter
Sleep 1s

# The old belief is gone — revision maintains consistency
Type "(agm/entails? K 'vm-running)"
Enter
Sleep 1.5s

Type "(agm/beliefs K)"
Enter
Sleep 1.5s

# Show possible worlds (Grove spheres)
Type "(agm/worlds K)"
Enter
Sleep 2s

# Show the Git correspondence
Type "(agm/grove-spheres)"
Enter
Sleep 3s

Type "(quit)"
Enter
Sleep 1s
