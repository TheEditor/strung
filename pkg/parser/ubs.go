// Package parser provides UBS (Ultimate Bug Scanner) JSON parsing functionality.
package parser

import (
	"encoding/json"
	"fmt"
	"io"
)

// UBSReport represents the full UBS JSON output
type UBSReport struct {
	Project      string       `json:"project"`
	FilesScanned int          `json:"files_scanned"`
	Findings     []UBSFinding `json:"findings"`
	Summary      UBSSummary   `json:"summary"`
}

// UBSFinding represents a single UBS finding
type UBSFinding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column,omitempty"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
	CodeSnippet string `json:"code_snippet,omitempty"`
}

// UBSSummary represents the summary section
type UBSSummary struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

// ParseUBS parses UBS JSON from a reader with validation
func ParseUBS(r io.Reader) (*UBSReport, error) {
	var report UBSReport
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("parse UBS JSON: %w (verify input is valid JSON from 'ubs --format=json')", err)
	}

	// Validate required structure
	if report.Findings == nil {
		return nil, fmt.Errorf("parse UBS JSON: missing 'findings' array")
	}

	return &report, nil
}

// FilterBySeverity returns findings matching minimum severity level.
// Severity levels: critical (0) > warning (1) > info (2)
// Invalid severity defaults to info (include all).
func (r *UBSReport) FilterBySeverity(minSeverity string) []UBSFinding {
	severityLevel := map[string]int{
		"critical": 0,
		"warning":  1,
		"info":     2,
	}

	minLevel, ok := severityLevel[minSeverity]
	if !ok {
		minLevel = 2 // Default to info (include everything)
	}

	var filtered []UBSFinding
	for _, finding := range r.Findings {
		level, ok := severityLevel[finding.Severity]
		if !ok {
			level = 2 // Unknown severity treated as info
		}
		if level <= minLevel {
			filtered = append(filtered, finding)
		}
	}

	return filtered
}

// Validate checks that a finding has required fields
func (f *UBSFinding) Validate() error {
	if f.File == "" {
		return fmt.Errorf("finding missing required field: file")
	}
	if f.Line <= 0 {
		return fmt.Errorf("finding has invalid line number: %d", f.Line)
	}
	if f.Category == "" {
		return fmt.Errorf("finding missing required field: category")
	}
	if f.Message == "" {
		return fmt.Errorf("finding missing required field: message")
	}
	return nil
}
