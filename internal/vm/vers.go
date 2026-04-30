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
)

// VersState represents lifecycle state for a vers.sh VM
type VersState int

const (
	VersStateNone    VersState = iota // no VM
	VersStateRunning                  // VM running (vers run)
	VersStatePaused                   // VM paused (snapshot available)
)

// VersVM wraps the vers CLI for Git-like VM branching
type VersVM struct {
	Name     string
	VMID     string // vers VM identifier
	Branch   string // current branch name
	BaseDir  string // ~/.boxxy/vers/<name>
	instance *VersInstance
}

// VersInstance represents a running vers VM
type VersInstance struct {
	VMID      string `json:"vm_id"`
	Branch    string `json:"branch"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// VersBranch represents a branch of a VM
type VersBranch struct {
	Name     string `json:"name"`
	CommitID string `json:"commit_id,omitempty"`
	Current  bool   `json:"current"`
}

func NewVersVM(name string) *VersVM {
	home, _ := os.UserHomeDir()
	return &VersVM{
		Name:    name,
		Branch:  "main",
		BaseDir: filepath.Join(home, ".boxxy", "vers", name),
	}
}

func (v *VersVM) versCmd(ctx context.Context, args ...string) (string, error) {
	versPath, err := exec.LookPath("vers")
	if err != nil {
		return "", fmt.Errorf("vers CLI not found: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, versPath, args...)
	cmd.Dir = v.BaseDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// State returns the current lifecycle state
func (v *VersVM) State() VersState {
	if v.instance == nil {
		return VersStateNone
	}
	switch v.instance.Status {
	case "running":
		return VersStateRunning
	case "paused":
		return VersStatePaused
	default:
		return VersStateNone
	}
}

func (v *VersVM) StateString() string {
	switch v.State() {
	case VersStateNone:
		return "none"
	case VersStateRunning:
		return "running"
	case VersStatePaused:
		return "paused (snapshot)"
	default:
		return "unknown"
	}
}

// Run starts a new vers VM
func (v *VersVM) Run(ctx context.Context, image string) error {
	if err := os.MkdirAll(v.BaseDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	out, err := v.versCmd(ctx, "run", image)
	if err != nil {
		return fmt.Errorf("vers run: %w\n%s", err, out)
	}

	// Parse VM ID from output
	v.VMID = strings.TrimSpace(out)
	v.instance = &VersInstance{
		VMID:      v.VMID,
		Branch:    "main",
		Status:    "running",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	return v.SaveMetadata()
}

// Execute runs a command inside the vers VM
func (v *VersVM) Execute(ctx context.Context, command string) (string, error) {
	if v.VMID == "" {
		return "", fmt.Errorf("no running vers VM")
	}
	return v.versCmd(ctx, "execute", v.VMID, "--", "sh", "-c", command)
}

// BranchVM creates a new branch (snapshot + fork)
func (v *VersVM) BranchVM(ctx context.Context, branchName string) error {
	if v.VMID == "" {
		return fmt.Errorf("no running vers VM")
	}
	out, err := v.versCmd(ctx, "branch", v.VMID, branchName)
	if err != nil {
		return fmt.Errorf("vers branch: %w\n%s", err, out)
	}
	return nil
}

// Checkout switches to a different branch
func (v *VersVM) Checkout(ctx context.Context, branchName string) error {
	if v.VMID == "" {
		return fmt.Errorf("no running vers VM")
	}
	out, err := v.versCmd(ctx, "checkout", v.VMID, branchName)
	if err != nil {
		return fmt.Errorf("vers checkout: %w\n%s", err, out)
	}
	v.Branch = branchName
	if v.instance != nil {
		v.instance.Branch = branchName
	}
	return v.SaveMetadata()
}

// Connect attaches to a running vers VM (SSH or serial)
func (v *VersVM) Connect(ctx context.Context) error {
	if v.VMID == "" {
		return fmt.Errorf("no running vers VM")
	}
	// Connect is interactive — exec directly
	versPath, _ := exec.LookPath("vers")
	cmd := exec.CommandContext(ctx, versPath, "connect", v.VMID)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Copy transfers a file to/from the vers VM
func (v *VersVM) Copy(ctx context.Context, src, dst string) error {
	if v.VMID == "" {
		return fmt.Errorf("no running vers VM")
	}
	out, err := v.versCmd(ctx, "copy", src, dst)
	if err != nil {
		return fmt.Errorf("vers copy: %w\n%s", err, out)
	}
	return nil
}

// Stop pauses/stops the vers VM
func (v *VersVM) Stop(ctx context.Context) error {
	if v.VMID == "" {
		return fmt.Errorf("no running vers VM")
	}
	out, err := v.versCmd(ctx, "stop", v.VMID)
	if err != nil {
		return fmt.Errorf("vers stop: %w\n%s", err, out)
	}
	if v.instance != nil {
		v.instance.Status = "paused"
	}
	return v.SaveMetadata()
}

// ListBranches returns all branches for this VM
func (v *VersVM) ListBranches(ctx context.Context) ([]VersBranch, error) {
	if v.VMID == "" {
		return nil, fmt.Errorf("no running vers VM")
	}
	out, err := v.versCmd(ctx, "branch", v.VMID, "--list")
	if err != nil {
		// Fallback: parse text output
		var branches []VersBranch
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			current := strings.HasPrefix(line, "* ")
			name := strings.TrimPrefix(line, "* ")
			branches = append(branches, VersBranch{Name: name, Current: current})
		}
		return branches, nil
	}
	var branches []VersBranch
	if err := json.Unmarshal([]byte(out), &branches); err != nil {
		// Text fallback
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				branches = append(branches, VersBranch{Name: line})
			}
		}
	}
	return branches, nil
}

// Instance returns the current instance
func (v *VersVM) Instance() *VersInstance {
	return v.instance
}

// Summary returns a status string
func (v *VersVM) Summary() string {
	s := fmt.Sprintf("Vers VM: %s\nState: %s\n", v.Name, v.StateString())
	if v.VMID != "" {
		s += fmt.Sprintf("VM ID: %s\n", v.VMID)
	}
	s += fmt.Sprintf("Branch: %s\nPath: %s\n", v.Branch, v.BaseDir)
	return s
}

// SaveMetadata persists VM info to disk
func (v *VersVM) SaveMetadata() error {
	if v.instance == nil {
		return nil
	}
	if err := os.MkdirAll(v.BaseDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v.instance, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(v.BaseDir, "instance.json"), data, 0644)
}

// LoadMetadata restores VM info from disk
func (v *VersVM) LoadMetadata() error {
	path := filepath.Join(v.BaseDir, "instance.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var inst VersInstance
	if err := json.Unmarshal(data, &inst); err != nil {
		return err
	}
	v.instance = &inst
	v.VMID = inst.VMID
	v.Branch = inst.Branch
	return nil
}
