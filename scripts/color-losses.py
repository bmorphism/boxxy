#!/usr/bin/env python3
"""Color-specific losses beyond cosine similarity.

Losses designed around what mp4 H.264 yuv420p actually does to color:
- YCbCr 4:2:0 chroma subsampling (2x2 blocks share one chroma sample)
- DCT block quantization (8x8 luma, 4x4 chroma in some profiles)
- 8-bit depth per channel

TRACTABLE (computed here):
  L1: CIE ΔE2000 — perceptual color difference in Lab space
  L2: Chroma-plane Wasserstein-1 — EMD on (a*,b*) marginals
  L3: Color gradient energy ratio — ||∇C||² revealing block artifacts
  L4: Chroma subsampling fidelity — what 4:2:0 destroyed
  L5: Color path length — arc length of Lab trajectory across frames
  L6: Color flow Jacobian — det(J) orientation preservation (diffeomorphic?)
  L7: Topological persistence proxy — connected components at multiple ε
  L8: Mutual information of color channels — what H.264 decorrelation preserved

INTRACTABLE ON DIGITAL COMPUTERS (framed here, need reservoir ensemble):
  R1: Generalized synchronization — do two reservoirs driven by color streams sync?
  R2: Information Processing Capacity — linear/nonlinear separation of color signal
  R3: Echo state kernel distance — reservoir state divergence
  R4: Lyapunov spectrum from color dynamics — continuous ODE, not discrete approx
  R5: Fading memory profile — how reservoir kernel decays for color sequences

These R-losses require continuous dynamics that digital computers approximate
with O(2^n) cost but reservoir computers compute in O(n) physical time.
The ensemble of multiple reservoirs with different spectral radii samples
the space of kernel functions, making the intractable tractable.
"""

import json
import math
import os
import struct
import subprocess
import sys
from pathlib import Path

import numpy as np
from PIL import Image

BOXXY = Path(__file__).parent.parent
VIDEO_A = BOXXY / "tapes/gifs/tile-self-awareness.mp4"
VIDEO_B = BOXXY / "tapes/gifs/triangulate-contrast.mp4"
N_FRAMES = 10
RESULTS_OUT = BOXXY / "tapes/gifs/color-losses.json"


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Color space conversions (pure numpy, no external deps)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def srgb_to_linear(c):
    """sRGB [0,1] → linear RGB [0,1] (inverse gamma)."""
    return np.where(c <= 0.04045, c / 12.92, ((c + 0.055) / 1.055) ** 2.4)

def linear_to_xyz(rgb):
    """Linear RGB → CIE XYZ (D65). rgb shape: (..., 3)."""
    M = np.array([
        [0.4124564, 0.3575761, 0.1804375],
        [0.2126729, 0.7151522, 0.0721750],
        [0.0193339, 0.1191920, 0.9503041],
    ])
    return rgb @ M.T

def xyz_to_lab(xyz):
    """CIE XYZ → CIE L*a*b* (D65 illuminant)."""
    # D65 reference white
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

def rgb_to_lab(img_uint8):
    """uint8 RGB image → Lab float64 array."""
    rgb = img_uint8.astype(np.float64) / 255.0
    linear = srgb_to_linear(rgb)
    xyz = linear_to_xyz(linear)
    return xyz_to_lab(xyz)

def rgb_to_ycbcr(img_uint8):
    """uint8 RGB → YCbCr float (BT.601, same as H.264 default)."""
    r, g, b = img_uint8[..., 0].astype(np.float64), img_uint8[..., 1].astype(np.float64), img_uint8[..., 2].astype(np.float64)
    Y  =  0.299 * r + 0.587 * g + 0.114 * b
    Cb = -0.168736 * r - 0.331264 * g + 0.5 * b + 128
    Cr =  0.5 * r - 0.418688 * g - 0.081312 * b + 128
    return np.stack([Y, Cb, Cr], axis=-1)


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Frame extraction via ffmpeg → raw RGB → numpy
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def get_duration(video):
    r = subprocess.run(
        ["ffprobe", "-v", "quiet", "-show_entries", "format=duration",
         "-of", "csv=p=0", str(video)],
        capture_output=True, text=True
    )
    return float(r.stdout.strip())

def extract_frame_rgb(video, t, w=1500, h=1000):
    """Extract a single frame as raw RGB numpy array."""
    r = subprocess.run(
        ["ffmpeg", "-v", "quiet", "-ss", f"{t:.3f}", "-i", str(video),
         "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1"],
        capture_output=True
    )
    if len(r.stdout) < w * h * 3:
        # Fallback: last frame
        r = subprocess.run(
            ["ffmpeg", "-v", "quiet", "-sseof", "-0.1", "-i", str(video),
             "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1"],
            capture_output=True
        )
    return np.frombuffer(r.stdout[:w * h * 3], dtype=np.uint8).reshape(h, w, 3)

def extract_frames(video, n=N_FRAMES):
    """Extract n evenly-spaced frames as list of (h, w, 3) uint8 arrays."""
    dur = get_duration(video)
    frames = []
    for i in range(n):
        t = dur * i / max(n - 1, 1)
        if t > dur - 0.05:
            t = max(0, dur - 0.1)
        frames.append(extract_frame_rgb(video, t))
    return frames


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L1: CIE ΔE2000 (perceptual color difference)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def delta_e_2000(lab1, lab2):
    """CIE ΔE2000 between two Lab arrays. Returns per-pixel ΔE."""
    L1, a1, b1 = lab1[..., 0], lab1[..., 1], lab1[..., 2]
    L2, a2, b2 = lab2[..., 0], lab2[..., 1], lab2[..., 2]

    Lbar = (L1 + L2) / 2
    C1 = np.sqrt(a1**2 + b1**2)
    C2 = np.sqrt(a2**2 + b2**2)
    Cbar = (C1 + C2) / 2
    Cbar7 = Cbar**7
    G = 0.5 * (1 - np.sqrt(Cbar7 / (Cbar7 + 25**7)))
    a1p = a1 * (1 + G)
    a2p = a2 * (1 + G)
    C1p = np.sqrt(a1p**2 + b1**2)
    C2p = np.sqrt(a2p**2 + b2**2)
    Cbarp = (C1p + C2p) / 2

    h1p = np.degrees(np.arctan2(b1, a1p)) % 360
    h2p = np.degrees(np.arctan2(b2, a2p)) % 360

    dLp = L2 - L1
    dCp = C2p - C1p

    dhp = np.where(
        C1p * C2p == 0, 0,
        np.where(np.abs(h2p - h1p) <= 180, h2p - h1p,
                 np.where(h2p - h1p > 180, h2p - h1p - 360, h2p - h1p + 360))
    )
    dHp = 2 * np.sqrt(C1p * C2p) * np.sin(np.radians(dhp / 2))

    Hbarp = np.where(
        C1p * C2p == 0, h1p + h2p,
        np.where(np.abs(h1p - h2p) <= 180, (h1p + h2p) / 2,
                 np.where(h1p + h2p < 360, (h1p + h2p + 360) / 2, (h1p + h2p - 360) / 2))
    )

    T = (1 - 0.17 * np.cos(np.radians(Hbarp - 30))
         + 0.24 * np.cos(np.radians(2 * Hbarp))
         + 0.32 * np.cos(np.radians(3 * Hbarp + 6))
         - 0.20 * np.cos(np.radians(4 * Hbarp - 63)))

    SL = 1 + 0.015 * (Lbar - 50)**2 / np.sqrt(20 + (Lbar - 50)**2)
    SC = 1 + 0.045 * Cbarp
    SH = 1 + 0.015 * Cbarp * T
    Cbarp7 = Cbarp**7
    RT = -2 * np.sqrt(Cbarp7 / (Cbarp7 + 25**7)) * np.sin(np.radians(60 * np.exp(-((Hbarp - 275) / 25)**2)))

    dE = np.sqrt(
        (dLp / SL)**2 + (dCp / SC)**2 + (dHp / SH)**2
        + RT * (dCp / SC) * (dHp / SH)
    )
    return dE


def loss_delta_e_2000(frames_a, frames_b):
    """Mean and max ΔE2000 across corresponding frames."""
    results = []
    for i, (fa, fb) in enumerate(zip(frames_a, frames_b)):
        lab_a = rgb_to_lab(fa)
        lab_b = rgb_to_lab(fb)
        de = delta_e_2000(lab_a, lab_b)
        results.append({
            "frame": i,
            "mean_de": float(np.mean(de)),
            "median_de": float(np.median(de)),
            "p95_de": float(np.percentile(de, 95)),
            "max_de": float(np.max(de)),
        })
    return results


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L2: Chroma-plane Wasserstein-1 (Earth Mover's Distance)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def wasserstein_1d(a, b, n_bins=256):
    """Wasserstein-1 distance between 1D distributions via CDF integral."""
    lo = min(a.min(), b.min())
    hi = max(a.max(), b.max())
    if hi == lo:
        return 0.0
    edges = np.linspace(lo, hi, n_bins + 1)
    cdf_a = np.cumsum(np.histogram(a, bins=edges)[0].astype(np.float64))
    cdf_b = np.cumsum(np.histogram(b, bins=edges)[0].astype(np.float64))
    cdf_a /= cdf_a[-1] if cdf_a[-1] > 0 else 1
    cdf_b /= cdf_b[-1] if cdf_b[-1] > 0 else 1
    bin_width = (hi - lo) / n_bins
    return float(np.sum(np.abs(cdf_a - cdf_b)) * bin_width)


def loss_chroma_wasserstein(frames_a, frames_b):
    """Wasserstein-1 on the a* and b* channels independently."""
    results = []
    for i, (fa, fb) in enumerate(zip(frames_a, frames_b)):
        lab_a = rgb_to_lab(fa)
        lab_b = rgb_to_lab(fb)
        w_astar = wasserstein_1d(lab_a[..., 1].ravel(), lab_b[..., 1].ravel())
        w_bstar = wasserstein_1d(lab_a[..., 2].ravel(), lab_b[..., 2].ravel())
        w_L = wasserstein_1d(lab_a[..., 0].ravel(), lab_b[..., 0].ravel())
        results.append({
            "frame": i,
            "w1_a_star": w_astar,
            "w1_b_star": w_bstar,
            "w1_L_star": w_L,
            "w1_chroma": w_astar + w_bstar,
            "chroma_to_luma_ratio": (w_astar + w_bstar) / max(w_L, 1e-10),
        })
    return results


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L3: Color gradient energy (reveals H.264 block artifacts)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def gradient_energy(img_float, channel):
    """||∇C||² for a single channel. Returns scalar energy."""
    c = img_float[..., channel]
    gx = np.diff(c, axis=1)  # horizontal gradient
    gy = np.diff(c, axis=0)  # vertical gradient
    # Pad to same size
    ex = np.sum(gx ** 2)
    ey = np.sum(gy ** 2)
    return float(ex + ey)


def loss_gradient_energy(frames_a, frames_b):
    """Ratio of gradient energies per channel (Lab). >1 means A has more texture."""
    results = []
    for i, (fa, fb) in enumerate(zip(frames_a, frames_b)):
        lab_a = rgb_to_lab(fa)
        lab_b = rgb_to_lab(fb)
        channels = {}
        for ch, name in [(0, "L"), (1, "a"), (2, "b")]:
            ea = gradient_energy(lab_a, ch)
            eb = gradient_energy(lab_b, ch)
            channels[f"grad_energy_{name}_A"] = ea
            channels[f"grad_energy_{name}_B"] = eb
            channels[f"grad_ratio_{name}"] = ea / max(eb, 1e-10)
        results.append({"frame": i, **channels})
    return results


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L4: Chroma subsampling fidelity (what 4:2:0 destroyed)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def simulate_420_subsample(img_uint8):
    """Simulate YCbCr 4:2:0: downsample chroma 2x, then upsample back."""
    ycbcr = rgb_to_ycbcr(img_uint8)
    h, w = ycbcr.shape[:2]
    # 2x2 block average for Cb, Cr
    h2, w2 = h // 2 * 2, w // 2 * 2
    cb = ycbcr[:h2, :w2, 1].reshape(h2 // 2, 2, w2 // 2, 2).mean(axis=(1, 3))
    cr = ycbcr[:h2, :w2, 2].reshape(h2 // 2, 2, w2 // 2, 2).mean(axis=(1, 3))
    # Upsample via nearest-neighbor (what decoder does before filtering)
    cb_up = np.repeat(np.repeat(cb, 2, axis=0), 2, axis=1)
    cr_up = np.repeat(np.repeat(cr, 2, axis=0), 2, axis=1)
    return ycbcr[:h2, :w2, :], np.stack([ycbcr[:h2, :w2, 0], cb_up, cr_up], axis=-1)


def loss_chroma_subsample_fidelity(frames_a, frames_b):
    """How much 4:2:0 destroys in each video. Measures what's irrecoverable."""
    results = []
    for i, (fa, fb) in enumerate(zip(frames_a, frames_b)):
        orig_a, sub_a = simulate_420_subsample(fa)
        orig_b, sub_b = simulate_420_subsample(fb)
        # Chroma RMSE from subsampling
        rmse_cb_a = float(np.sqrt(np.mean((orig_a[..., 1] - sub_a[..., 1]) ** 2)))
        rmse_cb_b = float(np.sqrt(np.mean((orig_b[..., 1] - sub_b[..., 1]) ** 2)))
        rmse_cr_a = float(np.sqrt(np.mean((orig_a[..., 2] - sub_a[..., 2]) ** 2)))
        rmse_cr_b = float(np.sqrt(np.mean((orig_b[..., 2] - sub_b[..., 2]) ** 2)))
        # Differential: does subsampling hurt both equally?
        results.append({
            "frame": i,
            "rmse_Cb_A": rmse_cb_a, "rmse_Cb_B": rmse_cb_b,
            "rmse_Cr_A": rmse_cr_a, "rmse_Cr_B": rmse_cr_b,
            "subsample_asymmetry_Cb": abs(rmse_cb_a - rmse_cb_b),
            "subsample_asymmetry_Cr": abs(rmse_cr_a - rmse_cr_b),
        })
    return results


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L5: Color path length in Lab space (trajectory arc length)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def mean_color(frame_uint8):
    """Mean color of frame in Lab space."""
    lab = rgb_to_lab(frame_uint8)
    return np.array([lab[..., 0].mean(), lab[..., 1].mean(), lab[..., 2].mean()])


def loss_path_length(frames_a, frames_b):
    """Arc length of mean-color trajectory in Lab space."""
    traj_a = np.array([mean_color(f) for f in frames_a])
    traj_b = np.array([mean_color(f) for f in frames_b])

    # Arc length = sum of segment lengths
    segs_a = np.sqrt(np.sum(np.diff(traj_a, axis=0) ** 2, axis=1))
    segs_b = np.sqrt(np.sum(np.diff(traj_b, axis=0) ** 2, axis=1))
    path_len_a = float(np.sum(segs_a))
    path_len_b = float(np.sum(segs_b))

    # Frechet-like distance (discrete): max over all corresponding points
    frechet = float(np.max(np.sqrt(np.sum((traj_a - traj_b) ** 2, axis=1))))

    return {
        "path_length_A": path_len_a,
        "path_length_B": path_len_b,
        "path_ratio": path_len_a / max(path_len_b, 1e-10),
        "discrete_frechet": frechet,
        "trajectory_A": [{"L": float(p[0]), "a": float(p[1]), "b": float(p[2])} for p in traj_a],
        "trajectory_B": [{"L": float(p[0]), "a": float(p[1]), "b": float(p[2])} for p in traj_b],
    }


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L6: Color flow Jacobian (diffeomorphic distinguishability)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def color_flow_jacobian(traj):
    """Approximate Jacobian of color flow: J[i] ≈ d(color_{i+1})/d(color_i).
    
    For a 1D trajectory in 3D Lab space, J is 3x3 from finite differences.
    A true diffeomorphism has det(J) > 0 everywhere.
    """
    n = len(traj)
    jacobians = []
    for i in range(1, n - 1):
        # Central difference: how does Lab at i+1 change w.r.t. Lab at i-1
        d_in = traj[i] - traj[i - 1]
        d_out = traj[i + 1] - traj[i]
        # Outer product approximation to Jacobian
        norm = np.dot(d_in, d_in)
        if norm < 1e-12:
            jacobians.append({"frame": i, "det": 0.0, "trace": 0.0, "diffeomorphic": False})
            continue
        J = np.outer(d_out, d_in) / norm
        det = float(np.linalg.det(J))
        trace = float(np.trace(J))
        jacobians.append({
            "frame": i,
            "det": det,
            "trace": trace,
            "diffeomorphic": det > 0,
        })
    return jacobians


def loss_jacobian_flow(frames_a, frames_b):
    """Compare Jacobian structure of color trajectories."""
    traj_a = np.array([mean_color(f) for f in frames_a])
    traj_b = np.array([mean_color(f) for f in frames_b])
    jac_a = color_flow_jacobian(traj_a)
    jac_b = color_flow_jacobian(traj_b)
    # Count diffeomorphic steps
    diffeo_a = sum(1 for j in jac_a if j["diffeomorphic"])
    diffeo_b = sum(1 for j in jac_b if j["diffeomorphic"])
    return {
        "jacobians_A": jac_a,
        "jacobians_B": jac_b,
        "diffeomorphic_steps_A": diffeo_a,
        "diffeomorphic_steps_B": diffeo_b,
        "total_steps": len(jac_a),
        "det_product_A": float(np.prod([j["det"] for j in jac_a if j["det"] != 0]) if jac_a else 0),
        "det_product_B": float(np.prod([j["det"] for j in jac_b if j["det"] != 0]) if jac_b else 0),
    }


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L7: Topological persistence proxy
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def color_persistence(frame_uint8, epsilons=None):
    """Count connected components in Lab color space at multiple thresholds.
    
    Sample pixels, build adjacency at threshold ε, count components.
    This is a discrete approximation to 0-dim persistent homology.
    """
    if epsilons is None:
        epsilons = [2, 5, 10, 20, 40, 80]

    lab = rgb_to_lab(frame_uint8)
    h, w = lab.shape[:2]
    # Sample 500 random pixels for tractability
    rng = np.random.RandomState(42)
    idx = rng.choice(h * w, min(500, h * w), replace=False)
    pts = lab.reshape(-1, 3)[idx]

    results = []
    for eps in epsilons:
        # Build adjacency via pairwise distance
        dists = np.sqrt(np.sum((pts[:, None, :] - pts[None, :, :]) ** 2, axis=-1))
        adj = dists < eps
        # BFS to count components
        visited = np.zeros(len(pts), dtype=bool)
        n_components = 0
        for start in range(len(pts)):
            if visited[start]:
                continue
            n_components += 1
            stack = [start]
            while stack:
                node = stack.pop()
                if visited[node]:
                    continue
                visited[node] = True
                neighbors = np.where(adj[node] & ~visited)[0]
                stack.extend(neighbors.tolist())
        results.append({"epsilon": eps, "components": n_components})
    return results


def loss_persistence(frames_a, frames_b):
    """Compare topological persistence signatures."""
    # Use middle frame for each video
    mid = len(frames_a) // 2
    pers_a = color_persistence(frames_a[mid])
    pers_b = color_persistence(frames_b[mid])
    # Birth-death difference: how fast do components merge?
    diff = []
    for pa, pb in zip(pers_a, pers_b):
        diff.append({
            "epsilon": pa["epsilon"],
            "components_A": pa["components"],
            "components_B": pb["components"],
            "delta": abs(pa["components"] - pb["components"]),
        })
    return diff


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# L8: Mutual information of color channels
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def mi_2d(x, y, n_bins=64):
    """Mutual information I(X;Y) via histogram estimator."""
    hist_xy, _, _ = np.histogram2d(x.ravel(), y.ravel(), bins=n_bins)
    pxy = hist_xy / hist_xy.sum()
    px = pxy.sum(axis=1)
    py = pxy.sum(axis=0)
    # I(X;Y) = sum p(x,y) log(p(x,y) / (p(x)p(y)))
    mask = pxy > 0
    mi = np.sum(pxy[mask] * np.log2(pxy[mask] / (px[:, None] * py[None, :])[mask]))
    return float(mi)


def loss_channel_mi(frames_a, frames_b):
    """MI between color channels reveals what H.264 decorrelation preserved."""
    results = []
    for i, (fa, fb) in enumerate(zip(frames_a, frames_b)):
        ycbcr_a = rgb_to_ycbcr(fa)
        ycbcr_b = rgb_to_ycbcr(fb)
        mi_ycb_a = mi_2d(ycbcr_a[..., 0], ycbcr_a[..., 1])
        mi_ycb_b = mi_2d(ycbcr_b[..., 0], ycbcr_b[..., 1])
        mi_ycr_a = mi_2d(ycbcr_a[..., 0], ycbcr_a[..., 2])
        mi_ycr_b = mi_2d(ycbcr_b[..., 0], ycbcr_b[..., 2])
        mi_cbcr_a = mi_2d(ycbcr_a[..., 1], ycbcr_a[..., 2])
        mi_cbcr_b = mi_2d(ycbcr_b[..., 1], ycbcr_b[..., 2])
        results.append({
            "frame": i,
            "MI_Y_Cb_A": mi_ycb_a, "MI_Y_Cb_B": mi_ycb_b,
            "MI_Y_Cr_A": mi_ycr_a, "MI_Y_Cr_B": mi_ycr_b,
            "MI_Cb_Cr_A": mi_cbcr_a, "MI_Cb_Cr_B": mi_cbcr_b,
            "total_MI_A": mi_ycb_a + mi_ycr_a + mi_cbcr_a,
            "total_MI_B": mi_ycb_b + mi_ycr_b + mi_cbcr_b,
        })
    return results


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# R-LOSSES: Reservoir-intractable loss signatures (framing)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def reservoir_loss_manifesto():
    """What an ensemble of reservoir computers could measure that we can't.
    
    The fundamental gap: color trajectories are continuous dynamical systems.
    Digital computers discretize them. Reservoir computers embody them.
    """
    return {
        "R1_generalized_sync": {
            "description": "Drive two reservoir computers with color streams A and B. "
                "If their internal states synchronize (auxiliary system approach), "
                "the videos are dynamically equivalent. Synchronization is a "
                "continuous-time phenomenon — digital simulation is O(2^n) but "
                "physical reservoir time is O(1).",
            "ensemble_role": "Different spectral radii sample different timescales. "
                "Ensemble of N reservoirs → N-point sample of the synchronization manifold.",
            "why_intractable": "Generalized sync requires checking ALL possible "
                "smooth maps h: X→Y such that h(x_t) = y_t. Digital: exponential "
                "in state dimension. Reservoir: implicit in physics.",
        },
        "R2_ipc_decomposition": {
            "description": "Information Processing Capacity decomposes reservoir "
                "computation into orthogonal Legendre polynomial basis: IPC_total = "
                "IPC_linear + IPC_quadratic + IPC_cubic + ... Each order captures "
                "different nonlinear mixing of the color signal.",
            "ensemble_role": "Ensemble of reservoirs with different coupling topologies "
                "(ring, random, small-world) provides different IPC profiles. The "
                "CONSENSUS across topologies is the invariant color property.",
            "why_intractable": "Complete IPC decomposition requires O(d^k) terms for "
                "order k in d dimensions. For d=3 (Lab) and k=10, digital: 59049 terms. "
                "Reservoir: all orders computed simultaneously in physical dynamics.",
        },
        "R3_echo_state_distance": {
            "description": "Drive identical reservoir with streams A and B. "
                "Measure ||state_A(t) - state_B(t)|| over time. The echo state "
                "property guarantees convergence for same input — so divergence "
                "measures genuine dynamical difference.",
            "ensemble_role": "Different reservoir sizes (10, 100, 1000 nodes) "
                "sample different memory depths. Small: short-term color difference. "
                "Large: long-range color correlation difference.",
            "why_intractable": "The echo state metric lives in the reservoir's "
                "high-dimensional state space (N nodes → R^N). Digital: explicit "
                "N×N matrix exponential at each step. Physical: free.",
        },
        "R4_lyapunov_spectrum": {
            "description": "Full Lyapunov spectrum of the color dynamical system "
                "from video frames. λ_1 > 0 = chaotic, all λ_i < 0 = contracting. "
                "The spectrum is the most complete invariant of a dynamical system.",
            "ensemble_role": "Each reservoir in the ensemble estimates one Lyapunov "
                "exponent via different time-delay embeddings. Ensemble → full spectrum.",
            "why_intractable": "Digital Lyapunov computation requires QR decomposition "
                "along the trajectory, O(d³ T) for d-dim system over T steps. "
                "But the TRUE spectrum is defined for continuous flow, not discrete map. "
                "Reservoir computers compute the continuous spectrum natively.",
        },
        "R5_fading_memory_kernel": {
            "description": "The fading memory of a reservoir defines a kernel "
                "K(t, s) = E[x(t)x(s)] that captures temporal color correlations. "
                "Two videos with different K(t,s) have different color dynamics "
                "even if their static distributions match.",
            "ensemble_role": "Reservoirs with different leak rates sample different "
                "exponential decay timescales. The ensemble reconstructs the full "
                "kernel by Laplace inversion of the leak-rate sweep.",
            "why_intractable": "Kernel estimation from finite samples is ill-posed "
                "(inverse problem). Reservoir: the kernel IS the physics. "
                "Multiple reservoirs = multiple kernel sections = stable reconstruction.",
        },
        "connection_to_boxxy_tile": {
            "description": "The SplitMix64 → Color → Syrup → witness chain in boxxy "
                "IS a deterministic dynamical system. The tile lattice with GF(3) "
                "trit conservation is a discrete analog of a conservation law. "
                "Reservoir computers respect conservation laws natively via "
                "Hamiltonian echo state networks (HESN). An ensemble of HESNs "
                "with different conserved quantities would measure the FULL "
                "invariant structure of the color flow — something no finite "
                "digital computation can achieve.",
        },
    }


# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Main: run all losses
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

def main():
    print(f"Extracting {N_FRAMES} frames from each video...")
    frames_a = extract_frames(VIDEO_A, N_FRAMES)
    frames_b = extract_frames(VIDEO_B, N_FRAMES)
    print(f"  A: {len(frames_a)} frames, {frames_a[0].shape}")
    print(f"  B: {len(frames_b)} frames, {frames_b[0].shape}")

    results = {}

    print("\nL1: CIE ΔE2000 (perceptual color difference)...")
    results["L1_delta_e_2000"] = loss_delta_e_2000(frames_a, frames_b)
    de_means = [r["mean_de"] for r in results["L1_delta_e_2000"]]
    print(f"  mean ΔE per frame: {[f'{x:.2f}' for x in de_means]}")
    print(f"  overall mean ΔE: {np.mean(de_means):.2f}")

    print("\nL2: Chroma Wasserstein-1 (EMD on a*,b* channels)...")
    results["L2_chroma_wasserstein"] = loss_chroma_wasserstein(frames_a, frames_b)
    w_chromas = [r["w1_chroma"] for r in results["L2_chroma_wasserstein"]]
    ratios = [r["chroma_to_luma_ratio"] for r in results["L2_chroma_wasserstein"]]
    print(f"  chroma W1 per frame: {[f'{x:.2f}' for x in w_chromas]}")
    print(f"  chroma/luma ratio: {[f'{x:.2f}' for x in ratios]}")

    print("\nL3: Color gradient energy (block artifact sensitivity)...")
    results["L3_gradient_energy"] = loss_gradient_energy(frames_a, frames_b)
    for ch in ["L", "a", "b"]:
        rats = [r[f"grad_ratio_{ch}"] for r in results["L3_gradient_energy"]]
        print(f"  {ch}-channel gradient ratio A/B: {[f'{x:.3f}' for x in rats]}")

    print("\nL4: Chroma subsampling fidelity (4:2:0 damage)...")
    results["L4_chroma_subsample"] = loss_chroma_subsample_fidelity(frames_a, frames_b)
    asym_cb = [r["subsample_asymmetry_Cb"] for r in results["L4_chroma_subsample"]]
    print(f"  Cb asymmetry: {[f'{x:.3f}' for x in asym_cb]}")

    print("\nL5: Color path length (Lab trajectory arc length)...")
    results["L5_path_length"] = loss_path_length(frames_a, frames_b)
    pl = results["L5_path_length"]
    print(f"  path A: {pl['path_length_A']:.2f}, path B: {pl['path_length_B']:.2f}")
    print(f"  ratio: {pl['path_ratio']:.3f}, Fréchet: {pl['discrete_frechet']:.2f}")

    print("\nL6: Color flow Jacobian (diffeomorphic structure)...")
    results["L6_jacobian_flow"] = loss_jacobian_flow(frames_a, frames_b)
    jf = results["L6_jacobian_flow"]
    print(f"  diffeomorphic steps: A={jf['diffeomorphic_steps_A']}/{jf['total_steps']}, "
          f"B={jf['diffeomorphic_steps_B']}/{jf['total_steps']}")
    print(f"  det product: A={jf['det_product_A']:.6f}, B={jf['det_product_B']:.6f}")

    print("\nL7: Topological persistence (color space components)...")
    results["L7_persistence"] = loss_persistence(frames_a, frames_b)
    for p in results["L7_persistence"]:
        print(f"  ε={p['epsilon']}: A={p['components_A']} B={p['components_B']} Δ={p['delta']}")

    print("\nL8: Channel mutual information (H.264 decorrelation)...")
    results["L8_channel_mi"] = loss_channel_mi(frames_a, frames_b)
    mi_totals_a = [r["total_MI_A"] for r in results["L8_channel_mi"]]
    mi_totals_b = [r["total_MI_B"] for r in results["L8_channel_mi"]]
    print(f"  total MI A: {[f'{x:.3f}' for x in mi_totals_a]}")
    print(f"  total MI B: {[f'{x:.3f}' for x in mi_totals_b]}")

    print("\n━━ RESERVOIR LOSS MANIFESTO ━━")
    results["reservoir_losses"] = reservoir_loss_manifesto()
    for key, val in results["reservoir_losses"].items():
        if isinstance(val, dict) and "description" in val:
            print(f"\n  {key}:")
            print(f"    {val['description'][:120]}...")
            if "why_intractable" in val:
                print(f"    WHY: {val['why_intractable'][:100]}...")

    # Write results
    with open(RESULTS_OUT, "w") as f:
        json.dump(results, f, indent=2, default=str)
    print(f"\n\nFull results: {RESULTS_OUT}")


if __name__ == "__main__":
    main()
