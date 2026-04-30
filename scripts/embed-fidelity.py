#!/usr/bin/env python3
"""Embed VHS .mp4 outputs at multiple Gemini embedding-2 dimensions.

Computes cosine similarity between two videos at each dim level
to find the fidelity boundary where bifurcating scrutiny decoheres.

Gemini embedding-2 supports: 768 (compact), 1536 (default), 3072 (max recall).
"""

import base64
import json
import os
import sys
import math

try:
    import google.generativeai as genai
except ImportError:
    print("pip install google-generativeai", file=sys.stderr)
    sys.exit(1)

API_KEY = (
    os.environ.get("GEMINI_API_KEY")
    or os.environ.get("GOOGLE_API_KEY")
    or os.environ.get("GOOGLE_GENERATIVE_AI_API_KEY")
)
if not API_KEY:
    print("No Gemini API key found", file=sys.stderr)
    sys.exit(1)

genai.configure(api_key=API_KEY)

DIMS = [768, 1536, 3072]
VIDEO_A = "tapes/gifs/tile-self-awareness.mp4"
VIDEO_B = "tapes/gifs/triangulate-contrast.mp4"


def cosine_sim(a, b):
    dot = sum(x * y for x, y in zip(a, b))
    norm_a = math.sqrt(sum(x * x for x in a))
    norm_b = math.sqrt(sum(x * x for x in b))
    if norm_a == 0 or norm_b == 0:
        return 0.0
    return dot / (norm_a * norm_b)


def l2_dist(a, b):
    return math.sqrt(sum((x - y) ** 2 for x, y in zip(a, b)))


def embed_video(path, dim):
    """Embed a video file using Gemini embedding-2 at specified dimensions."""
    with open(path, "rb") as f:
        video_bytes = f.read()

    b64 = base64.standard_b64encode(video_bytes).decode("utf-8")

    result = genai.embed_content(
        model="models/gemini-embedding-exp-03-07",
        content={
            "parts": [
                {"inline_data": {"mime_type": "video/mp4", "data": b64}}
            ]
        },
        output_dimensionality=dim,
        task_type="RETRIEVAL_DOCUMENT",
    )
    return result["embedding"]


def main():
    print(f"Video A: {VIDEO_A}")
    print(f"Video B: {VIDEO_B}")
    print(f"Dimensions to probe: {DIMS}")
    print()

    results = []

    for dim in DIMS:
        print(f"--- dim={dim} ---")
        try:
            emb_a = embed_video(VIDEO_A, dim)
            emb_b = embed_video(VIDEO_B, dim)
        except Exception as e:
            print(f"  ERROR: {e}")
            # Try text-based fallback description
            results.append({"dim": dim, "error": str(e)})
            continue

        sim = cosine_sim(emb_a, emb_b)
        dist = l2_dist(emb_a, emb_b)

        # Compute self-similarity as sanity check
        self_sim = cosine_sim(emb_a, emb_a)

        # Bifurcation score: how much the two videos diverge
        # 1.0 = identical, 0.0 = orthogonal, <0 = opposed
        bifurc = 1.0 - sim

        r = {
            "dim": dim,
            "cosine_sim": round(sim, 6),
            "l2_dist": round(dist, 4),
            "self_sim": round(self_sim, 6),
            "bifurcation": round(bifurc, 6),
            "emb_a_norm": round(math.sqrt(sum(x * x for x in emb_a)), 4),
            "emb_b_norm": round(math.sqrt(sum(x * x for x in emb_b)), 4),
        }
        results.append(r)
        print(f"  cosine_sim:   {r['cosine_sim']}")
        print(f"  l2_dist:      {r['l2_dist']}")
        print(f"  bifurcation:  {r['bifurcation']}")
        print(f"  norms:        A={r['emb_a_norm']} B={r['emb_b_norm']}")
        print()

    # Find decoherence boundary
    print("=== FIDELITY BOUNDARY ANALYSIS ===")
    valid = [r for r in results if "error" not in r]
    if len(valid) >= 2:
        # The boundary is where bifurcation score changes most
        for i in range(1, len(valid)):
            delta = valid[i]["bifurcation"] - valid[i - 1]["bifurcation"]
            print(
                f"  {valid[i-1]['dim']} → {valid[i]['dim']}: "
                f"Δbifurcation = {delta:+.6f}"
            )
            if delta > 0:
                print(
                    f"    ↑ MORE divergent at higher dim "
                    f"(scrutiny reveals difference)"
                )
            else:
                print(
                    f"    ↓ LESS divergent at higher dim "
                    f"(higher fidelity collapses distinction)"
                )

        # The decoherence point
        max_bifurc = max(valid, key=lambda r: r["bifurcation"])
        min_bifurc = min(valid, key=lambda r: r["bifurcation"])
        print()
        print(f"  Peak bifurcation at dim={max_bifurc['dim']}: {max_bifurc['bifurcation']}")
        print(f"  Min bifurcation at dim={min_bifurc['dim']}: {min_bifurc['bifurcation']}")
        print(
            f"  Decoherence boundary: dim ∈ [{min_bifurc['dim']}, {max_bifurc['dim']}]"
        )

    # Dump JSON
    out = json.dumps(results, indent=2)
    with open("tapes/gifs/embedding-fidelity.json", "w") as f:
        f.write(out)
    print(f"\nResults written to tapes/gifs/embedding-fidelity.json")


if __name__ == "__main__":
    main()
