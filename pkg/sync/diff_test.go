package sync

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/TheEditor/strung/pkg/db"
	"github.com/TheEditor/strung/pkg/parser"
)

func TestDiffer_NewFindings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Empty DB, new scan has findings
	currentFindings := []parser.UBSFinding{
		{File: "test.ts", Line: 42, Severity: "critical", Category: "null-safety", Message: "msg1"},
		{File: "test.ts", Line: 100, Severity: "warning", Category: "leak", Message: "msg2"},
	}

	differ := NewDiffer(database)
	result, err := differ.Diff(currentFindings)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(result.New) != 2 {
		t.Errorf("Expected 2 new, got %d", len(result.New))
	}
	if len(result.Changed) != 0 {
		t.Errorf("Expected 0 changed, got %d", len(result.Changed))
	}
	if len(result.Resolved) != 0 {
		t.Errorf("Expected 0 resolved, got %d", len(result.Resolved))
	}
}

func TestDiffer_ChangedSeverity(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	now := time.Now()

	// Store existing finding with warning severity
	fp := db.ComputeFingerprint("test.ts", "null-safety", "Old message", "", 42)
	database.Store(&db.Finding{
		Fingerprint: fp,
		IssueID:     "test-001",
		File:        "test.ts",
		Line:        42,
		Severity:    "warning",
		Category:    "null-safety",
		Message:     "Old message",
		FirstSeen:   now,
		LastSeen:    now,
	})

	// Current scan: same finding but now critical
	currentFindings := []parser.UBSFinding{
		{File: "test.ts", Line: 42, Severity: "critical", Category: "null-safety", Message: "Old message"},
	}

	differ := NewDiffer(database)
	result, err := differ.Diff(currentFindings)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(result.New) != 0 {
		t.Errorf("Expected 0 new, got %d", len(result.New))
	}
	if len(result.Changed) != 1 {
		t.Fatalf("Expected 1 changed, got %d", len(result.Changed))
	}

	change := result.Changed[0]
	if change.Previous.Severity != "warning" {
		t.Errorf("Previous severity wrong: %s", change.Previous.Severity)
	}
	if change.Current.Severity != "critical" {
		t.Errorf("Current severity wrong: %s", change.Current.Severity)
	}
}

func TestDiffer_ResolvedFindings(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	now := time.Now()

	// Store finding that will be resolved
	fp := db.ComputeFingerprint("gone.ts", "test", "Gone", "", 1)
	database.Store(&db.Finding{
		Fingerprint: fp,
		IssueID:     "test-001",
		File:        "gone.ts",
		Line:        1,
		Severity:    "critical",
		Category:    "test",
		Message:     "Gone",
		FirstSeen:   now,
		LastSeen:    now,
	})

	// Current scan is empty
	currentFindings := []parser.UBSFinding{}

	differ := NewDiffer(database)
	result, err := differ.Diff(currentFindings)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(result.Resolved) != 1 {
		t.Errorf("Expected 1 resolved, got %d", len(result.Resolved))
	}
	if result.Resolved[0].IssueID != "test-001" {
		t.Errorf("Wrong resolved issue: %s", result.Resolved[0].IssueID)
	}
}

func TestDiffer_MixedChanges(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	now := time.Now()

	// Store existing findings
	fp1 := db.ComputeFingerprint("keep.ts", "x", "Keep", "", 1)
	database.Store(&db.Finding{
		Fingerprint: fp1, IssueID: "test-001",
		File: "keep.ts", Line: 1, Severity: "warning",
		Category: "x", Message: "Keep",
		FirstSeen: now, LastSeen: now,
	})

	fp2 := db.ComputeFingerprint("remove.ts", "x", "Remove", "", 2)
	database.Store(&db.Finding{
		Fingerprint: fp2, IssueID: "test-002",
		File: "remove.ts", Line: 2, Severity: "critical",
		Category: "x", Message: "Remove",
		FirstSeen: now, LastSeen: now,
	})

	// Current scan: keep.ts (changed severity), new.ts (new), remove.ts (gone)
	currentFindings := []parser.UBSFinding{
		{File: "keep.ts", Line: 1, Severity: "critical", Category: "x", Message: "Keep"},
		{File: "new.ts", Line: 99, Severity: "info", Category: "y", Message: "New"},
	}

	differ := NewDiffer(database)
	result, err := differ.Diff(currentFindings)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if len(result.New) != 1 {
		t.Errorf("Expected 1 new, got %d", len(result.New))
	}
	if len(result.Changed) != 1 {
		t.Errorf("Expected 1 changed, got %d", len(result.Changed))
	}
	if len(result.Resolved) != 1 {
		t.Errorf("Expected 1 resolved, got %d", len(result.Resolved))
	}
}

func TestDiffer_NoChanges(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	now := time.Now()

	// Store finding
	fp := db.ComputeFingerprint("test.ts", "x", "Same", "", 1)
	database.Store(&db.Finding{
		Fingerprint: fp, IssueID: "test-001",
		File: "test.ts", Line: 1, Severity: "warning",
		Category: "x", Message: "Same",
		FirstSeen: now, LastSeen: now,
	})

	// Current scan: exact same finding
	currentFindings := []parser.UBSFinding{
		{File: "test.ts", Line: 1, Severity: "warning", Category: "x", Message: "Same"},
	}

	differ := NewDiffer(database)
	result, err := differ.Diff(currentFindings)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	if !result.IsEmpty() {
		t.Errorf("Expected empty result, got: %s", result.Stats())
	}
}

func TestDiffResult_Stats(t *testing.T) {
	result := &DiffResult{
		New:      make([]parser.UBSFinding, 3),
		Changed:  make([]ChangeRecord, 2),
		Resolved: make([]*db.Finding, 1),
	}

	stats := result.Stats()
	expected := "New: 3, Changed: 2, Resolved: 1"
	if stats != expected {
		t.Errorf("Stats wrong: got %s, want %s", stats, expected)
	}

	if result.TotalActions() != 6 {
		t.Errorf("TotalActions wrong: got %d, want 6", result.TotalActions())
	}
}

// Helper
func setupTestDB(t *testing.T) *db.TrackingDB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	return database
}
