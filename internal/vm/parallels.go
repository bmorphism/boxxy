//go:build darwin

package vm

import (
	"fmt"
	"os/exec"
	"strings"
)

const prlctlPath = "/Applications/Parallels Desktop.app/Contents/MacOS/prlctl"

// ParallelsVM wraps prlctl for managing an existing Parallels VM
type ParallelsVM struct {
	Name string
}

func NewParallelsVM(name string) *ParallelsVM {
	return &ParallelsVM{Name: name}
}

func (p *ParallelsVM) prlctl(args ...string) (string, error) {
	cmd := exec.Command(prlctlPath, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Start boots the VM
func (p *ParallelsVM) Start() (string, error) {
	return p.prlctl("start", p.Name)
}

// Stop shuts down the VM
func (p *ParallelsVM) Stop() (string, error) {
	return p.prlctl("stop", p.Name)
}

// Suspend suspends the VM
func (p *ParallelsVM) Suspend() (string, error) {
	return p.prlctl("suspend", p.Name)
}

// Status returns the VM state
func (p *ParallelsVM) Status() (string, error) {
	out, err := p.prlctl("status", p.Name)
	return out, err
}

// ListUSB lists available USB devices on the host
func (p *ParallelsVM) ListUSB() (string, error) {
	return p.prlctl("set", p.Name, "--device-list")
}

// ConnectUSB connects a USB device to the VM by its friendly name or address
func (p *ParallelsVM) ConnectUSB(deviceID string) (string, error) {
	return p.prlctl("set", p.Name, "--device-connect", deviceID)
}

// DisconnectUSB disconnects a USB device from the VM
func (p *ParallelsVM) DisconnectUSB(deviceID string) (string, error) {
	return p.prlctl("set", p.Name, "--device-disconnect", deviceID)
}

// SharedFolder adds a host directory as a shared folder in the VM
func (p *ParallelsVM) AddSharedFolder(name, path string) (string, error) {
	return p.prlctl("set", p.Name, "--shf-host-add", name, "--path", path)
}

// Exec runs a command inside the guest (requires Parallels Tools installed)
func (p *ParallelsVM) Exec(command string) (string, error) {
	return p.prlctl("exec", p.Name, "cmd", "/c", command)
}

// Summary returns VM info
func (p *ParallelsVM) Summary() string {
	status, _ := p.Status()
	return fmt.Sprintf("Parallels VM: %s\n%s\n", p.Name, status)
}
