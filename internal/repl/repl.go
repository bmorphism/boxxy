//go:build darwin

package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bmorphism/boxxy/internal/lisp"
	"github.com/bmorphism/boxxy/internal/vm"
)

func Start() {
	fmt.Println("boxxy REPL - Clojure SCI for Apple Virtualization.framework")
	fmt.Println("Type (help) for commands, (quit) to exit")
	fmt.Println()

	// Create environment with standard functions
	env := lisp.CreateStandardEnv()
	
	// Register vz namespace
	vm.RegisterNamespace(env)

	// Add REPL-specific commands
	env.Set("help", &lisp.Fn{"help", func(args []lisp.Value) lisp.Value {
		printHelp()
		return lisp.Nil{}
	}})

	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("boxxy=> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Handle special commands
		switch line {
		case "(quit)", "(exit)":
			fmt.Println("Goodbye!")
			return
		case "(help)":
			printHelp()
			continue
		}

		// Evaluate expression
		result := evalString(env, line)
		if result != nil {
			if _, ok := result.(lisp.Nil); !ok {
				fmt.Println(result)
			}
		}
	}
}

func evalString(env *lisp.Env, s string) lisp.Value {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Error: %v\n", r)
		}
	}()

	reader := lisp.NewReader(strings.NewReader(s))
	obj, err := reader.Read()
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return nil
	}

	return lisp.Eval(obj, env)
}

func printHelp() {
	fmt.Print(`
boxxy REPL Commands:

VM Creation:
  (vz/new-efi-variable-store path create?)   Create EFI NVRAM
  (vz/new-efi-boot-loader store)             EFI boot (HaikuOS, FreeBSD)
  (vz/new-linux-boot-loader kernel initrd cmdline)  Linux boot
  (vz/new-macos-boot-loader)                 macOS boot

  (vz/new-generic-platform)     Generic platform config
  (vz/new-disk-attachment path read-only?)   Disk/ISO attachment
  (vz/new-virtio-block-device att)           Virtio disk
  (vz/new-usb-mass-storage att)              USB storage (ISOs)
  (vz/new-nat-network)                       NAT network
  (vz/new-virtio-network att)                Virtio network

  (vz/new-vm-config cpus mem-gb boot platform)  Create config
  (vz/add-storage-devices config devices)    Add storage
  (vz/add-network-devices config devices)    Add network
  (vz/validate-config config)                Validate
  (vz/new-vm config)                         Create VM

VM Control:
  (vz/start-vm! vm)             Start VM
  (vz/stop-vm! vm)              Stop VM
  (vz/pause-vm! vm)             Pause VM
  (vz/resume-vm! vm)            Resume VM
  (vz/vm-state vm)              Get state

Utilities:
  (vz/create-disk-image path size-gb)  Create disk image

(quit)                          Exit REPL
`)
}
