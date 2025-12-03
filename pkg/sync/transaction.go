package sync

import (
	"fmt"
	"time"

	"github.com/TheEditor/strung/pkg/db"
)

// OperationStatus represents the status of a database operation
type OperationStatus string

const (
	OperationPending   OperationStatus = "pending"
	OperationCompleted OperationStatus = "completed"
	OperationFailed    OperationStatus = "failed"
)

// Transaction coordinates multi-step operations (create, update, close)
// It logs operations to the database for recovery/debugging purposes
type Transaction struct {
	database  *db.TrackingDB
	issueID   string
	operation string
	startTime time.Time
	endTime   time.Time
	status    OperationStatus
}

// BeginCreate initiates a finding creation operation
func (t *Transaction) BeginCreate(finding *db.Finding) error {
	t.operation = "create"
	t.startTime = time.Now()
	t.status = OperationPending

	// Log pending operation
	op := &db.Operation{
		Operation:   "create",
		Fingerprint: finding.Fingerprint,
		IssueID:     finding.IssueID,
		Status:      "pending",
		CreatedAt:   t.startTime,
	}
	_, err := t.database.LogOperation(op)
	return err
}

// CompleteCreate finalizes a successful creation
func (t *Transaction) CompleteCreate(finding *db.Finding) error {
	t.endTime = time.Now()
	t.status = OperationCompleted

	// Update operation status
	return t.database.UpdateOperationStatus(0, "completed", "")
}

// BeginUpdate initiates an update operation
func (t *Transaction) BeginUpdate(issueID string, fp string) error {
	t.operation = "update"
	t.issueID = issueID
	t.startTime = time.Now()
	t.status = OperationPending

	op := &db.Operation{
		Operation:   "update",
		Fingerprint: fp,
		IssueID:     issueID,
		Status:      "pending",
		CreatedAt:   t.startTime,
	}
	_, err := t.database.LogOperation(op)
	return err
}

// CompleteUpdate finalizes a successful update
func (t *Transaction) CompleteUpdate(fp string) error {
	t.endTime = time.Now()
	t.status = OperationCompleted

	return t.database.UpdateOperationStatus(0, "completed", "")
}

// BeginClose initiates a close operation
func (t *Transaction) BeginClose(issueID string, fp string) error {
	t.operation = "close"
	t.issueID = issueID
	t.startTime = time.Now()
	t.status = OperationPending

	op := &db.Operation{
		Operation:   "close",
		Fingerprint: fp,
		IssueID:     issueID,
		Status:      "pending",
		CreatedAt:   t.startTime,
	}
	_, err := t.database.LogOperation(op)
	return err
}

// CompleteClose finalizes a successful close
func (t *Transaction) CompleteClose(fp string) error {
	t.endTime = time.Now()
	t.status = OperationCompleted

	return t.database.UpdateOperationStatus(0, "completed", "")
}

// FailOperation marks an operation as failed
func (t *Transaction) FailOperation(fp string) error {
	t.endTime = time.Now()
	t.status = OperationFailed

	return t.database.UpdateOperationStatus(0, "failed", "operation failed")
}

// Summary returns operation summary
type Summary struct {
	TotalOperations  int
	Completed        int
	Pending          int
	Failed           int
	Duration         time.Duration
	LastOperationAt  time.Time
}

// Summary returns a summary of database operations
func (t *Transaction) Summary() Summary {
	return Summary{
		Duration:        t.endTime.Sub(t.startTime),
		LastOperationAt: t.endTime,
	}
}

// SummaryString returns a string representation of the summary
func (s Summary) String() string {
	return fmt.Sprintf("Operations: %d total (%d completed, %d pending, %d failed) in %v",
		s.TotalOperations, s.Completed, s.Pending, s.Failed, s.Duration)
}
