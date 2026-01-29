//go:build darwin

package vm

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/Code-Hex/vz/v3"
	"github.com/bmorphism/boxxy/internal/lisp"
)

// Config holds VM configuration
type Config struct {
	BootMode       string // efi, linux, macos
	Kernel         string
	Initrd         string
	Cmdline        string
	ISO            string
	Disk           string
	Memory         int // GB
	CPUs           int
	NVRAM          string
	DisableNetwork bool
	EnableRosetta  bool
	RosettaTag     string
}

// VMInstance wraps a running VM
type VMInstance struct {
	VM       *vz.VirtualMachine
	Config   *vz.VirtualMachineConfiguration
	mu       sync.Mutex
	shutdown chan struct{}
}

// CreateVM creates a new VM based on config
func CreateVM(cfg Config) (*VMInstance, error) {
	// Create boot loader based on mode
	var bootLoader vz.BootLoader
	var err error

	switch cfg.BootMode {
	case "efi":
		bootLoader, err = createEFIBootLoader(cfg.NVRAM)
	case "linux":
		bootLoader, err = createLinuxBootLoader(cfg.Kernel, cfg.Initrd, cfg.Cmdline)
	case "macos":
		bootLoader, err = createMacOSBootLoader()
	default:
		return nil, fmt.Errorf("unknown boot mode: %s", cfg.BootMode)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create boot loader: %w", err)
	}

	// Create platform config
	platform, err := vz.NewGenericPlatformConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create platform: %w", err)
	}

	// Create VM config
	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(cfg.CPUs),
		uint64(cfg.Memory)*1024*1024*1024,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM config: %w", err)
	}
	vmConfig.SetPlatformVirtualMachineConfiguration(platform)

	// Add storage devices
	var storageDevices []vz.StorageDeviceConfiguration

	// Add ISO if specified (USB mass storage for EFI boot)
	if cfg.ISO != "" {
		isoAtt, err := vz.NewDiskImageStorageDeviceAttachment(cfg.ISO, true)
		if err != nil {
			return nil, fmt.Errorf("failed to attach ISO: %w", err)
		}
		usb, err := vz.NewUSBMassStorageDeviceConfiguration(isoAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to create USB storage: %w", err)
		}
		storageDevices = append(storageDevices, usb)
	}

	// Add disk if specified
	if cfg.Disk != "" {
		diskAtt, err := vz.NewDiskImageStorageDeviceAttachment(cfg.Disk, false)
		if err != nil {
			return nil, fmt.Errorf("failed to attach disk: %w", err)
		}
		virtioBlock, err := vz.NewVirtioBlockDeviceConfiguration(diskAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to create virtio block: %w", err)
		}
		storageDevices = append(storageDevices, virtioBlock)
	}

	if len(storageDevices) > 0 {
		vmConfig.SetStorageDevicesVirtualMachineConfiguration(storageDevices)
	}

	if err := addRosettaDirectoryShare(vmConfig, cfg); err != nil {
		return nil, err
	}

	if !cfg.DisableNetwork {
		// Add network (NAT)
		natAtt, err := vz.NewNATNetworkDeviceAttachment()
		if err != nil {
			return nil, fmt.Errorf("failed to create NAT: %w", err)
		}
		virtioNet, err := vz.NewVirtioNetworkDeviceConfiguration(natAtt)
		if err != nil {
			return nil, fmt.Errorf("failed to create virtio network: %w", err)
		}
		vmConfig.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{virtioNet})
	}

	// Add serial console
	serialAtt, err := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to create serial attachment: %w", err)
	}
	serialPort, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialAtt)
	if err != nil {
		return nil, fmt.Errorf("failed to create serial port: %w", err)
	}
	vmConfig.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{serialPort})

	// Add entropy device
	entropy, err := vz.NewVirtioEntropyDeviceConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to create entropy device: %w", err)
	}
	vmConfig.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{entropy})

	// Validate
	ok, err := vmConfig.Validate()
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("config validation returned false")
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

func createEFIBootLoader(nvramPath string) (vz.BootLoader, error) {
	// Create or open EFI variable store
	store, err := vz.NewEFIVariableStore(nvramPath, vz.WithCreatingEFIVariableStore())
	if err != nil {
		// Try opening existing
		store, err = vz.NewEFIVariableStore(nvramPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create/open EFI store: %w", err)
		}
	}
	return vz.NewEFIBootLoader(vz.WithEFIVariableStore(store))
}

func createLinuxBootLoader(kernel, initrd, cmdline string) (vz.BootLoader, error) {
	if kernel == "" {
		return nil, fmt.Errorf("kernel path required for linux boot")
	}
	return vz.NewLinuxBootLoader(kernel,
		vz.WithInitrd(initrd),
		vz.WithCommandLine(cmdline),
	)
}

func createMacOSBootLoader() (vz.BootLoader, error) {
	return vz.NewMacOSBootLoader()
}

// StartVM starts the virtual machine
func StartVM(instance *VMInstance) error {
	instance.mu.Lock()
	defer instance.mu.Unlock()

	if err := instance.VM.Start(); err != nil {
		return err
	}
	return nil
}

// StopVM stops the virtual machine
func StopVM(instance *VMInstance) error {
	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.VM.CanStop() {
		return instance.VM.Stop()
	}
	return nil
}

// PauseVM pauses the virtual machine
func PauseVM(instance *VMInstance) error {
	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.VM.CanPause() {
		return instance.VM.Pause()
	}
	return fmt.Errorf("VM cannot be paused in current state")
}

// ResumeVM resumes a paused virtual machine
func ResumeVM(instance *VMInstance) error {
	instance.mu.Lock()
	defer instance.mu.Unlock()

	if instance.VM.CanResume() {
		return instance.VM.Resume()
	}
	return fmt.Errorf("VM cannot be resumed in current state")
}

// GetState returns the current VM state
func GetState(instance *VMInstance) string {
	switch instance.VM.State() {
	case vz.VirtualMachineStateStopped:
		return "stopped"
	case vz.VirtualMachineStateRunning:
		return "running"
	case vz.VirtualMachineStatePaused:
		return "paused"
	case vz.VirtualMachineStateError:
		return "error"
	case vz.VirtualMachineStateStarting:
		return "starting"
	case vz.VirtualMachineStateStopping:
		return "stopping"
	case vz.VirtualMachineStatePausing:
		return "pausing"
	case vz.VirtualMachineStateResuming:
		return "resuming"
	default:
		return "unknown"
	}
}

// WaitForShutdown waits for the VM to shutdown or signal interrupt
func WaitForShutdown(instance *VMInstance) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal, stopping VM...")
		StopVM(instance)
		close(instance.shutdown)
	}()

	// Wait for VM state changes
	for {
		select {
		case <-ctx.Done():
			return
		case <-instance.shutdown:
			return
		case newState := <-instance.VM.StateChangedNotify():
			switch newState {
			case vz.VirtualMachineStateStopped:
				fmt.Println("VM stopped")
				return
			case vz.VirtualMachineStateError:
				fmt.Println("VM error")
				return
			}
		}
	}
}

// Registry for active VMs (used by REPL)
var (
	vmRegistry   = make(map[string]*VMInstance)
	vmRegistryMu sync.RWMutex
)

// RegisterVMByName registers a VM with a name
func RegisterVMByName(name string, instance *VMInstance) {
	vmRegistryMu.Lock()
	defer vmRegistryMu.Unlock()
	vmRegistry[name] = instance
}

// GetVMByName retrieves a VM by name
func GetVMByName(name string) (*VMInstance, bool) {
	vmRegistryMu.RLock()
	defer vmRegistryMu.RUnlock()
	vm, ok := vmRegistry[name]
	return vm, ok
}

// =============================================================================
// Lisp Namespace Registration
// =============================================================================

// RegisterNamespace registers the vz namespace with the Lisp environment
func RegisterNamespace(env *lisp.Env) {
	// EFI functions
	env.Set("vz/new-efi-variable-store", &lisp.Fn{"vz/new-efi-variable-store", newEFIVariableStoreLisp})
	env.Set("vz/new-efi-boot-loader", &lisp.Fn{"vz/new-efi-boot-loader", newEFIBootLoaderLisp})

	// Linux boot
	env.Set("vz/new-linux-boot-loader", &lisp.Fn{"vz/new-linux-boot-loader", newLinuxBootLoaderLisp})

	// macOS boot
	env.Set("vz/new-macos-boot-loader", &lisp.Fn{"vz/new-macos-boot-loader", newMacOSBootLoaderLisp})

	// Platform
	env.Set("vz/new-generic-platform", &lisp.Fn{"vz/new-generic-platform", newGenericPlatformLisp})

	// Storage
	env.Set("vz/new-disk-attachment", &lisp.Fn{"vz/new-disk-attachment", newDiskAttachmentLisp})
	env.Set("vz/new-virtio-block-device", &lisp.Fn{"vz/new-virtio-block-device", newVirtioBlockDeviceLisp})
	env.Set("vz/new-usb-mass-storage", &lisp.Fn{"vz/new-usb-mass-storage", newUSBMassStorageLisp})

	// Network
	env.Set("vz/new-nat-network", &lisp.Fn{"vz/new-nat-network", newNATNetworkLisp})
	env.Set("vz/new-virtio-network", &lisp.Fn{"vz/new-virtio-network", newVirtioNetworkLisp})

	// VM config
	env.Set("vz/new-vm-config", &lisp.Fn{"vz/new-vm-config", newVMConfigLisp})
	env.Set("vz/add-storage-devices", &lisp.Fn{"vz/add-storage-devices", addStorageDevicesLisp})
	env.Set("vz/add-network-devices", &lisp.Fn{"vz/add-network-devices", addNetworkDevicesLisp})
	env.Set("vz/validate-config", &lisp.Fn{"vz/validate-config", validateConfigLisp})
	env.Set("vz/new-vm", &lisp.Fn{"vz/new-vm", newVMLisp})

	// VM control
	env.Set("vz/start-vm!", &lisp.Fn{"vz/start-vm!", startVMLisp})
	env.Set("vz/stop-vm!", &lisp.Fn{"vz/stop-vm!", stopVMLisp})
	env.Set("vz/pause-vm!", &lisp.Fn{"vz/pause-vm!", pauseVMLisp})
	env.Set("vz/resume-vm!", &lisp.Fn{"vz/resume-vm!", resumeVMLisp})
	env.Set("vz/vm-state", &lisp.Fn{"vz/vm-state", vmStateLisp})

	// Graphics
	env.Set("vz/new-virtio-graphics-device", &lisp.Fn{"vz/new-virtio-graphics-device", newVirtioGraphicsDeviceLisp})
	env.Set("vz/add-graphics-devices", &lisp.Fn{"vz/add-graphics-devices", addGraphicsDevicesLisp})
	env.Set("vz/start-graphic-app!", &lisp.Fn{"vz/start-graphic-app!", startGraphicAppLisp})

	// Utilities
	env.Set("vz/create-disk-image", &lisp.Fn{"vz/create-disk-image", createDiskImageLisp})
	env.Set("vz/wait-for-shutdown", &lisp.Fn{"vz/wait-for-shutdown", waitForShutdownLisp})
}

// =============================================================================
// Lisp Function Implementations
// =============================================================================

func newEFIVariableStoreLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-efi-variable-store requires path argument")
	}
	path := string(args[0].(lisp.String))
	create := true
	if len(args) > 1 {
		create = bool(args[1].(lisp.Bool))
	}

	var store *vz.EFIVariableStore
	var err error
	if create {
		store, err = vz.NewEFIVariableStore(path, vz.WithCreatingEFIVariableStore())
	} else {
		store, err = vz.NewEFIVariableStore(path)
	}
	if err != nil {
		panic(fmt.Sprintf("failed to create EFI store: %v", err))
	}
	return &lisp.ExternalValue{Value: store, Type: "EFIVariableStore"}
}

func newEFIBootLoaderLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-efi-boot-loader requires store argument")
	}
	store := args[0].(*lisp.ExternalValue).Value.(*vz.EFIVariableStore)
	loader, err := vz.NewEFIBootLoader(vz.WithEFIVariableStore(store))
	if err != nil {
		panic(fmt.Sprintf("failed to create EFI boot loader: %v", err))
	}
	return &lisp.ExternalValue{Value: loader, Type: "EFIBootLoader"}
}

func newLinuxBootLoaderLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-linux-boot-loader requires kernel path")
	}
	kernel := string(args[0].(lisp.String))
	initrd := ""
	cmdline := "console=hvc0"
	if len(args) > 1 {
		initrd = string(args[1].(lisp.String))
	}
	if len(args) > 2 {
		cmdline = string(args[2].(lisp.String))
	}

	opts := []vz.LinuxBootLoaderOption{}
	if initrd != "" {
		opts = append(opts, vz.WithInitrd(initrd))
	}
	opts = append(opts, vz.WithCommandLine(cmdline))

	loader, err := vz.NewLinuxBootLoader(kernel, opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create Linux boot loader: %v", err))
	}
	return &lisp.ExternalValue{Value: loader, Type: "LinuxBootLoader"}
}

func newMacOSBootLoaderLisp(args []lisp.Value) lisp.Value {
	loader, err := vz.NewMacOSBootLoader()
	if err != nil {
		panic(fmt.Sprintf("failed to create macOS boot loader: %v", err))
	}
	return &lisp.ExternalValue{Value: loader, Type: "MacOSBootLoader"}
}

func newGenericPlatformLisp(args []lisp.Value) lisp.Value {
	platform, err := vz.NewGenericPlatformConfiguration()
	if err != nil {
		panic(fmt.Sprintf("failed to create platform: %v", err))
	}
	return &lisp.ExternalValue{Value: platform, Type: "GenericPlatform"}
}

func newDiskAttachmentLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-disk-attachment requires path")
	}
	path := string(args[0].(lisp.String))
	readOnly := false
	if len(args) > 1 {
		readOnly = bool(args[1].(lisp.Bool))
	}
	att, err := vz.NewDiskImageStorageDeviceAttachment(path, readOnly)
	if err != nil {
		panic(fmt.Sprintf("failed to create disk attachment: %v", err))
	}
	return &lisp.ExternalValue{Value: att, Type: "DiskAttachment"}
}

func newVirtioBlockDeviceLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-virtio-block-device requires attachment")
	}
	att := args[0].(*lisp.ExternalValue).Value.(vz.StorageDeviceAttachment)
	dev, err := vz.NewVirtioBlockDeviceConfiguration(att)
	if err != nil {
		panic(fmt.Sprintf("failed to create virtio block device: %v", err))
	}
	return &lisp.ExternalValue{Value: dev, Type: "VirtioBlockDevice"}
}

func newUSBMassStorageLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-usb-mass-storage requires attachment")
	}
	att := args[0].(*lisp.ExternalValue).Value.(vz.StorageDeviceAttachment)
	dev, err := vz.NewUSBMassStorageDeviceConfiguration(att)
	if err != nil {
		panic(fmt.Sprintf("failed to create USB mass storage: %v", err))
	}
	return &lisp.ExternalValue{Value: dev, Type: "USBMassStorage"}
}

func newNATNetworkLisp(args []lisp.Value) lisp.Value {
	att, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		panic(fmt.Sprintf("failed to create NAT: %v", err))
	}
	return &lisp.ExternalValue{Value: att, Type: "NATNetwork"}
}

func newVirtioNetworkLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-virtio-network requires attachment")
	}
	att := args[0].(*lisp.ExternalValue).Value.(*vz.NATNetworkDeviceAttachment)
	dev, err := vz.NewVirtioNetworkDeviceConfiguration(att)
	if err != nil {
		panic(fmt.Sprintf("failed to create virtio network: %v", err))
	}
	return &lisp.ExternalValue{Value: dev, Type: "VirtioNetwork"}
}

func newVMConfigLisp(args []lisp.Value) lisp.Value {
	if len(args) < 4 {
		panic("vz/new-vm-config requires cpus, memory-gb, boot-loader, platform")
	}
	cpus := int(args[0].(lisp.Int))
	memGB := int(args[1].(lisp.Int))
	bootLoader := args[2].(*lisp.ExternalValue).Value.(vz.BootLoader)
	platform := args[3].(*lisp.ExternalValue).Value.(*vz.GenericPlatformConfiguration)

	config, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(cpus),
		uint64(memGB)*1024*1024*1024,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create VM config: %v", err))
	}
	config.SetPlatformVirtualMachineConfiguration(platform)
	return &lisp.ExternalValue{Value: config, Type: "VMConfig"}
}

func addStorageDevicesLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("vz/add-storage-devices requires config and devices vector")
	}
	config := args[0].(*lisp.ExternalValue).Value.(*vz.VirtualMachineConfiguration)
	devVec := args[1].(lisp.Vector)

	devices := make([]vz.StorageDeviceConfiguration, len(devVec))
	for i, v := range devVec {
		devices[i] = v.(*lisp.ExternalValue).Value.(vz.StorageDeviceConfiguration)
	}
	config.SetStorageDevicesVirtualMachineConfiguration(devices)
	return lisp.Nil{}
}

func addNetworkDevicesLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("vz/add-network-devices requires config and devices vector")
	}
	config := args[0].(*lisp.ExternalValue).Value.(*vz.VirtualMachineConfiguration)
	devVec := args[1].(lisp.Vector)

	devices := make([]*vz.VirtioNetworkDeviceConfiguration, len(devVec))
	for i, v := range devVec {
		devices[i] = v.(*lisp.ExternalValue).Value.(*vz.VirtioNetworkDeviceConfiguration)
	}
	config.SetNetworkDevicesVirtualMachineConfiguration(devices)
	return lisp.Nil{}
}

func validateConfigLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/validate-config requires config")
	}
	config := args[0].(*lisp.ExternalValue).Value.(*vz.VirtualMachineConfiguration)
	ok, err := config.Validate()
	if err != nil {
		return lisp.String(fmt.Sprintf("invalid: %v", err))
	}
	if !ok {
		return lisp.String("invalid: validation returned false")
	}
	return lisp.Bool(true)
}

func newVMLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/new-vm requires config")
	}
	config := args[0].(*lisp.ExternalValue).Value.(*vz.VirtualMachineConfiguration)
	vm, err := vz.NewVirtualMachine(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create VM: %v", err))
	}
	instance := &VMInstance{
		VM:       vm,
		Config:   config,
		shutdown: make(chan struct{}),
	}
	return &lisp.ExternalValue{Value: instance, Type: "VM"}
}

func startVMLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/start-vm! requires VM")
	}
	instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)
	if err := StartVM(instance); err != nil {
		panic(fmt.Sprintf("failed to start VM: %v", err))
	}
	return lisp.Bool(true)
}

func stopVMLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/stop-vm! requires VM")
	}
	instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)
	if err := StopVM(instance); err != nil {
		panic(fmt.Sprintf("failed to stop VM: %v", err))
	}
	return lisp.Bool(true)
}

func pauseVMLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/pause-vm! requires VM")
	}
	instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)
	if err := PauseVM(instance); err != nil {
		panic(fmt.Sprintf("failed to pause VM: %v", err))
	}
	return lisp.Bool(true)
}

func resumeVMLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/resume-vm! requires VM")
	}
	instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)
	if err := ResumeVM(instance); err != nil {
		panic(fmt.Sprintf("failed to resume VM: %v", err))
	}
	return lisp.Bool(true)
}

func vmStateLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/vm-state requires VM")
	}
	instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)
	return lisp.String(GetState(instance))
}

func createDiskImageLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("vz/create-disk-image requires path and size-gb")
	}
	path := string(args[0].(lisp.String))
	sizeGB := int64(args[1].(lisp.Int))

	// Create sparse file
	f, err := os.Create(path)
	if err != nil {
		panic(fmt.Sprintf("failed to create disk: %v", err))
	}
	defer f.Close()

	size := sizeGB * 1024 * 1024 * 1024
	if err := f.Truncate(size); err != nil {
		panic(fmt.Sprintf("failed to resize disk: %v", err))
	}
	return lisp.Bool(true)
}

func waitForShutdownLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/wait-for-shutdown requires VM")
	}
	instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)
	WaitForShutdown(instance)
	return lisp.Nil{}
}

// Graphics functions

func newVirtioGraphicsDeviceLisp(args []lisp.Value) lisp.Value {
	width := 1920
	height := 1200
	if len(args) > 0 {
		width = int(args[0].(lisp.Int))
	}
	if len(args) > 1 {
		height = int(args[1].(lisp.Int))
	}

	scanout, err := vz.NewVirtioGraphicsScanoutConfiguration(int64(width), int64(height))
	if err != nil {
		panic(fmt.Sprintf("failed to create graphics scanout: %v", err))
	}

	graphics, err := vz.NewVirtioGraphicsDeviceConfiguration()
	if err != nil {
		panic(fmt.Sprintf("failed to create graphics device: %v", err))
	}
	graphics.SetScanouts(scanout)

	return &lisp.ExternalValue{Value: graphics, Type: "VirtioGraphicsDevice"}
}

func addGraphicsDevicesLisp(args []lisp.Value) lisp.Value {
	if len(args) < 2 {
		panic("vz/add-graphics-devices requires config and devices vector")
	}
	config := args[0].(*lisp.ExternalValue).Value.(*vz.VirtualMachineConfiguration)
	devVec := args[1].(lisp.Vector)

	devices := make([]vz.GraphicsDeviceConfiguration, len(devVec))
	for i, v := range devVec {
		devices[i] = v.(*lisp.ExternalValue).Value.(vz.GraphicsDeviceConfiguration)
	}
	config.SetGraphicsDevicesVirtualMachineConfiguration(devices)
	return lisp.Nil{}
}

func startGraphicAppLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("vz/start-graphic-app! requires VM")
	}
	instance := args[0].(*lisp.ExternalValue).Value.(*VMInstance)

	width := float64(1920)
	height := float64(1200)
	title := "boxxy VM"
	if len(args) > 1 {
		width = float64(args[1].(lisp.Int))
	}
	if len(args) > 2 {
		height = float64(args[2].(lisp.Int))
	}
	if len(args) > 3 {
		title = string(args[3].(lisp.String))
	}

	// Lock OS thread as required by macOS GUI
	runtime.LockOSThread()

	instance.VM.StartGraphicApplication(width, height, vz.WithWindowTitle(title))

	return lisp.Bool(true)
}
