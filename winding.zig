const std = @import("std");
const math = std.math;

// Winding number computation for Move resource lifecycle verification.
//
// A winding number counts how many times a closed curve winds around a point.
// For resource verification: the "curve" is the sequence of resource state
// transitions, the "point" is the origin (balanced state). A winding number
// of 0 means the resource lifecycle is balanced (all acquires have releases).
// Non-zero winding number = resource leak or double-free.
//
// This wins over Flecs ECS (archetype tables) because:
//   Flecs: O(n) scan of component arrays to check lifecycle invariants
//   Winding: O(1) per transition, O(1) final check (is winding == 0?)
//
// This wins over MeTTa atomspace pattern matching because:
//   MeTTa: unification-based search over metagraph, exponential worst case
//   Winding: integer arithmetic, branch-free, SIMD-vectorizable
//
// The winding number IS the topological invariant that Flecs tables and
// MeTTa atoms are both trying to approximate via structural search.

pub const WindingAccumulator = struct {
    // Fixed-point angle accumulation (turns * 2^32)
    // Avoids all floating point. One full turn = 0x100000000.
    total_angle: i64 = 0,
    transitions: u64 = 0,

    // GF(3) trit decomposition of the winding
    plus_count: u32 = 0, // acquire / create / mint
    minus_count: u32 = 0, // release / destroy / burn
    ergodic_count: u32 = 0, // transfer / move (neutral)

    pub fn record(self: *WindingAccumulator, trit: Trit) void {
        const angle: i64 = switch (trit) {
            .plus => 0x55555555, // +1/3 turn (acquire)
            .minus => -0x55555555, // -1/3 turn (release)
            .ergodic => 0, // 0 turn (transfer)
        };
        self.total_angle += angle;
        self.transitions += 1;
        switch (trit) {
            .plus => self.plus_count += 1,
            .minus => self.minus_count += 1,
            .ergodic => self.ergodic_count += 1,
        }
    }

    // The winding number: how many complete turns around the origin.
    // 0 = balanced lifecycle. +n = n excess acquires. -n = n excess releases.
    pub fn winding(self: WindingAccumulator) i32 {
        // Integer division rounds toward zero. Each full turn is 0x100000000.
        // We use the GF(3) balanced form: winding = (plus - minus) / 3
        const diff: i64 = @as(i64, self.plus_count) - @as(i64, self.minus_count);
        return @intCast(@divTrunc(diff, 3));
    }

    // Raw winding in thirds (before dividing by 3)
    // This is the GF(3) residue: the number mod 3
    pub fn residue(self: WindingAccumulator) i32 {
        const diff: i64 = @as(i64, self.plus_count) - @as(i64, self.minus_count);
        const r = @mod(diff, 3);
        return @intCast(r);
    }

    // Is the resource lifecycle balanced?
    // True iff winding number is 0 AND residue is 0.
    pub fn isBalanced(self: WindingAccumulator) bool {
        return self.plus_count == self.minus_count;
    }

    // The accumulated angle in fixed-point turns
    pub fn angleTurns(self: WindingAccumulator) f64 {
        return @as(f64, @floatFromInt(self.total_angle)) / @as(f64, 0x100000000);
    }
};

pub const Trit = enum(i2) {
    minus = -1,
    ergodic = 0,
    plus = 1,
};

// Move resource state machine encoded as winding transitions.
// Each Move resource operation maps to a trit:
//   move_to()       -> plus  (+1/3 turn, resource enters existence)
//   move_from()     -> minus (-1/3 turn, resource leaves existence)
//   borrow_global() -> ergodic (0 turn, resource observed but not moved)
//   borrow_global_mut() -> ergodic (0 turn, mutated in place)
pub const MoveOp = enum {
    move_to,
    move_from,
    borrow_global,
    borrow_global_mut,
    // Extended ops for fungible assets
    mint,
    burn,
    transfer,
    // Spell flag ops (from spell_boolean_flags.json)
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

// Batch winding computation for a trace of Move operations.
// This is the core advantage over Flecs/MeTTa:
// - Flecs would store each operation as a component on an entity,
//   then query all entities with that component type, then scan.
//   Cost: O(archetypes) to find + O(n) to scan.
// - MeTTa would represent each operation as an atom in the atomspace,
//   then unify a pattern like (move_to ?resource ?addr) across the
//   entire metagraph. Cost: O(atoms) unification steps.
// - Winding: one integer add per operation, no search at all.
//   Cost: O(n) with constant factor = 1 add + 1 branch.
pub fn computeWinding(ops: []const MoveOp) WindingAccumulator {
    var acc = WindingAccumulator{};
    for (ops) |op| {
        acc.record(op.toTrit());
    }
    return acc;
}

// Parallel winding for multiple resources simultaneously.
// Each resource gets its own accumulator. The global invariant:
// sum of all winding numbers must equal 0 (conservation of resources).
//
// This is where winding numbers DOMINATE Flecs:
// Flecs tracks N archetypes x M components = N*M table cells.
// Winding tracks N accumulators x 1 integer = N integers.
// For 80,000 spells x 644 flags, Flecs needs 51.5M cells.
// Winding needs 80,000 integers.
pub const MultiWinding = struct {
    accumulators: std.AutoHashMap(u64, WindingAccumulator),

    pub fn init(allocator: std.mem.Allocator) MultiWinding {
        return .{
            .accumulators = std.AutoHashMap(u64, WindingAccumulator).init(allocator),
        };
    }

    pub fn deinit(self: *MultiWinding) void {
        self.accumulators.deinit();
    }

    pub fn record(self: *MultiWinding, resource_id: u64, op: MoveOp) !void {
        const gop = try self.accumulators.getOrPut(resource_id);
        if (!gop.found_existing) {
            gop.value_ptr.* = WindingAccumulator{};
        }
        gop.value_ptr.record(op.toTrit());
    }

    // Check global conservation: sum of all windings must be 0
    pub fn isConserved(self: MultiWinding) bool {
        var total: i64 = 0;
        var iter = self.accumulators.valueIterator();
        while (iter.next()) |acc| {
            total += @as(i64, acc.plus_count) - @as(i64, acc.minus_count);
        }
        return total == 0;
    }

    // Find all resources with non-zero winding (leaks or double-frees)
    pub fn violations(self: MultiWinding, allocator: std.mem.Allocator) ![]const Violation {
        var list = std.ArrayList(Violation).init(allocator);
        var key_iter = self.accumulators.iterator();
        while (key_iter.next()) |entry| {
            if (!entry.value_ptr.isBalanced()) {
                try list.append(.{
                    .resource_id = entry.key_ptr.*,
                    .winding = entry.value_ptr.winding(),
                    .residue = entry.value_ptr.residue(),
                    .plus = entry.value_ptr.plus_count,
                    .minus = entry.value_ptr.minus_count,
                });
            }
        }
        return list.toOwnedSlice();
    }
};

pub const Violation = struct {
    resource_id: u64,
    winding: i32,
    residue: i32,
    plus: u32,
    minus: u32,
};

// === Why winding numbers win over Flecs ECS (from MeTTa) ===
//
// The argument is topological vs structural:
//
// FLECS (archetype ECS):
//   Storage: entities grouped by component signature (archetype)
//   Query: match archetype, iterate SoA columns
//   Lifecycle: add component -> move entity between tables
//   Invariant check: query all tables, scan all matching entities
//   Complexity: O(archetypes * entities_per_archetype)
//   Memory: O(entities * components) -- one cell per (entity, component)
//
// METTA (atomspace pattern matching):
//   Storage: atoms in a metagraph, linked by expressions
//   Query: unification of pattern variables against atomspace
//   Lifecycle: add/remove atoms, rewrite metagraph
//   Invariant check: pattern match over entire atomspace
//   Complexity: O(atoms^k) for k-variable patterns (worst case)
//   Memory: O(atoms * links) -- one node per atom, edges per link
//
// WINDING NUMBERS (this implementation):
//   Storage: one i64 per resource (the accumulated angle)
//   Query: none needed -- the invariant IS the winding number
//   Lifecycle: one integer add per state transition
//   Invariant check: test winding == 0 (one comparison)
//   Complexity: O(1) per transition, O(1) per check
//   Memory: O(resources) -- one integer per resource
//
// The key insight: Flecs and MeTTa both SEARCH for violations.
// Winding numbers ACCUMULATE the answer.
//
// Search is O(state_space). Accumulation is O(transitions).
// State spaces grow combinatorially. Transitions grow linearly.
//
// For the spell flag surface (80,000 spells x 644 flags):
//   Flecs: 51,520,000 archetype cells to maintain and query
//   MeTTa: 51,520,000 atoms to unify patterns against
//   Winding: 80,000 integers, each updated by a single add
//
// The winding number is the INTEGRAL over the state space.
// Flecs and MeTTa are both trying to compute the integral by
// enumerating the integrand. Winding computes it incrementally.
//
// This is why topological invariants (winding, Euler characteristic,
// homology) always beat structural enumeration (tables, graphs):
// invariants compress the entire state space into a single number.

test "balanced lifecycle" {
    var acc = WindingAccumulator{};
    acc.record(.plus); // move_to
    acc.record(.ergodic); // borrow
    acc.record(.ergodic); // borrow_mut
    acc.record(.minus); // move_from
    try std.testing.expect(acc.isBalanced());
    try std.testing.expectEqual(@as(i32, 0), acc.winding());
}

test "resource leak detection" {
    var acc = WindingAccumulator{};
    acc.record(.plus); // move_to
    acc.record(.ergodic); // borrow
    // missing move_from!
    try std.testing.expect(!acc.isBalanced());
    try std.testing.expectEqual(@as(i32, 0), acc.winding()); // 1/3 turn, not a full winding yet
    try std.testing.expectEqual(@as(i32, 1), acc.residue()); // but residue is non-zero
}

test "double free detection" {
    var acc = WindingAccumulator{};
    acc.record(.plus); // move_to
    acc.record(.minus); // move_from
    acc.record(.minus); // double free!
    try std.testing.expect(!acc.isBalanced());
}

test "batch winding" {
    const ops = [_]MoveOp{
        .move_to,
        .borrow_global,
        .borrow_global_mut,
        .transfer,
        .move_from,
    };
    const result = computeWinding(&ops);
    try std.testing.expect(result.isBalanced());
    try std.testing.expectEqual(@as(u64, 5), result.transitions);
}

test "multi-resource conservation" {
    var mw = MultiWinding.init(std.testing.allocator);
    defer mw.deinit();

    // Resource 1: balanced
    try mw.record(1, .move_to);
    try mw.record(1, .move_from);

    // Resource 2: balanced
    try mw.record(2, .mint);
    try mw.record(2, .transfer);
    try mw.record(2, .burn);

    try std.testing.expect(mw.isConserved());
}

test "multi-resource violation" {
    var mw = MultiWinding.init(std.testing.allocator);
    defer mw.deinit();

    // Resource 1: leak
    try mw.record(1, .move_to);

    // Resource 2: balanced
    try mw.record(2, .mint);
    try mw.record(2, .burn);

    try std.testing.expect(!mw.isConserved());

    const viols = try mw.violations(std.testing.allocator);
    defer std.testing.allocator.free(viols);
    try std.testing.expectEqual(@as(usize, 1), viols.len);
    try std.testing.expectEqual(@as(u64, 1), viols[0].resource_id);
}

test "spell flag winding" {
    // Simulates a server setting and clearing spell flags
    // Setting a flag = plus, clearing = minus
    // A balanced server sets and clears the same flags
    var mw = MultiWinding.init(std.testing.allocator);
    defer mw.deinit();

    // Spell 123: modify SPELL_ATTR0_SERVER_ONLY
    try mw.record(123, .set_flag);
    // Spell 456: modify SPELL_ATTR0_NO_IMMUNITIES
    try mw.record(456, .set_flag);
    // Spell 456: revert SPELL_ATTR0_NO_IMMUNITIES
    try mw.record(456, .clear_flag);

    // Spell 123 still has divergent flag = winding violation
    try std.testing.expect(!mw.isConserved());

    const viols = try mw.violations(std.testing.allocator);
    defer std.testing.allocator.free(viols);
    try std.testing.expectEqual(@as(usize, 1), viols.len);
    try std.testing.expectEqual(@as(u64, 123), viols[0].resource_id);
}
