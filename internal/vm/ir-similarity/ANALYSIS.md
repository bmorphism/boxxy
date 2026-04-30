# LLVM IR Similarity Analysis: Cross-Implementation Identity Detection

## Overview

This analysis uses LLVM's `IRSimilarityIdentifier` (suffix-tree-based instruction
hashing) to prove that two independently-written C implementations of the same
mathematical operations produce **operationally identical** LLVM IR, despite
different variable names, struct layouts, and coding styles.

## Pipeline

```
ops_go_style.c   ──┐
                    ├──► clang -O2 -emit-llvm -S ──► llvm-link ──► opt -passes=print-ir-similarity
ops_java_style.c ──┘
```

- **Compiler**: Apple clang 17.0.0 (arm64-apple-darwin25)
- **Analyzer**: LLVM 21.1.2 opt (`/nix/store/3q0facdwp42jawa24zd01kp9wf49g7xd-llvm-21.1.2/bin/opt`)

## Implementations Compared

| Go-style function | Java-style function | Purpose |
|---|---|---|
| `splitmix64_go` | `mix64` | Gay.jl SplitMix64 bijection |
| `colorAt_go` | `deterministicHSL` | HSL from (seed, index) |
| `seedFromName_go` | `fnv1a_hash` | FNV-1a string hash |
| `clampHue_go` | `normalizeHue` | NaN/Inf-safe hue clamping |
| `clamp01_go` | `saturate` | NaN/Inf-safe [0,1] clamping |

## Results at -O2

**30 similarity groups** found. Key findings:

| Matched Pair | Longest Candidate (instructions) | What's Identical |
|---|---|---|
| `colorAt_go` ↔ `deterministicHSL` | **14** | Full SplitMix64 + hue computation |
| `clampHue_go` ↔ `normalizeHue` | **11** | Complete NaN/Inf check + fmod + wrap |
| `seedFromName_go` ↔ `fnv1a_hash` | **8** | FNV-1a loop body |
| `clamp01_go` ↔ `saturate` | **3** | NaN branch + select |

The SplitMix64 bijection is **perfectly inlined** into both `colorAt_go` and
`deterministicHSL` at -O2. The similarity detector identifies the full
14-instruction sequence from `xor seed,index` through `fmul hue, 360.0` as
identical across both implementations.

## Results at -O0

Without optimization, similarity candidates reach up to **55 instructions**
(the entire clamp function bodies before branch folding).

## Significance

This demonstrates that LLVM's `IRSimilarityIdentifier` can:

1. **Cross-language deduplication**: Find identical computation across implementations
   written in different styles (Go-style vs Java-style naming/structure)
2. **Semantic equality through structural hashing**: The suffix-tree approach
   canonicalizes SSA values, so `%6 = xor i64 %1, %0` matches `%3 = xor i64 %1, %0`
3. **Composable with TangoLLVM**: ByteDance's Go→LLVM IR transpiler would let us
   compile the actual Go source to bitcode and compare directly against C/Rust/Java
   implementations

## Connection to REBEL (Sodani, Moos, Mirman - ICML 2023)

The recursive decomposition insight from REBEL applies here: just as REBEL
decomposes complex problems into tool-assisted sub-problems, the IR similarity
pipeline decomposes programs into canonical instruction sequences. The suffix tree
is the "recursive decomposition" — each `SimilarityGroup` is an operationally
identical sub-computation discovered without source-level hints.

## Files

- `ops_go_style.c` — Go-style C translation of boxxy's core ops
- `ops_java_style.c` — Java-style C translation (same semantics, different structure)
- `merged.ll` — Linked LLVM IR at -O2
- `merged_O0.ll` — Linked LLVM IR at -O0
- `outlined.ll` — Post-outliner output
