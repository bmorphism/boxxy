//go:build darwin && arm64

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Code-Hex/vz/v3"
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

func DefaultMacOSPaths(name string) MacOSVMPaths {
	base := filepath.Join(os.Getenv("HOME"), ".boxxy", "macos", name)
	return MacOSVMPaths{
		BaseDir:          base,
		DiskImage:        filepath.Join(base, "disk.img"),
		AuxiliaryStorage: filepath.Join(base, "auxiliary.bin"),
		HardwareModel:    filepath.Join(base, "hardware_model.bin"),
		MachineID:        filepath.Join(base, "machine_id.bin"),
		RestoreImage:     filepath.Join(base, "restore.ipsw"),
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
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	fmt.Println("Downloading macOS restore image...")
	reader, err := vz.FetchLatestSupportedMacOSRestoreImage(ctx, destPath)
	if err != nil {
		fmt.Printf("VZ catalog unavailable: %v\nFalling back to ipsw.me...\n", err)
		return downloadIPSWDirect(ctx, destPath)
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
		fmt.Printf("VZ download failed: %v\nFalling back to ipsw.me...\n", err)
		return downloadIPSWDirect(ctx, destPath)
	}
	return nil
}

func downloadIPSWDirect(ctx context.Context, destPath string) error {
	ipswURL, err := queryIPSWURL(ctx)
	if err != nil {
		return fmt.Errorf("IPSW URL lookup failed: %w\n\nManual download:\n  1. Visit https://ipsw.me/product/Mac\n  2. Download latest IPSW\n  3. Run: boxxy macos up --ipsw /path/to/file.ipsw", err)
	}

	fmt.Printf("Downloading: %s\n(~15 GB, may take a while)\n", ipswURL)
	cmd := exec.CommandContext(ctx, "curl", "-L", "-C", "-", "-o", destPath, "--progress-bar", ipswURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("file not found after download: %w", err)
	}
	if info.Size() < 100*1024*1024 {
		os.Remove(destPath)
		return fmt.Errorf("file too small (%d bytes), likely error page", info.Size())
	}
	fmt.Printf("Downloaded: %.1f GB\n", float64(info.Size())/1e9)
	return nil
}

func queryIPSWURL(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.model").Output()
	if err != nil {
		return "", fmt.Errorf("cannot determine model: %w", err)
	}
	model := strings.TrimSpace(string(out))

	apiURL := fmt.Sprintf("https://api.ipsw.me/v4/device/%s?type=ipsw", model)
	body, err := exec.CommandContext(ctx, "curl", "-sL", apiURL).Output()
	if err != nil {
		return "", fmt.Errorf("ipsw.me request failed: %w", err)
	}

	type firmware struct {
		URL    string `json:"url"`
		Signed bool   `json:"signed"`
	}
	type device struct {
		Firmwares []firmware `json:"firmwares"`
	}

	var dev device
	if err := json.Unmarshal(body, &dev); err != nil {
		return "", fmt.Errorf("parse ipsw.me: %w", err)
	}
	for _, fw := range dev.Firmwares {
		if fw.Signed && fw.URL != "" {
			return fw.URL, nil
		}
	}
	return "", fmt.Errorf("no signed IPSW for %s", model)
}

// CreateMacOSVM creates a new macOS VM with platform persistence
func CreateMacOSVM(ctx context.Context, cfg MacOSVMConfig) (*VMInstance, error) {
	paths := cfg.Paths
	if err := os.MkdirAll(paths.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	// Download IPSW if needed
	if _, err := os.Stat(paths.RestoreImage); os.IsNotExist(err) {
		if err := DownloadLatestIPSW(ctx, paths.RestoreImage, func(p float64) {
			fmt.Printf("\rDownload: %.1f%%", p*100)
		}); err != nil {
			return nil, err
		}
		fmt.Println()
	}

	restoreImage, err := vz.LoadMacOSRestoreImageFromPath(paths.RestoreImage)
	if err != nil {
		return nil, fmt.Errorf("load IPSW: %w", err)
	}

	macOSConfig := restoreImage.MostFeaturefulSupportedConfiguration()
	if macOSConfig == nil {
		return nil, fmt.Errorf("no supported configuration for this host")
	}

	hardwareModel := macOSConfig.HardwareModel()
	if err := os.WriteFile(paths.HardwareModel, hardwareModel.DataRepresentation(), 0644); err != nil {
		return nil, fmt.Errorf("save hardware model: %w", err)
	}

	machineID, err := vz.NewMacMachineIdentifier()
	if err != nil {
		return nil, fmt.Errorf("create machine ID: %w", err)
	}
	if err := os.WriteFile(paths.MachineID, machineID.DataRepresentation(), 0644); err != nil {
		return nil, fmt.Errorf("save machine ID: %w", err)
	}

	auxStorage, err := vz.NewMacAuxiliaryStorage(paths.AuxiliaryStorage,
		vz.WithCreatingMacAuxiliaryStorage(hardwareModel))
	if err != nil {
		return nil, fmt.Errorf("aux storage: %w", err)
	}

	platformConfig, err := vz.NewMacPlatformConfiguration(
		vz.WithMacAuxiliaryStorage(auxStorage),
		vz.WithMacHardwareModel(hardwareModel),
		vz.WithMacMachineIdentifier(machineID),
	)
	if err != nil {
		return nil, fmt.Errorf("platform config: %w", err)
	}

	return buildMacOSVM(cfg, platformConfig)
}

// LoadMacOSVM loads an existing macOS VM from saved state
func LoadMacOSVM(cfg MacOSVMConfig) (*VMInstance, error) {
	paths := cfg.Paths

	hwData, err := os.ReadFile(paths.HardwareModel)
	if err != nil {
		return nil, fmt.Errorf("read hardware model: %w", err)
	}
	hw, err := vz.NewMacHardwareModelWithData(hwData)
	if err != nil {
		return nil, fmt.Errorf("parse hardware model: %w", err)
	}

	midData, err := os.ReadFile(paths.MachineID)
	if err != nil {
		return nil, fmt.Errorf("read machine ID: %w", err)
	}
	mid, err := vz.NewMacMachineIdentifierWithData(midData)
	if err != nil {
		return nil, fmt.Errorf("parse machine ID: %w", err)
	}

	aux, err := vz.NewMacAuxiliaryStorage(paths.AuxiliaryStorage)
	if err != nil {
		return nil, fmt.Errorf("load aux storage: %w", err)
	}

	platformConfig, err := vz.NewMacPlatformConfiguration(
		vz.WithMacAuxiliaryStorage(aux),
		vz.WithMacHardwareModel(hw),
		vz.WithMacMachineIdentifier(mid),
	)
	if err != nil {
		return nil, fmt.Errorf("platform config: %w", err)
	}

	return buildMacOSVM(cfg, platformConfig)
}

func buildMacOSVM(cfg MacOSVMConfig, platform *vz.MacPlatformConfiguration) (*VMInstance, error) {
	paths := cfg.Paths

	bootLoader, err := vz.NewMacOSBootLoader()
	if err != nil {
		return nil, fmt.Errorf("boot loader: %w", err)
	}

	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(cfg.CPUs),
		uint64(cfg.MemoryGB)*1024*1024*1024,
	)
	if err != nil {
		return nil, fmt.Errorf("vm config: %w", err)
	}
	vmConfig.SetPlatformVirtualMachineConfiguration(platform)

	// Disk
	if _, err := os.Stat(paths.DiskImage); os.IsNotExist(err) {
		f, err := os.Create(paths.DiskImage)
		if err != nil {
			return nil, fmt.Errorf("create disk: %w", err)
		}
		if err := f.Truncate(int64(cfg.DiskGB) * 1024 * 1024 * 1024); err != nil {
			f.Close()
			return nil, fmt.Errorf("resize disk: %w", err)
		}
		f.Close()
	}

	diskAtt, err := vz.NewDiskImageStorageDeviceAttachment(paths.DiskImage, false)
	if err != nil {
		return nil, fmt.Errorf("attach disk: %w", err)
	}
	diskDev, err := vz.NewVirtioBlockDeviceConfiguration(diskAtt)
	if err != nil {
		return nil, fmt.Errorf("disk device: %w", err)
	}
	vmConfig.SetStorageDevicesVirtualMachineConfiguration([]vz.StorageDeviceConfiguration{diskDev})

	// Network
	natAtt, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		return nil, fmt.Errorf("NAT: %w", err)
	}
	netDev, err := vz.NewVirtioNetworkDeviceConfiguration(natAtt)
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}
	vmConfig.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{netDev})

	// Graphics
	gfx, err := vz.NewMacGraphicsDeviceConfiguration()
	if err != nil {
		return nil, fmt.Errorf("graphics: %w", err)
	}
	display, err := vz.NewMacGraphicsDisplayConfiguration(1920, 1200, 144)
	if err != nil {
		return nil, fmt.Errorf("display: %w", err)
	}
	gfx.SetDisplays(display)
	vmConfig.SetGraphicsDevicesVirtualMachineConfiguration([]vz.GraphicsDeviceConfiguration{gfx})

	// Input
	trackpad, err := vz.NewMacTrackpadConfiguration()
	if err != nil {
		return nil, fmt.Errorf("trackpad: %w", err)
	}
	vmConfig.SetPointingDevicesVirtualMachineConfiguration([]vz.PointingDeviceConfiguration{trackpad})

	keyboard, err := vz.NewMacKeyboardConfiguration()
	if err != nil {
		return nil, fmt.Errorf("keyboard: %w", err)
	}
	vmConfig.SetKeyboardsVirtualMachineConfiguration([]vz.KeyboardConfiguration{keyboard})

	// Validate
	ok, err := vmConfig.Validate()
	if err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("config validation returned false")
	}

	vm, err := vz.NewVirtualMachine(vmConfig)
	if err != nil {
		return nil, fmt.Errorf("create vm: %w", err)
	}

	return &VMInstance{
		VM:       vm,
		Config:   vmConfig,
		shutdown: make(chan struct{}),
	}, nil
}

// InstallMacOS runs the macOS installer
func InstallMacOS(ctx context.Context, vm *vz.VirtualMachine, ipswPath string) error {
	installer, err := vz.NewMacOSInstaller(vm, ipswPath)
	if err != nil {
		return fmt.Errorf("create installer: %w", err)
	}

	fmt.Println("Installing macOS...")
	if err := installer.Install(ctx); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p := installer.FractionCompleted()
			fmt.Printf("\rInstall: %.1f%%", p*100)
			if p >= 1.0 {
				fmt.Println("\nInstallation complete!")
				return nil
			}
		}
	}
}
