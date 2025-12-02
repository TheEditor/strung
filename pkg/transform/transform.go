// Package transform converts UBS findings to Beads issues.
package transform

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/TheEditor/strung/pkg/beads"
	"github.com/TheEditor/strung/pkg/parser"
)

// Transformer converts UBS findings to Beads issues
type Transformer struct {
	Verbose bool // Enable debug logging
}

// NewTransformer creates a new transformer
func NewTransformer() *Transformer {
	return &Transformer{}
}

// Transform converts a UBS finding to a Beads issue.
// Returns error if finding fails validation.
func (t *Transformer) Transform(finding parser.UBSFinding) (*beads.Issue, error) {
	// Validate input
	if err := finding.Validate(); err != nil {
		return nil, fmt.Errorf("invalid finding: %w", err)
	}

	// Apply defaults for missing optional fields
	severity := finding.Severity
	if severity == "" {
		severity = "info"
		if t.Verbose {
			log.Printf("WARN: finding in %s:%d missing severity, defaulting to info",
				finding.File, finding.Line)
		}
	}

	issue := beads.NewIssue(t.makeTitle(finding))

	// Map severity to priority and type
	issue.Priority = t.SeverityToPriority(severity)
	issue.Type = t.SeverityToType(severity)

	// Build description
	issue.Description = t.makeDescription(finding)

	// Build design notes
	issue.Design = t.makeDesign(finding)

	// Build acceptance criteria
	issue.Acceptance = t.makeAcceptance(finding)

	// Add tags
	issue.Tags = []string{"ubs", finding.Category}

	return issue, nil
}

// TransformAll converts all findings to issues, skipping invalid ones.
// Returns successfully transformed issues and logs warnings for failures.
func (t *Transformer) TransformAll(findings []parser.UBSFinding) []*beads.Issue {
	issues := make([]*beads.Issue, 0, len(findings))
	for i, finding := range findings {
		issue, err := t.Transform(finding)
		if err != nil {
			log.Printf("WARN: skipping finding %d: %v", i, err)
			continue
		}
		issues = append(issues, issue)
	}
	return issues
}

// makeTitle creates issue title from finding
func (t *Transformer) makeTitle(f parser.UBSFinding) string {
	filename := filepath.Base(f.File)
	return fmt.Sprintf("UBS: %s in %s:%d", f.Category, filename, f.Line)
}

// makeDescription builds issue description
func (t *Transformer) makeDescription(f parser.UBSFinding) string {
	desc := fmt.Sprintf("**Location:** `%s:%d`\n\n", f.File, f.Line)
	desc += fmt.Sprintf("**Message:** %s\n\n", f.Message)

	if f.CodeSnippet != "" {
		desc += fmt.Sprintf("**Code:**\n```\n%s\n```\n", f.CodeSnippet)
	}

	if f.Suggestion != "" {
		desc += fmt.Sprintf("\n**Suggestion:** %s\n", f.Suggestion)
	}

	return desc
}

// makeDesign builds design notes
func (t *Transformer) makeDesign(f parser.UBSFinding) string {
	return fmt.Sprintf("Category: %s\nSeverity: %s\nDetected by: UBS static analysis",
		f.Category, f.Severity)
}

// makeAcceptance builds acceptance criteria
func (t *Transformer) makeAcceptance(f parser.UBSFinding) string {
	if f.Suggestion != "" {
		return fmt.Sprintf("Fixed when: %s", f.Suggestion)
	}
	return "Code passes UBS scan without this finding"
}

// SeverityToPriority maps UBS severity to Beads priority (exported for Phase 2)
func (t *Transformer) SeverityToPriority(severity string) int {
	switch severity {
	case "critical":
		return beads.PriorityCritical
	case "warning":
		return beads.PriorityHigh
	case "info":
		return beads.PriorityMedium
	default:
		return beads.PriorityMedium
	}
}

// SeverityToType maps UBS severity to Beads issue type (exported for Phase 2)
func (t *Transformer) SeverityToType(severity string) string {
	switch severity {
	case "critical":
		return beads.TypeBug
	case "warning":
		return beads.TypeTask
	case "info":
		return beads.TypeChore
	default:
		return beads.TypeTask
	}
}
