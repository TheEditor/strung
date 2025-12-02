// Package beads provides types for interacting with the Beads issue tracker.
// It defines the Issue struct and related constants for creating issues
// from external sources like UBS (Ultimate Bug Scanner).
package beads

import (
	"encoding/json"
	"time"
)

// Issue represents a Beads issue in JSONL format.
// Fields match Beads import schema - see https://github.com/steveyegge/beads
type Issue struct {
	ID          string     `json:"id,omitempty"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Priority    int        `json:"priority"`
	Status      string     `json:"status"`
	Description string     `json:"description,omitempty"`
	Design      string     `json:"design,omitempty"`
	Acceptance  string     `json:"acceptance,omitempty"`
	Assignee    *string    `json:"assignee,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// Valid issue types - must match Beads issue tracker schema
const (
	TypeTask    = "task"
	TypeBug     = "bug"
	TypeFeature = "feature"
	TypeEpic    = "epic"
	TypeChore   = "chore"
)

// Valid statuses
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusClosed     = "closed"
)

// Valid priorities (0=critical, 3=low)
const (
	PriorityCritical = 0
	PriorityHigh     = 1
	PriorityMedium   = 2
	PriorityLow      = 3
)

// ToJSONL serializes issue to JSONL (single line JSON)
func (i *Issue) ToJSONL() (string, error) {
	data, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// NewIssue creates a new issue with sensible defaults
func NewIssue(title string) *Issue {
	now := time.Now()
	return &Issue{
		Title:     title,
		Type:      TypeTask,
		Priority:  PriorityMedium,
		Status:    StatusOpen,
		CreatedAt: &now,
		UpdatedAt: &now,
	}
}
