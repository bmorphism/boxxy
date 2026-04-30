//go:build darwin && arm64

package vm

import (
        "context"
        "fmt"
        "hash/fnv"
        "io"
        "math"
        "os"
        "os/signal"
        "runtime"
        "sort"
        "strings"
        "sync"
        "syscall"

        "github.com/Code-Hex/vz/v3"
        "github.com/bmorphism/boxxy/internal/lisp"
)

// URI scheme constants (borrowing from vibespace-mcp pattern).
// color:// is a view/lens over vm://; it never stores separately.
const (
        VMScheme    = "vm://"
        VMListURI   = "vm://list"
        ColorScheme = "color://"
)

func init() {
        runtime.LockOSThread()
}

// Config holds VM configuration
type Config struct {
        BootMode          string // efi, linux, macos
        Kernel            string
        Initrd            string
        Cmdline           string
        ISO               string
        Disk              string
        Memory            int // GB
        CPUs              int
        NVRAM             string
        DisableNetwork    bool
        EnableRosetta     bool
        RosettaTag        string
        PinholeMode       bool
        PinholePorts      []int
        SharedDirs        map[string]string // tag -> path
        Graphics          bool
        Width             int
        Height            int
        Keyboard          bool
        Pointer           bool
        EnableNestedVirt  bool
        EnableXHCI        bool
        SaveStatePath     string
}

// VMInstance wraps a running VM.
// Attrs is the open accumulator: keywords appear when placed, nothing forces them.
// color:// reads from here as a lens; any interaction can deposit attributes.
//
// SPI fields (Gay.jl SplitMix64):
//   Seed        — deterministic identity derived from name via FNV-1a
//   Invocation  — monotonic counter advanced on every SetAttr call
//   Fingerprint — XOR accumulator of splitmix64(seed^invocation) at each step;
//                 identical seed + identical interaction sequence → identical fingerprint
type VMInstance struct {
        VM          *vz.VirtualMachine
        Config      *vz.VirtualMachineConfiguration
        Attrs       map[string]any
        Seed        uint64
        Invocation  uint64
        Fingerprint uint64
        mu          sync.Mutex
        shutdown    chan struct{}
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
        if cfg.EnableNestedVirt {
                if err := platform.SetNestedVirtualizationEnabled(true); err != nil {
                        return nil, fmt.Errorf("nested virtualization: %w", err)
                }
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

        if cfg.EnableXHCI {
                xhci, err := vz.NewXHCIControllerConfiguration()
                if err != nil {
                        return nil, fmt.Errorf("xhci controller: %w", err)
                }
                vmConfig.SetUSBControllersVirtualMachineConfiguration([]vz.USBControllerConfiguration{xhci})
        }

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
                Attrs:    make(map[string]any),
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
        if instance.VM == nil {
                return "unknown"
        }
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
        if instance.Seed == 0 {
                instance.Seed = seedFromName(name)
        }
        vmRegistry[name] = instance
}

func GetVM(name string) (*VMInstance, bool) {
        vmRegistryMu.RLock()
        defer vmRegistryMu.RUnlock()
        v, ok := vmRegistry[name]
        return v, ok
}

func ListVMs() map[string]*VMInstance {
        vmRegistryMu.RLock()
        defer vmRegistryMu.RUnlock()
        out := make(map[string]*VMInstance, len(vmRegistry))
        for k, v := range vmRegistry {
                out[k] = v
        }
        return out
}

// ResolveVMURI dispatches a vm:// URI and returns a Lisp value.
//
//   vm://list              → vector of {name, state} hashmaps
//   vm://{name}            → {name, state, nested-virt, xhci, save-path} hashmap
//   vm://{name}/state      → state string
//   vm://{name}/usb        → bool (XHCI enabled)
//   vm://{name}/save-path  → string or nil
func ResolveVMURI(uri string) lisp.Value {
        if !strings.HasPrefix(uri, VMScheme) {
                panic(fmt.Sprintf("vm/resolve: not a vm:// URI: %s", uri))
        }

        path := strings.TrimPrefix(uri, VMScheme)
        if path == "" {
                return lisp.Nil{}
        }

        if path == "list" {
                vms := ListVMs()
                items := make(lisp.Vector, 0, len(vms))
                for name, inst := range vms {
                        inst.mu.Lock()
                        state := getStateLocked(inst)
                        inst.mu.Unlock()
                        m := make(lisp.HashMap)
                        m[lisp.Keyword("name")] = lisp.String(name)
                        m[lisp.Keyword("state")] = lisp.String(state)
                        items = append(items, m)
                }
                return items
        }

        parts := strings.SplitN(path, "/", 2)
        name := parts[0]
        if name == "" {
                return lisp.Nil{}
        }
        inst, ok := GetVM(name)
        if !ok {
                return lisp.Nil{}
        }

        // Read state under lock — consistent with any concurrent SetAttr / color:// reads.
        inst.mu.Lock()
        state := getStateLocked(inst)
        inst.mu.Unlock()

        if len(parts) == 1 {
                m := make(lisp.HashMap)
                m[lisp.Keyword("name")] = lisp.String(name)
                m[lisp.Keyword("state")] = lisp.String(state)
                m[lisp.Keyword("nested-virt-supported")] = lisp.Bool(vz.IsNestedVirtualizationSupported())
                return m
        }

        sub := parts[1]
        switch sub {
        case "state":
                return lisp.String(state)
        case "usb":
                return lisp.Bool(true)
        case "save-path":
                return lisp.Nil{}
        default:
                return lisp.Nil{} // unknown sub-resource returns nil, no panic
        }
}

// SetAttr deposits a key-value pair into the VM's open accumulator.
// Nothing validates; anything can be placed. Perceivable but not forced.
//
// Self-fuzzing: every call advances the invocation counter and XOR-accumulates
// the splitmix64 output into the fingerprint. Same seed + same interaction
// sequence → identical fingerprint (SPI property).
func SetAttr(inst *VMInstance, key string, val any) {
        inst.mu.Lock()
        defer inst.mu.Unlock()
        if inst.Attrs == nil {
                inst.Attrs = make(map[string]any)
        }
        inst.Attrs[key] = val
        inst.Invocation++
        inst.Fingerprint ^= splitmix64(inst.Seed ^ inst.Invocation)
}

// GetAttr reads a single attribute. Returns (val, true) or (nil, false).
func GetAttr(inst *VMInstance, key string) (any, bool) {
        inst.mu.Lock()
        defer inst.mu.Unlock()
        v, ok := inst.Attrs[key]
        return v, ok
}

// stateHue maps VM state to a hue degree — the only non-optional projection.
func stateHue(state string) float64 {
        switch state {
        case "running":
                return 120 // green
        case "paused":
                return 60 // yellow
        case "starting":
                return 180 // cyan
        case "error":
                return 0 // red
        default:
                return 240 // blue (stopped / unknown)
        }
}

// ResolveColorURI is a lens over vm://.
// It reads the same VMInstance but projects color semantics from the open Attrs.
// Accumulation happens elsewhere (vm/set-attr!); this only reads.
//
// The entire read is done under a single lock acquisition to guarantee
// snapshot consistency (no torn reads between hue/sat/lit and merged attrs).
//
//   color://{name}          → hashmap with :hue :saturation :lightness :name :source-uri + any attrs
//   color://{name}/hue      → float
//   color://{name}/hex      → string "#RRGGBB"
func ResolveColorURI(uri string) lisp.Value {
        if !strings.HasPrefix(uri, ColorScheme) {
                return lisp.Nil{}
        }

        path := strings.TrimPrefix(uri, ColorScheme)
        if path == "" {
                return lisp.Nil{}
        }
        parts := strings.SplitN(path, "/", 2)
        name := parts[0]
        if name == "" {
                return lisp.Nil{}
        }
        inst, ok := GetVM(name)
        if !ok {
                return lisp.Nil{}
        }

        // Single lock for the entire snapshot — no torn reads.
        inst.mu.Lock()
        state := getStateLocked(inst)
        hue := stateHue(state)
        sat := 0.6
        lit := 0.5

        // Overlay attrs if deposited.
        if inst.Attrs != nil {
                if v, exists := inst.Attrs["hue"]; exists {
                        if f, ok := v.(float64); ok {
                                hue = f
                        }
                }
                if v, exists := inst.Attrs["saturation"]; exists {
                        if f, ok := v.(float64); ok {
                                sat = f
                        }
                }
                if v, exists := inst.Attrs["lightness"]; exists {
                        if f, ok := v.(float64); ok {
                                lit = f
                        }
                }
        }

        // Clamp to valid HSL ranges.
        hue = clampHue(hue)
        sat = clamp01(sat)
        lit = clamp01(lit)

        // Snapshot attrs for sub-resource passthrough and full merge.
        attrsCopy := make(map[string]any, len(inst.Attrs))
        for k, v := range inst.Attrs {
                attrsCopy[k] = v
        }
        inst.mu.Unlock()

        if len(parts) == 2 {
                switch parts[1] {
                case "hue":
                        return lisp.Float(hue)
                case "hex":
                        return lisp.String(hslToHex(hue, sat, lit))
                case "spi":
                        // SPI sub-resource: seed, invocation, fingerprint snapshot.
                        // Read under lock was already done above; use the copy.
                        inst.mu.Lock()
                        seed := inst.Seed
                        inv := inst.Invocation
                        fp := inst.Fingerprint
                        inst.mu.Unlock()
                        m := make(lisp.HashMap)
                        m[lisp.Keyword("seed")] = lisp.Float(float64(seed))
                        m[lisp.Keyword("invocation")] = lisp.Float(float64(inv))
                        m[lisp.Keyword("fingerprint")] = lisp.Float(float64(fp))
                        // Current splitmix64 color at this invocation
                        ch, cs, cl := colorAt(seed, inv)
                        m[lisp.Keyword("current-hex")] = lisp.String(hslToHex(ch, cs, cl))
                        return m
                case "walk":
                        // Walk sub-resource: 8-step random walk from this VM.
                        inst.mu.Lock()
                        seed := inst.Seed
                        inst.mu.Unlock()
                        trail := RandomWalk(name, 8, seed)
                        items := make(lisp.Vector, len(trail))
                        for i, step := range trail {
                                sm := make(lisp.HashMap)
                                sm[lisp.Keyword("step")] = lisp.Float(float64(i))
                                sm[lisp.Keyword("name")] = lisp.String(step.Name)
                                sm[lisp.Keyword("hex")] = lisp.String(step.Hex)
                                items[i] = sm
                        }
                        return items
                default:
                        if v, exists := attrsCopy[parts[1]]; exists {
                                return anyToLisp(v)
                        }
                        return lisp.Nil{}
                }
        }

        // Full projection — built from the already-snapshotted data.
        inst.mu.Lock()
        seed := inst.Seed
        inv := inst.Invocation
        fp := inst.Fingerprint
        inst.mu.Unlock()

        m := make(lisp.HashMap)
        m[lisp.Keyword("name")] = lisp.String(name)
        m[lisp.Keyword("source-uri")] = lisp.String(VMScheme + name)
        m[lisp.Keyword("state")] = lisp.String(state)
        m[lisp.Keyword("hue")] = lisp.Float(hue)
        m[lisp.Keyword("saturation")] = lisp.Float(sat)
        m[lisp.Keyword("lightness")] = lisp.Float(lit)
        m[lisp.Keyword("hex")] = lisp.String(hslToHex(hue, sat, lit))
        m[lisp.Keyword("seed")] = lisp.Float(float64(seed))
        m[lisp.Keyword("invocation")] = lisp.Float(float64(inv))
        m[lisp.Keyword("fingerprint")] = lisp.Float(float64(fp))

        for k, v := range attrsCopy {
                kw := lisp.Keyword(k)
                if _, already := m[kw]; !already {
                        m[kw] = anyToLisp(v)
                }
        }

        return m
}

// getStateLocked reads VM state while caller already holds inst.mu.
func getStateLocked(inst *VMInstance) string {
        if inst.VM == nil {
                return "unknown"
        }
        switch inst.VM.State() {
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
        case vz.VirtualMachineStatePausing:
                return "pausing"
        case vz.VirtualMachineStateResuming:
                return "resuming"
        case vz.VirtualMachineStateStopping:
                return "stopping"
        case vz.VirtualMachineStateSaving:
                return "saving"
        case vz.VirtualMachineStateRestoring:
                return "restoring"
        default:
                return "unknown"
        }
}

func clampHue(h float64) float64 {
        if math.IsNaN(h) || math.IsInf(h, 0) {
                return 0
        }
        h = math.Mod(h, 360.0)
        if h < 0 {
                h += 360.0
        }
        if h >= 360.0 {
                h = 0
        }
        return h
}

func clamp01(v float64) float64 {
        if math.IsNaN(v) || math.IsInf(v, -1) || v < 0 {
                return 0
        }
        if math.IsInf(v, 1) || v > 1 {
                return 1
        }
        return v
}

// --- Gay.jl SplitMix64 bijection (exact constants) ---
//
// GOLDEN = 0x9e3779b97f4a7c15 (golden ratio fractional part × 2^64)
// MIX1   = 0xbf58476d1ce4e5b9
// MIX2   = 0x94d049bb133111eb
//
// splitmix64 is a bijection on uint64 — every output is unique for every input.
// This is the same function used in Gay.jl, Julia's default SplittableRandom, and Java's.
func splitmix64(x uint64) uint64 {
        x += 0x9e3779b97f4a7c15
        x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
        x = (x ^ (x >> 27)) * 0x94d049bb133111eb
        return x ^ (x >> 31)
}

// colorAt returns the HSL color at a given (seed, index) pair.
// Pure function: same (seed, index) → same (h, s, l), always.
// O(1) random access — no need to iterate from 0.
func colorAt(seed, index uint64) (h, s, l float64) {
        mixed := splitmix64(seed ^ index)
        // Use 65536.0 (not 65535.0) so hue stays in [0, 360) — never hits 360 exactly.
        h = float64(mixed&0xFFFF) / 65536.0 * 360.0
        s = 0.5 + float64((mixed>>16)&0xFFFF)/65536.0*0.5 // [0.5, ~1.0)
        l = 0.4 + float64((mixed>>32)&0xFFFF)/65536.0*0.2 // [0.4, ~0.6)
        return h, s, l
}

// seedFromName derives a deterministic uint64 seed from a VM name via FNV-1a.
func seedFromName(name string) uint64 {
        h := fnv.New64a()
        h.Write([]byte(name))
        return h.Sum64()
}

// RandomWalk walks the VM registry for `steps` steps, starting from `startName`.
// At each step, splitmix64(walkSeed ^ step) determines the next VM by index.
// Returns a trail of (name, hex-color) pairs. Same walkSeed + same registry → same trail.
func RandomWalk(startName string, steps int, walkSeed uint64) []struct {
        Name string
        Hex  string
} {
        vmRegistryMu.RLock()
        names := make([]string, 0, len(vmRegistry))
        for n := range vmRegistry {
                names = append(names, n)
        }
        vmRegistryMu.RUnlock()

        if len(names) == 0 {
                return nil
        }
        sort.Strings(names) // deterministic order

        trail := make([]struct {
                Name string
                Hex  string
        }, steps)

        current := startName
        for i := 0; i < steps; i++ {
                idx := splitmix64(walkSeed ^ uint64(i)) % uint64(len(names))
                current = names[idx]
                h, s, l := colorAt(walkSeed, uint64(i))
                trail[i].Name = current
                trail[i].Hex = hslToHex(h, s, l)
        }
        return trail
}

func anyToLisp(v any) lisp.Value {
        switch x := v.(type) {
        case string:
                return lisp.String(x)
        case float64:
                return lisp.Float(x)
        case bool:
                return lisp.Bool(x)
        case nil:
                return lisp.Nil{}
        default:
                return lisp.String(fmt.Sprintf("%v", x))
        }
}

func hslToHex(h, s, l float64) string {
        h = h / 360.0
        var r, g, b float64
        if s == 0 {
                r, g, b = l, l, l
        } else {
                var q float64
                if l < 0.5 {
                        q = l * (1 + s)
                } else {
                        q = l + s - l*s
                }
                p := 2*l - q
                r = hueToRGB(p, q, h+1.0/3.0)
                g = hueToRGB(p, q, h)
                b = hueToRGB(p, q, h-1.0/3.0)
        }
        ri := int(r*255 + 0.5)
        gi := int(g*255 + 0.5)
        bi := int(b*255 + 0.5)
        return fmt.Sprintf("#%02X%02X%02X", ri, gi, bi)
}

func hueToRGB(p, q, t float64) float64 {
        if t < 0 {
                t += 1
        }
        if t > 1 {
                t -= 1
        }
        switch {
        case t < 1.0/6.0:
                return p + (q-p)*6*t
        case t < 1.0/2.0:
                return q
        case t < 2.0/3.0:
                return p + (q-p)*(2.0/3.0-t)*6
        default:
                return p
        }
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
                fn := &lisp.Fn{Name: name, Func: f}
                env.Set(lisp.Symbol(name), fn)
                // Also populate proper namespace: "vz/foo" → boxxy.vz ns, "foo" binding
                if idx := strings.Index(name, "/"); idx > 0 {
                        nsPrefix := name[:idx]
                        localName := name[idx+1:]
                        lisp.InternInNS("boxxy."+nsPrefix, lisp.Symbol(localName), fn)
                }
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

        // -- Nested Virtualization --

        reg("vz/nested-virt-supported?", func(args []lisp.Value) lisp.Value {
                return lisp.Bool(vz.IsNestedVirtualizationSupported())
        })

        // -- XHCI (USB 3.0) Controller --

        reg("vz/new-xhci-controller", func(args []lisp.Value) lisp.Value {
                xhci, err := vz.NewXHCIControllerConfiguration()
                if err != nil {
                        panic(fmt.Sprintf("vz/new-xhci-controller: %v", err))
                }
                return &lisp.ExternalValue{Value: xhci, Type: "XHCIController"}
        })

        // -- Save / Restore State --

        reg("vz/save-state!", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vz/save-state!: requires (vm path)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                path := string(args[1].(lisp.String))
                instance.mu.Lock()
                defer instance.mu.Unlock()
                if err := instance.VM.SaveMachineStateToPath(path); err != nil {
                        panic(fmt.Sprintf("vz/save-state!: %v", err))
                }
                return lisp.Bool(true)
        })

        reg("vz/restore-state!", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vz/restore-state!: requires (vm path)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                path := string(args[1].(lisp.String))
                instance.mu.Lock()
                defer instance.mu.Unlock()
                if err := instance.VM.RestoreMachineStateFromURL(path); err != nil {
                        panic(fmt.Sprintf("vz/restore-state!: %v", err))
                }
                return lisp.Bool(true)
        })

        reg("vz/validate-save-restore", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vz/validate-save-restore: requires (vm)")
                }
                ext := args[0].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                ok, err := instance.Config.ValidateSaveRestoreSupport()
                if err != nil {
                        return lisp.Bool(false)
                }
                return lisp.Bool(ok)
        })

        // -- vm:// URI Resource Protocol (vibespace-mcp pattern) --

        reg("vm/resolve", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("vm/resolve: requires (uri-string)")
                }
                uri := string(args[0].(lisp.String))
                return ResolveVMURI(uri)
        })

        reg("vm/register!", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vm/register!: requires (name vm)")
                }
                name := string(args[0].(lisp.String))
                ext := args[1].(*lisp.ExternalValue)
                instance := ext.Value.(*VMInstance)
                RegisterVM(name, instance)
                return lisp.String(VMScheme + name)
        })

        reg("vm/list-uris", func(args []lisp.Value) lisp.Value {
                vms := ListVMs()
                uris := make(lisp.Vector, 0, len(vms))
                for name := range vms {
                        uris = append(uris, lisp.String(VMScheme+name))
                }
                return uris
        })

        // Open accumulator: deposit any keyword into a VM's Attrs.
        // (vm/set-attr! "alpine" "mood" "calm")
        reg("vm/set-attr!", func(args []lisp.Value) lisp.Value {
                if len(args) < 3 {
                        panic("vm/set-attr!: requires (name key value)")
                }
                name := string(args[0].(lisp.String))
                key := string(args[1].(lisp.String))
                inst, ok := GetVM(name)
                if !ok {
                        return lisp.Nil{}
                }
                var val any
                switch v := args[2].(type) {
                case lisp.String:
                        val = string(v)
                case lisp.Float:
                        val = float64(v)
                case lisp.Int:
                        val = float64(v)
                case lisp.Bool:
                        val = bool(v)
                default:
                        val = v.String()
                }
                SetAttr(inst, key, val)
                return lisp.Keyword(key)
        })

        // (vm/get-attr "alpine" "mood") → value or nil
        reg("vm/get-attr", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("vm/get-attr: requires (name key)")
                }
                name := string(args[0].(lisp.String))
                key := string(args[1].(lisp.String))
                inst, ok := GetVM(name)
                if !ok {
                        return lisp.Nil{}
                }
                v, ok := GetAttr(inst, key)
                if !ok {
                        return lisp.Nil{}
                }
                return anyToLisp(v)
        })

        // color:// lens: (color/resolve "color://alpine") → hashmap with color projection
        reg("color/resolve", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("color/resolve: requires (uri)")
                }
                uri := string(args[0].(lisp.String))
                return ResolveColorURI(uri)
        })

        // (color/walk "alpine" 16 42) → vector of {step, name, hex} hashmaps
        // walkSeed is optional; defaults to the VM's own seed.
        reg("color/walk", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("color/walk: requires (name [steps] [walk-seed])")
                }
                name := string(args[0].(lisp.String))
                steps := 8
                if len(args) >= 2 {
                        steps = int(args[1].(lisp.Int))
                }
                var ws uint64
                if len(args) >= 3 {
                        ws = uint64(args[2].(lisp.Int))
                } else {
                        inst, ok := GetVM(name)
                        if ok {
                                inst.mu.Lock()
                                ws = inst.Seed
                                inst.mu.Unlock()
                        }
                }
                trail := RandomWalk(name, steps, ws)
                items := make(lisp.Vector, len(trail))
                for i, step := range trail {
                        sm := make(lisp.HashMap)
                        sm[lisp.Keyword("step")] = lisp.Float(float64(i))
                        sm[lisp.Keyword("name")] = lisp.String(step.Name)
                        sm[lisp.Keyword("hex")] = lisp.String(step.Hex)
                        items[i] = sm
                }
                return items
        })

        // (color/spi-fingerprint "alpine") → {:seed :invocation :fingerprint :current-hex}
        reg("color/spi-fingerprint", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("color/spi-fingerprint: requires (name)")
                }
                name := string(args[0].(lisp.String))
                inst, ok := GetVM(name)
                if !ok {
                        return lisp.Nil{}
                }
                inst.mu.Lock()
                seed := inst.Seed
                inv := inst.Invocation
                fp := inst.Fingerprint
                inst.mu.Unlock()
                m := make(lisp.HashMap)
                m[lisp.Keyword("seed")] = lisp.Float(float64(seed))
                m[lisp.Keyword("invocation")] = lisp.Float(float64(inv))
                m[lisp.Keyword("fingerprint")] = lisp.Float(float64(fp))
                ch, cs, cl := colorAt(seed, inv)
                m[lisp.Keyword("current-hex")] = lisp.String(hslToHex(ch, cs, cl))
                return m
        })

        // (color/fuzz-step "alpine") → advance invocation + return new color hex
        reg("color/fuzz-step", func(args []lisp.Value) lisp.Value {
                if len(args) < 1 {
                        panic("color/fuzz-step: requires (name)")
                }
                name := string(args[0].(lisp.String))
                inst, ok := GetVM(name)
                if !ok {
                        return lisp.Nil{}
                }
                inst.mu.Lock()
                inst.Invocation++
                inst.Fingerprint ^= splitmix64(inst.Seed ^ inst.Invocation)
                h, s, l := colorAt(inst.Seed, inst.Invocation)
                inst.mu.Unlock()
                return lisp.String(hslToHex(h, s, l))
        })

        // -- Trace / Functoriality --

        // (trace/record-splitmix64 42 100 999) → record trace, return fingerprint hex
        reg("trace/record-splitmix64", func(args []lisp.Value) lisp.Value {
                inputs := make([]uint64, len(args))
                for i, a := range args {
                        inputs[i] = uint64(a.(lisp.Float))
                }
                bt := globalTraceCache.RecordSplitMix64(inputs)
                return lisp.String(fmt.Sprintf("%x", bt.Fingerprint))
        })

        // (trace/record-color-at seed1 idx1 seed2 idx2 ...) → record pairs, return fingerprint
        reg("trace/record-color-at", func(args []lisp.Value) lisp.Value {
                if len(args)%2 != 0 {
                        panic("trace/record-color-at: requires even number of args (seed idx pairs)")
                }
                pairs := make([][2]uint64, len(args)/2)
                for i := 0; i < len(args); i += 2 {
                        pairs[i/2] = [2]uint64{uint64(args[i].(lisp.Float)), uint64(args[i+1].(lisp.Float))}
                }
                bt := globalTraceCache.RecordColorAt(pairs)
                return lisp.String(fmt.Sprintf("%x", bt.Fingerprint))
        })

        // (trace/record-seed-from-name "alice" "bob") → record name→seed trace
        reg("trace/record-seed-from-name", func(args []lisp.Value) lisp.Value {
                names := make([]string, len(args))
                for i, a := range args {
                        names[i] = string(a.(lisp.String))
                }
                bt := globalTraceCache.RecordSeedFromName(names)
                return lisp.String(fmt.Sprintf("%x", bt.Fingerprint))
        })

        // (trace/behavioral-equal "splitmix64" "splitmix64-copy") → true/false/nil
        reg("trace/behavioral-equal", func(args []lisp.Value) lisp.Value {
                if len(args) < 2 {
                        panic("trace/behavioral-equal: requires (name-a name-b)")
                }
                a := string(args[0].(lisp.String))
                b := string(args[1].(lisp.String))
                eq, err := globalTraceCache.BehaviorallyEqual(a, b)
                if err != nil {
                        return lisp.Nil{}
                }
                return lisp.Bool(eq)
        })

        // (trace/verify-functoriality n) → {:preserved true :count n :composed "..." :decomposed "..."}
        reg("trace/verify-functoriality", func(args []lisp.Value) lisp.Value {
                n := 100
                if len(args) > 0 {
                        n = int(args[0].(lisp.Float))
                }
                pairs := make([][2]uint64, n)
                for i := 0; i < n; i++ {
                        pairs[i] = [2]uint64{uint64(i * 7), uint64(i)}
                }
                r := globalTraceCache.VerifyFunctoriality(pairs)
                m := make(lisp.HashMap)
                m[lisp.Keyword("preserved")] = lisp.Bool(r.Preserved)
                m[lisp.Keyword("count")] = lisp.Float(float64(r.InputCount))
                m[lisp.Keyword("composed")] = lisp.String(fmt.Sprintf("%x", r.ComposedFingerprint))
                m[lisp.Keyword("decomposed")] = lisp.String(fmt.Sprintf("%x", r.DecomposedFingerprint))
                return m
        })

        // (trace/verify-e2e "alice" "bob" "carol") → {:preserved true :names 3 ...}
        reg("trace/verify-e2e", func(args []lisp.Value) lisp.Value {
                names := make([]string, len(args))
                for i, a := range args {
                        names[i] = string(a.(lisp.String))
                }
                r := globalTraceCache.VerifyEndToEndFunctoriality(names, 1)
                m := make(lisp.HashMap)
                m[lisp.Keyword("preserved")] = lisp.Bool(r.Preserved)
                m[lisp.Keyword("names")] = lisp.Float(float64(len(r.Names)))
                m[lisp.Keyword("composed")] = lisp.String(fmt.Sprintf("%x", r.ComposedFingerprint))
                m[lisp.Keyword("decomposed")] = lisp.String(fmt.Sprintf("%x", r.DecomposedFingerprint))
                return m
        })

        lisp.SetupDefaultAliases()
}

// package-level TraceCache singleton (matches vmRegistry pattern)
var globalTraceCache = NewTraceCache()

// GlobalTraceCache returns the package-level trace cache.
func GlobalTraceCache() *TraceCache { return globalTraceCache }

// Silence unused import warnings
var _ = io.EOF
