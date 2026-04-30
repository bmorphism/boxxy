//go:build darwin

package tile

import (
	"fmt"

	"github.com/bmorphism/boxxy/internal/color"
	"github.com/bmorphism/boxxy/internal/gf3"
	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/vm"

	"github.com/charmbracelet/lipgloss"
)

// RegisterNamespace registers the tile/ namespace into the boxxy Lisp env.
func RegisterNamespace(env *lisp.Env) {
	reg := func(name string, f func([]lisp.Value) lisp.Value) {
		env.Set(lisp.Symbol(name), &lisp.Fn{Name: name, Func: f})
	}

	// tile/color-identity — seed → full identity map
	reg("tile/color-identity", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/color-identity: requires (seed)")
		}
		seed := uint64(args[0].(lisp.Int))
		ci := NewColorIdentity(seed)
		return lisp.HashMap{
			lisp.Keyword("seed"):  lisp.Int(ci.Seed),
			lisp.Keyword("hex"):   lisp.String(ci.HexCode),
			lisp.Keyword("r"):     lisp.Int(int64(ci.Color.R)),
			lisp.Keyword("g"):     lisp.Int(int64(ci.Color.G)),
			lisp.Keyword("b"):     lisp.Int(int64(ci.Color.B)),
			lisp.Keyword("trit"):  lisp.Int(int64(ci.Trit)),
			lisp.Keyword("role"):  lisp.String(ci.Role.String()),
		}
	})

	// tile/syrup-encode — seed → syrup wire bytes as string
	reg("tile/syrup-encode", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/syrup-encode: requires (seed)")
		}
		seed := uint64(args[0].(lisp.Int))
		ci := NewColorIdentity(seed)
		wire := ci.EncodeSyrupColor()
		return lisp.String(string(wire))
	})

	// tile/syrup-checkpoint — seed worker-id invocation → checkpoint wire bytes
	reg("tile/syrup-checkpoint", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tile/syrup-checkpoint: requires (seed worker-id invocation)")
		}
		seed := uint64(args[0].(lisp.Int))
		workerID := uint64(args[1].(lisp.Int))
		invocation := uint64(args[2].(lisp.Int))
		ci := NewColorIdentity(seed)
		wire := ci.EncodeSyrupCheckpoint(workerID, invocation)
		return lisp.String(string(wire))
	})

	// tile/message-frame — wrap payload bytes in 4-byte BE length prefix
	reg("tile/message-frame", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/message-frame: requires (payload-string)")
		}
		payload := string(args[0].(lisp.String))
		frame := EncodeMessageFrame([]byte(payload))
		return lisp.String(string(frame))
	})

	// tile/rainbow-parens — colorize s-expr string using a seed's palette
	reg("tile/rainbow-parens", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tile/rainbow-parens: requires (seed sexp-string)")
		}
		seed := uint64(args[0].(lisp.Int))
		input := string(args[1].(lisp.String))
		ci := NewColorIdentity(seed)
		palette := ci.RainbowPalette(8)
		styles := color.PaletteToLipgloss(palette)
		return lisp.String(color.RainbowParens(input, styles))
	})

	// tile/new-tileable-vm — create a tile-wrapped VM config
	reg("tile/new-tileable-vm", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tile/new-tileable-vm: requires (name seed vm-config)")
		}
		name := string(args[0].(lisp.String))
		seed := uint64(args[1].(lisp.Int))
		cfgExt := args[2].(*lisp.ExternalValue)
		cfg := cfgExt.Value.(*vm.Config)
		tv := NewTileableVM(name, seed, *cfg)
		return &lisp.ExternalValue{Value: tv, Type: "TileableVM"}
	})

	// tile/lattice-new — create an empty tile lattice
	reg("tile/lattice-new", func(args []lisp.Value) lisp.Value {
		return &lisp.ExternalValue{Value: NewTileLattice(), Type: "TileLattice"}
	})

	// tile/lattice-add — add a tileable VM to the lattice
	reg("tile/lattice-add", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tile/lattice-add: requires (lattice tileable-vm)")
		}
		lattice := args[0].(*lisp.ExternalValue).Value.(*TileLattice)
		tv := args[1].(*lisp.ExternalValue).Value.(*TileableVM)
		lattice.Add(tv)
		return lisp.Bool(true)
	})

	// tile/lattice-balanced? — check if the lattice trits sum to 0 mod 3
	reg("tile/lattice-balanced?", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/lattice-balanced?: requires (lattice)")
		}
		lattice := args[0].(*lisp.ExternalValue).Value.(*TileLattice)
		return lisp.Bool(lattice.IsBalanced())
	})

	// tile/lattice-find-balancer — find a seed that balances the lattice
	reg("tile/lattice-find-balancer", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/lattice-find-balancer: requires (lattice)")
		}
		lattice := args[0].(*lisp.ExternalValue).Value.(*TileLattice)
		start := uint64(0)
		if len(args) > 1 {
			start = uint64(args[1].(lisp.Int))
		}
		seed, ok := lattice.FindBalancerSeed(start, 10000)
		if !ok {
			return lisp.Nil{}
		}
		ci := NewColorIdentity(seed)
		return lisp.HashMap{
			lisp.Keyword("seed"): lisp.Int(int64(seed)),
			lisp.Keyword("hex"):  lisp.String(ci.HexCode),
			lisp.Keyword("trit"): lisp.Int(int64(ci.Trit)),
		}
	})

	// tile/lattice-wire-colors — all tile colors as syrup records
	reg("tile/lattice-wire-colors", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/lattice-wire-colors: requires (lattice)")
		}
		lattice := args[0].(*lisp.ExternalValue).Value.(*TileLattice)
		wires := lattice.WireColors()
		result := make(lisp.Vector, len(wires))
		for i, w := range wires {
			result[i] = lisp.String(string(w))
		}
		return result
	})

	// tile/balanced-trit — compute GF(3) trit from seed
	reg("tile/balanced-trit", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/balanced-trit: requires (seed)")
		}
		seed := uint64(args[0].(lisp.Int))
		ci := NewColorIdentity(seed)
		bt := gf3.ToBalanced(ci.Trit)
		return lisp.Int(int64(bt))
	})

	// tile/find-balancer — given 2 seeds, find a third that balances
	reg("tile/find-balancer", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tile/find-balancer: requires (seed-a seed-b)")
		}
		a := NewColorIdentity(uint64(args[0].(lisp.Int)))
		b := NewColorIdentity(uint64(args[1].(lisp.Int)))
		needed := gf3.FindBalancer(a.Trit, b.Trit, gf3.Zero)
		// Search for a seed with this trit
		for s := uint64(0); s < 10000; s++ {
			ci := NewColorIdentity(s)
			if ci.Trit == needed {
				return lisp.HashMap{
					lisp.Keyword("seed"): lisp.Int(int64(s)),
					lisp.Keyword("hex"):  lisp.String(ci.HexCode),
					lisp.Keyword("trit"): lisp.Int(int64(ci.Trit)),
				}
			}
		}
		return lisp.Nil{}
	})

	// tile/triangulate — 3 seeds → behavioral distance triangle
	// Returns distances and whether strong/weak triangle inequality holds.
	// This is the REBEL bridge: identical memo patterns => d=0 => IR-equivalent.
	reg("tile/triangulate", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("tile/triangulate: requires (seed-a seed-b seed-c)")
		}
		seedA := uint64(args[0].(lisp.Int))
		seedB := uint64(args[1].(lisp.Int))
		seedC := uint64(args[2].(lisp.Int))

		trA := NewBehaviorTrace(seedA)
		trB := NewBehaviorTrace(seedB)
		trC := NewBehaviorTrace(seedC)

		// Simulate a shared computation: derive color, check trit, memoize
		// Each seed runs the same "program" but with different state
		for i := 0; i < 20; i++ {
			ciA := NewColorIdentity(seedA + uint64(i))
			ciB := NewColorIdentity(seedB + uint64(i))
			ciC := NewColorIdentity(seedC + uint64(i))
			// "hit" if trit matches the seed's base trit (structural reuse)
			trA.Record(ciA.Trit == NewColorIdentity(seedA).Trit)
			trB.Record(ciB.Trit == NewColorIdentity(seedB).Trit)
			trC.Record(ciC.Trit == NewColorIdentity(seedC).Trit)
		}

		tri := NewTriangulation(trA, trB, trC)
		return lisp.HashMap{
			lisp.Keyword("seeds"):    lisp.Vector{lisp.Int(int64(seedA)), lisp.Int(int64(seedB)), lisp.Int(int64(seedC))},
			lisp.Keyword("d-ab"):     lisp.Float(tri.DAB),
			lisp.Keyword("d-bc"):     lisp.Float(tri.DBC),
			lisp.Keyword("d-ac"):     lisp.Float(tri.DAC),
			lisp.Keyword("strong?"):  lisp.Bool(tri.StrongTriangleInequality()),
			lisp.Keyword("ultra?"):   lisp.Bool(tri.WeakTriangleInequality()),
			lisp.Keyword("rebel?"):   lisp.Bool(tri.IsSemanticallyEquivalent()),
		}
	})

	// tile/trace — create a behavior trace and record N steps
	reg("tile/trace", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tile/trace: requires (seed n-steps)")
		}
		seed := uint64(args[0].(lisp.Int))
		n := int(args[1].(lisp.Int))
		tr := NewBehaviorTrace(seed)
		baseTrit := NewColorIdentity(seed).Trit
		for i := 0; i < n; i++ {
			ci := NewColorIdentity(seed + uint64(i))
			tr.Record(ci.Trit == baseTrit)
		}
		return lisp.HashMap{
			lisp.Keyword("seed"):       lisp.Int(int64(seed)),
			lisp.Keyword("calls"):      lisp.Int(int64(tr.CallCount)),
			lisp.Keyword("memo-hits"):  lisp.Int(int64(tr.MemoHits)),
			lisp.Keyword("memo-misses"): lisp.Int(int64(tr.MemoMisses)),
			lisp.Keyword("trace-hash"): lisp.Int(int64(tr.TraceHash)),
		}
	})

	// --- TileVerifier builtins ---

	// tile/verifier-new — seed → TileVerifier (unified resource + color tracking)
	reg("tile/verifier-new", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/verifier-new: requires (seed)")
		}
		seed := uint64(args[0].(lisp.Int))
		tv := NewTileVerifier(seed)
		return &lisp.ExternalValue{Value: tv, Type: "TileVerifier"}
	})

	// tile/verifier-op — record a lifecycle operation (op is keyword)
	reg("tile/verifier-op", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tile/verifier-op: requires (verifier op-keyword)")
		}
		tv := args[0].(*lisp.ExternalValue).Value.(*TileVerifier)
		opName := string(args[1].(lisp.Keyword))
		op := parseMoveOp(opName)
		if len(args) >= 3 {
			cseed := uint64(args[2].(lisp.Int))
			tv.RecordTransition(op, ColorFromSeed(cseed))
		} else {
			tv.RecordOp(op)
		}
		return lisp.Bool(true)
	})

	// tile/verifier-status — return verification status as map
	reg("tile/verifier-status", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/verifier-status: requires (verifier)")
		}
		tv := args[0].(*lisp.ExternalValue).Value.(*TileVerifier)
		return lisp.HashMap{
			lisp.Keyword("seed"):      lisp.Int(int64(tv.Seed)),
			lisp.Keyword("hex"):       lisp.String(tv.IdentityColor.Hex()),
			lisp.Keyword("balanced?"):  lisp.Bool(tv.IsLifecycleBalanced()),
			lisp.Keyword("diffeo?"):    lisp.Bool(tv.IsDiffeomorphic()),
			lisp.Keyword("verified?"):  lisp.Bool(tv.IsVerified()),
			lisp.Keyword("winding"):    lisp.Int(int64(tv.Winding())),
			lisp.Keyword("residue"):    lisp.Int(int64(tv.Losses.Residue())),
			lisp.Keyword("delta-e"):    lisp.Float(tv.MeanDeltaE()),
			lisp.Keyword("transitions"): lisp.Int(int64(tv.Transitions)),
		}
	})

	// tile/verifier-report — compact report string
	reg("tile/verifier-report", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/verifier-report: requires (verifier)")
		}
		tv := args[0].(*lisp.ExternalValue).Value.(*TileVerifier)
		return lisp.String(tv.Report())
	})

	// tile/multi-verifier-new — create a multi-tile verifier
	reg("tile/multi-verifier-new", func(args []lisp.Value) lisp.Value {
		mtv := NewMultiTileVerifier()
		return &lisp.ExternalValue{Value: mtv, Type: "MultiTileVerifier"}
	})

	// tile/multi-verifier-add — add a tile seed to the multi-verifier
	reg("tile/multi-verifier-add", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tile/multi-verifier-add: requires (multi-verifier seed)")
		}
		mtv := args[0].(*lisp.ExternalValue).Value.(*MultiTileVerifier)
		seed := uint64(args[1].(lisp.Int))
		tv := mtv.AddTile(seed)
		return &lisp.ExternalValue{Value: tv, Type: "TileVerifier"}
	})

	// tile/multi-verifier-conserved? — global conservation check
	reg("tile/multi-verifier-conserved?", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/multi-verifier-conserved?: requires (multi-verifier)")
		}
		mtv := args[0].(*lisp.ExternalValue).Value.(*MultiTileVerifier)
		return lisp.Bool(mtv.IsGloballyConserved())
	})

	// --- Parallel SPI builtins ---

	// tile/parallel-verify — (seeds-vec ops-vec) → {results, conserved?}
	// Runs N goroutines with value-copy closures. Zero shared mutable state.
	reg("tile/parallel-verify", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("tile/parallel-verify: requires (seeds-vec ops-vec)")
		}
		seedsV := args[0].(lisp.Vector)
		opsV := args[1].(lisp.Vector)

		seeds := make([]uint64, len(seedsV))
		for i, v := range seedsV {
			seeds[i] = uint64(v.(lisp.Int))
		}
		ops := make([]MoveOp, len(opsV))
		for i, v := range opsV {
			ops[i] = parseMoveOp(string(v.(lisp.Keyword)))
		}

		results, conserved := ParallelVerify(seeds, DefaultWork(ops))

		rvec := make(lisp.Vector, len(results))
		for i, r := range results {
			rvec[i] = lisp.HashMap{
				lisp.Keyword("seed"):     lisp.Int(int64(r.Seed)),
				lisp.Keyword("hex"):      lisp.String(r.Color.Hex()),
				lisp.Keyword("trit"):     lisp.Int(int64(r.Trit)),
				lisp.Keyword("winding"):  lisp.Int(int64(r.Winding)),
				lisp.Keyword("residue"):  lisp.Int(int64(r.Residue)),
				lisp.Keyword("delta-e"):  lisp.Float(r.DeltaE),
				lisp.Keyword("verified"): lisp.Bool(r.Verified),
				lisp.Keyword("report"):   lisp.String(r.Report),
			}
		}
		return lisp.HashMap{
			lisp.Keyword("results"):    rvec,
			lisp.Keyword("conserved?"): lisp.Bool(conserved),
			lisp.Keyword("count"):      lisp.Int(int64(len(results))),
		}
	})

	// tile/parallel-map — (seeds-vec) → vector of color hex strings
	reg("tile/parallel-map", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("tile/parallel-map: requires (seeds-vec)")
		}
		seedsV := args[0].(lisp.Vector)
		seeds := make([]uint64, len(seedsV))
		for i, v := range seedsV {
			seeds[i] = uint64(v.(lisp.Int))
		}
		colors := ParallelMap(seeds)
		result := make(lisp.Vector, len(colors))
		for i, c := range colors {
			result[i] = lisp.HashMap{
				lisp.Keyword("hex"):  lisp.String(c.Hex()),
				lisp.Keyword("trit"): lisp.Int(int64(c.Trit())),
			}
		}
		return result
	})

	// tile/find-triad — (start max-search) → balanced triad or nil
	reg("tile/find-triad", func(args []lisp.Value) lisp.Value {
		start := uint64(0)
		maxSearch := 10000
		if len(args) >= 1 {
			start = uint64(args[0].(lisp.Int))
		}
		if len(args) >= 2 {
			maxSearch = int(args[1].(lisp.Int))
		}
		seeds, found := FindBalancedTriad(start, maxSearch)
		if !found {
			return lisp.Nil{}
		}
		result := make(lisp.Vector, 3)
		for i, s := range seeds {
			ci := NewColorIdentity(s)
			result[i] = lisp.HashMap{
				lisp.Keyword("seed"): lisp.Int(int64(s)),
				lisp.Keyword("hex"):  lisp.String(ci.HexCode),
				lisp.Keyword("trit"): lisp.Int(int64(ci.Trit)),
			}
		}
		return result
	})

	// Silence unused imports
	_ = fmt.Sprint
	_ = lipgloss.NewStyle
}

func parseMoveOp(name string) MoveOp {
	switch name {
	case "move-to":
		return OpMoveTo
	case "move-from":
		return OpMoveFrom
	case "borrow-global":
		return OpBorrowGlobal
	case "borrow-global-mut":
		return OpBorrowGlobalMut
	case "mint":
		return OpMint
	case "burn":
		return OpBurn
	case "transfer":
		return OpTransfer
	case "set-flag":
		return OpSetFlag
	case "clear-flag":
		return OpClearFlag
	default:
		panic("tile/verifier-op: unknown op " + name)
	}
}
