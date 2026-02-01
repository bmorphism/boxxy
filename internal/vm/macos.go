//go:build darwin

package vm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/bmorphism/boxxy/internal/lisp"
)

// MacOSVMPaths holds paths for macOS VM persistent state
type MacOSVMPaths struct {
	BaseDir          string
	DiskImage        string
	AuxiliaryStorage string
	HardwareModel    string
	MachineID        string
	RestoreImage     string
}

// DefaultMacOSPaths returns default paths for a named macOS VM
func DefaultMacOSPaths(name string) MacOSVMPaths {
	baseDir := filepath.Join(os.Getenv("HOME"), ".boxxy", "macos", name)
	return MacOSVMPaths{
		BaseDir:          baseDir,
		DiskImage:        filepath.Join(baseDir, "disk.img"),
		AuxiliaryStorage: filepath.Join(baseDir, "auxiliary.bin"),
		HardwareModel:    filepath.Join(baseDir, "hardware_model.bin"),
		MachineID:        filepath.Join(baseDir, "machine_id.bin"),
		RestoreImage:     filepath.Join(baseDir, "restore.ipsw"),
	}
}

// MacOSVMConfig holds macOS-specific VM configuration
type MacOSVMConfig struct {
	Name     string
	CPUs     int
	MemoryGB int
	DiskGB   int
	Paths    MacOSVMPaths
}

// DownloadLatestIPSW downloads the latest macOS restore image
func DownloadLatestIPSW(ctx context.Context, destPath string, progress func(float64)) error {
	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Println("Downloading macOS restore image...")
	reader, err := vz.FetchLatestSupportedMacOSRestoreImage(ctx, destPath)
	if err != nil {
		return fmt.Errorf("failed to download restore image: %w", err)
	}

	if progress != nil {
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-reader.Finished():
					progress(reader.FractionCompleted())
					return
				case <-ticker.C:
					progress(reader.FractionCompleted())
				}
			}
		}()
	}

	<-reader.Finished()
	if err := reader.Err(); err != nil {
		return fmt.Errorf("restore image download failed: %w", err)
	}

	return nil
}

// CreateMacOSVM creates a new macOS VM with installation
func CreateMacOSVM(ctx context.Context, cfg MacOSVMConfig) (*VMInstance, error) {
	paths := cfg.Paths

	// Ensure base directory exists
	if err := os.MkdirAll(paths.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create VM directory: %w", err)
	}

	// Download IPSW if not present
	if _, err := os.Stat(paths.RestoreImage); os.IsNotExist(err) {
		fmt.Println("Downloading macOS restore image...")
		if err := DownloadLatestIPSW(ctx, paths.RestoreImage, func(p float64) {
			fmt.Printf("\rDownload progress: %.1f%%", p*100)
		}); err != nil {
			return nil, err
		}
		fmt.Println()
	}

	// Load restore image
	restoreImage, err := vz.LoadMacOSRestoreImageFromPath(paths.RestoreImage)
	if err != nil {
		return nil, fmt.Errorf("failed to load restore image: %w", err)
	}

	// Get configuration requirements
	macOSConfig := restoreImage.MostFeaturefulSupportedConfiguration()
	if macOSConfig == nil {
		return nil, fmt.Errorf("no supported configuration found for this host")
	}

	// Create hardware model
	hardwareModel := macOSConfig.HardwareModel()

	// Save hardware model
	if err := os.WriteFile(paths.HardwareModel, hardwareModel.DataRepresentation(), 0644); err != nil {
		return nil, fmt.Errorf("failed to save hardware model: %w", err)
	}

	// Create machine identifier
	machineID, err := vz.NewMacMachineIdentifier()
	if err != nil {
		return nil, fmt.Errorf("failed to create machine identifier: %w", err)
	}

	// Save machine identifier
	if err := os.WriteFile(paths.MachineID, machineID.DataRepresentation(), 0644); err != nil {
		return nil, fmt.Errorf("failed to save machine identifier: %w", err)
	}

	// Create auxiliary storage
	auxStorage, err := vz.NewMacAuxiliaryStorage(paths.AuxiliaryStorage,
		vz.WithCreatingMacAuxiliaryStorage(hardwareModel))
	if err != nil {
		return nil, fmt.Errorf("failed to create auxiliary storage: %w", err)
	}

	// Create platform configuration
	platformConfig, err := vz.NewMacPlatformConfiguration(
		vz.WithMacAuxiliaryStorage(auxStorage),
		vz.WithMacHardwareModel(hardwareModel),
		vz.WithMacMachineIdentifier(machineID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create platform configuration: %w", err)
	}

	// Create boot loader
	bootLoader, err := vz.NewMacOSBootLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create boot loader: %w", err)
	}

	// Create VM configuration
	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(cfg.CPUs),
		uint64(cfg.MemoryGB)*1024*1024*1024,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM configuration: %w", err)
	}

	// Set platform
	vmConfig.SetPlatformVirtualMachineConfiguration(platformConfig)

	// Create disk image if not exists
	if _, err := os.Stat(paths.DiskImage); os.IsNotExist(err) {
		f, err := os.Create(paths.DiskImage)
		if err != nil {
			return nil, fmt.Errorf("failed to create disk image: %w", err)
		}
		size := int64(cfg.DiskGB) * 1024 * 1024 * 1024
		if err := f.Truncate(size); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to resize disk: %w", err)
		}
		f.Close()
	}

	// Add disk
	diskAtt, err := vz.NewDiskImageStorageDeviceAttachment(paths.DiskImage, false)
	if err != nil {
		return nil, fmt.Errorf("failed to attach disk: %w", err)
	}
	diskDev, err := vz.NewVirtioBlockDeviceConfiguration(diskAtt)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk device: %w", err)
	}
	vmConfig.SetStorageDevicesVirtualMachineConfiguration([]vz.StorageDeviceConfiguration{diskDev})

	// Add network
	natAtt, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		return nil, fmt.Errorf("failed to create NAT: %w", err)
	}
	netDev, err := vz.NewVirtioNetworkDeviceConfiguration(natAtt)
	if err != nil {
		return nil, fmt.Errorf("failed to create network device: %w", err)
	}
	vmConfig.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{netDev})

	// Add graphics
	graphicsDev, err := vz.NewMacGraphicsDeviceConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create graphics device: %w", err)
	}
	display, err := vz.NewMacGraphicsDisplayConfiguration(1920, 1200, 144)
	if err != nil {
		return nil, fmt.Errorf("failed to create display: %w", err)
	}
	graphicsDev.SetDisplays(display)
	vmConfig.SetGraphicsDevicesVirtualMachineConfiguration([]vz.GraphicsDeviceConfiguration{graphicsDev})

	// Add pointing device
	pointingDev, err := vz.NewMacTrackpadConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create trackpad: %w", err)
	}
	vmConfig.SetPointingDevicesVirtualMachineConfiguration([]vz.PointingDeviceConfiguration{pointingDev})

	// Add keyboard
	keyboardDev, err := vz.NewMacKeyboardConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create keyboard: %w", err)
	}
	vmConfig.SetKeyboardsVirtualMachineConfiguration([]vz.KeyboardConfiguration{keyboardDev})

	// Validate
	ok, err := vmConfig.Validate()
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("configuration validation returned false")
	}

	// Create VM
	vm, err := vz.NewVirtualMachine(vmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	return &VMInstance{
		VM:       vm,
		Config:   vmConfig,
		shutdown: make(chan struct{}),
	}, nil
}

// InstallMacOS runs the macOS installer on the VM
func InstallMacOS(ctx context.Context, vm *vz.VirtualMachine, ipswPath string) error {
	installer, err := vz.NewMacOSInstaller(vm, ipswPath)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	fmt.Println("Starting macOS installation...")

	// Start installation
	if err := installer.Install(ctx); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Monitor progress
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			progress := installer.FractionCompleted()
			fmt.Printf("\rInstallation progress: %.1f%%", progress*100)
			if progress >= 1.0 {
				fmt.Println("\nInstallation complete!")
				return nil
			}
		}
	}
}

// LoadMacOSVM loads an existing macOS VM from saved state
func LoadMacOSVM(cfg MacOSVMConfig) (*VMInstance, error) {
	paths := cfg.Paths

	// Load hardware model
	hardwareModelData, err := os.ReadFile(paths.HardwareModel)
	if err != nil {
		return nil, fmt.Errorf("failed to read hardware model: %w", err)
	}
	hardwareModel, err := vz.NewMacHardwareModelWithData(hardwareModelData)
	if err != nil {
		return nil, fmt.Errorf("failed to create hardware model: %w", err)
	}

	// Load machine identifier
	machineIDData, err := os.ReadFile(paths.MachineID)
	if err != nil {
		return nil, fmt.Errorf("failed to read machine identifier: %w", err)
	}
	machineID, err := vz.NewMacMachineIdentifierWithData(machineIDData)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine identifier: %w", err)
	}

	// Load auxiliary storage
	auxStorage, err := vz.NewMacAuxiliaryStorage(paths.AuxiliaryStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to load auxiliary storage: %w", err)
	}

	// Create platform configuration
	platformConfig, err := vz.NewMacPlatformConfiguration(
		vz.WithMacAuxiliaryStorage(auxStorage),
		vz.WithMacHardwareModel(hardwareModel),
		vz.WithMacMachineIdentifier(machineID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create platform configuration: %w", err)
	}

	// Create boot loader
	bootLoader, err := vz.NewMacOSBootLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create boot loader: %w", err)
	}

	// Create VM configuration
	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(cfg.CPUs),
		uint64(cfg.MemoryGB)*1024*1024*1024,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM configuration: %w", err)
	}

	vmConfig.SetPlatformVirtualMachineConfiguration(platformConfig)

	// Add disk
	diskAtt, err := vz.NewDiskImageStorageDeviceAttachment(paths.DiskImage, false)
	if err != nil {
		return nil, fmt.Errorf("failed to attach disk: %w", err)
	}
	diskDev, err := vz.NewVirtioBlockDeviceConfiguration(diskAtt)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk device: %w", err)
	}
	vmConfig.SetStorageDevicesVirtualMachineConfiguration([]vz.StorageDeviceConfiguration{diskDev})

	// Add network
	natAtt, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		return nil, fmt.Errorf("failed to create NAT: %w", err)
	}
	netDev, err := vz.NewVirtioNetworkDeviceConfiguration(natAtt)
	if err != nil {
		return nil, fmt.Errorf("failed to create network device: %w", err)
	}
	vmConfig.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{netDev})

	// Add graphics
	graphicsDev, err := vz.NewMacGraphicsDeviceConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create graphics device: %w", err)
	}
	display, err := vz.NewMacGraphicsDisplayConfiguration(1920, 1200, 144)
	if err != nil {
		return nil, fmt.Errorf("failed to create display: %w", err)
	}
	graphicsDev.SetDisplays(display)
	vmConfig.SetGraphicsDevicesVirtualMachineConfiguration([]vz.GraphicsDeviceConfiguration{graphicsDev})

	// Add pointing device
	pointingDev, err := vz.NewMacTrackpadConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create trackpad: %w", err)
	}
	vmConfig.SetPointingDevicesVirtualMachineConfiguration([]vz.PointingDeviceConfiguration{pointingDev})

	// Add keyboard
	keyboardDev, err := vz.NewMacKeyboardConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create keyboard: %w", err)
	}
	vmConfig.SetKeyboardsVirtualMachineConfiguration([]vz.KeyboardConfiguration{keyboardDev})

	// Validate
	ok, err := vmConfig.Validate()
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("configuration validation returned false")
	}

	// Create VM
	vm, err := vz.NewVirtualMachine(vmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	return &VMInstance{
		VM:       vm,
		Config:   vmConfig,
		shutdown: make(chan struct{}),
	}, nil
}

// RegisterMacOSLispFunctions registers macOS-specific vz functions
func RegisterMacOSLispFunctions(env *lisp.Env) {
	// vz/new-mac-platform - Create macOS platform configuration
	env.Set("vz/new-mac-platform", &lisp.Fn{"vz/new-mac-platform", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("vz/new-mac-platform requires hardware-model-path, machine-id-path, aux-storage-path")
		}
		hwPath := string(args[0].(lisp.String))
		machineIDPath := string(args[1].(lisp.String))
		auxPath := string(args[2].(lisp.String))

		// Load hardware model
		hwData, err := os.ReadFile(hwPath)
		if err != nil {
			panic(fmt.Sprintf("failed to read hardware model: %v", err))
		}
		hw, err := vz.NewMacHardwareModelWithData(hwData)
		if err != nil {
			panic(fmt.Sprintf("failed to create hardware model: %v", err))
		}

		// Load machine ID
		midData, err := os.ReadFile(machineIDPath)
		if err != nil {
			panic(fmt.Sprintf("failed to read machine ID: %v", err))
		}
		mid, err := vz.NewMacMachineIdentifierWithData(midData)
		if err != nil {
			panic(fmt.Sprintf("failed to create machine ID: %v", err))
		}

		// Load aux storage
		aux, err := vz.NewMacAuxiliaryStorage(auxPath)
		if err != nil {
			panic(fmt.Sprintf("failed to load aux storage: %v", err))
		}

		// Create platform
		platform, err := vz.NewMacPlatformConfiguration(
			vz.WithMacAuxiliaryStorage(aux),
			vz.WithMacHardwareModel(hw),
			vz.WithMacMachineIdentifier(mid),
		)
		if err != nil {
			panic(fmt.Sprintf("failed to create Mac platform: %v", err))
		}

		return &lisp.ExternalValue{Value: platform, Type: "MacPlatform"}
	}})

	// vz/download-macos-ipsw - Download latest macOS IPSW
	env.Set("vz/download-macos-ipsw", &lisp.Fn{"vz/download-macos-ipsw", func(args []lisp.Value) lisp.Value {
		if len(args) < 1 {
			panic("vz/download-macos-ipsw requires destination path")
		}
		destPath := string(args[0].(lisp.String))

		ctx := context.Background()
		err := DownloadLatestIPSW(ctx, destPath, func(p float64) {
			fmt.Printf("\rDownload: %.1f%%", p*100)
		})
		if err != nil {
			panic(fmt.Sprintf("download failed: %v", err))
		}
		fmt.Println()
		return lisp.Bool(true)
	}})

	// vz/create-macos-vm - Create a complete macOS VM
	env.Set("vz/create-macos-vm", &lisp.Fn{"vz/create-macos-vm", func(args []lisp.Value) lisp.Value {
		if len(args) < 4 {
			panic("vz/create-macos-vm requires name, cpus, memory-gb, disk-gb")
		}
		name := string(args[0].(lisp.String))
		cpus := int(args[1].(lisp.Int))
		memGB := int(args[2].(lisp.Int))
		diskGB := int(args[3].(lisp.Int))

		cfg := MacOSVMConfig{
			Name:     name,
			CPUs:     cpus,
			MemoryGB: memGB,
			DiskGB:   diskGB,
			Paths:    DefaultMacOSPaths(name),
		}

		ctx := context.Background()
		instance, err := CreateMacOSVM(ctx, cfg)
		if err != nil {
			panic(fmt.Sprintf("failed to create macOS VM: %v", err))
		}

		return &lisp.ExternalValue{Value: instance, Type: "MacOSVM"}
	}})

	// vz/load-macos-vm - Load existing macOS VM
	env.Set("vz/load-macos-vm", &lisp.Fn{"vz/load-macos-vm", func(args []lisp.Value) lisp.Value {
		if len(args) < 3 {
			panic("vz/load-macos-vm requires name, cpus, memory-gb")
		}
		name := string(args[0].(lisp.String))
		cpus := int(args[1].(lisp.Int))
		memGB := int(args[2].(lisp.Int))

		cfg := MacOSVMConfig{
			Name:     name,
			CPUs:     cpus,
			MemoryGB: memGB,
			Paths:    DefaultMacOSPaths(name),
		}

		instance, err := LoadMacOSVM(cfg)
		if err != nil {
			panic(fmt.Sprintf("failed to load macOS VM: %v", err))
		}

		return &lisp.ExternalValue{Value: instance, Type: "MacOSVM"}
	}})

	// vz/install-macos - Run macOS installer
	env.Set("vz/install-macos", &lisp.Fn{"vz/install-macos", func(args []lisp.Value) lisp.Value {
		if len(args) < 2 {
			panic("vz/install-macos requires vm and ipsw-path")
		}
		instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)
		ipswPath := string(args[1].(lisp.String))

		ctx := context.Background()
		if err := InstallMacOS(ctx, instance.VM, ipswPath); err != nil {
			panic(fmt.Sprintf("installation failed: %v", err))
		}

		return lisp.Bool(true)
	}})

	// vz/new-mac-graphics - Create macOS graphics device
	env.Set("vz/new-mac-graphics", &lisp.Fn{"vz/new-mac-graphics", func(args []lisp.Value) lisp.Value {
		width := int64(1920)
		height := int64(1200)
		ppi := int64(144)

		if len(args) > 0 {
			width = int64(args[0].(lisp.Int))
		}
		if len(args) > 1 {
			height = int64(args[1].(lisp.Int))
		}
		if len(args) > 2 {
			ppi = int64(args[2].(lisp.Int))
		}

		graphicsDev, err := vz.NewMacGraphicsDeviceConfiguration()
		if err != nil {
			panic(fmt.Sprintf("failed to create graphics device: %v", err))
		}

		display, err := vz.NewMacGraphicsDisplayConfiguration(width, height, ppi)
		if err != nil {
			panic(fmt.Sprintf("failed to create display: %v", err))
		}
		graphicsDev.SetDisplays(display)

		return &lisp.ExternalValue{Value: graphicsDev, Type: "MacGraphics"}
	}})
}
