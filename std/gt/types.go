// Package gt provides boxxy ↔ gastown bridge types.
//
// These structs mirror gastown's domain model as EDN-serializable
// structures that flow between boxxy's Joker REPL and gastown's
// Go internals via the gt/ Joker namespace.
package gt

import "time"

// Rig is a managed repository workspace (gastown rig).
type Rig struct {
	Name         string   `json:"name"          edn:"name"`
	Path         string   `json:"path"          edn:"path"`
	GitURL       string   `json:"git_url"       edn:"git-url"`
	Polecats     []string `json:"polecats"      edn:"polecats"`
	Crew         []string `json:"crew"          edn:"crew"`
	HasWitness   bool     `json:"has_witness"   edn:"has-witness?"`
	HasRefinery  bool     `json:"has_refinery"  edn:"has-refinery?"`
	PolecatCount int      `json:"polecat_count" edn:"polecat-count"`
	CrewCount    int      `json:"crew_count"    edn:"crew-count"`
}

// PolecatState mirrors gastown's polecat lifecycle.
type PolecatState string

const (
	Working PolecatState = "working"
	Done    PolecatState = "done"
	Stuck   PolecatState = "stuck"
)

// Polecat is a worker agent (git worktree + tmux session).
type Polecat struct {
	Name      string       `json:"name"       edn:"name"`
	Rig       string       `json:"rig"        edn:"rig"`
	State     PolecatState `json:"state"      edn:"state"`
	ClonePath string       `json:"clone_path" edn:"clone-path"`
	Branch    string       `json:"branch"     edn:"branch"`
	Issue     string       `json:"issue"      edn:"issue"`
	CreatedAt time.Time    `json:"created_at" edn:"created-at"`
}

// Issue is a git-backed work item (gastown beads).
type Issue struct {
	ID       string   `json:"id"       edn:"id"`
	Title    string   `json:"title"    edn:"title"`
	Body     string   `json:"body"     edn:"body"`
	State    string   `json:"state"    edn:"state"`
	Assignee string   `json:"assignee" edn:"assignee"`
	Labels   []string `json:"labels"   edn:"labels"`
	Priority string   `json:"priority" edn:"priority"`
}

// MessagePriority for inter-agent mail.
type MessagePriority string

const (
	PriorityLow    MessagePriority = "low"
	PriorityNormal MessagePriority = "normal"
	PriorityHigh   MessagePriority = "high"
	PriorityUrgent MessagePriority = "urgent"
)

// MessageType indicates purpose of a message.
type MessageType string

const (
	TypeTask         MessageType = "task"
	TypeScavenge     MessageType = "scavenge"
	TypeNotification MessageType = "notification"
	TypeReply        MessageType = "reply"
)

// Delivery mode for messages.
type Delivery string

const (
	DeliveryQueue     Delivery = "queue"
	DeliveryInterrupt Delivery = "interrupt"
)

// Message is inter-agent mail.
type Message struct {
	ID       string          `json:"id"       edn:"id"`
	From     string          `json:"from"     edn:"from"`
	To       string          `json:"to"       edn:"to"`
	Subject  string          `json:"subject"  edn:"subject"`
	Body     string          `json:"body"     edn:"body"`
	Time     time.Time       `json:"timestamp" edn:"timestamp"`
	Read     bool            `json:"read"     edn:"read?"`
	Priority MessagePriority `json:"priority" edn:"priority"`
	Type     MessageType     `json:"type"     edn:"type"`
	Delivery Delivery        `json:"delivery" edn:"delivery"`
	ThreadID string          `json:"thread_id" edn:"thread-id"`
	ReplyTo  string          `json:"reply_to" edn:"reply-to"`
	Queue    string          `json:"queue"    edn:"queue"`
	Channel  string          `json:"channel"  edn:"channel"`
}

// MergeRequest tracks a branch → main merge.
type MergeRequest struct {
	ID       string `json:"id"       edn:"id"`
	Polecat  string `json:"polecat"  edn:"polecat"`
	Rig      string `json:"rig"      edn:"rig"`
	Branch   string `json:"branch"   edn:"branch"`
	Status   string `json:"status"   edn:"status"`
	Issue    string `json:"issue"    edn:"issue"`
}

// DaemonStatus reports operational state.
type DaemonStatus struct {
	Running      bool   `json:"running"       edn:"running?"`
	RigCount     int    `json:"rig_count"     edn:"rig-count"`
	AgentTotal   int    `json:"agent_total"   edn:"agent-total"`
	AgentWorking int    `json:"agent_working" edn:"agent-working"`
	OpState      string `json:"op_state"      edn:"op-state"`
}

// CheckResult from doctor health checks.
type CheckResult struct {
	Check   string `json:"check"   edn:"check"`
	Status  string `json:"status"  edn:"status"`
	Message string `json:"message" edn:"message"`
	CanFix  bool   `json:"can_fix" edn:"can-fix?"`
}
