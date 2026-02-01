# boxxy Skill Rendering Demo
# Shows GF(3) trit-colored SKILL.md rendering via glamour
# Demonstrates: entropy -> seed -> color pipeline + agentskills spec
# Run: vhs tapes/skill-render.tape

Output tapes/skill-render.gif
Output tapes/skill-render.mp4

Set FontSize 16
Set FontFamily "Berkeley Mono"
Set Width 1400
Set Height 900
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms
Set Padding 20

# Title
Type "# boxxy skill renderer — GF(3) trit-colored SKILL.md"
Enter
Sleep 1s

# Show help
Type "boxxy help"
Enter
Sleep 2s

# Render a single skill (ASI integrated)
Type "boxxy skill ~/i/asi/skills/_integrated/SKILL.md --width 120"
Enter
Sleep 4s

# Scroll to see the output
Sleep 3s

# Triad summary of all ASI skills
Type "boxxy skill ~/i/asi/skills/ --summary"
Enter
Sleep 5s

# Show GF(3) conservation across the skill lattice
Type "echo '---'"
Enter
Sleep 0.5s
Type "echo 'GF(3) Conservation: sum of trits = 0 (mod 3)'"
Enter
Sleep 0.5s
Type "echo 'Coordinator(0) = amber #F59E0B  | hue ~85 OKLCH'"
Enter
Sleep 0.5s
Type "echo 'Generator(+1)  = purple #A855F7 | hue ~303 OKLCH'"
Enter
Sleep 0.5s
Type "echo 'Verifier(-1)   = blue #2E5FA3   | hue ~250 OKLCH'"
Enter
Sleep 0.5s
Type "echo 'Min perceptual spread: 53 degrees (>50 required)'"
Enter
Sleep 2s

# Render a specific letter-skill
Type "boxxy skill ~/i/asi/skills/a/SKILL.md --width 100"
Enter
Sleep 4s

# End
Type "echo 'RFC 6943 s4: entropy from interaction -> gay_seed -> deterministic colors'"
Enter
Sleep 3s
