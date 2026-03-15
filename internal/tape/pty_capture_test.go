//go:build darwin

package tape

import (
	"testing"
	"time"
)

func TestSessionColorStream(t *testing.T) {
	scs := NewSessionColorStream()

	// Before any frames, should use fallback palette
	c0 := scs.ColorFor(0)
	if c0 == "" {
		t.Fatal("expected non-empty color before seeding")
	}

	// Feed frames to seed the entropy
	for i := 0; i < 5; i++ {
		scs.FeedFrame(Frame{
			Width:   80,
			Height:  24,
			Content: "test content line",
		})
	}

	// After seeding, colors should come from the golden angle stream
	c1 := scs.ColorFor(1)
	c2 := scs.ColorFor(2)
	if c1 == c2 {
		t.Fatalf("expected different colors for different indices, got %s and %s", c1, c2)
	}

	seed := scs.Seed()
	if seed == 0 {
		t.Fatal("expected non-zero seed after feeding frames")
	}
}

func TestPTYCaptureFunc(t *testing.T) {
	capFn := PTYCaptureFunc()
	content, w, h, err := capFn()
	if err != nil {
		t.Fatalf("pty capture error: %v", err)
	}
	if w <= 0 || h <= 0 {
		t.Fatalf("expected positive dimensions, got %dx%d", w, h)
	}
	if content == "" {
		t.Fatal("expected non-empty content from pty capture")
	}
}

func TestProcessListCaptureFunc(t *testing.T) {
	capFn := ProcessListCaptureFunc()
	content, w, h, err := capFn()
	if err != nil {
		t.Fatalf("process list capture error: %v", err)
	}
	if w <= 0 || h <= 0 {
		t.Fatalf("expected positive dimensions, got %dx%d", w, h)
	}
	if content == "" {
		t.Fatal("expected non-empty process list")
	}
	if len(content) < 10 {
		t.Fatalf("process list suspiciously short: %q", content)
	}
}

func TestSSHCaptureFuncFallback(t *testing.T) {
	// Use a non-existent host to test the error fallback
	capFn := SSHCaptureFunc("nonexistent.invalid.host.test")
	content, w, h, err := capFn()
	if err != nil {
		t.Fatalf("ssh capture should not return error (has fallback): %v", err)
	}
	if w != 80 || h != 24 {
		t.Fatalf("expected 80x24 fallback, got %dx%d", w, h)
	}
	if content == "" {
		t.Fatal("expected non-empty fallback content")
	}
}

func TestColoredRecorderIntegration(t *testing.T) {
	scs := NewSessionColorStream()
	callCount := 0
	capFn := func() (string, int, int, error) {
		callCount++
		return "colored frame", 80, 24, nil
	}

	rec := NewRecorder("color-node", "color-test", capFn)
	rec.OnFrame(func(f Frame) {
		scs.FeedFrame(f)
	})

	rec.Start()
	time.Sleep(2500 * time.Millisecond)
	tape := rec.Stop()

	if tape.Len() < 2 {
		t.Fatalf("expected at least 2 frames, got %d", tape.Len())
	}

	// Color stream should be seeded
	if scs.Seed() == 0 {
		t.Fatal("expected color stream to be seeded after recording")
	}

	// Colors should be unique per index
	colors := make(map[string]bool)
	for i := 0; i < 10; i++ {
		c := scs.ColorFor(i)
		colors[c] = true
	}
	if len(colors) < 5 {
		t.Fatalf("expected diverse colors, got only %d unique", len(colors))
	}
}
