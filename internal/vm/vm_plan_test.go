//go:build darwin

package vm

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

type validCfg struct {
	BootMode string
	Kernel   string
	Initrd   string
	Cmdline  string
	ISO      string
	Disk     string
	Memory   int
	CPUs     int
	NVRAM    string
}

func (validCfg) Generate(r *rand.Rand, _ int) reflect.Value {
	modes := []string{"efi", "linux", "macos"}
	mode := modes[r.Intn(len(modes))]
	kernel := ""
	initrd := ""
	if mode == "linux" {
		kernel = "kernel.img"
		if r.Intn(2) == 0 {
			initrd = "initrd.img"
		}
	}
	iso := ""
	disk := ""
	if r.Intn(2) == 0 {
		iso = "boot.iso"
	}
	if r.Intn(2) == 0 {
		disk = "disk.img"
	}
	return reflect.ValueOf(validCfg{
		BootMode: mode,
		Kernel:   kernel,
		Initrd:   initrd,
		Cmdline:  "console=hvc0",
		ISO:      iso,
		Disk:     disk,
		Memory:   r.Intn(32) + 1,
		CPUs:     r.Intn(16) + 1,
		NVRAM:    "boxxy-nvram",
	})
}

type invalidCfg struct {
	BootMode string
	Kernel   string
	Memory   int
	CPUs     int
	NVRAM    string
}

func (invalidCfg) Generate(r *rand.Rand, _ int) reflect.Value {
	cases := []func() invalidCfg{
		func() invalidCfg { return invalidCfg{BootMode: "", Memory: 1, CPUs: 1, NVRAM: "boxxy-nvram"} },
		func() invalidCfg { return invalidCfg{BootMode: "unknown", Memory: 1, CPUs: 1, NVRAM: "boxxy-nvram"} },
		func() invalidCfg { return invalidCfg{BootMode: "linux", Kernel: "", Memory: 1, CPUs: 1, NVRAM: "boxxy-nvram"} },
		func() invalidCfg { return invalidCfg{BootMode: "efi", Memory: 1, CPUs: 1, NVRAM: ""} },
		func() invalidCfg { return invalidCfg{BootMode: "macos", Memory: 0, CPUs: 1, NVRAM: "boxxy-nvram"} },
		func() invalidCfg { return invalidCfg{BootMode: "macos", Memory: 1, CPUs: 0, NVRAM: "boxxy-nvram"} },
	}
	return reflect.ValueOf(cases[r.Intn(len(cases))]())
}

func TestValidateConfigAcceptsValid(t *testing.T) {
	prop := func(v validCfg) bool {
		cfg := Config{
			BootMode: v.BootMode,
			Kernel:   v.Kernel,
			Initrd:   v.Initrd,
			Cmdline:  v.Cmdline,
			ISO:      v.ISO,
			Disk:     v.Disk,
			Memory:   v.Memory,
			CPUs:     v.CPUs,
			NVRAM:    v.NVRAM,
		}
		return validateConfig(cfg) == nil
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestValidateConfigRejectsInvalid(t *testing.T) {
	prop := func(v invalidCfg) bool {
		cfg := Config{
			BootMode: v.BootMode,
			Kernel:   v.Kernel,
			Memory:   v.Memory,
			CPUs:     v.CPUs,
			NVRAM:    v.NVRAM,
		}
		return validateConfig(cfg) != nil
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Fatalf("invalid config accepted: %v", err)
	}
}

func TestStoragePlanInvariants(t *testing.T) {
	prop := func(hasISO, hasDisk bool) bool {
		cfg := Config{}
		if hasISO {
			cfg.ISO = "boot.iso"
		}
		if hasDisk {
			cfg.Disk = "disk.img"
		}
		plan := storagePlanFromConfig(cfg)
		return plan.HasISO == hasISO && plan.HasDisk == hasDisk
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Fatalf("storage plan invariant failed: %v", err)
	}
}

func TestNetworkPlanInvariants(t *testing.T) {
	prop := func(disable bool) bool {
		cfg := Config{DisableNetwork: disable}
		devices, err := networkDevicesFromConfig(cfg)
		if disable {
			return err == nil && len(devices) == 0
		}
		return err == nil && len(devices) == 1
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Fatalf("network plan invariant failed: %v", err)
	}
}

func TestPinholeModeUsesNAT(t *testing.T) {
	// PinholeMode=true should still create NAT networking (pf filters externally)
	cfg := Config{PinholeMode: true, DisableNetwork: false}
	devices, err := networkDevicesFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 network device with PinholeMode, got %d", len(devices))
	}
}

func TestPinholeModeHardenedOverrides(t *testing.T) {
	// DisableNetwork=true should override PinholeMode
	cfg := Config{PinholeMode: true, DisableNetwork: true}
	devices, err := networkDevicesFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("expected 0 network devices when DisableNetwork=true, got %d", len(devices))
	}
}

func TestConfigPinholePortsStored(t *testing.T) {
	cfg := Config{
		BootMode:     "macos",
		Memory:       4,
		CPUs:         2,
		PinholeMode:  true,
		PinholePorts: []int{8080, 9090, 4222},
	}
	if len(cfg.PinholePorts) != 3 {
		t.Fatalf("expected 3 pinhole ports, got %d", len(cfg.PinholePorts))
	}
	if cfg.PinholePorts[0] != 8080 {
		t.Errorf("PinholePorts[0] = %d, want 8080", cfg.PinholePorts[0])
	}
}
