const std = @import("std");
const color_losses = @import("color_losses.zig");

// Unified bridge: color losses + winding numbers for tileable VM verification.
//
// This module connects two orthogonal measurement systems:
//   - winding.zig: topological lifecycle invariant (resource balance)
//   - color_losses.zig: perceptual/information-theoretic color fidelity
//
// Together they form the complete verification surface:
//   Winding answers: "Is the resource lifecycle balanced?"
//   Color losses answer: "Is the color identity preserved through transformations?"
//
// The bridge uses color_losses.Trit as the canonical trit type since
// ColorLossAccumulator already embeds winding state (plus/minus/ergodic counts).
// Winding-only callers can use @intFromEnum/@enumFromInt to convert.

pub const Trit = color_losses.Trit;
pub const RGB = color_losses.RGB;
pub const LabColor = color_losses.LabColor;
pub const YCbCr = color_losses.YCbCr;
pub const colorFromSeed = color_losses.colorFromSeed;
pub const ColorLossAccumulator = color_losses.ColorLossAccumulator;

// MoveOp: resource state machine transitions, duplicated from winding.zig
// to avoid cross-file import issues. Same semantics.
pub const MoveOp = enum {
    move_to,
    move_from,
    borrow_global,
    borrow_global_mut,
    mint,
    burn,
    transfer,
    set_flag,
    clear_flag,

    pub fn toTrit(self: MoveOp) Trit {
        return switch (self) {
            .move_to => .plus,
            .move_from => .minus,
            .borrow_global => .ergodic,
            .borrow_global_mut => .ergodic,
            .mint => .plus,
            .burn => .minus,
            .transfer => .ergodic,
            .set_flag => .plus,
            .clear_flag => .minus,
        };
    }
};

// TileVerifier: unified verification for a tileable VM instance.
// Tracks both resource lifecycle (winding via trit counts) and color fidelity (losses)
// from a single seed-derived identity.
pub const TileVerifier = struct {
    seed: u64,
    identity_color: RGB,
    losses: ColorLossAccumulator,
    transitions: u64,

    pub fn init(seed: u64) TileVerifier {
        const c = colorFromSeed(seed);
        var losses = ColorLossAccumulator{};
        losses.recordColor(c);
        return .{
            .seed = seed,
            .identity_color = c,
            .losses = losses,
            .transitions = 0,
        };
    }

    // Record a lifecycle transition and its associated color observation.
    pub fn recordTransition(self: *TileVerifier, op: MoveOp, observed: RGB) void {
        const trit = op.toTrit();
        self.losses.recordTrit(trit);
        self.losses.recordColor(observed);
        self.losses.checkDiffeomorphic();
        self.transitions += 1;
    }

    // Record a lifecycle transition without color observation.
    pub fn recordOp(self: *TileVerifier, op: MoveOp) void {
        self.losses.recordTrit(op.toTrit());
        self.transitions += 1;
    }

    // Is the resource lifecycle balanced?
    pub fn isLifecycleBalanced(self: TileVerifier) bool {
        return self.losses.isBalanced();
    }

    // Is the color trajectory diffeomorphic (orientation-preserving)?
    pub fn isDiffeomorphic(self: TileVerifier) bool {
        if (self.losses.total_steps == 0) return true;
        return self.losses.diffeo_steps > 0;
    }

    // Mean perceptual color difference from identity.
    pub fn meanDeltaE(self: TileVerifier) f64 {
        return self.losses.meanDeltaE();
    }

    // Summary: is the tile verified?
    // A tile passes when:
    //   1. Resource lifecycle is balanced (winding == 0)
    //   2. Color trajectory is orientation-preserving (diffeomorphic)
    //   3. GF(3) trit residue is zero
    pub fn isVerified(self: TileVerifier) bool {
        return self.isLifecycleBalanced() and
            self.isDiffeomorphic() and
            self.losses.isBalanced();
    }

    // Winding number (from embedded trit counts)
    pub fn winding(self: TileVerifier) i32 {
        const diff: i64 = @as(i64, self.losses.plus_count) - @as(i64, self.losses.minus_count);
        return @intCast(@divTrunc(diff, 3));
    }

    pub fn residue(self: TileVerifier) i32 {
        return self.losses.residue();
    }

    // Compact report suitable for OSC 8 hyperlink hover text.
    pub fn report(self: TileVerifier, buf: []u8) []const u8 {
        const r = std.fmt.bufPrint(buf, "seed={d} wind={d} res={d} de={d:.1} traj={d} diffeo={s} {s}", .{
            self.seed,
            self.winding(),
            self.residue(),
            self.meanDeltaE(),
            self.losses.traj_len,
            if (self.isDiffeomorphic()) "ok" else "FLIP",
            if (self.isVerified()) "PASS" else "FAIL",
        }) catch buf[0..0];
        return r;
    }
};

// Multi-tile verifier: tracks N tiles simultaneously.
// The global invariant: all individual tiles verified AND
// sum of all windings conserved.
pub const MultiTileVerifier = struct {
    tiles: std.AutoHashMap(u64, TileVerifier),

    pub fn init(allocator: std.mem.Allocator) MultiTileVerifier {
        return .{
            .tiles = std.AutoHashMap(u64, TileVerifier).init(allocator),
        };
    }

    pub fn deinit(self: *MultiTileVerifier) void {
        self.tiles.deinit();
    }

    pub fn addTile(self: *MultiTileVerifier, seed: u64) !*TileVerifier {
        const gop = try self.tiles.getOrPut(seed);
        if (!gop.found_existing) {
            gop.value_ptr.* = TileVerifier.init(seed);
        }
        return gop.value_ptr;
    }

    pub fn isGloballyConserved(self: MultiTileVerifier) bool {
        var total_plus: i64 = 0;
        var total_minus: i64 = 0;
        var iter = self.tiles.valueIterator();
        while (iter.next()) |tile| {
            total_plus += @as(i64, tile.losses.plus_count);
            total_minus += @as(i64, tile.losses.minus_count);
        }
        return total_plus == total_minus;
    }

    pub fn allVerified(self: MultiTileVerifier) bool {
        var iter = self.tiles.valueIterator();
        while (iter.next()) |tile| {
            if (!tile.isVerified()) return false;
        }
        return true;
    }
};

// ── Tests ──────────────────────────────────────────────────────

test "TileVerifier init from seed" {
    const v = TileVerifier.init(1069);
    try std.testing.expectEqual(@as(u8, 0x76), v.identity_color.r);
    try std.testing.expectEqual(@as(u8, 0x9C), v.identity_color.g);
    try std.testing.expectEqual(@as(u8, 0x7D), v.identity_color.b);
    try std.testing.expect(v.isLifecycleBalanced());
}

test "TileVerifier balanced lifecycle with colors" {
    var v = TileVerifier.init(42);
    const c1 = colorFromSeed(100);
    const c2 = colorFromSeed(200);
    v.recordTransition(.move_to, c1);
    v.recordTransition(.borrow_global, c2);
    v.recordTransition(.move_from, c1);
    try std.testing.expect(v.isLifecycleBalanced());
    try std.testing.expect(v.transitions == 3);
}

test "TileVerifier unbalanced lifecycle" {
    var v = TileVerifier.init(7);
    v.recordOp(.mint);
    v.recordOp(.transfer);
    // missing burn
    try std.testing.expect(!v.isLifecycleBalanced());
    try std.testing.expect(!v.isVerified());
}

test "MultiTileVerifier conservation" {
    var mtv = MultiTileVerifier.init(std.testing.allocator);
    defer mtv.deinit();

    const t1 = try mtv.addTile(1069);
    t1.recordOp(.move_to);
    t1.recordOp(.move_from);

    const t2 = try mtv.addTile(42);
    t2.recordOp(.mint);
    t2.recordOp(.burn);

    try std.testing.expect(mtv.isGloballyConserved());
}

test "MultiTileVerifier violation" {
    var mtv = MultiTileVerifier.init(std.testing.allocator);
    defer mtv.deinit();

    const t1 = try mtv.addTile(1069);
    t1.recordOp(.move_to);

    try std.testing.expect(!mtv.isGloballyConserved());
    try std.testing.expect(!mtv.allVerified());
}

test "TileVerifier report formatting" {
    var v = TileVerifier.init(1069);
    v.recordOp(.move_to);
    v.recordOp(.move_from);
    var buf: [256]u8 = undefined;
    const r = v.report(&buf);
    try std.testing.expect(r.len > 0);
}
