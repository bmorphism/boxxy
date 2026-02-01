package boxxy_test

import (
	"os/exec"
	"testing"
)

func TestHofJoke(t *testing.T) {
	if _, err := exec.LookPath("joker"); err != nil {
		t.Skip("joker not installed")
	}
	cmd := exec.Command("joker", "hof.joke")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("joker hof.joke failed: %v\n%s", err, string(out))
	}
}

func TestHofCue(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	cmd := exec.Command("cue", "vet", "hof.cue")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cue vet hof.cue failed: %v\n%s", err, string(out))
	}
}
