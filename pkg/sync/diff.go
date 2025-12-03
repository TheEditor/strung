// Package sync provides diff detection and synchronization between UBS scans and Beads.
package sync

import (
	"fmt"

	"github.com/TheEditor/strung/pkg/db"
	"github.com/TheEditor/strung/pkg/parser"
)

// DiffResult contains categorized findings after comparing scan vs DB
type DiffResult struct {
	New      []parser.UBSFinding // Not in DB
	Changed  []ChangeRecord      // In DB but severity changed
	Resolved []*db.Finding       // In DB but not in current scan
}

// ChangeRecord represents a changed finding
type ChangeRecord struct {
	Previous *db.Finding
	Current  parser.UBSFinding
}

// Differ computes diffs between scans and DB state
type Differ struct {
	db *db.TrackingDB
}

// NewDiffer creates a new differ
func NewDiffer(database *db.TrackingDB) *Differ {
	return &Differ{db: database}
}

// Diff computes diff between current scan and DB state.
// Returns categorized findings: new, changed, resolved.
func (d *Differ) Diff(currentFindings []parser.UBSFinding) (*DiffResult, error) {
	result := &DiffResult{
		New:      make([]parser.UBSFinding, 0),
		Changed:  make([]ChangeRecord, 0),
		Resolved: make([]*db.Finding, 0),
	}

	// Build map of current findings by fingerprint
	currentMap := make(map[string]parser.UBSFinding)
	for _, f := range currentFindings {
		fp := db.ComputeFingerprint(f.File, f.Category, f.Message, f.CodeSnippet, f.Line)
		currentMap[fp] = f
	}

	// Get all unresolved findings from DB
	dbFindings, err := d.db.GetUnresolved()
	if err != nil {
		return nil, fmt.Errorf("get unresolved findings: %w", err)
	}

	// Build map of DB findings by fingerprint
	dbMap := make(map[string]*db.Finding)
	for _, f := range dbFindings {
		dbMap[f.Fingerprint] = f
	}

	// Find new and changed
	for fp, current := range currentMap {
		previous, exists := dbMap[fp]
		if !exists {
			// New finding
			result.New = append(result.New, current)
		} else if previous.Severity != current.Severity {
			// Severity changed
			result.Changed = append(result.Changed, ChangeRecord{
				Previous: previous,
				Current:  current,
			})
		}
		// Note: Same fingerprint + same severity = no action needed
	}

	// Find resolved (in DB but not in current scan)
	for fp, previous := range dbMap {
		if _, exists := currentMap[fp]; !exists {
			result.Resolved = append(result.Resolved, previous)
		}
	}

	return result, nil
}

// Stats returns summary string
func (dr *DiffResult) Stats() string {
	return fmt.Sprintf("New: %d, Changed: %d, Resolved: %d",
		len(dr.New), len(dr.Changed), len(dr.Resolved))
}

// IsEmpty returns true if no changes detected
func (dr *DiffResult) IsEmpty() bool {
	return len(dr.New) == 0 && len(dr.Changed) == 0 && len(dr.Resolved) == 0
}

// TotalActions returns total number of actions needed
func (dr *DiffResult) TotalActions() int {
	return len(dr.New) + len(dr.Changed) + len(dr.Resolved)
}
