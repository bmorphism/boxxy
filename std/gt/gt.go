// Package gt bridges boxxy's Joker REPL to gastown workspaces.
//
// gastown manages multi-agent coding workspaces where each agent
// (polecat) runs in its own git worktree + tmux session. boxxy
// wraps this via the gt/ Joker namespace so you can orchestrate
// agents from the REPL.
//
// Architecture:
//
//	boxxy REPL (Joker)
//	    ↓ gt/ namespace calls
//	gt.go (this package, Go bridge)
//	    ↓ exec gt CLI
//	gastown daemon
//	    ↓ manages
//	polecats (tmux + git worktrees)
package gt

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// gt shells out to the gastown CLI. All state lives in gastown;
// boxxy is a thin orchestration layer.
func run(args ...string) ([]byte, error) {
	cmd := exec.Command("gt", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gt %v: %s: %w", args, out, err)
	}
	return out, nil
}

func runJSON(v interface{}, args ...string) error {
	args = append(args, "--json")
	out, err := run(args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, v)
}

// ListRigs returns all registered rigs.
func ListRigs() ([]Rig, error) {
	var rigs []Rig
	return rigs, runJSON(&rigs, "rig", "list")
}

// RigInfo returns full info for a named rig.
func RigInfo(name string) (*Rig, error) {
	var r Rig
	return &r, runJSON(&r, "rig", "info", name)
}

// Spawn creates a new polecat in a rig.
func Spawn(rig, issue, account string) (*Polecat, error) {
	args := []string{"polecat", "add", "--rig", rig}
	if issue != "" {
		args = append(args, "--issue", issue)
	}
	if account != "" {
		args = append(args, "--account", account)
	}
	var p Polecat
	return &p, runJSON(&p, args...)
}

// ListPolecats returns polecats in a rig.
func ListPolecats(rig string) ([]Polecat, error) {
	var ps []Polecat
	return ps, runJSON(&ps, "polecat", "list", "--rig", rig)
}

// GetPolecatState returns the state of a polecat.
func GetPolecatState(rig, name string) (PolecatState, error) {
	var p Polecat
	if err := runJSON(&p, "polecat", "show", name, "--rig", rig); err != nil {
		return "", err
	}
	return p.State, nil
}

// Nuke removes a polecat worktree.
func Nuke(rig, name string) error {
	_, err := run("polecat", "nuke", name, "--rig", rig)
	return err
}

// CreateIssue creates a beads issue.
func CreateIssue(title, body string, labels []string, assignee, priority string) (string, error) {
	args := []string{"beads", "create", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	if assignee != "" {
		args = append(args, "--assignee", assignee)
	}
	if priority != "" {
		args = append(args, "--priority", priority)
	}
	var issue Issue
	if err := runJSON(&issue, args...); err != nil {
		return "", err
	}
	return issue.ID, nil
}

// ShowIssue gets an issue by id.
func ShowIssue(id string) (*Issue, error) {
	var i Issue
	return &i, runJSON(&i, "beads", "show", id)
}

// ListIssues lists issues with optional filters.
func ListIssues(state, assignee string) ([]Issue, error) {
	args := []string{"beads", "list"}
	if state != "" {
		args = append(args, "--state", state)
	}
	if assignee != "" {
		args = append(args, "--assignee", assignee)
	}
	var issues []Issue
	return issues, runJSON(&issues, args...)
}

// CloseIssue closes an issue with a reason.
func CloseIssue(id, reason string) error {
	_, err := run("beads", "close", id, "--reason", reason)
	return err
}

// SendMail sends a message between agents.
func SendMail(m *Message) error {
	args := []string{"mail", "send",
		"--from", m.From,
		"--subject", m.Subject,
	}
	if m.To != "" {
		args = append(args, "--to", m.To)
	}
	if m.Queue != "" {
		args = append(args, "--queue", m.Queue)
	}
	if m.Channel != "" {
		args = append(args, "--channel", m.Channel)
	}
	if m.Body != "" {
		args = append(args, "--body", m.Body)
	}
	if m.Priority != "" {
		args = append(args, "--priority", string(m.Priority))
	}
	if m.Type != "" {
		args = append(args, "--type", string(m.Type))
	}
	_, err := run(args...)
	return err
}

// Inbox reads an agent's inbox.
func Inbox(agent string) ([]Message, error) {
	var msgs []Message
	return msgs, runJSON(&msgs, "mail", "list", "--agent", agent)
}

// ClaimMail claims a queue message.
func ClaimMail(agent, msgID string) error {
	_, err := run("mail", "claim", msgID, "--agent", agent)
	return err
}

// MergeQueue gets the merge queue for a rig.
func MergeQueue(rig string) ([]MergeRequest, error) {
	var mrs []MergeRequest
	return mrs, runJSON(&mrs, "mq", "list", "--rig", rig)
}

// MergeReady signals a branch is ready for merge.
func MergeReady(rig, polecat string) error {
	_, err := run("mq", "ready", polecat, "--rig", rig)
	return err
}

// Status gets daemon status.
func Status() (*DaemonStatus, error) {
	var s DaemonStatus
	return &s, runJSON(&s, "status")
}

// Doctor runs health checks.
func Doctor() ([]CheckResult, error) {
	var checks []CheckResult
	return checks, runJSON(&checks, "doctor", "run")
}
