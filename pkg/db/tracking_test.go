package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrackingDB_OpenClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if db.Path() != dbPath {
		t.Errorf("Path mismatch: got %s, want %s", db.Path(), dbPath)
	}

	if err := db.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify file created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file not created")
	}
}

func TestTrackingDB_StoreAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now().Truncate(time.Second) // SQLite precision
	fp := ComputeFingerprint("test.ts", "null-safety", "Test message", "", 42)

	finding := &Finding{
		Fingerprint: fp,
		IssueID:     "test-001",
		File:        "test.ts",
		Line:        42,
		Severity:    "critical",
		Category:    "null-safety",
		Message:     "Test message",
		FirstSeen:   now,
		LastSeen:    now,
	}

	// Store
	if err := db.Store(finding); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Get by fingerprint
	retrieved, err := db.Get(fp)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Finding not found")
	}
	if retrieved.IssueID != "test-001" {
		t.Errorf("IssueID mismatch: got %s", retrieved.IssueID)
	}
	if retrieved.ResolvedAt != nil {
		t.Error("ResolvedAt should be nil")
	}

	// Get by issue ID
	byIssue, err := db.GetByIssueID("test-001")
	if err != nil {
		t.Fatalf("GetByIssueID failed: %v", err)
	}
	if byIssue.Fingerprint != fp {
		t.Errorf("Fingerprint mismatch")
	}
}

func TestTrackingDB_MarkResolved(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	fp := ComputeFingerprint("test.ts", "x", "msg", "", 1)

	db.Store(&Finding{
		Fingerprint: fp,
		IssueID:     "test-001",
		File:        "test.ts",
		Line:        1,
		Severity:    "critical",
		Category:    "x",
		Message:     "msg",
		FirstSeen:   now,
		LastSeen:    now,
	})

	// Mark resolved
	resolvedTime := now.Add(time.Hour)
	if err := db.MarkResolved(fp, resolvedTime); err != nil {
		t.Fatalf("MarkResolved failed: %v", err)
	}

	// Verify
	finding, _ := db.Get(fp)
	if finding.ResolvedAt == nil {
		t.Error("ResolvedAt not set")
	}

	// Should not appear in unresolved
	unresolved, _ := db.GetUnresolved()
	for _, f := range unresolved {
		if f.Fingerprint == fp {
			t.Error("Resolved finding in unresolved list")
		}
	}
}

func TestTrackingDB_GetUnresolved(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()

	// Store unresolved
	fp1 := ComputeFingerprint("a.ts", "x", "m1", "", 1)
	db.Store(&Finding{
		Fingerprint: fp1, IssueID: "test-001",
		File: "a.ts", Line: 1, Severity: "critical",
		Category: "x", Message: "m1",
		FirstSeen: now, LastSeen: now,
	})

	// Store and resolve
	fp2 := ComputeFingerprint("b.ts", "x", "m2", "", 2)
	db.Store(&Finding{
		Fingerprint: fp2, IssueID: "test-002",
		File: "b.ts", Line: 2, Severity: "warning",
		Category: "x", Message: "m2",
		FirstSeen: now, LastSeen: now,
	})
	db.MarkResolved(fp2, now)

	// Get unresolved
	unresolved, err := db.GetUnresolved()
	if err != nil {
		t.Fatalf("GetUnresolved failed: %v", err)
	}

	if len(unresolved) != 1 {
		t.Errorf("Expected 1 unresolved, got %d", len(unresolved))
	}
	if unresolved[0].IssueID != "test-001" {
		t.Errorf("Wrong unresolved: %s", unresolved[0].IssueID)
	}
}

func TestTrackingDB_Upsert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	fp := ComputeFingerprint("test.ts", "x", "msg", "", 1)

	// Initial store
	db.Store(&Finding{
		Fingerprint: fp, IssueID: "test-001",
		File: "test.ts", Line: 1, Severity: "warning",
		Category: "x", Message: "msg",
		FirstSeen: now, LastSeen: now,
	})

	// Update with new severity
	later := now.Add(time.Hour)
	db.Store(&Finding{
		Fingerprint: fp, IssueID: "test-001",
		File: "test.ts", Line: 1, Severity: "critical",
		Category: "x", Message: "msg",
		FirstSeen: later, LastSeen: later,
	})

	// Verify update
	finding, _ := db.Get(fp)
	if finding.Severity != "critical" {
		t.Errorf("Severity not updated: %s", finding.Severity)
	}
	// FirstSeen should NOT change on upsert
	if finding.FirstSeen.After(now.Add(time.Minute)) {
		t.Error("FirstSeen should not change on update")
	}
}

func TestComputeFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		category    string
		message     string
		snippet     string
		line        int
		expectSame  string // another test case that should produce same fingerprint
		expectDiff  string // another test case that should produce different fingerprint
	}{
		{
			name:     "with snippet ignores line",
			file:     "test.ts",
			category: "null-safety",
			message:  "Unguarded access",
			snippet:  "const x = obj.prop;",
			line:     42,
		},
		{
			name:     "without snippet uses line",
			file:     "test.ts",
			category: "null-safety",
			message:  "Unguarded access",
			snippet:  "",
			line:     42,
		},
	}

	// Same snippet, different line → same fingerprint
	fp1 := ComputeFingerprint("test.ts", "null-safety", "msg", "code", 42)
	fp2 := ComputeFingerprint("test.ts", "null-safety", "msg", "code", 99)
	if fp1 != fp2 {
		t.Error("Same snippet, different line should produce same fingerprint")
	}

	// No snippet, different line → different fingerprint
	fp3 := ComputeFingerprint("test.ts", "null-safety", "msg", "", 42)
	fp4 := ComputeFingerprint("test.ts", "null-safety", "msg", "", 99)
	if fp3 == fp4 {
		t.Error("No snippet, different line should produce different fingerprint")
	}

	// Different category → different fingerprint
	fp5 := ComputeFingerprint("test.ts", "null-safety", "msg", "code", 42)
	fp6 := ComputeFingerprint("test.ts", "memory-leak", "msg", "code", 42)
	if fp5 == fp6 {
		t.Error("Different category should produce different fingerprint")
	}

	// Verify tests array compiles (for documentation)
	_ = tests
}

func TestComputeFingerprint_CodeNormalization(t *testing.T) {
	// Whitespace changes shouldn't affect fingerprint
	snippet1 := "const x = 1;\nconst y = 2;"
	snippet2 := "const x = 1;   \n  const y = 2;  "

	fp1 := ComputeFingerprint("test.ts", "x", "m", snippet1, 1)
	fp2 := ComputeFingerprint("test.ts", "x", "m", snippet2, 1)

	if fp1 != fp2 {
		t.Error("Whitespace-only changes should not affect fingerprint")
	}
}

func TestTrackingDB_OperationLog(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()

	// Log operation
	op := &Operation{
		Operation:   "create",
		Fingerprint: "abc123",
		Status:      "pending",
		CreatedAt:   now,
	}

	id, err := db.LogOperation(op)
	if err != nil {
		t.Fatalf("LogOperation failed: %v", err)
	}

	// Get pending
	pending, err := db.GetPendingOperations()
	if err != nil {
		t.Fatalf("GetPendingOperations failed: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending, got %d", len(pending))
	}

	// Update status
	if err := db.UpdateOperationStatus(id, "completed", ""); err != nil {
		t.Fatalf("UpdateOperationStatus failed: %v", err)
	}

	// Should no longer be pending
	pending, _ = db.GetPendingOperations()
	if len(pending) != 0 {
		t.Error("Should have no pending operations")
	}
}

func TestTrackingDB_Stats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()

	// Empty
	total, unres, res, err := db.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if total != 0 || unres != 0 || res != 0 {
		t.Error("Empty DB should have all zeros")
	}

	// Add findings
	db.Store(&Finding{
		Fingerprint: "fp1", IssueID: "t1",
		File: "a.ts", Line: 1, Severity: "x", Category: "x", Message: "x",
		FirstSeen: now, LastSeen: now,
	})
	db.Store(&Finding{
		Fingerprint: "fp2", IssueID: "t2",
		File: "b.ts", Line: 2, Severity: "x", Category: "x", Message: "x",
		FirstSeen: now, LastSeen: now,
	})
	db.MarkResolved("fp2", now)

	total, unres, res, _ = db.Stats()
	if total != 2 || unres != 1 || res != 1 {
		t.Errorf("Stats wrong: total=%d, unres=%d, res=%d", total, unres, res)
	}
}

// Helper
func setupTestDB(t *testing.T) *TrackingDB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	return db
}
