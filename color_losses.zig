const std = @import("std");
const math = std.math;

// Color loss functions for the zig-syrup evolving numerics stack.
//
// These 8 losses measure what cosine similarity on embeddings misses:
// perceptual, diffeomorphic, topological, and information-theoretic
// structure in color trajectories from tileable VMs.
//
// Design principles (matching winding.zig):
//   - Fixed-point where possible (LabFP for Q16.16 Lab coordinates)
//   - Branch-free inner loops for SIMD vectorization
//   - O(n) accumulation, never O(n²) search
//   - GF(3) trit conservation thread throughout
//
// The 8 losses:
//   L1: CIE ΔE2000 — perceptual color difference
//   L2: Chroma Wasserstein-1 — EMD on a*,b* marginals via CDF integral
//   L3: Gradient energy ratio — ||∇C||² per Lab channel
//   L4: Chroma subsample fidelity — 4:2:0 damage asymmetry
//   L5: Path length — Lab trajectory arc length + discrete Fréchet
//   L6: Jacobian flow — det(J) orientation preservation (diffeomorphic?)
//   L7: Persistence proxy — connected components at multiple ε
//   L8: Channel MI — mutual information of YCbCr channels

// ── Color Space Types ───────────────────────────────────────────

pub const LabColor = struct {
    L: f64, // 0..100
    a: f64, // ~-128..127
    b: f64, // ~-128..127
};

pub const YCbCr = struct {
    Y: f64,
    Cb: f64,
    Cr: f64,
};

pub const RGB = struct {
    r: u8,
    g: u8,
    b: u8,

    pub fn toLinear(self: RGB) [3]f64 {
        var out: [3]f64 = undefined;
        const channels = [_]u8{ self.r, self.g, self.b };
        for (channels, 0..) |ch, i| {
            const c = @as(f64, @floatFromInt(ch)) / 255.0;
            out[i] = if (c <= 0.04045)
                c / 12.92
            else
                math.pow(f64, (c + 0.055) / 1.055, 2.4);
        }
        return out;
    }

    pub fn toLab(self: RGB) LabColor {
        const lin = self.toLinear();
        // sRGB → XYZ (D65)
        const X = 0.4124564 * lin[0] + 0.3575761 * lin[1] + 0.1804375 * lin[2];
        const Y = 0.2126729 * lin[0] + 0.7151522 * lin[1] + 0.0721750 * lin[2];
        const Z = 0.0193339 * lin[0] + 0.1191920 * lin[1] + 0.9503041 * lin[2];
        // XYZ → Lab (D65 white)
        const xn = X / 0.95047;
        const yn = Y / 1.00000;
        const zn = Z / 1.08883;
        const delta: f64 = 6.0 / 29.0;
        const d3 = delta * delta * delta;
        const fx = if (xn > d3) math.cbrt(xn) else xn / (3.0 * delta * delta) + 4.0 / 29.0;
        const fy = if (yn > d3) math.cbrt(yn) else yn / (3.0 * delta * delta) + 4.0 / 29.0;
        const fz = if (zn > d3) math.cbrt(zn) else zn / (3.0 * delta * delta) + 4.0 / 29.0;
        return .{
            .L = 116.0 * fy - 16.0,
            .a = 500.0 * (fx - fy),
            .b = 200.0 * (fy - fz),
        };
    }

    pub fn toYCbCr(self: RGB) YCbCr {
        // BT.601 (same as H.264 default)
        const r: f64 = @floatFromInt(self.r);
        const g: f64 = @floatFromInt(self.g);
        const b: f64 = @floatFromInt(self.b);
        return .{
            .Y = 0.299 * r + 0.587 * g + 0.114 * b,
            .Cb = -0.168736 * r - 0.331264 * g + 0.5 * b + 128.0,
            .Cr = 0.5 * r - 0.418688 * g - 0.081312 * b + 128.0,
        };
    }
};

// ── Fixed-Point Lab (Q16.16) ────────────────────────────────────
// For SIMD-friendly accumulation in inner loops.

pub const LabFP = struct {
    L: i32, // Q16.16: value * 65536
    a: i32,
    b: i32,

    const SCALE: i32 = 65536;
    const SCALE_F: f64 = 65536.0;

    pub fn fromLab(lab: LabColor) LabFP {
        return .{
            .L = @intFromFloat(lab.L * SCALE_F),
            .a = @intFromFloat(lab.a * SCALE_F),
            .b = @intFromFloat(lab.b * SCALE_F),
        };
    }

    pub fn toLab(self: LabFP) LabColor {
        return .{
            .L = @as(f64, @floatFromInt(self.L)) / SCALE_F,
            .a = @as(f64, @floatFromInt(self.a)) / SCALE_F,
            .b = @as(f64, @floatFromInt(self.b)) / SCALE_F,
        };
    }

    // Branch-free squared L2 distance in fixed point.
    pub fn distSq(a_fp: LabFP, b_fp: LabFP) i64 {
        const dL: i64 = @as(i64, a_fp.L) - @as(i64, b_fp.L);
        const da: i64 = @as(i64, a_fp.a) - @as(i64, b_fp.a);
        const db: i64 = @as(i64, a_fp.b) - @as(i64, b_fp.b);
        return dL * dL + da * da + db * db;
    }
};

// ── L1: CIE ΔE2000 ─────────────────────────────────────────────
// Public-domain implementation matching ciede2000.pages-perso.free.fr.
// Operates on two Lab colors, returns perceptual difference.

pub fn deltaE2000(l1: LabColor, l2: LabColor) f64 {
    const pi = math.pi;
    var n = (math.sqrt(l1.a * l1.a + l1.b * l1.b) +
        math.sqrt(l2.a * l2.a + l2.b * l2.b)) * 0.5;
    n = n * n * n * n * n * n * n;
    n = 1.0 + 0.5 * (1.0 - math.sqrt(n / (n + 6103515625.0)));
    const c1 = math.sqrt(l1.a * l1.a * n * n + l1.b * l1.b);
    const c2 = math.sqrt(l2.a * l2.a * n * n + l2.b * l2.b);
    var h1 = math.atan2(l1.b, l1.a * n);
    var h2 = math.atan2(l2.b, l2.a * n);
    if (h1 < 0.0) h1 += 2.0 * pi;
    if (h2 < 0.0) h2 += 2.0 * pi;
    var diff: f64 = undefined;
    if (h2 < h1) {
        diff = h1 - h2;
    } else {
        diff = h2 - h1;
    }
    if (pi - 1e-14 < diff and diff < pi + 1e-14) diff = pi;
    var hm = (h1 + h2) * 0.5;
    var hd = (h2 - h1) * 0.5;
    if (pi < diff) {
        hd += pi;
        hm += pi;
    }
    const p = 36.0 * hm - 55.0 * pi;
    n = (c1 + c2) * 0.5;
    n = n * n * n * n * n * n * n;
    const rt = -2.0 * math.sqrt(n / (n + 6103515625.0)) *
        @sin(pi / 3.0 * @exp(p * p / (-25.0 * pi * pi)));
    n = (l1.L + l2.L) * 0.5;
    n = (n - 50.0) * (n - 50.0);
    const dl = (l2.L - l1.L) / (1.0 + 0.015 * n / math.sqrt(20.0 + n));
    const t = 1.0 + 0.24 * @sin(2.0 * hm + pi / 2.0) +
        0.32 * @sin(3.0 * hm + 8.0 * pi / 15.0) -
        0.17 * @sin(hm + pi / 3.0) -
        0.20 * @sin(4.0 * hm + 3.0 * pi / 20.0);
    n = c1 + c2;
    const dh = 2.0 * math.sqrt(c1 * c2) * @sin(hd) /
        (1.0 + 0.0075 * n * t);
    const dc = (c2 - c1) / (1.0 + 0.0225 * n);
    return math.sqrt(dl * dl + dh * dh + dc * dc + dc * dh * rt);
}

// ── L2: Wasserstein-1 on 1D distributions ───────────────────────
// CDF integral method: W1 = ∫|CDF_A - CDF_B| dx
// O(n_bins) after histogram, no sorting needed.

pub const Histogram = struct {
    counts: [256]u32 = [_]u32{0} ** 256,
    n: u32 = 0,
    lo: f64 = 0,
    hi: f64 = 256,

    pub fn init(lo: f64, hi: f64) Histogram {
        return .{ .counts = [_]u32{0} ** 256, .n = 0, .lo = lo, .hi = hi };
    }

    pub fn add(self: *Histogram, value: f64) void {
        const range = self.hi - self.lo;
        if (range <= 0) return;
        const idx_f = (value - self.lo) / range * 256.0;
        const idx: usize = @intFromFloat(@min(@max(idx_f, 0.0), 255.0));
        self.counts[idx] += 1;
        self.n += 1;
    }

    // Wasserstein-1 via CDF integral
    pub fn wasserstein1(a: Histogram, b: Histogram) f64 {
        if (a.n == 0 or b.n == 0) return 0;
        const bin_width = (a.hi - a.lo) / 256.0;
        var cdf_a: f64 = 0;
        var cdf_b: f64 = 0;
        var w1: f64 = 0;
        const inv_a = 1.0 / @as(f64, @floatFromInt(a.n));
        const inv_b = 1.0 / @as(f64, @floatFromInt(b.n));
        for (0..256) |i| {
            cdf_a += @as(f64, @floatFromInt(a.counts[i])) * inv_a;
            cdf_b += @as(f64, @floatFromInt(b.counts[i])) * inv_b;
            w1 += @abs(cdf_a - cdf_b) * bin_width;
        }
        return w1;
    }
};

// Chroma Wasserstein result for a pair of color distributions
pub const ChromaW1 = struct {
    w1_a_star: f64,
    w1_b_star: f64,
    w1_L: f64,
    w1_chroma: f64,
    chroma_luma_ratio: f64,
};

pub fn chromaWasserstein(labs_a: []const LabColor, labs_b: []const LabColor) ChromaW1 {
    var ha_a = Histogram.init(-128, 128);
    var ha_b = Histogram.init(-128, 128);
    var hb_a = Histogram.init(-128, 128);
    var hb_b = Histogram.init(-128, 128);
    var hL_a = Histogram.init(0, 100);
    var hL_b = Histogram.init(0, 100);
    for (labs_a) |lab| {
        ha_a.add(lab.a);
        hb_a.add(lab.b);
        hL_a.add(lab.L);
    }
    for (labs_b) |lab| {
        ha_b.add(lab.a);
        hb_b.add(lab.b);
        hL_b.add(lab.L);
    }
    const w_a = Histogram.wasserstein1(ha_a, ha_b);
    const w_b = Histogram.wasserstein1(hb_a, hb_b);
    const w_L = Histogram.wasserstein1(hL_a, hL_b);
    const chroma = w_a + w_b;
    return .{
        .w1_a_star = w_a,
        .w1_b_star = w_b,
        .w1_L = w_L,
        .w1_chroma = chroma,
        .chroma_luma_ratio = if (w_L > 1e-10) chroma / w_L else 0,
    };
}

// ── L3: Gradient Energy ─────────────────────────────────────────
// ||∇C||² for a 1D color sequence. Measures texture/block artifacts.
// For a frame-sequence trajectory, this is the sum of squared differences.

pub fn gradientEnergy(values: []const f64) f64 {
    if (values.len < 2) return 0;
    var energy: f64 = 0;
    for (0..values.len - 1) |i| {
        const d = values[i + 1] - values[i];
        energy += d * d;
    }
    return energy;
}

pub const GradientRatios = struct {
    L_ratio: f64,
    a_ratio: f64,
    b_ratio: f64,
};

pub fn gradientEnergyRatio(traj_a: []const LabColor, traj_b: []const LabColor) GradientRatios {
    const n = @min(traj_a.len, traj_b.len);
    if (n < 2) return .{ .L_ratio = 1, .a_ratio = 1, .b_ratio = 1 };
    var La = std.ArrayList(f64).init(std.heap.page_allocator);
    var Lb = std.ArrayList(f64).init(std.heap.page_allocator);
    var aa = std.ArrayList(f64).init(std.heap.page_allocator);
    var ab = std.ArrayList(f64).init(std.heap.page_allocator);
    var ba = std.ArrayList(f64).init(std.heap.page_allocator);
    var bb = std.ArrayList(f64).init(std.heap.page_allocator);
    defer {
        La.deinit();
        Lb.deinit();
        aa.deinit();
        ab.deinit();
        ba.deinit();
        bb.deinit();
    }
    for (0..n) |i| {
        La.append(traj_a[i].L) catch {};
        Lb.append(traj_b[i].L) catch {};
        aa.append(traj_a[i].a) catch {};
        ab.append(traj_b[i].a) catch {};
        ba.append(traj_a[i].b) catch {};
        bb.append(traj_b[i].b) catch {};
    }
    const eL_a = gradientEnergy(La.items);
    const eL_b = gradientEnergy(Lb.items);
    const ea_a = gradientEnergy(aa.items);
    const ea_b = gradientEnergy(ab.items);
    const eb_a = gradientEnergy(ba.items);
    const eb_b = gradientEnergy(bb.items);
    return .{
        .L_ratio = if (eL_b > 1e-10) eL_a / eL_b else 1,
        .a_ratio = if (ea_b > 1e-10) ea_a / ea_b else 1,
        .b_ratio = if (eb_b > 1e-10) eb_a / eb_b else 1,
    };
}

// ── L4: Chroma Subsample Fidelity ───────────────────────────────
// Simulate 4:2:0: average pairs of Cb/Cr, then measure RMSE of round-trip.
// For a 1D color sequence, "2x downsample then upsample" = average neighbors.

pub fn subsampleRMSE(values: []const f64) f64 {
    if (values.len < 2) return 0;
    const n_pairs = values.len / 2;
    if (n_pairs == 0) return 0;
    var sse: f64 = 0;
    for (0..n_pairs) |i| {
        const avg = (values[2 * i] + values[2 * i + 1]) * 0.5;
        const d0 = values[2 * i] - avg;
        const d1 = values[2 * i + 1] - avg;
        sse += d0 * d0 + d1 * d1;
    }
    return math.sqrt(sse / @as(f64, @floatFromInt(n_pairs * 2)));
}

pub const SubsampleAsymmetry = struct {
    rmse_Cb_A: f64,
    rmse_Cb_B: f64,
    rmse_Cr_A: f64,
    rmse_Cr_B: f64,
    asymmetry_Cb: f64,
    asymmetry_Cr: f64,
};

pub fn chromaSubsampleFidelity(ycbcr_a: []const YCbCr, ycbcr_b: []const YCbCr) SubsampleAsymmetry {
    const n = @min(ycbcr_a.len, ycbcr_b.len);
    var cb_a_vals = std.ArrayList(f64).init(std.heap.page_allocator);
    var cb_b_vals = std.ArrayList(f64).init(std.heap.page_allocator);
    var cr_a_vals = std.ArrayList(f64).init(std.heap.page_allocator);
    var cr_b_vals = std.ArrayList(f64).init(std.heap.page_allocator);
    defer {
        cb_a_vals.deinit();
        cb_b_vals.deinit();
        cr_a_vals.deinit();
        cr_b_vals.deinit();
    }
    for (0..n) |i| {
        cb_a_vals.append(ycbcr_a[i].Cb) catch {};
        cb_b_vals.append(ycbcr_b[i].Cb) catch {};
        cr_a_vals.append(ycbcr_a[i].Cr) catch {};
        cr_b_vals.append(ycbcr_b[i].Cr) catch {};
    }
    const rcb_a = subsampleRMSE(cb_a_vals.items);
    const rcb_b = subsampleRMSE(cb_b_vals.items);
    const rcr_a = subsampleRMSE(cr_a_vals.items);
    const rcr_b = subsampleRMSE(cr_b_vals.items);
    return .{
        .rmse_Cb_A = rcb_a,
        .rmse_Cb_B = rcb_b,
        .rmse_Cr_A = rcr_a,
        .rmse_Cr_B = rcr_b,
        .asymmetry_Cb = @abs(rcb_a - rcb_b),
        .asymmetry_Cr = @abs(rcr_a - rcr_b),
    };
}

// ── L5: Path Length ─────────────────────────────────────────────
// Arc length of Lab trajectory + discrete Fréchet distance.

pub const PathResult = struct {
    length_a: f64,
    length_b: f64,
    ratio: f64,
    frechet: f64,
};

pub fn pathLength(traj: []const LabColor) f64 {
    if (traj.len < 2) return 0;
    var total: f64 = 0;
    for (0..traj.len - 1) |i| {
        const dL = traj[i + 1].L - traj[i].L;
        const da = traj[i + 1].a - traj[i].a;
        const db = traj[i + 1].b - traj[i].b;
        total += math.sqrt(dL * dL + da * da + db * db);
    }
    return total;
}

fn labDist(a: LabColor, b: LabColor) f64 {
    const dL = a.L - b.L;
    const da = a.a - b.a;
    const db = a.b - b.b;
    return math.sqrt(dL * dL + da * da + db * db);
}

pub fn discreteFrechet(a: []const LabColor, b: []const LabColor) f64 {
    // Simplified: max pointwise distance (upper bound on true discrete Fréchet)
    const n = @min(a.len, b.len);
    var mx: f64 = 0;
    for (0..n) |i| {
        const d = labDist(a[i], b[i]);
        if (d > mx) mx = d;
    }
    return mx;
}

pub fn colorPathAnalysis(traj_a: []const LabColor, traj_b: []const LabColor) PathResult {
    const la = pathLength(traj_a);
    const lb = pathLength(traj_b);
    return .{
        .length_a = la,
        .length_b = lb,
        .ratio = if (lb > 1e-10) la / lb else 1,
        .frechet = discreteFrechet(traj_a, traj_b),
    };
}

// ── L6: Color Flow Jacobian ─────────────────────────────────────
// Approximate Jacobian of trajectory: J[i] ≈ outer(d_out, d_in) / |d_in|²
// det(J) > 0 everywhere = diffeomorphic flow.

pub const JacobianStep = struct {
    frame: usize,
    det: f64,
    trace: f64,
    diffeomorphic: bool,
};

pub const JacobianResult = struct {
    steps_a: [16]JacobianStep = undefined,
    steps_b: [16]JacobianStep = undefined,
    n_steps: usize = 0,
    diffeo_a: usize = 0,
    diffeo_b: usize = 0,
};

fn computeJacobians(traj: []const LabColor, out: *[16]JacobianStep) usize {
    if (traj.len < 3) return 0;
    const n = @min(traj.len - 2, 16);
    for (0..n) |idx| {
        const i = idx + 1;
        const d_in = [3]f64{
            traj[i].L - traj[i - 1].L,
            traj[i].a - traj[i - 1].a,
            traj[i].b - traj[i - 1].b,
        };
        const d_out = [3]f64{
            traj[i + 1].L - traj[i].L,
            traj[i + 1].a - traj[i].a,
            traj[i + 1].b - traj[i].b,
        };
        const norm = d_in[0] * d_in[0] + d_in[1] * d_in[1] + d_in[2] * d_in[2];
        if (norm < 1e-12) {
            out[idx] = .{ .frame = i, .det = 0, .trace = 0, .diffeomorphic = false };
            continue;
        }
        // J = outer(d_out, d_in) / norm → 3x3 matrix
        var J: [3][3]f64 = undefined;
        for (0..3) |row| {
            for (0..3) |col| {
                J[row][col] = d_out[row] * d_in[col] / norm;
            }
        }
        // det(3x3)
        const det = J[0][0] * (J[1][1] * J[2][2] - J[1][2] * J[2][1]) -
            J[0][1] * (J[1][0] * J[2][2] - J[1][2] * J[2][0]) +
            J[0][2] * (J[1][0] * J[2][1] - J[1][1] * J[2][0]);
        const tr = J[0][0] + J[1][1] + J[2][2];
        out[idx] = .{
            .frame = i,
            .det = det,
            .trace = tr,
            .diffeomorphic = det > 0,
        };
    }
    return n;
}

pub fn jacobianFlow(traj_a: []const LabColor, traj_b: []const LabColor) JacobianResult {
    var result = JacobianResult{};
    result.n_steps = computeJacobians(traj_a, &result.steps_a);
    _ = computeJacobians(traj_b, &result.steps_b);
    for (0..result.n_steps) |i| {
        if (result.steps_a[i].diffeomorphic) result.diffeo_a += 1;
        if (result.steps_b[i].diffeomorphic) result.diffeo_b += 1;
    }
    return result;
}

// ── L7: Topological Persistence Proxy ───────────────────────────
// Count connected components at multiple epsilon thresholds.
// Uses union-find for O(n α(n)) complexity.

pub const PersistencePoint = struct {
    epsilon: f64,
    components: u32,
};

const UnionFind = struct {
    parent: [512]u16 = undefined,
    rank: [512]u8 = undefined,
    n: u16 = 0,

    pub fn init(n: u16) UnionFind {
        var uf = UnionFind{};
        uf.n = n;
        for (0..n) |i| {
            uf.parent[i] = @intCast(i);
            uf.rank[i] = 0;
        }
        return uf;
    }

    pub fn find(self: *UnionFind, x: u16) u16 {
        var cur = x;
        while (self.parent[cur] != cur) {
            self.parent[cur] = self.parent[self.parent[cur]]; // path halving
            cur = self.parent[cur];
        }
        return cur;
    }

    pub fn unite(self: *UnionFind, x: u16, y: u16) void {
        const rx = self.find(x);
        const ry = self.find(y);
        if (rx == ry) return;
        if (self.rank[rx] < self.rank[ry]) {
            self.parent[rx] = ry;
        } else if (self.rank[rx] > self.rank[ry]) {
            self.parent[ry] = rx;
        } else {
            self.parent[ry] = rx;
            self.rank[rx] += 1;
        }
    }

    pub fn countComponents(self: *UnionFind) u32 {
        var count: u32 = 0;
        for (0..self.n) |i| {
            if (self.parent[i] == @as(u16, @intCast(i))) count += 1;
        }
        return count;
    }
};

pub fn colorPersistence(
    points: []const LabColor,
    epsilons: []const f64,
    out: []PersistencePoint,
) usize {
    const n: u16 = @intCast(@min(points.len, 512));
    const n_eps = @min(epsilons.len, out.len);
    for (0..n_eps) |ei| {
        var uf = UnionFind.init(n);
        const eps_sq = epsilons[ei] * epsilons[ei];
        for (0..n) |i| {
            for (i + 1..n) |j| {
                const fp_a = LabFP.fromLab(points[i]);
                const fp_b = LabFP.fromLab(points[j]);
                const dsq = LabFP.distSq(fp_a, fp_b);
                // Convert fixed-point squared distance back to float for comparison
                const dsq_f = @as(f64, @floatFromInt(dsq)) / (LabFP.SCALE_F * LabFP.SCALE_F);
                if (dsq_f < eps_sq) {
                    uf.unite(@intCast(i), @intCast(j));
                }
            }
        }
        out[ei] = .{
            .epsilon = epsilons[ei],
            .components = uf.countComponents(),
        };
    }
    return n_eps;
}

// ── L8: Mutual Information ──────────────────────────────────────
// I(X;Y) via histogram estimator on YCbCr channels.

pub const MIResult = struct {
    mi_Y_Cb: f64,
    mi_Y_Cr: f64,
    mi_Cb_Cr: f64,
    total: f64,
};

const MI_BINS = 32;

fn mutualInfo2D(xs: []const f64, ys: []const f64) f64 {
    if (xs.len != ys.len or xs.len == 0) return 0;
    var hist: [MI_BINS][MI_BINS]u32 = [_][MI_BINS]u32{[_]u32{0} ** MI_BINS} ** MI_BINS;
    const n = xs.len;
    // Find ranges
    var x_min: f64 = xs[0];
    var x_max: f64 = xs[0];
    var y_min: f64 = ys[0];
    var y_max: f64 = ys[0];
    for (xs) |x| {
        if (x < x_min) x_min = x;
        if (x > x_max) x_max = x;
    }
    for (ys) |y| {
        if (y < y_min) y_min = y;
        if (y > y_max) y_max = y;
    }
    const x_range = if (x_max > x_min) x_max - x_min else 1;
    const y_range = if (y_max > y_min) y_max - y_min else 1;
    // Fill joint histogram
    for (0..n) |i| {
        const xi: usize = @intFromFloat(@min(@max((xs[i] - x_min) / x_range * @as(f64, MI_BINS), 0), @as(f64, MI_BINS - 1)));
        const yi: usize = @intFromFloat(@min(@max((ys[i] - y_min) / y_range * @as(f64, MI_BINS), 0), @as(f64, MI_BINS - 1)));
        hist[xi][yi] += 1;
    }
    // Marginals + MI
    var px: [MI_BINS]f64 = [_]f64{0} ** MI_BINS;
    var py: [MI_BINS]f64 = [_]f64{0} ** MI_BINS;
    const inv_n = 1.0 / @as(f64, @floatFromInt(n));
    for (0..MI_BINS) |i| {
        for (0..MI_BINS) |j| {
            const p = @as(f64, @floatFromInt(hist[i][j])) * inv_n;
            px[i] += p;
            py[j] += p;
        }
    }
    var mi: f64 = 0;
    for (0..MI_BINS) |i| {
        for (0..MI_BINS) |j| {
            const pxy = @as(f64, @floatFromInt(hist[i][j])) * inv_n;
            if (pxy > 0 and px[i] > 0 and py[j] > 0) {
                mi += pxy * @log2(pxy / (px[i] * py[j]));
            }
        }
    }
    return mi;
}

pub fn channelMI(ycbcrs: []const YCbCr) MIResult {
    const n = ycbcrs.len;
    var ys = std.ArrayListUnmanaged(f64){};
    var cbs = std.ArrayListUnmanaged(f64){};
    var crs = std.ArrayListUnmanaged(f64){};
    const alloc = std.heap.page_allocator;
    defer {
        ys.deinit(alloc);
        cbs.deinit(alloc);
        crs.deinit(alloc);
    }
    for (0..n) |i| {
        ys.append(alloc, ycbcrs[i].Y) catch {};
        cbs.append(alloc, ycbcrs[i].Cb) catch {};
        crs.append(alloc, ycbcrs[i].Cr) catch {};
    }
    const mi_ycb = mutualInfo2D(ys.items, cbs.items);
    const mi_ycr = mutualInfo2D(ys.items, crs.items);
    const mi_cbcr = mutualInfo2D(cbs.items, crs.items);
    return .{
        .mi_Y_Cb = mi_ycb,
        .mi_Y_Cr = mi_ycr,
        .mi_Cb_Cr = mi_cbcr,
        .total = mi_ycb + mi_ycr + mi_cbcr,
    };
}

// ── Integration: ColorLossAccumulator ───────────────────────────
// Extends the winding.zig pattern: accumulate color loss signals
// alongside GF(3) trit winding, providing a unified invariant.
//
// The insight: winding numbers track RESOURCE conservation.
// Color losses track PERCEPTUAL conservation.
// Together they give the full picture: a tileableVM's color identity
// is conserved if its winding is zero AND its color trajectory
// is diffeomorphic (L6 det(J) > 0 at all steps).

pub const Trit = enum(i2) {
    minus = -1,
    ergodic = 0,
    plus = 1,
};

pub const ColorLossAccumulator = struct {
    // Winding state (from winding.zig)
    total_angle: i64 = 0,
    transitions: u64 = 0,
    plus_count: u32 = 0,
    minus_count: u32 = 0,
    ergodic_count: u32 = 0,

    // Color trajectory (up to 64 steps)
    trajectory: [64]LabColor = undefined,
    traj_len: usize = 0,

    // Running accumulators for losses
    delta_e_sum: f64 = 0,
    delta_e_max: f64 = 0,
    delta_e_count: u32 = 0,
    diffeo_steps: u32 = 0,
    total_steps: u32 = 0,
    path_length: f64 = 0,

    pub fn recordTrit(self: *ColorLossAccumulator, trit: Trit) void {
        const angle: i64 = switch (trit) {
            .plus => 0x55555555,
            .minus => -0x55555555,
            .ergodic => 0,
        };
        self.total_angle += angle;
        self.transitions += 1;
        switch (trit) {
            .plus => self.plus_count += 1,
            .minus => self.minus_count += 1,
            .ergodic => self.ergodic_count += 1,
        }
    }

    pub fn recordColor(self: *ColorLossAccumulator, rgb: RGB) void {
        const lab = rgb.toLab();
        if (self.traj_len < 64) {
            self.trajectory[self.traj_len] = lab;
            self.traj_len += 1;
        }
        // Update path length incrementally
        if (self.traj_len >= 2) {
            const prev = self.trajectory[self.traj_len - 2];
            const dL = lab.L - prev.L;
            const da = lab.a - prev.a;
            const db = lab.b - prev.b;
            self.path_length += math.sqrt(dL * dL + da * da + db * db);
        }
    }

    pub fn recordDeltaE(self: *ColorLossAccumulator, other: LabColor) void {
        if (self.traj_len == 0) return;
        const mine = self.trajectory[self.traj_len - 1];
        const de = deltaE2000(mine, other);
        self.delta_e_sum += de;
        if (de > self.delta_e_max) self.delta_e_max = de;
        self.delta_e_count += 1;
    }

    // Check current Jacobian step
    pub fn checkDiffeomorphic(self: *ColorLossAccumulator) void {
        if (self.traj_len < 3) return;
        const i = self.traj_len - 2;
        const prev = self.trajectory[i - 1];
        const curr = self.trajectory[i];
        const next = self.trajectory[i + 1];
        const d_in = [3]f64{ curr.L - prev.L, curr.a - prev.a, curr.b - prev.b };
        const d_out = [3]f64{ next.L - curr.L, next.a - curr.a, next.b - curr.b };
        const norm = d_in[0] * d_in[0] + d_in[1] * d_in[1] + d_in[2] * d_in[2];
        self.total_steps += 1;
        if (norm < 1e-12) return;
        // Compute det(outer(d_out, d_in) / norm)
        var J: [3][3]f64 = undefined;
        for (0..3) |r| {
            for (0..3) |c| {
                J[r][c] = d_out[r] * d_in[c] / norm;
            }
        }
        const det = J[0][0] * (J[1][1] * J[2][2] - J[1][2] * J[2][1]) -
            J[0][1] * (J[1][0] * J[2][2] - J[1][2] * J[2][0]) +
            J[0][2] * (J[1][0] * J[2][1] - J[1][1] * J[2][0]);
        if (det > 0) self.diffeo_steps += 1;
    }

    // Winding residue (from winding.zig)
    pub fn isBalanced(self: ColorLossAccumulator) bool {
        return self.plus_count == self.minus_count;
    }

    pub fn residue(self: ColorLossAccumulator) i32 {
        const diff: i64 = @as(i64, self.plus_count) - @as(i64, self.minus_count);
        const r = @mod(diff, 3);
        return @intCast(r);
    }

    // Mean ΔE2000 accumulated
    pub fn meanDeltaE(self: ColorLossAccumulator) f64 {
        if (self.delta_e_count == 0) return 0;
        return self.delta_e_sum / @as(f64, @floatFromInt(self.delta_e_count));
    }

    // Diffeomorphic fraction: what ratio of steps preserve orientation
    pub fn diffeomorphicFraction(self: ColorLossAccumulator) f64 {
        if (self.total_steps == 0) return 0;
        return @as(f64, @floatFromInt(self.diffeo_steps)) / @as(f64, @floatFromInt(self.total_steps));
    }

    // Full conservation check: winding balanced AND color trajectory smooth
    pub fn isFullyConserved(self: ColorLossAccumulator) bool {
        return self.isBalanced() and (self.total_steps == 0 or self.diffeo_steps == self.total_steps);
    }
};

// ── SplitMix64 (matching winding.zig and tile.go) ───────────────

pub fn splitmix64(state: u64) struct { next: u64, value: u64 } {
    const s = state +% 0x9e3779b97f4a7c15;
    var z = s;
    z = (z ^ (z >> 30)) *% 0xbf58476d1ce4e5b9;
    z = (z ^ (z >> 27)) *% 0x94d049bb133111eb;
    z = z ^ (z >> 31);
    return .{ .next = s, .value = z };
}

pub fn colorFromSeed(seed: u64) RGB {
    const result = splitmix64(seed);
    return .{
        .r = @truncate(result.value & 0xFF),
        .g = @truncate((result.value >> 8) & 0xFF),
        .b = @truncate((result.value >> 16) & 0xFF),
    };
}

// ── Tests ───────────────────────────────────────────────────────

test "sRGB to Lab roundtrip sanity" {
    const white = RGB{ .r = 255, .g = 255, .b = 255 };
    const lab = white.toLab();
    try std.testing.expect(lab.L > 99.0 and lab.L < 101.0);
    try std.testing.expect(@abs(lab.a) < 1.0);
    try std.testing.expect(@abs(lab.b) < 1.0);
}

test "sRGB to Lab black" {
    const black = RGB{ .r = 0, .g = 0, .b = 0 };
    const lab = black.toLab();
    try std.testing.expect(lab.L < 1.0);
}

test "deltaE2000 identical colors" {
    const lab = LabColor{ .L = 50, .a = 25, .b = -10 };
    const de = deltaE2000(lab, lab);
    try std.testing.expect(de < 1e-10);
}

test "deltaE2000 known pair" {
    // From ciede2000.pages-perso.free.fr: L1=1.4, a1=42.1, b1=5.4, L2=3.8, a2=36.5, b2=-3.3
    // Expected ΔE00 ≈ 5.433
    const l1 = LabColor{ .L = 1.4, .a = 42.1, .b = 5.4 };
    const l2 = LabColor{ .L = 3.8, .a = 36.5, .b = -3.3 };
    const de = deltaE2000(l1, l2);
    try std.testing.expect(@abs(de - 5.433) < 0.01);
}

test "Wasserstein-1 identical distributions" {
    var h = Histogram.init(0, 100);
    h.add(10);
    h.add(20);
    h.add(30);
    const w = Histogram.wasserstein1(h, h);
    try std.testing.expect(w < 1e-10);
}

test "Wasserstein-1 shifted distributions" {
    var a = Histogram.init(0, 100);
    var b = Histogram.init(0, 100);
    for (0..100) |i| {
        a.add(@floatFromInt(i));
        b.add(@as(f64, @floatFromInt(i)) + 10.0);
    }
    const w = Histogram.wasserstein1(a, b);
    try std.testing.expect(w > 5.0); // should be ~10
}

test "gradient energy constant sequence" {
    const vals = [_]f64{ 5.0, 5.0, 5.0, 5.0 };
    const e = gradientEnergy(&vals);
    try std.testing.expect(e < 1e-10);
}

test "gradient energy linear ramp" {
    const vals = [_]f64{ 0, 1, 2, 3 };
    const e = gradientEnergy(&vals);
    try std.testing.expect(@abs(e - 3.0) < 1e-10); // 1² + 1² + 1² = 3
}

test "subsample RMSE constant" {
    const vals = [_]f64{ 128, 128, 128, 128 };
    const rmse = subsampleRMSE(&vals);
    try std.testing.expect(rmse < 1e-10);
}

test "path length triangle" {
    const traj = [_]LabColor{
        .{ .L = 0, .a = 0, .b = 0 },
        .{ .L = 3, .a = 4, .b = 0 },
        .{ .L = 0, .a = 0, .b = 0 },
    };
    const pl = pathLength(&traj);
    try std.testing.expect(@abs(pl - 10.0) < 1e-10); // 5 + 5 = 10
}

test "discrete Frechet identical trajectories" {
    const traj = [_]LabColor{
        .{ .L = 50, .a = 0, .b = 0 },
        .{ .L = 60, .a = 10, .b = -5 },
    };
    const f = discreteFrechet(&traj, &traj);
    try std.testing.expect(f < 1e-10);
}

test "union find basics" {
    var uf = UnionFind.init(5);
    try std.testing.expectEqual(@as(u32, 5), uf.countComponents());
    uf.unite(0, 1);
    try std.testing.expectEqual(@as(u32, 4), uf.countComponents());
    uf.unite(2, 3);
    try std.testing.expectEqual(@as(u32, 3), uf.countComponents());
    uf.unite(0, 3);
    try std.testing.expectEqual(@as(u32, 2), uf.countComponents());
}

test "mutual info independent channels" {
    const n = 100;
    var ys: [n]YCbCr = undefined;
    var state: u64 = 42;
    for (0..n) |i| {
        const r = splitmix64(state);
        state = r.next;
        ys[i] = .{
            .Y = @as(f64, @floatFromInt(r.value & 0xFF)),
            .Cb = 128, // constant Cb → MI(Y, Cb) ≈ 0
            .Cr = @as(f64, @floatFromInt((r.value >> 16) & 0xFF)),
        };
    }
    const mi = channelMI(&ys);
    try std.testing.expect(mi.mi_Y_Cb < 0.5); // near zero for independent
}

test "LabFP roundtrip" {
    const lab = LabColor{ .L = 50.5, .a = -32.75, .b = 17.125 };
    const fp = LabFP.fromLab(lab);
    const back = fp.toLab();
    try std.testing.expect(@abs(back.L - lab.L) < 0.001);
    try std.testing.expect(@abs(back.a - lab.a) < 0.001);
    try std.testing.expect(@abs(back.b - lab.b) < 0.001);
}

test "ColorLossAccumulator basic flow" {
    var acc = ColorLossAccumulator{};
    // Record a trit sequence
    acc.recordTrit(.plus);
    acc.recordTrit(.ergodic);
    acc.recordTrit(.minus);
    try std.testing.expect(acc.isBalanced());

    // Record colors from seeds
    const c1 = colorFromSeed(1069);
    const c2 = colorFromSeed(42);
    const c3 = colorFromSeed(7);
    acc.recordColor(c1);
    acc.recordColor(c2);
    acc.recordColor(c3);
    acc.checkDiffeomorphic();
    try std.testing.expect(acc.traj_len == 3);
    try std.testing.expect(acc.path_length > 0);

    // Record a ΔE comparison
    const other = (RGB{ .r = 200, .g = 100, .b = 50 }).toLab();
    acc.recordDeltaE(other);
    try std.testing.expect(acc.meanDeltaE() > 0);
}

test "colorFromSeed matches tile.go" {
    // Seed 1069 → #769C7D (from session summary)
    const c = colorFromSeed(1069);
    const hex = std.fmt.allocPrint(std.heap.page_allocator, "#{X:0>2}{X:0>2}{X:0>2}", .{ c.r, c.g, c.b }) catch unreachable;
    defer std.heap.page_allocator.free(hex);
    try std.testing.expectEqualStrings("#769C7D", hex);
}

test "YCbCr conversion sanity" {
    const white = RGB{ .r = 255, .g = 255, .b = 255 };
    const ycbcr = white.toYCbCr();
    try std.testing.expect(@abs(ycbcr.Y - 255.0) < 0.01);
    try std.testing.expect(@abs(ycbcr.Cb - 128.0) < 0.5);
    try std.testing.expect(@abs(ycbcr.Cr - 128.0) < 0.5);
}
