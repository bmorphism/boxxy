# Tape Verification: Gemini Embedding 2 Matryoshka Multimodal

Every VHS tape produced by boxxy **must** be verified using `scripts/tape-check.sh`
before it is considered complete. This is not optional.

## Protocol

```bash
./scripts/tape-check.sh tapes/my-tape.tape
# or directly on mp4:
./scripts/tape-check.sh tapes/gifs/my-tape.mp4
# with cross-reference:
./scripts/tape-check.sh tapes/my-tape.tape --reference tapes/baseline.tape
```

## What It Does

`tape-check.sh` uses **Gemini Embedding 2** (`gemini-embedding-2`) at three
matryoshka dimensions to verify tape integrity:

| Dimension | Purpose |
|-----------|---------|
| 768 | Fast/compact screening — catches gross failures |
| 1536 | Default production fidelity — standard verification |
| 3072 | Maximum recall — catches subtle decoherence |

**Matryoshka property**: the 768-dim vector is a strict prefix of the 1536-dim
vector, which is a strict prefix of the 3072-dim vector. The script verifies
this nesting invariant.

## Five Embedding Axes

1. **Full video → embedding**: Embed the entire mp4 as a multimodal input.
2. **Keyframes → embeddings**: Extract N keyframes (PNG), embed each independently.
3. **Tape script text → embedding**: Embed the `.tape` source as text (cross-modal reference).
4. **Text+Video → embedding**: Grounded multimodal — combine tape text with video.
5. **Frame-to-frame trajectory**: Temporal coherence of keyframe embeddings across the sequence.

## Color Losses (Inline)

The verifier also runs two color-loss metrics from `color_losses.zig` / `color-losses.py`
on extracted keyframes:

- **L5 (Path Length)**: Total arc length of the color trajectory through CIE L\*a\*b\* space.
  Detects if the tape's color sequence covers the expected perceptual distance.
- **L6 (Jacobian Spectral Norm)**: Maximum singular value of the color Jacobian.
  Detects sudden color discontinuities that indicate rendering glitches.

## Output

Results are written to `tapes/gifs/<tape-name>-check.json` with:
- Embedding vectors at all three dimensions (or hashes thereof)
- Matryoshka nesting verification (pass/fail)
- Cross-modal cosine similarities
- Temporal coherence scores
- Color loss values (L5, L6)
- Overall pass/fail verdict

## Requirements

- `GEMINI_API_KEY` (or `GOOGLE_API_KEY` or `GOOGLE_GENERATIVE_AI_API_KEY`)
- `ffmpeg` / `ffprobe` (keyframe extraction)
- `python3` with `numpy` and `PIL` (color losses)
- `jq` (JSON processing)
- `base64` (payload encoding)

## When to Run

- **After rendering any `.tape` → `.mp4`**: Always.
- **After modifying tape scripts**: Re-render and re-verify.
- **Before committing tape changes**: Gate on verification passing.
- **Cross-referencing**: Use `--reference` to compare a new tape against a known-good baseline.

## Integration with Skills

All skills that produce or consume VHS tapes should invoke `tape-check.sh` as a
post-processing step. The color losses in `color_losses.zig` (17 tests, 8 loss
functions) provide the numerical backbone; the Gemini embeddings provide the
perceptual/semantic backbone.

## The Eight Color Losses

For reference, the full set from `color_losses.zig` and `scripts/color-losses.py`:

| # | Loss | What It Measures |
|---|------|-----------------|
| L1 | Winding Number | Topological charge of hue trajectory |
| L2 | Spectral Gap | Energy gap between dominant modes |
| L3 | Transport Cost (Wasserstein-1) | Earth-mover distance between color distributions |
| L4 | Fisher-Rao Geodesic | Information-geometric distance on the statistical manifold |
| L5 | Path Length | Total arc length in CIE L\*a\*b\* |
| L6 | Jacobian Spectral Norm | Maximum rate of color change |
| L7 | Entropy Production | Irreversibility of the color trajectory |
| L8 | Lyapunov Exponent | Sensitivity to initial conditions |

## Creative Uses

The matryoshka dimensions enable creative verification strategies:

- **768-dim as canary**: Quick CI gate that fails fast on broken tapes.
- **1536→3072 delta**: Measure how much information the extra 1536 dims add.
  If the delta is large, the tape has subtle structure worth preserving.
- **Cross-modal gap**: Compare text-embedding vs video-embedding similarity.
  A large gap suggests the tape's visual output diverges from its script intent.
- **Temporal gradient**: Plot keyframe embeddings over time. Monotonic drift =
  narrative progression; sudden jumps = scene breaks or glitches.
