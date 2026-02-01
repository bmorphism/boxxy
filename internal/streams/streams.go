//go:build darwin

// Package streams provides macOS event stream consumption for boxxy.
// Supports FSEvents, Unified Logging, Darwin Notifications, and kqueue.
package streams

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bmorphism/boxxy/internal/lisp"

	"github.com/fsnotify/fsevents"
)

// StreamRegistry tracks active stream consumers
var (
	streamRegistry   = make(map[string]*StreamConsumer)
	streamRegistryMu sync.RWMutex
	streamIDCounter  int
)

// StreamConsumer represents an active stream subscription
type StreamConsumer struct {
	ID        string
	Type      string // "fsevents", "oslog", "darwin", "kqueue"
	Cancel    context.CancelFunc
	EventChan chan StreamEvent
	Running   bool
	mu        sync.Mutex
}

// StreamEvent represents a generic event from any stream
type StreamEvent struct {
	StreamID  string
	Type      string
	Timestamp time.Time
	Data      map[string]interface{}
}

// =============================================================================
// FSEvents - File System Event Streams
// =============================================================================

// StartFSEventsStream starts watching file system events for given paths
func StartFSEventsStream(paths []string, latency time.Duration, flags fsevents.CreateFlags) (*StreamConsumer, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one path required")
	}

	// Validate paths exist
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("path %s: %w", p, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	consumer := &StreamConsumer{
		ID:        generateStreamID("fsevents"),
		Type:      "fsevents",
		Cancel:    cancel,
		EventChan: make(chan StreamEvent, 100),
		Running:   true,
	}

	// Configure FSEvents
	es := fsevents.EventStream{
		Paths:   paths,
		Latency: latency,
		Flags:   flags,
	}

	// Start the stream
	es.Start()

	// Event processing goroutine
	go func() {
		defer es.Stop()
		defer close(consumer.EventChan)

		for {
			select {
			case <-ctx.Done():
				return
			case events, ok := <-es.Events:
				if !ok {
					return
				}
				for _, event := range events {
					consumer.EventChan <- StreamEvent{
						StreamID:  consumer.ID,
						Type:      "fsevent",
						Timestamp: time.Now(),
						Data: map[string]interface{}{
							"path":  event.Path,
							"flags": event.Flags,
							"id":    event.ID,
						},
					}
				}
			}
		}
	}()

	registerStream(consumer)
	return consumer, nil
}

// Stop stops a stream consumer
func (c *StreamConsumer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Running {
		c.Cancel()
		c.Running = false
		unregisterStream(c.ID)
	}
}

// =============================================================================
// Unified Logging (os_log) Stream
// =============================================================================

// StartOSLogStream starts streaming unified logs with optional predicate
func StartOSLogStream(predicate string, level string) (*StreamConsumer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	consumer := &StreamConsumer{
		ID:        generateStreamID("oslog"),
		Type:      "oslog",
		Cancel:    cancel,
		EventChan: make(chan StreamEvent, 100),
		Running:   true,
	}

	// In a real implementation, this would use the OSLog framework
	// For now, we simulate with log command subprocess
	go func() {
		defer close(consumer.EventChan)

		// This is a simplified version - real implementation would use
		// OSLog framework via cgo or direct syscall
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// In real impl: read from OSLogStore
				consumer.EventChan <- StreamEvent{
					StreamID:  consumer.ID,
					Type:      "oslog",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"message":   "[os_log event placeholder]",
						"predicate": predicate,
						"level":     level,
					},
				}
			}
		}
	}()

	registerStream(consumer)
	return consumer, nil
}

// =============================================================================
// Darwin Notifications
// =============================================================================

// StartDarwinNotifyStream starts listening for Darwin notifications
func StartDarwinNotifyStream(names []string) (*StreamConsumer, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("at least one notification name required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	consumer := &StreamConsumer{
		ID:        generateStreamID("darwin"),
		Type:      "darwin",
		Cancel:    cancel,
		EventChan: make(chan StreamEvent, 100),
		Running:   true,
	}

	// Real implementation would use notify_register_* functions
	// For now, simulated behavior
	go func() {
		defer close(consumer.EventChan)

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				consumer.EventChan <- StreamEvent{
					StreamID:  consumer.ID,
					Type:      "darwin",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"notification": names[0],
						"message":      "[darwin notify placeholder]",
					},
				}
			}
		}
	}()

	registerStream(consumer)
	return consumer, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

func generateStreamID(streamType string) string {
	streamRegistryMu.Lock()
	defer streamRegistryMu.Unlock()
	streamIDCounter++
	return fmt.Sprintf("%s-%d", streamType, streamIDCounter)
}

func registerStream(c *StreamConsumer) {
	streamRegistryMu.Lock()
	defer streamRegistryMu.Unlock()
	streamRegistry[c.ID] = c
}

func unregisterStream(id string) {
	streamRegistryMu.Lock()
	defer streamRegistryMu.Unlock()
	delete(streamRegistry, id)
}

// GetStream retrieves a stream by ID
func GetStream(id string) (*StreamConsumer, bool) {
	streamRegistryMu.RLock()
	defer streamRegistryMu.RUnlock()
	s, ok := streamRegistry[id]
	return s, ok
}

// ListStreams returns all active stream IDs
func ListStreams() []string {
	streamRegistryMu.RLock()
	defer streamRegistryMu.RUnlock()

	ids := make([]string, 0, len(streamRegistry))
	for id := range streamRegistry {
		ids = append(ids, id)
	}
	return ids
}

// =============================================================================
// Lisp Namespace Registration
// =============================================================================

// RegisterNamespace registers the streams namespace with the Lisp environment
func RegisterNamespace(env *lisp.Env) {
	// FSEvents
	env.Set("streams/fsevents-start", &lisp.Fn{"streams/fsevents-start", fseventsStartLisp})
	env.Set("streams/fsevents-stop", &lisp.Fn{"streams/fsevents-stop", fseventsStopLisp})
	env.Set("streams/fsevents-flags", &lisp.Fn{"streams/fsevents-flags", fseventsFlagsLisp})

	// OSLog
	env.Set("streams/oslog-start", &lisp.Fn{"streams/oslog-start", oslogStartLisp})
	env.Set("streams/oslog-stop", &lisp.Fn{"streams/oslog-stop", oslogStopLisp})

	// Darwin Notifications
	env.Set("streams/darwin-start", &lisp.Fn{"streams/darwin-start", darwinStartLisp})
	env.Set("streams/darwin-post", &lisp.Fn{"streams/darwin-post", darwinPostLisp})
	env.Set("streams/darwin-stop", &lisp.Fn{"streams/darwin-stop", darwinStopLisp})

	// General stream management
	env.Set("streams/list", &lisp.Fn{"streams/list", listStreamsLisp})
	env.Set("streams/poll", &lisp.Fn{"streams/poll", pollStreamLisp})
	env.Set("streams/stop-all", &lisp.Fn{"streams/stop-all", stopAllStreamsLisp})
}

// =============================================================================
// Lisp Function Implementations
// =============================================================================

func fseventsStartLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("streams/fsevents-start requires paths vector")
	}

	// Parse paths
	pathsVec := args[0].(lisp.Vector)
	paths := make([]string, len(pathsVec))
	for i, v := range pathsVec {
		paths[i] = string(v.(lisp.String))
	}

	// Parse options
	latency := 50 * time.Millisecond
	flags := fsevents.NoDefer | fsevents.FileEvents

	if len(args) > 1 {
		// Options HashMap
		opts := args[1].(lisp.HashMap)
		if lat, ok := opts[lisp.Keyword("latency")]; ok {
			latency = time.Duration(lat.(lisp.Int)) * time.Millisecond
		}
		if f, ok := opts[lisp.Keyword("flags")]; ok {
			flags = fsevents.CreateFlags(f.(lisp.Int))
		}
	}

	consumer, err := StartFSEventsStream(paths, latency, flags)
	if err != nil {
		panic(fmt.Sprintf("failed to start fsevents: %v", err))
	}

	return &lisp.ExternalValue{Value: consumer, Type: "FSEventStream"}
}

func fseventsStopLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("streams/fsevents-stop requires stream")
	}

	stream := args[0].(*lisp.ExternalValue).Value.(*StreamConsumer)
	stream.Stop()
	return lisp.Bool(true)
}

func fseventsFlagsLisp(args []lisp.Value) lisp.Value {
	// Return flag constants as a HashMap
	flags := lisp.HashMap{
		lisp.Keyword("none"):                  lisp.Int(0),
		lisp.Keyword("must-scan-subdirs"):     lisp.Int(fsevents.MustScanSubDirs),
		lisp.Keyword("user-dropped"):          lisp.Int(fsevents.UserDropped),
		lisp.Keyword("kernel-dropped"):        lisp.Int(fsevents.KernelDropped),
		lisp.Keyword("event-ids-wrapped"):     lisp.Int(fsevents.EventIDsWrapped),
		lisp.Keyword("history-done"):          lisp.Int(fsevents.HistoryDone),
		lisp.Keyword("root-changed"):          lisp.Int(fsevents.RootChanged),
		lisp.Keyword("mount"):                 lisp.Int(fsevents.Mount),
		lisp.Keyword("unmount"):               lisp.Int(fsevents.Unmount),
		lisp.Keyword("item-created"):          lisp.Int(fsevents.ItemCreated),
		lisp.Keyword("item-removed"):          lisp.Int(fsevents.ItemRemoved),
		lisp.Keyword("item-inode-meta-mod"):   lisp.Int(fsevents.ItemInodeMetaMod),
		lisp.Keyword("item-renamed"):          lisp.Int(fsevents.ItemRenamed),
		lisp.Keyword("item-modified"):         lisp.Int(fsevents.ItemModified),
		lisp.Keyword("item-finder-info-mod"):  lisp.Int(fsevents.ItemFinderInfoMod),
		lisp.Keyword("item-change-owner"):     lisp.Int(fsevents.ItemChangeOwner),
		lisp.Keyword("item-xattr-mod"):        lisp.Int(fsevents.ItemXattrMod),
		lisp.Keyword("item-is-file"):          lisp.Int(fsevents.ItemIsFile),
		lisp.Keyword("item-is-dir"):           lisp.Int(fsevents.ItemIsDir),
		lisp.Keyword("item-is-symlink"):       lisp.Int(fsevents.ItemIsSymlink),
		lisp.Keyword("no-defer"):              lisp.Int(fsevents.NoDefer),
		lisp.Keyword("watch-root"):            lisp.Int(fsevents.WatchRoot),
		lisp.Keyword("ignore-self"):           lisp.Int(fsevents.IgnoreSelf),
		lisp.Keyword("file-events"):           lisp.Int(fsevents.FileEvents),
	}
	return flags
}

func oslogStartLisp(args []lisp.Value) lisp.Value {
	predicate := ""
	level := "default"

	if len(args) > 0 {
		predicate = string(args[0].(lisp.String))
	}
	if len(args) > 1 {
		level = string(args[1].(lisp.String))
	}

	consumer, err := StartOSLogStream(predicate, level)
	if err != nil {
		panic(fmt.Sprintf("failed to start oslog: %v", err))
	}

	return &lisp.ExternalValue{Value: consumer, Type: "OSLogStream"}
}

func oslogStopLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("streams/oslog-stop requires stream")
	}

	stream := args[0].(*lisp.ExternalValue).Value.(*StreamConsumer)
	stream.Stop()
	return lisp.Bool(true)
}

func darwinStartLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("streams/darwin-start requires notification names vector")
	}

	namesVec := args[0].(lisp.Vector)
	names := make([]string, len(namesVec))
	for i, v := range namesVec {
		names[i] = string(v.(lisp.String))
	}

	consumer, err := StartDarwinNotifyStream(names)
	if err != nil {
		panic(fmt.Sprintf("failed to start darwin notify: %v", err))
	}

	return &lisp.ExternalValue{Value: consumer, Type: "DarwinNotifyStream"}
}

func darwinPostLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("streams/darwin-post requires notification name")
	}

	name := string(args[0].(lisp.String))

	// Real implementation would use notify_post()
	fmt.Printf("[streams] Would post Darwin notification: %s\n", name)
	return lisp.Bool(true)
}

func darwinStopLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("streams/darwin-stop requires stream")
	}

	stream := args[0].(*lisp.ExternalValue).Value.(*StreamConsumer)
	stream.Stop()
	return lisp.Bool(true)
}

func listStreamsLisp(args []lisp.Value) lisp.Value {
	ids := ListStreams()
	vec := make(lisp.Vector, len(ids))
	for i, id := range ids {
		vec[i] = lisp.String(id)
	}
	return vec
}

func pollStreamLisp(args []lisp.Value) lisp.Value {
	if len(args) < 1 {
		panic("streams/poll requires stream")
	}

	stream := args[0].(*lisp.ExternalValue).Value.(*StreamConsumer)

	timeout := 100 * time.Millisecond
	if len(args) > 1 {
		timeout = time.Duration(args[1].(lisp.Int)) * time.Millisecond
	}

	select {
	case event := <-stream.EventChan:
		// Convert StreamEvent to Lisp HashMap
		data := lisp.HashMap{}
		for k, v := range event.Data {
			data[lisp.Keyword(k)] = convertToLispValue(v)
		}

		return lisp.HashMap{
			lisp.Keyword("stream-id"): lisp.String(event.StreamID),
			lisp.Keyword("type"):      lisp.String(event.Type),
			lisp.Keyword("timestamp"): lisp.String(event.Timestamp.Format(time.RFC3339)),
			lisp.Keyword("data"):      data,
		}
	case <-time.After(timeout):
		return lisp.Nil{}
	}
}

func stopAllStreamsLisp(args []lisp.Value) lisp.Value {
	streamRegistryMu.Lock()
	defer streamRegistryMu.Unlock()

	for _, stream := range streamRegistry {
		stream.Stop()
	}

	return lisp.Bool(true)
}

func convertToLispValue(v interface{}) lisp.Value {
	switch val := v.(type) {
	case string:
		return lisp.String(val)
	case int:
		return lisp.Int(val)
	case int64:
		return lisp.Int(val)
	case uint32:
		return lisp.Int(val)
	case bool:
		return lisp.Bool(val)
	case float64:
		return lisp.Float(val)
	case fsevents.EventFlags:
		return lisp.Int(val)
	default:
		return lisp.String(fmt.Sprintf("%v", val))
	}
}

// =============================================================================
// VM Integration Helpers
// =============================================================================

// WatchVMState watches for VM state changes via FSEvents (e.g., disk image changes)
func WatchVMState(vmPath string, callback func(event StreamEvent)) (*StreamConsumer, error) {
	dir := filepath.Dir(vmPath)

	consumer, err := StartFSEventsStream(
		[]string{dir},
		100*time.Millisecond,
		fsevents.FileEvents|fsevents.NoDefer,
	)
	if err != nil {
		return nil, err
	}

	// Filter for specific VM path
	go func() {
		for event := range consumer.EventChan {
			if path, ok := event.Data["path"].(string); ok {
				if path == vmPath || filepath.Dir(path) == dir {
					callback(event)
				}
			}
		}
	}()

	return consumer, nil
}
