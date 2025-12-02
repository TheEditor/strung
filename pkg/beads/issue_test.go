package beads

import (
	"encoding/json"
	"testing"
	"time"
)

func TestIssueToJSONL(t *testing.T) {
	now := time.Now()
	issue := &Issue{
		ID:          "test-001",
		Title:       "Test issue",
		Type:        TypeBug,
		Priority:    PriorityCritical,
		Status:      StatusOpen,
		Description: "Test description",
		CreatedAt:   &now,
	}

	jsonl, err := issue.ToJSONL()
	if err != nil {
		t.Fatalf("ToJSONL failed: %v", err)
	}

	// Verify it's valid JSON
	var decoded Issue
	if err := json.Unmarshal([]byte(jsonl), &decoded); err != nil {
		t.Fatalf("Failed to decode JSONL: %v", err)
	}

	// Verify fields
	if decoded.Title != issue.Title {
		t.Errorf("Title mismatch: got %s, want %s", decoded.Title, issue.Title)
	}
	if decoded.Type != TypeBug {
		t.Errorf("Type mismatch: got %s, want %s", decoded.Type, TypeBug)
	}
	if decoded.Priority != PriorityCritical {
		t.Errorf("Priority mismatch: got %d, want %d", decoded.Priority, PriorityCritical)
	}
}

func TestNewIssue(t *testing.T) {
	issue := NewIssue("Test")

	if issue.Title != "Test" {
		t.Errorf("Title not set")
	}
	if issue.Type != TypeTask {
		t.Errorf("Default type not task, got %s", issue.Type)
	}
	if issue.Priority != PriorityMedium {
		t.Errorf("Default priority not medium, got %d", issue.Priority)
	}
	if issue.Status != StatusOpen {
		t.Errorf("Default status not open, got %s", issue.Status)
	}
	if issue.CreatedAt == nil {
		t.Error("CreatedAt not set")
	}
	if issue.UpdatedAt == nil {
		t.Error("UpdatedAt not set")
	}
}

func TestIssueToJSONL_OmitsEmptyFields(t *testing.T) {
	issue := &Issue{
		Title:    "Minimal",
		Type:     TypeTask,
		Priority: PriorityLow,
		Status:   StatusOpen,
	}

	jsonl, err := issue.ToJSONL()
	if err != nil {
		t.Fatalf("ToJSONL failed: %v", err)
	}

	// Verify omitted fields not present
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(jsonl), &raw); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if _, exists := raw["id"]; exists {
		t.Error("Empty ID should be omitted")
	}
	if _, exists := raw["description"]; exists {
		t.Error("Empty description should be omitted")
	}
}
