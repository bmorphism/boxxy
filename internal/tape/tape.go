//go:build darwin

package tape

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// Frame is a single 1-FPS terminal snapshot with causal metadata.
type Frame struct {
	SeqNo     uint64            `json:"seq"`
	LamportTS uint64            `json:"lamport"`
	NodeID    string            `json:"node"`
	WallTime  time.Time         `json:"wall"`
	Width     int               `json:"w"`
	Height    int               `json:"h"`
	Content   string            `json:"content"`
	Trit      gf3.Elem          `json:"trit"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// Tape is an ordered sequence of frames from one or more nodes.
type Tape struct {
	mu      sync.RWMutex
	NodeID  string  `json:"node_id"`
	Label   string  `json:"label"`
	Frames  []Frame `json:"frames"`
	Created time.Time `json:"created"`
}

// NewTape creates a new tape for the given node.
func NewTape(nodeID, label string) *Tape {
	return &Tape{
		NodeID:  nodeID,
		Label:   label,
		Created: time.Now(),
	}
}

// Append adds a frame to the tape.
func (t *Tape) Append(f Frame) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Frames = append(t.Frames, f)
}

// Len returns the number of frames.
func (t *Tape) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.Frames)
}

// At returns the frame at index i.
func (t *Tape) At(i int) (Frame, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if i < 0 || i >= len(t.Frames) {
		return Frame{}, false
	}
	return t.Frames[i], true
}

// Last returns the most recent frame.
func (t *Tape) Last() (Frame, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.Frames) == 0 {
		return Frame{}, false
	}
	return t.Frames[len(t.Frames)-1], true
}

// Duration returns the wall-clock duration of the tape.
func (t *Tape) Duration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.Frames) < 2 {
		return 0
	}
	return t.Frames[len(t.Frames)-1].WallTime.Sub(t.Frames[0].WallTime)
}

// Recorder captures terminal frames at 1 FPS with causal timestamps.
type Recorder struct {
	mu      sync.Mutex
	tape    *Tape
	clock   *LamportClock
	vclock  *VectorClock
	seq     uint64
	capture CaptureFunc
	stop    chan struct{}
	done    chan struct{}
	running bool
	onFrame func(Frame) // callback for network broadcast
}

// CaptureFunc reads the current terminal content.
// Implementations can read from a pty, a remote SSH session, etc.
type CaptureFunc func() (content string, width, height int, err error)

// NewRecorder creates a recorder that captures at 1 FPS.
func NewRecorder(nodeID, label string, capture CaptureFunc) *Recorder {
	return &Recorder{
		tape:    NewTape(nodeID, label),
		clock:   NewLamportClock(nodeID),
		vclock:  NewVectorClock(),
		capture: capture,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// OnFrame sets a callback invoked on each captured frame (for network broadcast).
func (r *Recorder) OnFrame(fn func(Frame)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onFrame = fn
}

// Start begins recording at 1 FPS.
func (r *Recorder) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("recorder already running")
	}
	r.running = true
	r.mu.Unlock()

	go r.loop()
	return nil
}

// Stop halts recording and returns the tape.
func (r *Recorder) Stop() *Tape {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return r.tape
	}
	r.running = false
	r.mu.Unlock()

	close(r.stop)
	<-r.done
	return r.tape
}

// Tape returns the current tape (read-only snapshot).
func (r *Recorder) Tape() *Tape {
	return r.tape
}

// ApplyParams wraps the capture function with evolved CaptureParams,
// applying diff-thresholding, content truncation, and compression.
// This is the hot-swap mechanism: the DGM daemon calls this when
// it discovers a better capture strategy.
func (r *Recorder) ApplyParams(params CaptureParams) {
	r.mu.Lock()
	defer r.mu.Unlock()

	original := r.capture
	var lastContent string

	r.capture = func() (string, int, int, error) {
		content, w, h, err := original()
		if err != nil {
			return content, w, h, err
		}

		// Apply max content length
		if params.MaxContentLen > 0 && len(content) > params.MaxContentLen {
			content = content[:params.MaxContentLen]
		}

		// Apply diff threshold (skip near-identical frames)
		if params.DiffThreshold > 0 && lastContent != "" {
			diff := diffRatio(lastContent, content)
			if diff < params.DiffThreshold {
				return "", 0, 0, fmt.Errorf("below diff threshold")
			}
		}

		// Apply compression (skip exact duplicates)
		if params.CompressFrames && content == lastContent {
			return "", 0, 0, fmt.Errorf("duplicate frame compressed")
		}

		lastContent = content
		return content, w, h, nil
	}
}

// IngestRemoteFrame processes a frame received from the network,
// advancing the Lamport clock to maintain causal ordering.
func (r *Recorder) IngestRemoteFrame(f Frame) {
	r.clock.Witness(f.LamportTS)
	r.vclock.Merge(map[string]uint64{f.NodeID: f.LamportTS})
	r.tape.Append(f)
}

func (r *Recorder) loop() {
	defer close(r.done)

	ticker := time.NewTicker(time.Second) // 1 FPS
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.captureFrame()
		}
	}
}

func (r *Recorder) captureFrame() {
	content, w, h, err := r.capture()
	if err != nil {
		return
	}

	r.mu.Lock()
	r.seq++
	seq := r.seq
	cb := r.onFrame
	r.mu.Unlock()

	ts := r.clock.Tick()
	r.vclock.Increment(r.tape.NodeID)

	f := Frame{
		SeqNo:     seq,
		LamportTS: ts,
		NodeID:    r.tape.NodeID,
		WallTime:  time.Now(),
		Width:     w,
		Height:    h,
		Content:   content,
		Trit:      gf3.Elem(seq % 3),
	}

	r.tape.Append(f)

	if cb != nil {
		cb(f)
	}
}

// Player replays a tape at 1 FPS to a writer.
type Player struct {
	tape   *Tape
	writer io.Writer
	speed  float64
}

// NewPlayer creates a tape player.
func NewPlayer(tape *Tape, w io.Writer) *Player {
	return &Player{tape: tape, writer: w, speed: 1.0}
}

// SetSpeed sets playback speed multiplier.
func (p *Player) SetSpeed(s float64) {
	if s > 0 {
		p.speed = s
	}
}

// Play replays the tape, blocking until complete or context cancelled.
func (p *Player) Play(stop <-chan struct{}) error {
	p.tape.mu.RLock()
	frames := make([]Frame, len(p.tape.Frames))
	copy(frames, p.tape.Frames)
	p.tape.mu.RUnlock()

	for i, f := range frames {
		select {
		case <-stop:
			return nil
		default:
		}

		// Clear screen and write frame
		fmt.Fprint(p.writer, "\033[2J\033[H")
		fmt.Fprint(p.writer, f.Content)
		fmt.Fprintf(p.writer, "\n\033[7m [%s] frame %d/%d  lamport=%d  trit=%s \033[0m",
			f.NodeID, i+1, len(frames), f.LamportTS, f.Trit)

		if i < len(frames)-1 {
			delay := time.Second
			if p.speed != 1.0 {
				delay = time.Duration(float64(delay) / p.speed)
			}
			select {
			case <-stop:
				return nil
			case <-time.After(delay):
			}
		}
	}
	return nil
}

// SaveJSONL writes the tape as newline-delimited JSON.
func (t *Tape) SaveJSONL(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Write header
	header := map[string]interface{}{
		"type":    "tape-header",
		"node_id": t.NodeID,
		"label":   t.Label,
		"created": t.Created,
		"frames":  len(t.Frames),
	}
	if err := enc.Encode(header); err != nil {
		return err
	}

	for _, frame := range t.Frames {
		if err := enc.Encode(frame); err != nil {
			return err
		}
	}
	return nil
}

// LoadJSONL loads a tape from newline-delimited JSON.
func LoadJSONL(path string) (*Tape, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)

	// Read header
	var header map[string]interface{}
	if err := dec.Decode(&header); err != nil {
		return nil, fmt.Errorf("reading tape header: %w", err)
	}

	nodeID, _ := header["node_id"].(string)
	label, _ := header["label"].(string)
	tape := NewTape(nodeID, label)

	for {
		var frame Frame
		if err := dec.Decode(&frame); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("reading frame: %w", err)
		}
		tape.Frames = append(tape.Frames, frame)
	}

	return tape, nil
}
