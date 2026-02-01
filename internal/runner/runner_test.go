//go:build darwin

package runner

import (
	"math/rand"
	"testing"
)

func TestParseRunArgsRequiresMode(t *testing.T) {
	_, err := parseRunArgs([]string{})
	if err == nil {
		t.Fatalf("expected error when no boot mode provided")
	}
}

func TestParseRunArgsGuixRequiresInput(t *testing.T) {
	_, err := parseRunArgs([]string{"--guix"})
	if err == nil {
		t.Fatalf("expected error for guix without iso or kernel/initrd")
	}
}

func TestParseRunArgsGuixArchEnablesRosetta(t *testing.T) {
	cfg, err := parseRunArgs([]string{"--guix", "--iso", "x.iso", "--guix-arch", "x86_64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.EnableRosetta {
		t.Fatalf("expected rosetta enabled for x86_64")
	}
}

func TestParseRunArgsModes(t *testing.T) {
	cases := []struct {
		args     []string
		mode     string
		wantErr  bool
		msg      string
	}{
		{[]string{"--efi", "--iso", "x.iso"}, "efi", false, "efi"},
		{[]string{"--linux", "--kernel", "vmlinuz"}, "linux", false, "linux"},
		{[]string{"--macos"}, "macos", false, "macos"},
		{[]string{"--guix", "--kernel", "vmlinuz"}, "linux", false, "guix kernel"},
		{[]string{"--guix", "--iso", "x.iso"}, "efi", false, "guix iso"},
	}
	for _, c := range cases {
		cfg, err := parseRunArgs(c.args)
		if c.wantErr {
			if err == nil {
				t.Fatalf("expected error for %s", c.msg)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", c.msg, err)
		}
		if cfg.BootMode != c.mode {
			t.Fatalf("%s: expected mode %s, got %s", c.msg, c.mode, cfg.BootMode)
		}
	}
}

func TestParseRunArgsStress(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	modes := [][]string{{"--efi"}, {"--linux"}, {"--macos"}, {"--guix"}, {}}
	for i := 0; i < 2000; i++ {
		args := []string{}
		args = append(args, modes[rng.Intn(len(modes))]...)
		if rng.Intn(2) == 0 {
			args = append(args, "--iso", "x.iso")
		}
		if rng.Intn(2) == 0 {
			args = append(args, "--kernel", "vmlinuz")
		}
		if rng.Intn(2) == 0 {
			args = append(args, "--initrd", "initrd.img")
		}
		if rng.Intn(2) == 0 {
			args = append(args, "--disk", "disk.img")
		}
		if rng.Intn(2) == 0 {
			args = append(args, "--guix-arch", "x86_64")
		}
		_, _ = parseRunArgs(args)
	}
}
