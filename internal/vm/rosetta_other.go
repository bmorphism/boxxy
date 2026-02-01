//go:build darwin && !arm64

package vm

import (
	"fmt"

	"github.com/Code-Hex/vz/v3"
)

func addRosettaDirectoryShare(vmConfig *vz.VirtualMachineConfiguration, cfg Config) error {
	if !cfg.EnableRosetta {
		return nil
	}
	return fmt.Errorf("rosetta for linux is only available on apple silicon hosts")
}
