//go:build darwin && arm64

package vm

import (
	"fmt"

	"github.com/Code-Hex/vz/v3"
)

func addRosettaDirectoryShare(vmConfig *vz.VirtualMachineConfiguration, cfg Config) error {
	if !cfg.EnableRosetta {
		return nil
	}
	if cfg.BootMode != "linux" {
		return fmt.Errorf("rosetta requires linux boot mode")
	}

	switch vz.LinuxRosettaDirectoryShareAvailability() {
	case vz.LinuxRosettaAvailabilityNotSupported:
		return fmt.Errorf("rosetta for linux is not supported on this host")
	case vz.LinuxRosettaAvailabilityNotInstalled:
		return fmt.Errorf("rosetta for linux is not installed (run `softwareupdate --install-rosetta --agree-to-license`)")
	}

	share, err := vz.NewLinuxRosettaDirectoryShare()
	if err != nil {
		return fmt.Errorf("failed to create rosetta directory share: %w", err)
	}

	tag := cfg.RosettaTag
	if tag == "" {
		tag = "rosetta"
	}
	fsDev, err := vz.NewVirtioFileSystemDeviceConfiguration(tag)
	if err != nil {
		return fmt.Errorf("failed to create rosetta virtiofs device: %w", err)
	}
	fsDev.SetDirectoryShare(share)
	vmConfig.SetDirectorySharingDevicesVirtualMachineConfiguration(
		[]vz.DirectorySharingDeviceConfiguration{fsDev},
	)
	return nil
}
