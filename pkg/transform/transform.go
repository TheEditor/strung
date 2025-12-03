// Package transform converts UBS findings to Beads issues.
package transform

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

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

// TransformConfig provides context for enhanced transformation
type TransformConfig struct {
	RepoURL    string    // e.g., "https://github.com/user/repo"
	RepoBranch string    // e.g., "main"
	ScanTime   time.Time // When scan was performed
}

// TransformerWithConfig embeds base transformer with enrichment
type TransformerWithConfig struct {
	*Transformer
	config *TransformConfig
}

// NewTransformerWithConfig creates enriched transformer
func NewTransformerWithConfig(config *TransformConfig) *TransformerWithConfig {
	return &TransformerWithConfig{
		Transformer: NewTransformer(),
		config:      config,
	}
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

// Transform converts finding with enrichment (overrides base method)
func (t *TransformerWithConfig) Transform(finding parser.UBSFinding) (*beads.Issue, error) {
	issue, err := t.Transformer.Transform(finding)
	if err != nil {
		return nil, err
	}

	// Enrich description with metadata
	issue.Description = t.makeEnrichedDescription(finding)

	// Add richer tags
	issue.Tags = t.makeEnrichedTags(finding)

	return issue, nil
}

// makeEnrichedDescription builds description with links and timestamps
func (t *TransformerWithConfig) makeEnrichedDescription(f parser.UBSFinding) string {
	var desc strings.Builder

	// Scan timestamp
	if !t.config.ScanTime.IsZero() {
		desc.WriteString(fmt.Sprintf("**Detected:** %s\n\n",
			t.config.ScanTime.Format("2006-01-02 15:04:05")))
	}

	// File link (if repo URL provided)
	if t.config.RepoURL != "" {
		fileLink := fmt.Sprintf("%s/blob/%s/%s#L%d",
			strings.TrimSuffix(t.config.RepoURL, "/"),
			t.config.RepoBranch,
			f.File,
			f.Line)
		desc.WriteString(fmt.Sprintf("**Location:** [%s:%d](%s)\n\n",
			f.File, f.Line, fileLink))
	} else {
		desc.WriteString(fmt.Sprintf("**Location:** `%s:%d`\n\n", f.File, f.Line))
	}

	desc.WriteString(fmt.Sprintf("**Message:** %s\n\n", f.Message))

	if f.CodeSnippet != "" {
		desc.WriteString(fmt.Sprintf("**Code:**\n```\n%s\n```\n\n", f.CodeSnippet))
	}

	if f.Suggestion != "" {
		desc.WriteString(fmt.Sprintf("**Suggestion:** %s\n", f.Suggestion))
	}

	return desc.String()
}

// makeEnrichedTags creates comprehensive tag set
func (t *TransformerWithConfig) makeEnrichedTags(f parser.UBSFinding) []string {
	tags := []string{
		"ubs",
		fmt.Sprintf("ubs:%s", f.Category),
		fmt.Sprintf("severity:%s", f.Severity),
	}

	// Add file extension as tag for filtering
	if ext := filepath.Ext(f.File); ext != "" {
		tags = append(tags, fmt.Sprintf("lang:%s", strings.TrimPrefix(ext, ".")))
	}

	return tags
}
