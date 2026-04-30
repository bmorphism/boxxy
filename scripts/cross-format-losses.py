#!/usr/bin/env python3
"""Cross-format (GIF vs MP4) color distinguishability analysis.

Gärdenfors' Conceptual Spaces framing:
  Quality dimensions: L*, a*, b* (CIE Lab) form a geometric color space.
  Concepts = convex regions in this space.
  Similarity = distance in conceptual space.
  Betweenness: if C is between A and B, compression must preserve this.

GIF specifics:
  - 256-color palette (indexed color), optional dithering
  - Lossless within the palette; lossy from palette quantization
  - No temporal compression; each frame independent

MP4 (H.264 yuv420p) specifics:
  - YCbCr 4:2:0 chroma subsampling (2x2 blocks share chroma)
  - DCT block quantization, inter-frame prediction
  - 8-bit per channel; temporal redundancy exploitation

Analysis dimensions:
  D1: Palette collapse (GIF) — distinct colors merged into one palette entry
  D2: Chroma smear (MP4) — spatial chroma resolution halved
  D3: Concept preservation — do convex regions in Lab survive both formats?
  D4: Betweenness violation — does compression break interpolation ordering?
  D5: Cross-format divergence — how far apart are GIF and MP4 of same content?
  D6: Temporal coherence delta — per-frame loss stability across time
  D7: Information channel capacity — bits of color info each format preserves
  D8: Voronoi shift — does format change nearest-concept assignment?
"""

import json
import math
import subprocess
import sys
from pathlib import Path

import numpy as np
from PIL import Image

BOXXY = Path(__file__).parent.parent
PAIRS = [
    ("parallel-spi", "tapes/gifs/parallel-spi"),
    ("tile-self-awareness", "tapes/gifs/tile-self-awareness"),
    ("triangulate-contrast", "tapes/gifs/triangulate-contrast"),
]
N_FRAMES = 8


# ━━━━━━ Color space conversions ━━━━━━

def srgb_to_linear(c):
    return np.where(c <= 0.04045, c / 12.92, ((c + 0.055) / 1.055) ** 2.4)

def linear_to_xyz(rgb):
    M = np.array([
        [0.4124564, 0.3575761, 0.1804375],
        [0.2126729, 0.7151522, 0.0721750],
        [0.0193339, 0.1191920, 0.9503041],
    ])
    return rgb @ M.T

def xyz_to_lab(xyz):
    Xn, Yn, Zn = 0.95047, 1.00000, 1.08883
    xyz_n = xyz / np.array([Xn, Yn, Zn])
    delta = 6.0 / 29.0
    f = np.where(
        xyz_n > delta ** 3,
        np.cbrt(xyz_n),
        xyz_n / (3 * delta ** 2) + 4.0 / 29.0,
    )
    L = 116 * f[..., 1] - 16
    a = 500 * (f[..., 0] - f[..., 1])
    b = 200 * (f[..., 1] - f[..., 2])
    return np.stack([L, a, b], axis=-1)

def rgb_to_lab(img_array):
    """uint8 RGB array -> Lab array."""
    srgb = img_array.astype(np.float64) / 255.0
    linear = srgb_to_linear(srgb)
    xyz = linear_to_xyz(linear)
    return xyz_to_lab(xyz)


# ━━━━━━ Frame extraction ━━━━━━

def extract_frames_mp4(path, n_frames):
    """Extract n_frames evenly spaced from mp4."""
    probe = subprocess.run(
        ["ffprobe", "-v", "error", "-count_frames",
         "-select_streams", "v:0",
         "-show_entries", "stream=nb_read_frames,width,height",
         "-of", "csv=p=0", str(path)],
        capture_output=True, text=True
    )
    parts = probe.stdout.strip().split(",")
    w, h = int(parts[0]), int(parts[1])
    total = int(parts[2]) if len(parts) > 2 and parts[2].isdigit() else 30

    indices = np.linspace(0, max(total - 1, 0), n_frames, dtype=int)
    frames = []
    for idx in indices:
        ts = idx / max(total, 1)
        result = subprocess.run(
            ["ffmpeg", "-ss", str(ts), "-i", str(path),
             "-vframes", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
            capture_output=True
        )
        if result.returncode == 0 and len(result.stdout) == w * h * 3:
            arr = np.frombuffer(result.stdout, dtype=np.uint8).reshape(h, w, 3)
            frames.append(arr)
    return frames

def extract_frames_gif(path, n_frames):
    """Extract n_frames evenly spaced from gif."""
    img = Image.open(str(path))
    total = 0
    try:
        while True:
            total += 1
            img.seek(img.tell() + 1)
    except EOFError:
        pass

    indices = np.linspace(0, max(total - 1, 0), n_frames, dtype=int)
    frames = []
    for idx in indices:
        img.seek(int(idx))
        frame = img.convert("RGB")
        frames.append(np.array(frame))
    return frames


# ━━━━━━ Gärdenfors quality-dimension losses ━━━━━━

def delta_e2000(lab1, lab2):
    """CIE DE2000 per-pixel, returns mean."""
    dL = lab2[..., 0] - lab1[..., 0]
    C1 = np.sqrt(lab1[..., 1]**2 + lab1[..., 2]**2)
    C2 = np.sqrt(lab2[..., 1]**2 + lab2[..., 2]**2)
    Cm = (C1 + C2) / 2
    G = 0.5 * (1 - np.sqrt(Cm**7 / (Cm**7 + 25**7)))
    a1p = lab1[..., 1] * (1 + G)
    a2p = lab2[..., 1] * (1 + G)
    C1p = np.sqrt(a1p**2 + lab1[..., 2]**2)
    C2p = np.sqrt(a2p**2 + lab2[..., 2]**2)
    h1p = np.degrees(np.arctan2(lab1[..., 2], a1p)) % 360
    h2p = np.degrees(np.arctan2(lab2[..., 2], a2p)) % 360
    dCp = C2p - C1p
    dh = h2p - h1p
    dh = np.where(np.abs(dh) > 180, dh - np.sign(dh) * 360, dh)
    dHp = 2 * np.sqrt(C1p * C2p) * np.sin(np.radians(dh / 2))
    Lm = (lab1[..., 0] + lab2[..., 0]) / 2
    Cm_p = (C1p + C2p) / 2
    SL = 1 + 0.015 * (Lm - 50)**2 / np.sqrt(20 + (Lm - 50)**2)
    SC = 1 + 0.045 * Cm_p
    hm = (h1p + h2p) / 2
    hm = np.where(np.abs(h1p - h2p) > 180, hm + 180, hm)
    T = (1 - 0.17 * np.cos(np.radians(hm - 30))
         + 0.24 * np.cos(np.radians(2 * hm))
         + 0.32 * np.cos(np.radians(3 * hm + 6))
         - 0.20 * np.cos(np.radians(4 * hm - 63)))
    SH = 1 + 0.015 * Cm_p * T
    RT = (-2 * np.sqrt(Cm_p**7 / (Cm_p**7 + 25**7))
          * np.sin(np.radians(60 * np.exp(-((hm - 275) / 25)**2))))
    de = np.sqrt((dL / SL)**2 + (dCp / SC)**2 + (dHp / SH)**2
                 + RT * (dCp / SC) * (dHp / SH))
    return float(np.nanmean(de))


def unique_colors(img_array):
    """Count distinct RGB triples."""
    flat = img_array.reshape(-1, 3)
    return len(np.unique(flat, axis=0))


def palette_collapse_ratio(gif_frame, mp4_frame):
    """D1: Ratio of unique colors. GIF palette quantization reduces this."""
    gif_uc = unique_colors(gif_frame)
    mp4_uc = unique_colors(mp4_frame)
    return {"gif_unique": gif_uc, "mp4_unique": mp4_uc,
            "ratio_gif_to_mp4": gif_uc / max(mp4_uc, 1)}


def chroma_plane_divergence(lab_gif, lab_mp4):
    """D2: Wasserstein-like distance on (a*,b*) marginals."""
    ab_gif = lab_gif[..., 1:].reshape(-1, 2)
    ab_mp4 = lab_mp4[..., 1:].reshape(-1, 2)
    # Approximate W1 via sorted 1D projections (sliced Wasserstein)
    dists = []
    for theta in np.linspace(0, np.pi, 8, endpoint=False):
        proj_g = ab_gif[:, 0] * np.cos(theta) + ab_gif[:, 1] * np.sin(theta)
        proj_m = ab_mp4[:, 0] * np.cos(theta) + ab_mp4[:, 1] * np.sin(theta)
        n = min(len(proj_g), len(proj_m))
        sg = np.sort(np.random.choice(proj_g, n, replace=False))
        sm = np.sort(np.random.choice(proj_m, n, replace=False))
        dists.append(float(np.mean(np.abs(sg - sm))))
    return float(np.mean(dists))


def concept_preservation(lab_gif, lab_mp4, n_clusters=5):
    """D3: Do convex regions (concepts) in Lab survive both formats?
    Uses k-means-like clustering on one format, checks containment in other."""
    flat_g = lab_gif.reshape(-1, 3)
    flat_m = lab_mp4.reshape(-1, 3)
    # Simple k-means on gif colors
    np.random.seed(42)
    n = min(2000, len(flat_g))
    sample_g = flat_g[np.random.choice(len(flat_g), n, replace=False)]
    sample_m = flat_m[np.random.choice(len(flat_m), min(n, len(flat_m)), replace=False)]
    # Initialize centroids from gif
    centroids = sample_g[np.random.choice(n, n_clusters, replace=False)]
    for _ in range(20):
        dists = np.linalg.norm(sample_g[:, None, :] - centroids[None, :, :], axis=2)
        labels = np.argmin(dists, axis=1)
        for k in range(n_clusters):
            mask = labels == k
            if np.any(mask):
                centroids[k] = sample_g[mask].mean(axis=0)
    # Compute radii (max distance from centroid in each cluster)
    radii = np.zeros(n_clusters)
    for k in range(n_clusters):
        mask = labels == k
        if np.any(mask):
            radii[k] = np.max(np.linalg.norm(sample_g[mask] - centroids[k], axis=1))
    # Check how many mp4 samples fall within gif-defined concepts
    dists_m = np.linalg.norm(sample_m[:, None, :] - centroids[None, :, :], axis=2)
    nearest_m = np.argmin(dists_m, axis=1)
    dist_to_centroid = dists_m[np.arange(len(sample_m)), nearest_m]
    in_concept = dist_to_centroid <= radii[nearest_m] * 1.1  # 10% tolerance
    preservation = float(np.mean(in_concept))
    return preservation


def betweenness_violation(lab_gif, lab_mp4, n_tests=200):
    """D4: Does compression break betweenness?
    If color C is between A and B in gif, is it still between in mp4?
    Betweenness: d(A,C) + d(C,B) ≈ d(A,B) (collinearity)."""
    flat_g = lab_gif.reshape(-1, 3)
    flat_m = lab_mp4.reshape(-1, 3)
    n = min(len(flat_g), len(flat_m))
    np.random.seed(42)
    violations = 0
    for _ in range(n_tests):
        i, j, k = np.random.choice(n, 3, replace=False)
        # Check if k is between i and j in gif
        d_ij_g = np.linalg.norm(flat_g[i] - flat_g[j])
        d_ik_g = np.linalg.norm(flat_g[i] - flat_g[k])
        d_kj_g = np.linalg.norm(flat_g[k] - flat_g[j])
        if d_ij_g < 1e-6:
            continue
        colinearity_g = (d_ik_g + d_kj_g) / d_ij_g
        if colinearity_g < 1.05:  # approximately between
            # Check if still between in mp4
            d_ij_m = np.linalg.norm(flat_m[i] - flat_m[j])
            d_ik_m = np.linalg.norm(flat_m[i] - flat_m[k])
            d_kj_m = np.linalg.norm(flat_m[k] - flat_m[j])
            if d_ij_m < 1e-6:
                continue
            colinearity_m = (d_ik_m + d_kj_m) / d_ij_m
            if colinearity_m > 1.2:  # no longer between
                violations += 1
    return violations / max(n_tests, 1)


def cross_format_de(lab_gif, lab_mp4):
    """D5: Mean CIE DE2000 between GIF and MP4 of same frame."""
    h, w = min(lab_gif.shape[0], lab_mp4.shape[0]), min(lab_gif.shape[1], lab_mp4.shape[1])
    return delta_e2000(lab_gif[:h, :w], lab_mp4[:h, :w])


def color_gradient_energy(lab):
    """Gradient energy ||nabla C||^2 in Lab."""
    dy = np.diff(lab, axis=0)
    dx = np.diff(lab, axis=1)
    gy = np.sum(dy**2, axis=-1).mean()
    gx = np.sum(dx**2, axis=-1).mean()
    return float(gy + gx)


def mutual_info_channels(img_array, bins=32):
    """D7: Mutual information between R,G,B channels."""
    flat = img_array.reshape(-1, 3).astype(float)
    mi_total = 0.0
    for i in range(3):
        for j in range(i + 1, 3):
            hist2d, _, _ = np.histogram2d(flat[:, i], flat[:, j], bins=bins)
            pxy = hist2d / hist2d.sum()
            px = pxy.sum(axis=1)
            py = pxy.sum(axis=0)
            mask = pxy > 0
            mi = np.sum(pxy[mask] * np.log2(pxy[mask] / (px[:, None] * py[None, :])[mask]))
            mi_total += mi
    return float(mi_total / 3)  # average over 3 pairs


def voronoi_shift_ratio(lab_gif, lab_mp4, n_anchors=8):
    """D8: Does the format change nearest-concept assignment?
    Uses anchor points as concept exemplars, checks if nearest-anchor changes."""
    flat_g = lab_gif.reshape(-1, 3)
    flat_m = lab_mp4.reshape(-1, 3)
    n = min(len(flat_g), len(flat_m))
    np.random.seed(42)
    anchors = flat_g[np.random.choice(len(flat_g), n_anchors, replace=False)]
    sample_idx = np.random.choice(n, min(2000, n), replace=False)
    sg = flat_g[sample_idx]
    sm = flat_m[sample_idx]
    nearest_g = np.argmin(np.linalg.norm(sg[:, None, :] - anchors[None, :, :], axis=2), axis=1)
    nearest_m = np.argmin(np.linalg.norm(sm[:, None, :] - anchors[None, :, :], axis=2), axis=1)
    shifted = float(np.mean(nearest_g != nearest_m))
    return shifted


# ━━━━━━ Main analysis ━━━━━━

def analyze_pair(name, base_path):
    """Run all Gärdenfors dimensions on one GIF/MP4 pair."""
    gif_path = BOXXY / f"{base_path}.gif"
    mp4_path = BOXXY / f"{base_path}.mp4"

    if not gif_path.exists() or not mp4_path.exists():
        print(f"  SKIP {name}: missing file(s)", file=sys.stderr)
        return None

    print(f"\n{'='*60}", file=sys.stderr)
    print(f"  Analyzing: {name}", file=sys.stderr)
    print(f"  GIF: {gif_path.stat().st_size / 1024:.0f}KB", file=sys.stderr)
    print(f"  MP4: {mp4_path.stat().st_size / 1024:.0f}KB", file=sys.stderr)

    gif_frames = extract_frames_gif(gif_path, N_FRAMES)
    mp4_frames = extract_frames_mp4(mp4_path, N_FRAMES)
    n = min(len(gif_frames), len(mp4_frames))
    if n == 0:
        print(f"  SKIP {name}: no frames extracted", file=sys.stderr)
        return None

    print(f"  Frames: {n} (gif={len(gif_frames)}, mp4={len(mp4_frames)})", file=sys.stderr)

    results = {
        "name": name,
        "gif_size_kb": gif_path.stat().st_size / 1024,
        "mp4_size_kb": mp4_path.stat().st_size / 1024,
        "compression_ratio": gif_path.stat().st_size / max(mp4_path.stat().st_size, 1),
        "n_frames": n,
        "per_frame": [],
        "aggregated": {},
    }

    d1_vals, d2_vals, d3_vals, d4_vals, d5_vals = [], [], [], [], []
    d6_gif_ge, d6_mp4_ge = [], []
    d7_gif_mi, d7_mp4_mi = [], []
    d8_vals = []

    for i in range(n):
        gf = gif_frames[i]
        mf = mp4_frames[i]
        # Resize to common dimensions if needed
        h, w = min(gf.shape[0], mf.shape[0]), min(gf.shape[1], mf.shape[1])
        gf = gf[:h, :w]
        mf = mf[:h, :w]

        lab_g = rgb_to_lab(gf)
        lab_m = rgb_to_lab(mf)

        d1 = palette_collapse_ratio(gf, mf)
        d2 = chroma_plane_divergence(lab_g, lab_m)
        d3 = concept_preservation(lab_g, lab_m)
        d4 = betweenness_violation(lab_g, lab_m)
        d5 = cross_format_de(lab_g, lab_m)
        ge_g = color_gradient_energy(lab_g)
        ge_m = color_gradient_energy(lab_m)
        mi_g = mutual_info_channels(gf)
        mi_m = mutual_info_channels(mf)
        d8 = voronoi_shift_ratio(lab_g, lab_m)

        frame_data = {
            "frame": i,
            "D1_palette_collapse": d1,
            "D2_chroma_divergence": round(d2, 4),
            "D3_concept_preservation": round(d3, 4),
            "D4_betweenness_violation": round(d4, 4),
            "D5_cross_format_de2000": round(d5, 4),
            "D6_gradient_energy_gif": round(ge_g, 2),
            "D6_gradient_energy_mp4": round(ge_m, 2),
            "D7_mutual_info_gif": round(mi_g, 4),
            "D7_mutual_info_mp4": round(mi_m, 4),
            "D8_voronoi_shift": round(d8, 4),
        }
        results["per_frame"].append(frame_data)

        d1_vals.append(d1["ratio_gif_to_mp4"])
        d2_vals.append(d2)
        d3_vals.append(d3)
        d4_vals.append(d4)
        d5_vals.append(d5)
        d6_gif_ge.append(ge_g)
        d6_mp4_ge.append(ge_m)
        d7_gif_mi.append(mi_g)
        d7_mp4_mi.append(mi_m)
        d8_vals.append(d8)

        print(f"  Frame {i}: DE={d5:.2f} concept={d3:.3f} voronoi_shift={d8:.3f}",
              file=sys.stderr)

    results["aggregated"] = {
        "D1_mean_palette_ratio": round(np.mean(d1_vals), 4),
        "D2_mean_chroma_divergence": round(np.mean(d2_vals), 4),
        "D3_mean_concept_preservation": round(np.mean(d3_vals), 4),
        "D4_mean_betweenness_violation": round(np.mean(d4_vals), 4),
        "D5_mean_cross_format_de2000": round(np.mean(d5_vals), 4),
        "D6_gradient_energy_ratio": round(np.mean(d6_gif_ge) / max(np.mean(d6_mp4_ge), 0.01), 4),
        "D6_temporal_stability_gif": round(float(np.std(d6_gif_ge) / max(np.mean(d6_gif_ge), 0.01)), 4),
        "D6_temporal_stability_mp4": round(float(np.std(d6_mp4_ge) / max(np.mean(d6_mp4_ge), 0.01)), 4),
        "D7_mi_gif": round(np.mean(d7_gif_mi), 4),
        "D7_mi_mp4": round(np.mean(d7_mp4_mi), 4),
        "D7_mi_loss": round(np.mean(d7_gif_mi) - np.mean(d7_mp4_mi), 4),
        "D8_mean_voronoi_shift": round(np.mean(d8_vals), 4),
    }

    # Gärdenfors summary
    agg = results["aggregated"]
    results["gardenfors_summary"] = {
        "quality_dimension_fidelity": "HIGH" if agg["D5_mean_cross_format_de2000"] < 3.0 else
                                      "MEDIUM" if agg["D5_mean_cross_format_de2000"] < 8.0 else "LOW",
        "concept_convexity_preserved": bool(agg["D3_mean_concept_preservation"] > 0.85),
        "betweenness_intact": bool(agg["D4_mean_betweenness_violation"] < 0.05),
        "voronoi_stable": bool(agg["D8_mean_voronoi_shift"] < 0.1),
        "format_with_more_info": "GIF" if agg["D7_mi_loss"] > 0 else "MP4",
        "palette_compression_severe": bool(agg["D1_mean_palette_ratio"] < 0.3),
        "verdict": "",
    }
    gs = results["gardenfors_summary"]
    if gs["concept_convexity_preserved"] and gs["betweenness_intact"] and gs["voronoi_stable"]:
        gs["verdict"] = "CONCEPTUAL_EQUIVALENCE: Both formats preserve the same conceptual space structure"
    elif gs["concept_convexity_preserved"]:
        gs["verdict"] = "PARTIAL_PRESERVATION: Concepts survive but betweenness or Voronoi boundaries shift"
    else:
        gs["verdict"] = "CONCEPTUAL_DIVERGENCE: Formats induce different conceptual space partitions"

    return results


def main():
    all_results = {"pairs": [], "cross_pair_summary": {}}

    for name, base in PAIRS:
        result = analyze_pair(name, base)
        if result:
            all_results["pairs"].append(result)

    if not all_results["pairs"]:
        print("No pairs analyzed.", file=sys.stderr)
        sys.exit(1)

    # Cross-pair aggregation
    de_vals = [p["aggregated"]["D5_mean_cross_format_de2000"] for p in all_results["pairs"]]
    cp_vals = [p["aggregated"]["D3_mean_concept_preservation"] for p in all_results["pairs"]]
    vs_vals = [p["aggregated"]["D8_mean_voronoi_shift"] for p in all_results["pairs"]]
    bv_vals = [p["aggregated"]["D4_mean_betweenness_violation"] for p in all_results["pairs"]]

    all_results["cross_pair_summary"] = {
        "mean_de2000_across_pairs": round(np.mean(de_vals), 4),
        "mean_concept_preservation": round(np.mean(cp_vals), 4),
        "mean_voronoi_shift": round(np.mean(vs_vals), 4),
        "mean_betweenness_violation": round(np.mean(bv_vals), 4),
        "worst_pair_by_de": max(all_results["pairs"], key=lambda p: p["aggregated"]["D5_mean_cross_format_de2000"])["name"],
        "best_pair_by_de": min(all_results["pairs"], key=lambda p: p["aggregated"]["D5_mean_cross_format_de2000"])["name"],
        "gardenfors_overall": "",
    }

    mean_de = all_results["cross_pair_summary"]["mean_de2000_across_pairs"]
    mean_cp = all_results["cross_pair_summary"]["mean_concept_preservation"]
    if mean_de < 5.0 and mean_cp > 0.85:
        all_results["cross_pair_summary"]["gardenfors_overall"] = (
            "GIF and MP4 occupy overlapping regions in Gardenfors color space. "
            "Format choice is a compression-efficiency trade-off, not a conceptual one."
        )
    elif mean_de < 10.0:
        all_results["cross_pair_summary"]["gardenfors_overall"] = (
            "GIF and MP4 show measurable conceptual divergence. "
            "Some color concepts merge or shift between formats."
        )
    else:
        all_results["cross_pair_summary"]["gardenfors_overall"] = (
            "GIF and MP4 induce substantially different conceptual spaces. "
            "Color reasoning depends on format choice."
        )

    out_path = BOXXY / "tapes/gifs/cross-format-losses.json"
    with open(out_path, "w") as f:
        json.dump(all_results, f, indent=2)
    print(f"\nResults written to {out_path}", file=sys.stderr)
    print(json.dumps(all_results, indent=2))


if __name__ == "__main__":
    main()
