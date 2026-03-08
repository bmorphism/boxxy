//go:build darwin && arm64

package vm

import (
        "context"
        "fmt"
        "io"
        "os"
        "os/signal"
        "runtime"
        "sync"
        "syscall"

        "github.com/Code-Hex/vz/v3"
        "github.com/bmorphism/boxxy/internal/lisp"
)

func init() {
        runtime.LockOSThread()
}

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
        PinholeMode    bool
        PinholePorts   []int
        SharedDirs     map[string]string // tag -> path
        Graphics       bool
        Width          int
        Height         int
        Keyboard       bool
        Pointer        bool
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
        if err := validateConfig(cfg); err != nil {
                return nil, err
        }

        bootLoader, err := bootLoaderFromConfig(cfg)
        if err != nil {
                return nil, err
        }

        platform, err := vz.NewGenericPlatformConfiguration()
        if err != nil {
                return nil, fmt.Errorf("platform: %w", err)
        }

        vmConfig, err := vz.NewVirtualMachineConfiguration(
                bootLoader,
                uint(cfg.CPUs),
                uint64(cfg.Memory)*1024*1024*1024,
        )
        if err != nil {
                return nil, fmt.Errorf("vm config: %w", err)
        }
        vmConfig.SetPlatformVirtualMachineConfiguration(platform)

        storageDevices, err := storageDevicesFromConfig(cfg)
        if err != nil {
                return nil, err
        }
        if len(storageDevices) > 0 {
                vmConfig.SetStorageDevicesVirtualMachineConfiguration(storageDevices)
        }

        networkDevices, err := networkDevicesFromConfig(cfg)
        if err != nil {
                return nil, err
        }
        if len(networkDevices) > 0 {
                vmConfig.SetNetworkDevicesVirtualMachineConfiguration(networkDevices)
        }

        if cfg.Graphics {
                width := cfg.Width
                height := cfg.Height
                if width <= 0 {
                        width = 1280
                }
                if height <= 0 {
                        height = 800
                }

                scanout, err := vz.NewVirtioGraphicsScanoutConfiguration(int64(width), int64(height))
                if err != nil {
                        return nil, fmt.Errorf("graphics scanout: %w", err)
                }
                graphics, err := vz.NewVirtioGraphicsDeviceConfiguration()
                if err != nil {
                        return nil, fmt.Errorf("graphics device: %w", err)
                }
                graphics.SetScanouts(scanout)
                vmConfig.SetGraphicsDevicesVirtualMachineConfiguration([]vz.GraphicsDeviceConfiguration{graphics})

                if cfg.Keyboard {
                        keyboard, err := vz.NewUSBKeyboardConfiguration()
                        if err != nil {
                                return nil, fmt.Errorf("keyboard: %w", err)
                        }
                        vmConfig.SetKeyboardsVirtualMachineConfiguration([]vz.KeyboardConfiguration{keyboard})
                }

                if cfg.Pointer {
                        pointer, err := vz.NewUSBScreenCoordinatePointingDeviceConfiguration()
                        if err != nil {
                                return nil, fmt.Errorf("pointer: %w", err)
                        }
                        vmConfig.SetPointingDevicesVirtualMachineConfiguration([]vz.PointingDeviceConfiguration{pointer})
                }
        }
        
        // Directory Shares (VirtioFS)
        if len(cfg.SharedDirs) > 0 {
            var shares []vz.DirectorySharingDeviceConfiguration
            for tag, path := range cfg.SharedDirs {
                share, err := vz.NewSharedDirectory(path, false) // read-only by default for now
                if err != nil {
                    return nil, fmt.Errorf("shared directory: %w", err)
                }
                singleShare, err := vz.NewSingleDirectoryShare(share)
                if err != nil {
                    return nil, fmt.Errorf("single dir share: %w", err)
                }
                fs, err := vz.NewVirtioFileSystemDeviceConfiguration(tag)
                if err != nil {
                    return nil, fmt.Errorf("virtio fs config: %w", err)
                }
                fs.SetDirectoryShare(singleShare)
                shares = append(shares, fs)
            }
            vmConfig.SetDirectorySharingDevicesVirtualMachineConfiguration(shares)
        }

        // Serial console
        serialAtt, err := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
        if err != nil {
                return nil, fmt.Errorf("serial attachment: %w", err)
        }
        serialPort, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialAtt)
        if err != nil {
                return nil, fmt.Errorf("serial port: %w", err)
        }
        vmConfig.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{serialPort})

        // Entropy
        entropy, err := vz.NewVirtioEntropyDeviceConfiguration()
        if err != nil {
                return nil, fmt.Errorf("entropy: %w", err)
        }
        vmConfig.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{entropy})

        // VirtIO socket for host-guest IPC
        socketDev, err := vz.NewVirtioSocketDeviceConfiguration()
        if err != nil {
                return nil, fmt.Errorf("vsock: %w", err)
        }
        vmConfig.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{socketDev})

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

func validateConfig(cfg Config) error {
        if cfg.Memory <= 0 {
                return fmt.Errorf("memory must be > 0")
        }
        if cfg.CPUs <= 0 {
                return fmt.Errorf("cpus must be > 0")
        }
        switch cfg.BootMode {
        case "efi", "linux", "macos":
        default:
                return fmt.Errorf("unknown boot mode: %s", cfg.BootMode)
        }
        if cfg.BootMode == "linux" && cfg.Kernel == "" {
                return fmt.Errorf("kernel required for linux boot")
        }
        if cfg.BootMode == "efi" && cfg.NVRAM == "" {
                return fmt.Errorf("nvram required for efi boot")
        }
        if cfg.Graphics {
                if cfg.Width < 0 || cfg.Height < 0 {
                        return fmt.Errorf("graphics dimensions must be >= 0")
                }
        }
        return nil
}


func storageDevicesFromConfig(cfg Config) ([]vz.StorageDeviceConfiguration, error) {
        var devices []vz.StorageDeviceConfiguration

        if cfg.ISO != "" {
                att, err := vz.NewDiskImageStorageDeviceAttachment(cfg.ISO, true)
                if err != nil {
                        return nil, fmt.Errorf("attach ISO: %w", err)
                }
                usb, err := vz.NewUSBMassStorageDeviceConfiguration(att)
                if err != nil {
                        return nil, fmt.Errorf("USB storage: %w", err)
                }
                devices = append(devices, usb)
        }

        if cfg.Disk != "" {
                att, err := vz.NewDiskImageStorageDeviceAttachment(cfg.Disk, false)
                if err != nil {
                        return nil, fmt.Errorf("attach disk: %w", err)
                }
                blk, err := vz.NewVirtioBlockDeviceConfiguration(att)
                if err != nil {
                        return nil, fmt.Errorf("virtio block: %w", err)
                }
                devices = append(devices, blk)
        }

        return devices, nil
}

func networkDevicesFromConfig(cfg Config) ([]*vz.VirtioNetworkDeviceConfiguration, error) {
        if cfg.DisableNetwork {
                return nil, nil
        }
        nat, err := vz.NewNATNetworkDeviceAttachment()
        if err != nil {
                return nil, fmt.Errorf("NAT: %w", err)
        }
        net, err := vz.NewVirtioNetworkDeviceConfiguration(nat)
        if err != nil {
                return nil, fmt.Errorf("virtio net: %w", err)
        }
        return []*vz.VirtioNetworkDeviceConfiguration{net}, nil
}

func bootLoaderFromConfig(cfg Config) (vz.BootLoader, error) {
        switch cfg.BootMode {
        case "efi":
                store, err := vz.NewEFIVariableStore(cfg.NVRAM, vz.WithCreatingEFIVariableStore())
                if err != nil {
                        store, err = vz.NewEFIVariableStore(cfg.NVRAM)
                        if err != nil {
                                return nil, fmt.Errorf("EFI store: %w", err)
                        }
                }
                return vz.NewEFIBootLoader(vz.WithEFIVariableStore(store))
        case "linux":
                return vz.NewLinuxBootLoader(cfg.Kernel,
                        vz.WithInitrd(cfg.Initrd),
                        vz.WithCommandLine(cfg.Cmdline),
                )
        case "macos":
                return vz.NewMacOSBootLoader()
        default:
                return nil, fmt.Errorf("unknown boot mode: %s", cfg.BootMode)
        }
}

// StartVM starts the virtual machine
func StartVM(instance *VMInstance) error {
        instance.mu.Lock()
        defer instance.mu.Unlock()
        return instance.VM.Start()
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

// GetState returns the current VM state as a string
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
        default:
                return "unknown"
        }
}

// WaitForShutdown waits for the VM to stop or a signal interrupt
func WaitForShutdown(instance *VMInstance) {
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

        go func() {
                <-sigChan
                fmt.Println("\nShutting down VM...")
                StopVM(instance)
                close(instance.shutdown)
        }()

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

// VM registry for named VMs
var (
        vmRegistry   = make(map[string]*VMInstance)
        vmRegistryMu sync.RWMutex
)

func RegisterVM(name string, instance *VMInstance) {
        vmRegistryMu.Lock()
        defer vmRegistryMu.Unlock()
        vmRegistry[name] = instance
}

func GetVM(name string) (*VMInstance, bool) {
        vmRegistryMu.RLock()
        defer vmRegistryMu.RUnlock()
        v, ok := vmRegistry[name]
        return v, ok
}

// StoragePlan describes the storage configuration for test verification.
type StoragePlan struct {
        HasISO  bool
        HasDisk bool
}

func storagePlanFromConfig(cfg Config) StoragePlan {
        return StoragePlan{
                HasISO:  cfg.ISO != "",
                HasDisk: cfg.Disk != "",
        }
}


// RegisterNamespace registers the vz namespace functions into the boxxy Lisp
// environment, wiring vz.joke declarations to real Go Virtualization.framework
// implementations.
func RegisterNamespace(env *lisp.Env) {
        reg := func(name string, f func([]lisp.Value) lisp.Value) {
                env.Set(lisp.Symbol(name), &lisp.Fn{Name: name, Func: f})
        }

        // -- Boot Loaders --

        reg("vz/new-efi-variable-store", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vz/new-efi-variable-store: requires (path create?)")
                }
                path := string(args[0].(lisp.String))
                create := bool(args[1].(lisp.Bool))
                return &lisp.ExternalValue{Value: map[string]interface{}{"path": path, "create": create}, Type: "EFIVariableStore"}
        })

        reg("vz/new-efi-boot-loader", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/new-efi-boot-loader: requires (store)")
                }
                return &lisp.ExternalValue{Value: args[0], Type: "EFIBootLoader"}
        })

        reg("vz/new-linux-boot-loader", func(args []lisp.Value) lisp.Value {
                if len(args) < 3 {
                        panic("vz/new-linux-boot-loader: requires (kernel initrd cmdline)")
                }
                kernel := string(args[0].(lisp.String))
                initrd := string(args[1].(lisp.String))
                cmdline := string(args[2].(lisp.String))
                return &lisp.ExternalValue{
                        Value: map[string]string{"kernel": kernel, "initrd": initrd, "cmdline": cmdline},
                        Type:  "LinuxBootLoader",
                }
        })

        reg("vz/new-macos-boot-loader", func(args []lisp.Value) lisp.Value {
                return &lisp.ExternalValue{Value: nil, Type: "MacOSBootLoader"}
        })

        // -- Platform --

        reg("vz/new-generic-platform", func(args []lisp.Value) lisp.Value {
                return &lisp.ExternalValue{Value: nil, Type: "GenericPlatform"}
        })

        // -- Storage --

        reg("vz/new-disk-attachment", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vz/new-disk-attachment: requires (path read-only?)")
                }
                path := string(args[0].(lisp.String))
                ro := bool(args[1].(lisp.Bool))
                return &lisp.ExternalValue{Value: map[string]interface{}{"path": path, "read-only": ro}, Type: "DiskAttachment"}
        })

        reg("vz/new-virtio-block-device", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/new-virtio-block-device: requires (attachment)")
                }
                return &lisp.ExternalValue{Value: args[0], Type: "VirtioBlockDevice"}
        })

        reg("vz/new-usb-mass-storage", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/new-usb-mass-storage: requires (attachment)")
                }
                return &lisp.ExternalValue{Value: args[0], Type: "USBMassStorage"}
        })

        // -- File System --

        reg("vz/new-virtio-fs-device", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vz/new-virtio-fs-device: requires (tag path)")
                }
                tag := string(args[0].(lisp.String))
                path := string(args[1].(lisp.String))
                return &lisp.ExternalValue{Value: map[string]string{"tag": tag, "path": path}, Type: "VirtioFileSystemDevice"}
        })

        // -- Network --

        reg("vz/new-nat-network", func(args []lisp.Value) lisp.Value {
                return &lisp.ExternalValue{Value: nil, Type: "NATNetwork"}
        })

        reg("vz/new-virtio-network", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/new-virtio-network: requires (attachment)")
                }
                return &lisp.ExternalValue{Value: args[0], Type: "VirtioNetwork"}
        })

        // -- VM Configuration --

        reg("vz/new-vm-config", func(args []lisp.Value) lisp.Value {
                if len(args) < 4 {
                        panic("vz/new-vm-config: requires (cpus memory-gb boot platform)")
                }
                cpus := int(args[0].(lisp.Int))
                memGB := int(args[1].(lisp.Int))
                cfg := &Config{
                        CPUs:   cpus,
                        Memory: memGB,
                        SharedDirs: make(map[string]string),
                }
                
                if bootLoader, ok := args[2].(*lisp.ExternalValue); ok {
                    if props, ok := bootLoader.Value.(map[string]interface{}); ok {
                        if path, ok := props["path"].(string); ok {
                             cfg.BootMode = "efi"
                             cfg.NVRAM = path
                        }
                    } else if props, ok := bootLoader.Value.(map[string]string); ok {
                        if kernel, ok := props["kernel"]; ok {
                            cfg.BootMode = "linux"
                            cfg.Kernel = kernel
                            cfg.Initrd = props["initrd"]
                            cfg.Cmdline = props["cmdline"]
                        }
                    } else if bootLoader.Type == "MacOSBootLoader" {
                        cfg.BootMode = "macos"
                    }
                }
                
                return &lisp.ExternalValue{Value: cfg, Type: "VMConfig"}
        })

        reg("vz/add-storage-devices", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vz/add-storage-devices: requires (config [devices])")
                }
                cfgExt := args[0].(*lisp.ExternalValue)
                cfg := cfgExt.Value.(*Config)
                
                devices := args[1].(lisp.Vector)
                for _, dev := range devices {
                    if devExt, ok := dev.(*lisp.ExternalValue); ok {
                        if props, ok := devExt.Value.(map[string]interface{}); ok {
                             if path, ok := props["path"].(string); ok {
                                 if ro, ok := props["read-only"].(bool); ok && ro {
                                     cfg.ISO = path
                                 } else {
                                     cfg.Disk = path
                                 }
                             }
                        }
                    }
                }
                return lisp.Nil{}
        })

        reg("vz/add-network-devices", func(args []lisp.Value) lisp.Value {
             if len(args) < 2 {
                        panic("vz/add-network-devices: requires (config [devices])")
                }
                cfgExt := args[0].(*lisp.ExternalValue)
                cfg := cfgExt.Value.(*Config)
                
                devices := args[1].(lisp.Vector)
                if len(devices) > 0 {
                    cfg.DisableNetwork = false
                } else {
                    cfg.DisableNetwork = true
                }
                return lisp.Nil{}
        })
        
        reg("vz/add-directory-shares", func(args []lisp.Value) lisp.Value {
             if len(args) < 2 {
                 panic("vz/add-directory-shares: requires (config [shares])")
             }
             cfgExt := args[0].(*lisp.ExternalValue)
             cfg := cfgExt.Value.(*Config)
             
             shares := args[1].(lisp.Vector)
             for _, share := range shares {
                 if shareExt, ok := share.(*lisp.ExternalValue); ok {
                     if props, ok := shareExt.Value.(map[string]string); ok {
                         tag := props["tag"]
                         path := props["path"]
                         cfg.SharedDirs[tag] = path
                     }
                 }
             }
             return lisp.Nil{}
        })

        reg("vz/validate-config", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/validate-config: requires (config)")
                }
                ext, ok := args[0].(*lisp.ExternalValue)
                if !ok {
                        return lisp.Bool(false)
                }
                cfg, ok := ext.Value.(*Config)
                if !ok {
                        return lisp.Bool(false)
                }
                return lisp.Bool(validateConfig(*cfg) == nil)
        })

        reg("vz/new-vm", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/new-vm: requires (config)")
                }
                ext, ok := args[0].(*lisp.ExternalValue)
                if !ok {
                        panic("vz/new-vm: expected VMConfig")
                }
                cfg, ok := ext.Value.(*Config)
                if !ok {
                        panic("vz/new-vm: expected VMConfig")
                }
                instance, err := CreateVM(*cfg)
                if err != nil {
                        panic(fmt.Sprintf("vz/new-vm: %v", err))
                }
                return &lisp.ExternalValue{Value: instance, Type: "VM"}
        })

        // -- VM Control --

        reg("vz/start-vm!", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/start-vm!: requires (vm)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                if err := StartVM(instance); err != nil {
                        panic(fmt.Sprintf("vz/start-vm!: %v", err))
                }
                return lisp.Bool(true)
        })

        reg("vz/stop-vm!", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/stop-vm!: requires (vm)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                if err := StopVM(instance); err != nil {
                        panic(fmt.Sprintf("vz/stop-vm!: %v", err))
                }
                return lisp.Bool(true)
        })

        reg("vz/pause-vm!", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/pause-vm!: requires (vm)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                instance.mu.Lock()
                defer instance.mu.Unlock()
                if err := instance.VM.Pause(); err != nil {
                        panic(fmt.Sprintf("vz/pause-vm!: %v", err))
                }
                return lisp.Bool(true)
        })

        reg("vz/resume-vm!", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/resume-vm!: requires (vm)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                instance.mu.Lock()
                defer instance.mu.Unlock()
                if err := instance.VM.Resume(); err != nil {
                        panic(fmt.Sprintf("vz/resume-vm!: %v", err))
                }
                return lisp.Bool(true)
        })

        reg("vz/vm-state", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/vm-state: requires (vm)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                return lisp.String(GetState(instance))
        })

        // -- Utilities --

        reg("vz/create-disk-image", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vz/create-disk-image: requires (path size-gb)")
                }
                path := string(args[0].(lisp.String))
                sizeGB := int(args[1].(lisp.Int))
                f, err := os.Create(path)
                if err != nil {
                        panic(fmt.Sprintf("vz/create-disk-image: %v", err))
                }
                if err := f.Truncate(int64(sizeGB) * 1024 * 1024 * 1024); err != nil {
                        f.Close()
                        panic(fmt.Sprintf("vz/create-disk-image: %v", err))
                }
                f.Close()
                return lisp.Bool(true)
        })
}

// Silence unused import warnings
var _ = io.EOF
