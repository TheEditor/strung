package parser

import (
	"strings"
	"testing"
)

func TestParseUBS(t *testing.T) {
	input := `{
		"project": "/test",
		"files_scanned": 10,
		"findings": [
			{
				"file": "test.ts",
				"line": 42,
				"severity": "critical",
				"category": "null-safety",
				"message": "Test message"
			}
		],
		"summary": {"critical": 1, "warning": 0, "info": 0}
	}`

	report, err := ParseUBS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseUBS failed: %v", err)
	}

	if len(report.Findings) != 1 {
		t.Errorf("Expected 1 finding, got %d", len(report.Findings))
	}

	finding := report.Findings[0]
	if finding.File != "test.ts" {
		t.Errorf("File mismatch: got %s", finding.File)
	}
	if finding.Severity != "critical" {
		t.Errorf("Severity mismatch: got %s", finding.Severity)
	}
}

func TestParseUBS_EmptyInput(t *testing.T) {
	_, err := ParseUBS(strings.NewReader(""))
	if err == nil {
		t.Error("Expected error for empty input")
	}
}

func TestParseUBS_InvalidJSON(t *testing.T) {
	_, err := ParseUBS(strings.NewReader("{not valid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse UBS JSON") {
		t.Errorf("Error should have context, got: %v", err)
	}
}

func TestParseUBS_MissingFindings(t *testing.T) {
	input := `{"project": "/test", "summary": {}}`
	_, err := ParseUBS(strings.NewReader(input))
	if err == nil {
		t.Error("Expected error for missing findings")
	}
	if !strings.Contains(err.Error(), "missing 'findings'") {
		t.Errorf("Error should mention missing findings, got: %v", err)
	}
}

func TestParseUBS_EmptyFindings(t *testing.T) {
	input := `{"project": "/test", "findings": [], "summary": {}}`
	report, err := ParseUBS(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Empty findings should be valid: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(report.Findings))
	}
}

func TestFilterBySeverity(t *testing.T) {
	report := &UBSReport{
		Findings: []UBSFinding{
			{Severity: "critical", File: "a.ts", Line: 1, Category: "x", Message: "x"},
			{Severity: "warning", File: "b.ts", Line: 2, Category: "x", Message: "x"},
			{Severity: "info", File: "c.ts", Line: 3, Category: "x", Message: "x"},
		},
	}

	tests := []struct {
		minSeverity string
		expected    int
	}{
		{"critical", 1},
		{"warning", 2},
		{"info", 3},
		{"invalid", 3}, // Should default to info
	}

	for _, tt := range tests {
		filtered := report.FilterBySeverity(tt.minSeverity)
		if len(filtered) != tt.expected {
			t.Errorf("FilterBySeverity(%q): expected %d, got %d",
				tt.minSeverity, tt.expected, len(filtered))
		}
	}
}

func TestFindingValidate(t *testing.T) {
	tests := []struct {
		name    string
		finding UBSFinding
		wantErr bool
	}{
		{
			name: "valid",
			finding: UBSFinding{
				File: "test.ts", Line: 42, Category: "null-safety", Message: "msg",
			},
			wantErr: false,
		},
		{
			name:    "missing file",
			finding: UBSFinding{Line: 42, Category: "x", Message: "x"},
			wantErr: true,
		},
		{
			name:    "invalid line",
			finding: UBSFinding{File: "test.ts", Line: 0, Category: "x", Message: "x"},
			wantErr: true,
		},
		{
			name:    "missing category",
			finding: UBSFinding{File: "test.ts", Line: 1, Message: "x"},
			wantErr: true,
		},
		{
			name:    "missing message",
			finding: UBSFinding{File: "test.ts", Line: 1, Category: "x"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.finding.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
