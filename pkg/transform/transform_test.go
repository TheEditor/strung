package transform

import (
	"strings"
	"testing"
	"time"

	"github.com/TheEditor/strung/pkg/parser"
)

func TestTransform(t *testing.T) {
	finding := parser.UBSFinding{
		File:       "src/test.ts",
		Line:       42,
		Severity:   "critical",
		Category:   "null-safety",
		Message:    "Unguarded access",
		Suggestion: "Add null check",
	}

	transformer := NewTransformer()
	issue, err := transformer.Transform(finding)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Verify title format
	if !strings.Contains(issue.Title, "null-safety") {
		t.Errorf("Title missing category: %s", issue.Title)
	}
	if !strings.Contains(issue.Title, "test.ts:42") {
		t.Errorf("Title missing location: %s", issue.Title)
	}

	// Verify priority mapping
	if issue.Priority != 0 {
		t.Errorf("Critical should map to priority 0, got %d", issue.Priority)
	}

	// Verify type mapping
	if issue.Type != "bug" {
		t.Errorf("Critical should map to bug, got %s", issue.Type)
	}

	// Verify description contains key info
	if !strings.Contains(issue.Description, "src/test.ts:42") {
		t.Error("Description missing location")
	}
	if !strings.Contains(issue.Description, "Unguarded access") {
		t.Error("Description missing message")
	}

	// Verify acceptance criteria
	if !strings.Contains(issue.Acceptance, "Add null check") {
		t.Error("Acceptance missing suggestion")
	}

	// Verify tags
	if len(issue.Tags) != 2 || issue.Tags[0] != "ubs" || issue.Tags[1] != "null-safety" {
		t.Errorf("Tags incorrect: %v", issue.Tags)
	}
}

func TestTransform_InvalidFinding(t *testing.T) {
	transformer := NewTransformer()

	// Missing required field
	finding := parser.UBSFinding{
		Line:     42,
		Severity: "critical",
		Category: "null-safety",
		Message:  "test",
		// Missing: File
	}

	_, err := transformer.Transform(finding)
	if err == nil {
		t.Error("Expected error for invalid finding")
	}
}

func TestSeverityMapping(t *testing.T) {
	transformer := NewTransformer()

	tests := []struct {
		severity  string
		priority  int
		issueType string
	}{
		{"critical", 0, "bug"},
		{"warning", 1, "task"},
		{"info", 2, "chore"},
		{"unknown", 2, "task"}, // Default handling
	}

	for _, tt := range tests {
		finding := parser.UBSFinding{
			File:     "test.ts",
			Line:     1,
			Severity: tt.severity,
			Category: "test",
			Message:  "test",
		}

		issue, err := transformer.Transform(finding)
		if err != nil {
			t.Fatalf("Transform failed for %s: %v", tt.severity, err)
		}

		if issue.Priority != tt.priority {
			t.Errorf("%s: expected priority %d, got %d",
				tt.severity, tt.priority, issue.Priority)
		}
		if issue.Type != tt.issueType {
			t.Errorf("%s: expected type %s, got %s",
				tt.severity, tt.issueType, issue.Type)
		}
	}
}

func TestTransformAll_SkipsInvalid(t *testing.T) {
	transformer := NewTransformer()

	findings := []parser.UBSFinding{
		{File: "valid.ts", Line: 1, Category: "x", Message: "valid"},
		{File: "", Line: 2, Category: "x", Message: "invalid - missing file"},
		{File: "valid2.ts", Line: 3, Category: "y", Message: "also valid"},
	}

	issues := transformer.TransformAll(findings)
	if len(issues) != 2 {
		t.Errorf("Expected 2 valid issues, got %d", len(issues))
	}
}

func TestTransformWithConfig_FileLinks(t *testing.T) {
	config := &TransformConfig{
		RepoURL:    "https://github.com/user/repo",
		RepoBranch: "main",
		ScanTime:   time.Now(),
	}

	transformer := NewTransformerWithConfig(config)

	finding := parser.UBSFinding{
		File:     "src/test.ts",
		Line:     42,
		Severity: "critical",
		Category: "null-safety",
		Message:  "Test",
	}

	issue, err := transformer.Transform(finding)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Check for clickable link
	expectedLink := "https://github.com/user/repo/blob/main/src/test.ts#L42"
	if !strings.Contains(issue.Description, expectedLink) {
		t.Errorf("Description missing file link: %s", issue.Description)
	}

	// Check for timestamp
	if !strings.Contains(issue.Description, "**Detected:**") {
		t.Error("Description missing timestamp")
	}
}

func TestTransformWithConfig_Tags(t *testing.T) {
	config := &TransformConfig{
		ScanTime: time.Now(),
	}

	transformer := NewTransformerWithConfig(config)

	finding := parser.UBSFinding{
		File:     "src/handler.ts",
		Line:     1,
		Severity: "warning",
		Category: "resource-leak",
		Message:  "Test",
	}

	issue, err := transformer.Transform(finding)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Check tags
	expectedTags := []string{"ubs", "ubs:resource-leak", "severity:warning", "lang:ts"}
	for _, expected := range expectedTags {
		found := false
		for _, tag := range issue.Tags {
			if tag == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing tag: %s (got: %v)", expected, issue.Tags)
		}
	}
}

func TestTransformWithConfig_NoRepoURL(t *testing.T) {
	config := &TransformConfig{
		ScanTime: time.Now(),
		// No RepoURL
	}

	transformer := NewTransformerWithConfig(config)

	finding := parser.UBSFinding{
		File:     "test.ts",
		Line:     1,
		Severity: "info",
		Category: "x",
		Message:  "Test",
	}

	issue, err := transformer.Transform(finding)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Should have backtick location, not link
	if !strings.Contains(issue.Description, "`test.ts:1`") {
		t.Errorf("Should have backtick location: %s", issue.Description)
	}
	if strings.Contains(issue.Description, "](http") {
		t.Error("Should not have link without RepoURL")
	}
}
