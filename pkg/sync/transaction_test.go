package sync

import (
	"testing"
	"time"

	"github.com/TheEditor/strung/pkg/db"
)

func TestTransaction_CreateFlow(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open database: %v", err)
	}
	defer database.Close()

	txn := &Transaction{database: database}

	finding := &db.Finding{
		Fingerprint: "test-fp-1",
		IssueID:     "proj-001",
		File:        "test.ts",
		Line:        42,
		Severity:    "critical",
		Category:    "null-safety",
		Message:     "Test finding",
	}

	// Begin creation
	if err := txn.BeginCreate(finding); err != nil {
		t.Fatalf("BeginCreate: %v", err)
	}

	if txn.status != OperationPending {
		t.Errorf("Expected pending status, got %v", txn.status)
	}

	// Complete creation
	if err := txn.CompleteCreate(finding); err != nil {
		t.Fatalf("CompleteCreate: %v", err)
	}

	if txn.status != OperationCompleted {
		t.Errorf("Expected completed status, got %v", txn.status)
	}
}

func TestTransaction_FailFlow(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open database: %v", err)
	}
	defer database.Close()

	txn := &Transaction{database: database}

	fp := "test-fp-fail"
	issueID := "proj-fail"

	// Begin update
	if err := txn.BeginUpdate(issueID, fp); err != nil {
		t.Fatalf("BeginUpdate: %v", err)
	}

	// Fail operation
	if err := txn.FailOperation(fp); err != nil {
		t.Fatalf("FailOperation: %v", err)
	}

	if txn.status != OperationFailed {
		t.Errorf("Expected failed status, got %v", txn.status)
	}
}

func TestTransaction_UpdateAndClose(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open database: %v", err)
	}
	defer database.Close()

	txn := &Transaction{database: database}

	fp := "test-fp-update"
	issueID := "proj-update"

	// Begin and complete update
	if err := txn.BeginUpdate(issueID, fp); err != nil {
		t.Fatalf("BeginUpdate: %v", err)
	}

	if err := txn.CompleteUpdate(fp); err != nil {
		t.Fatalf("CompleteUpdate: %v", err)
	}

	if txn.status != OperationCompleted {
		t.Errorf("Expected completed status, got %v", txn.status)
	}

	// Begin and complete close
	fp2 := "test-fp-close"
	if err := txn.BeginClose(issueID, fp2); err != nil {
		t.Fatalf("BeginClose: %v", err)
	}

	if err := txn.CompleteClose(fp2); err != nil {
		t.Fatalf("CompleteClose: %v", err)
	}

	if txn.status != OperationCompleted {
		t.Errorf("Expected completed status, got %v", txn.status)
	}
}

func TestTransaction_Summary(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open database: %v", err)
	}
	defer database.Close()

	txn := &Transaction{database: database}
	txn.startTime = time.Now()
	time.Sleep(10 * time.Millisecond) // Ensure duration > 0
	txn.endTime = time.Now()

	summary := txn.Summary()

	if summary.Duration <= 0 {
		t.Errorf("Expected positive duration, got %v", summary.Duration)
	}

	if summary.LastOperationAt != txn.endTime {
		t.Errorf("Expected last operation at %v, got %v", txn.endTime, summary.LastOperationAt)
	}

	// Verify string representation doesn't panic
	str := summary.String()
	if str == "" {
		t.Error("Expected non-empty summary string")
	}
}
