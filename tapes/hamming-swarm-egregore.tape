# boxxy Hamming Swarm × Egregore Integration Demo
# Shows: vertex generation → GF(3) conservation → Aptos bridge → Emacs viz
# Requires: bb, emacsclient (causality), egregore-curve-labs
# Run: vhs tapes/hamming-swarm-egregore.tape

Output tapes/hamming-swarm-egregore.gif
Output tapes/hamming-swarm-egregore.mp4

Set FontSize 16
Set FontFamily "Berkeley Mono"
Set Width 1600
Set Height 1000
Set Theme "Catppuccin Mocha"
Set TypingSpeed 25ms
Set Padding 20

# ═══ Act 1: Hamming swarm walk ═══
Type "echo '═══ Act 1: Hamming Swarm Walk — 9 validators in {0,1}^42 ═══'"
Enter
Sleep 1.5s

Type "bb ~/i/egregore-curve-labs/bin/hamming-swarm-vertices.clj 2 9 4 | head -3"
Enter
Sleep 3s

Type "echo '... (9 vertices, Shamir secret=2 over GF(27), threshold=4)'"
Enter
Sleep 2s

# ═══ Act 2: GF(3) conservation check ═══
Type "echo ''"
Enter
Type "echo '═══ Act 2: GF(3) Conservation ═══'"
Enter
Sleep 1s

Type "bb ~/i/egregore-curve-labs/bin/hamming-swarm-vertices.clj 2 9 4 | bb ~/i/egregore-curve-labs/src/aptos_hamming_bridge.clj"
Enter
Sleep 5s

# ═══ Act 3: Continuation map ═══
Type "echo ''"
Enter
Type "echo '═══ Act 3: Continuation Map (adjacent vertices d_H ≤ 3) ═══'"
Enter
Sleep 1s

Type "bb ~/i/egregore-curve-labs/bin/hamming-swarm-vertices.clj 2 9 4 | bb ~/i/egregore-curve-labs/src/aptos_hamming_bridge.clj --continuation-map"
Enter
Sleep 4s

# ═══ Act 4: Aptos transaction payloads ═══
Type "echo ''"
Enter
Type "echo '═══ Act 4: Aptos swarm_bootstrap payloads (zubyul module) ═══'"
Enter
Sleep 1s

Type "bb ~/i/egregore-curve-labs/bin/hamming-swarm-vertices.clj 2 9 4 | bb ~/i/egregore-curve-labs/src/aptos_hamming_bridge.clj --generate-payloads 0xzubyul | head -4"
Enter
Sleep 4s

# ═══ Act 5: Emacs visualization ═══
Type "echo ''"
Enter
Type "echo '═══ Act 5: Push to Emacs (causality — GNU Emacs 30.2) ═══'"
Enter
Sleep 1s

Type "emacsclient --eval '(progn (load-file \"~/i/egregore-curve-labs/src/hamming-swarm-emacs.el\") (hamming-swarm-run))'"
Enter
Sleep 3s

Type "echo 'Hamming swarm buffer opened in Emacs — vertices colored via gay.el SplitMix64'"
Enter
Sleep 2s

# ═══ Act 6: Tailscale mesh status ═══
Type "echo ''"
Enter
Type "echo '═══ Act 6: Tailscale Mesh (abyss.pictures) ═══'"
Enter
Sleep 1s

Type "tailscale status | grep -v offline"
Enter
Sleep 3s

Type "echo 'Emacs on: causality only. GX10 nodes: compute-only (no emacs).'"
Enter
Sleep 2s

# ═══ Fin ═══
Type "echo ''"
Enter
Type "echo '42 dimensions × 9 validators × GF(27) Shamir × Aptos swarm_bootstrap'"
Enter
Type "echo 'Conservation: Σ trits ≡ 0 (mod 3) at every scale'"
Enter
Sleep 3s
