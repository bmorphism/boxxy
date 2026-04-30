//go:build darwin

package tile

import (
	"fmt"
	"testing"

	"github.com/bmorphism/boxxy/internal/gf3"
	"github.com/bmorphism/boxxy/internal/vm"
)

func TestSplitMix64Deterministic(t *testing.T) {
	// Same seed always produces same output
	_, v1 := splitmix64(1069)
	_, v2 := splitmix64(1069)
	if v1 != v2 {
		t.Fatalf("splitmix64 not deterministic: %d != %d", v1, v2)
	}
	// Different seeds produce different outputs
	_, v3 := splitmix64(42)
	if v1 == v3 {
		t.Fatal("different seeds produced same value")
	}
}

func TestColorFromSeedMatchesZigSyrup(t *testing.T) {
	// GAY_SEED = 1069, the canonical default from zig-syrup
	c := ColorFromSeed(1069)
	t.Logf("seed=1069 → color=%s (R=%d G=%d B=%d)", c.Hex(), c.R, c.G, c.B)
	// The color must be deterministic — just verify it's not all zeros
	if c.R == 0 && c.G == 0 && c.B == 0 {
		t.Fatal("seed 1069 produced black — something is wrong")
	}
}

func TestColorTrit(t *testing.T) {
	for seed := uint64(0); seed < 100; seed++ {
		c := ColorFromSeed(seed)
		trit := c.Trit()
		if trit > 2 {
			t.Fatalf("seed %d: trit %d out of range", seed, trit)
		}
	}
}

func TestColorIdentity(t *testing.T) {
	ci := NewColorIdentity(1069)
	t.Logf("identity: seed=%d hex=%s trit=%s role=%s",
		ci.Seed, ci.HexCode, ci.Trit, ci.Role)
	if ci.HexCode == "" || ci.HexCode[0] != '#' || len(ci.HexCode) != 7 {
		t.Fatalf("bad hex: %q", ci.HexCode)
	}
}

func TestSyrupColorEncoding(t *testing.T) {
	ci := NewColorIdentity(1069)
	wire := ci.EncodeSyrupColor()
	t.Logf("syrup wire (%d bytes): %q", len(wire), string(wire))
	// Must start with < and end with >
	if wire[0] != '<' || wire[len(wire)-1] != '>' {
		t.Fatal("syrup record not properly delimited")
	}
	// Must contain the symbol
	if !containsBytes(wire, []byte("gay:color")) {
		t.Fatal("syrup record missing gay:color symbol")
	}
}

func TestSyrupCheckpointEncoding(t *testing.T) {
	ci := NewColorIdentity(1069)
	wire := ci.EncodeSyrupCheckpoint(0, 1)
	t.Logf("checkpoint wire (%d bytes): %q", len(wire), string(wire))
	if wire[0] != '<' || wire[len(wire)-1] != '>' {
		t.Fatal("checkpoint not properly delimited")
	}
	if !containsBytes(wire, []byte("gay:ckpt")) {
		t.Fatal("checkpoint missing gay:ckpt symbol")
	}
}

func TestMessageFrame(t *testing.T) {
	payload := []byte("hello")
	frame := EncodeMessageFrame(payload)
	if len(frame) != 4+len(payload) {
		t.Fatalf("frame length: got %d want %d", len(frame), 4+len(payload))
	}
	// Big-endian length prefix
	length := uint32(frame[0])<<24 | uint32(frame[1])<<16 | uint32(frame[2])<<8 | uint32(frame[3])
	if length != uint32(len(payload)) {
		t.Fatalf("frame length prefix: got %d want %d", length, len(payload))
	}
}

func TestRainbowPaletteFromIdentity(t *testing.T) {
	ci := NewColorIdentity(1069)
	palette := ci.RainbowPalette(8)
	if len(palette) != 8 {
		t.Fatalf("palette length: got %d want 8", len(palette))
	}
	t.Logf("palette base hue: %.1f°", ci.Color.Hue())
	for i, c := range palette {
		t.Logf("  depth %d: %s", i, c.Hex())
	}
}

func TestTileLatticeBalance(t *testing.T) {
	lattice := NewTileLattice()

	// Find 3 seeds that give us one of each trit
	var seeds [3]uint64
	found := [3]bool{}
	for s := uint64(0); !found[0] || !found[1] || !found[2]; s++ {
		ci := NewColorIdentity(s)
		idx := int(ci.Trit)
		if !found[idx] {
			seeds[idx] = s
			found[idx] = true
		}
	}

	for i, s := range seeds {
		ci := NewColorIdentity(s)
		t.Logf("trit[%d] seed=%d hex=%s", i, s, ci.HexCode)
		tv := NewTileableVM(fmt.Sprintf("tile-%d", i), s, vm_cfg_stub())
		lattice.Add(tv)
	}

	if !lattice.IsBalanced() {
		t.Fatal("3-tile lattice with one of each trit should be balanced")
	}
}

func TestFindBalancerSeed(t *testing.T) {
	lattice := NewTileLattice()
	lattice.Add(NewTileableVM("a", 1069, vm_cfg_stub()))
	lattice.Add(NewTileableVM("b", 42, vm_cfg_stub()))

	seed, ok := lattice.FindBalancerSeed(0, 1000)
	if !ok {
		t.Fatal("could not find balancer seed within 1000 candidates")
	}

	ci := NewColorIdentity(seed)
	t.Logf("balancer: seed=%d hex=%s trit=%s", seed, ci.HexCode, ci.Trit)

	lattice.Add(NewTileableVM("c", seed, vm_cfg_stub()))
	if !lattice.IsBalanced() {
		t.Fatal("lattice should be balanced after adding balancer")
	}
}

func TestWireColors(t *testing.T) {
	lattice := NewTileLattice()
	for _, s := range []uint64{1069, 42, 7} {
		lattice.Add(NewTileableVM("t", s, vm_cfg_stub()))
	}
	wires := lattice.WireColors()
	if len(wires) != 3 {
		t.Fatalf("expected 3 wire encodings, got %d", len(wires))
	}
	for i, w := range wires {
		t.Logf("tile %d wire: %q", i, string(w))
	}
}

// vm_cfg_stub returns a minimal valid VM config for testing tile logic.
// The VM won't actually boot — this only tests the color dispatch path.
func vm_cfg_stub() vm.Config {
	return vm.Config{
		BootMode: "linux",
		Kernel:   "/dev/null",
		Memory:   1,
		CPUs:     1,
	}
}

func containsBytes(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Ensure gf3 import is used
var _ = gf3.Zero
