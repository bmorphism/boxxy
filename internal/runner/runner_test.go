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

func TestParseRunArgsPinholeFlag(t *testing.T) {
	cfg, err := parseRunArgs([]string{"--efi", "--iso", "x.iso", "--pinhole"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.PinholeMode {
		t.Fatal("expected PinholeMode=true with --pinhole flag")
	}
}

func TestParseRunArgsPinholeDefaultFalse(t *testing.T) {
	cfg, err := parseRunArgs([]string{"--efi", "--iso", "x.iso"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PinholeMode {
		t.Fatal("expected PinholeMode=false without --pinhole flag")
	}
}

func TestParseRunArgsPinholeWithHardened(t *testing.T) {
	// Both flags can be set — hardened disables networking entirely,
	// pinhole is advisory (pf applied externally). No conflict at parse time.
	cfg, err := parseRunArgs([]string{"--efi", "--iso", "x.iso", "--pinhole", "--hardened"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.PinholeMode {
		t.Fatal("expected PinholeMode=true")
	}
	if !cfg.DisableNetwork {
		t.Fatal("expected DisableNetwork=true")
	}
}

func TestParseRunArgsDefaults(t *testing.T) {
	cfg, err := parseRunArgs([]string{"--efi", "--iso", "x.iso"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Memory != 4 {
		t.Errorf("default memory = %d, want 4", cfg.Memory)
	}
	if cfg.CPUs != 2 {
		t.Errorf("default cpus = %d, want 2", cfg.CPUs)
	}
	if cfg.NVRAM != "boxxy-nvram" {
		t.Errorf("default nvram = %q, want boxxy-nvram", cfg.NVRAM)
	}
}

func TestParseRunArgsCustomResources(t *testing.T) {
	cfg, err := parseRunArgs([]string{"--efi", "--iso", "x.iso", "--memory", "8", "--cpus", "4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Memory != 8 {
		t.Errorf("memory = %d, want 8", cfg.Memory)
	}
	if cfg.CPUs != 4 {
		t.Errorf("cpus = %d, want 4", cfg.CPUs)
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
