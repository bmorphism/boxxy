//go:build darwin

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/Code-Hex/vz/v3"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	isoPath := "/Users/bob/Downloads/haiku/haiku-r1beta5-x86_64-anyboot.iso"
	diskPath := "/Users/bob/Downloads/haiku/haiku.img"
	nvramPath := "/Users/bob/Downloads/haiku/haiku.nvram"
	cpus := uint(4)
	memoryGB := uint64(8)

	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║  boxxy - HaikuOS Virtual Machine (GUI)     ║")
	fmt.Println("║  Apple Virtualization.framework            ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Println()

	// Lock OS thread early - required for macOS GUI operations
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Create EFI variable store
	store, err := vz.NewEFIVariableStore(nvramPath, vz.WithCreatingEFIVariableStore())
	if err != nil {
		store, err = vz.NewEFIVariableStore(nvramPath)
		if err != nil {
			return fmt.Errorf("failed to create EFI store: %w", err)
		}
	}

	// Create EFI boot loader
	bootLoader, err := vz.NewEFIBootLoader(vz.WithEFIVariableStore(store))
	if err != nil {
		return fmt.Errorf("failed to create EFI boot loader: %w", err)
	}

	// Create platform
	platform, err := vz.NewGenericPlatformConfiguration()
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}

	// Create VM config
	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		cpus,
		memoryGB*1024*1024*1024,
	)
	if err != nil {
		return fmt.Errorf("failed to create VM config: %w", err)
	}
	vmConfig.SetPlatformVirtualMachineConfiguration(platform)

	// Storage devices
	var storageDevices []vz.StorageDeviceConfiguration

	// ISO
	isoAtt, err := vz.NewDiskImageStorageDeviceAttachment(isoPath, true)
	if err != nil {
		return fmt.Errorf("failed to attach ISO: %w", err)
	}
	usb, err := vz.NewUSBMassStorageDeviceConfiguration(isoAtt)
	if err != nil {
		return fmt.Errorf("failed to create USB storage: %w", err)
	}
	storageDevices = append(storageDevices, usb)

	// Disk - create if not exists
	if _, err := os.Stat(diskPath); os.IsNotExist(err) {
		fmt.Println("Creating disk image...")
		if err := vz.CreateDiskImage(diskPath, 32*1024*1024*1024); err != nil {
			return fmt.Errorf("failed to create disk: %w", err)
		}
	}

	diskAtt, err := vz.NewDiskImageStorageDeviceAttachment(diskPath, false)
	if err != nil {
		return fmt.Errorf("failed to attach disk: %w", err)
	}
	virtioBlock, err := vz.NewVirtioBlockDeviceConfiguration(diskAtt)
	if err != nil {
		return fmt.Errorf("failed to create virtio block: %w", err)
	}
	storageDevices = append(storageDevices, virtioBlock)
	vmConfig.SetStorageDevicesVirtualMachineConfiguration(storageDevices)

	// Network
	natAtt, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		return fmt.Errorf("failed to create NAT: %w", err)
	}
	virtioNet, err := vz.NewVirtioNetworkDeviceConfiguration(natAtt)
	if err != nil {
		return fmt.Errorf("failed to create virtio network: %w", err)
	}
	vmConfig.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{virtioNet})

	// Graphics
	scanout, err := vz.NewVirtioGraphicsScanoutConfiguration(1920, 1200)
	if err != nil {
		return fmt.Errorf("failed to create graphics scanout: %w", err)
	}
	graphics, err := vz.NewVirtioGraphicsDeviceConfiguration()
	if err != nil {
		return fmt.Errorf("failed to create graphics device: %w", err)
	}
	graphics.SetScanouts(scanout)
	vmConfig.SetGraphicsDevicesVirtualMachineConfiguration([]vz.GraphicsDeviceConfiguration{graphics})

	// Keyboard
	keyboard, err := vz.NewUSBKeyboardConfiguration()
	if err != nil {
		return fmt.Errorf("failed to create keyboard: %w", err)
	}
	vmConfig.SetKeyboardsVirtualMachineConfiguration([]vz.KeyboardConfiguration{keyboard})

	// Pointer
	pointer, err := vz.NewUSBScreenCoordinatePointingDeviceConfiguration()
	if err != nil {
		return fmt.Errorf("failed to create pointer: %w", err)
	}
	vmConfig.SetPointingDevicesVirtualMachineConfiguration([]vz.PointingDeviceConfiguration{pointer})

	// Serial console for debugging
	serialAtt, err := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to create serial attachment: %w", err)
	}
	serialPort, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialAtt)
	if err != nil {
		return fmt.Errorf("failed to create serial port: %w", err)
	}
	vmConfig.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{serialPort})

	// Entropy
	entropy, err := vz.NewVirtioEntropyDeviceConfiguration()
	if err != nil {
		return fmt.Errorf("failed to create entropy device: %w", err)
	}
	vmConfig.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{entropy})

	// Validate
	ok, err := vmConfig.Validate()
	if err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("config validation returned false")
	}

	fmt.Println("Configuration valid, creating VM...")

	// Create VM
	vm, err := vz.NewVirtualMachine(vmConfig)
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	fmt.Println("Starting HaikuOS VM...")

	// Start VM
	if err := vm.Start(); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	// Monitor VM state in background
	errCh := make(chan error, 1)
	go func() {
		for {
			select {
			case newState := <-vm.StateChangedNotify():
				if newState == vz.VirtualMachineStateRunning {
					log.Println("VM is running")
				}
				if newState == vz.VirtualMachineStateStopped || newState == vz.VirtualMachineStateStopping {
					log.Println("VM stopped")
					errCh <- nil
					return
				}
			}
		}
	}()

	fmt.Println("VM started, opening graphics window...")
	fmt.Println("Close the window to stop the VM.")

	// Start graphics application (this blocks until window is closed)
	// WithController(true) enables the VM controller UI
	vm.StartGraphicApplication(1920, 1200,
		vz.WithWindowTitle("HaikuOS - boxxy"),
		vz.WithController(true))

	// Cleanup after window closes
	cleanup(vm)

	return <-errCh
}

func cleanup(vm *vz.VirtualMachine) {
	for i := 1; vm.CanRequestStop(); i++ {
		result, err := vm.RequestStop()
		log.Printf("Sent stop request(%d): %t, %v", i, result, err)
		time.Sleep(time.Second * 3)
		if i > 3 {
			log.Println("Force stopping VM...")
			if err := vm.Stop(); err != nil {
				log.Println("Stop error:", err)
			}
			break
		}
	}
	log.Println("Cleanup finished")
}
