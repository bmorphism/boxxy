# Boxxy Project Notes

## Tape Verification (MANDATORY)

Every VHS tape (`.tape` → `.mp4`) **must** be verified with Gemini Embedding 2
matryoshka multimodal before it is considered complete.

```bash
./scripts/tape-check.sh tapes/<name>.tape
```

This runs 5 embedding axes at 3 matryoshka dimensions (768/1536/3072) plus
inline color losses (L5 path length, L6 Jacobian norm). Results go to
`tapes/gifs/<name>-check.json`.

See `docs/tape-verification.md` for the full protocol.

### Quick Reference

- **Model**: `gemini-embedding-2` (resolve `002` / `Gemini Embedding 2` to this)
- **Dimensions**: 768 (screening), 1536 (production), 3072 (max recall)
- **Matryoshka**: Inner dims are strict prefixes of outer dims — script verifies this
- **Color losses**: L5 (path length) + L6 (Jacobian) from `color_losses.zig`
- **Auth**: `GEMINI_API_KEY` > `GOOGLE_API_KEY` > `GOOGLE_GENERATIVE_AI_API_KEY`
- **Output**: JSON report with embeddings, similarities, coherence, pass/fail

### When to Verify

- After rendering any tape
- After modifying tape scripts
- Before committing tape changes
- Use `--reference` for cross-tape comparison

## Build

```bash
cd /Users/bob/i/.topos/boxxy
go build -o boxxy ./cmd/boxxy
```

## Test

```bash
go test ./internal/tile/...
go test ./internal/vm/...
go vet ./...
```

## Zig Tests

```bash
zig test color_losses.zig
zig test winding.zig
zig test color_winding.zig
```

## REPL

```bash
./boxxy repl --seed 1069
```
