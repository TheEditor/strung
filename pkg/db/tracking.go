// Package db provides SQLite-based tracking for UBS findings.
// It maps fingerprinted findings to Beads issue IDs for incremental sync.
package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS findings (
	fingerprint TEXT PRIMARY KEY,
	issue_id TEXT NOT NULL,
	file TEXT NOT NULL,
	line INTEGER NOT NULL,
	severity TEXT NOT NULL,
	category TEXT NOT NULL,
	message TEXT NOT NULL,
	first_seen TIMESTAMP NOT NULL,
	last_seen TIMESTAMP NOT NULL,
	resolved_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_findings_issue_id ON findings(issue_id);
CREATE INDEX IF NOT EXISTS idx_findings_file ON findings(file);
CREATE INDEX IF NOT EXISTS idx_findings_resolved ON findings(resolved_at);

CREATE TABLE IF NOT EXISTS operation_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	operation TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	issue_id TEXT,
	status TEXT NOT NULL,
	error TEXT,
	created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_op_status ON operation_log(status);
CREATE INDEX IF NOT EXISTS idx_op_fingerprint ON operation_log(fingerprint);
`

// TrackingDB manages the findings database
type TrackingDB struct {
	db   *sql.DB
	path string
}

// Finding represents a tracked finding
type Finding struct {
	Fingerprint string
	IssueID     string
	File        string
	Line        int
	Severity    string
	Category    string
	Message     string
	FirstSeen   time.Time
	LastSeen    time.Time
	ResolvedAt  *time.Time
}

// Operation represents a logged operation
type Operation struct {
	ID          int64
	Operation   string // "create", "update", "close"
	Fingerprint string
	IssueID     string
	Status      string // "pending", "completed", "failed"
	Error       string
	CreatedAt   time.Time
}

// Open creates or opens a tracking database
func Open(path string) (*TrackingDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Initialize schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	return &TrackingDB{db: db, path: path}, nil
}

// Close closes the database
func (t *TrackingDB) Close() error {
	return t.db.Close()
}

// Path returns the database file path
func (t *TrackingDB) Path() string {
	return t.path
}

// ComputeFingerprint generates a stable fingerprint for a finding.
// Uses code context when available for stability across line number changes.
func ComputeFingerprint(file, category, message, codeSnippet string, line int) string {
	h := sha256.New()

	if codeSnippet != "" {
		// Use code context for stability
		context := normalizeCodeContext(codeSnippet)
		fmt.Fprintf(h, "%s:%s:%s:%s", file, category, message, context)
	} else {
		// Fallback to line-based fingerprint
		fmt.Fprintf(h, "%s:%d:%s:%s", file, line, category, message)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// normalizeCodeContext extracts first 3 + last 3 lines, normalized
func normalizeCodeContext(snippet string) string {
	lines := strings.Split(snippet, "\n")

	var selected []string
	if len(lines) <= 6 {
		selected = lines
	} else {
		selected = append(selected, lines[0:3]...)
		selected = append(selected, lines[len(lines)-3:]...)
	}

	var normalized []string
	for _, line := range selected {
		line = strings.TrimSpace(line)
		line = strings.ToLower(line)
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			normalized = append(normalized, line)
		}
	}

	return strings.Join(normalized, "|")
}

// Store stores or updates a finding (upsert)
func (t *TrackingDB) Store(f *Finding) error {
	query := `
		INSERT INTO findings (fingerprint, issue_id, file, line, severity, category, message, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET
			last_seen = excluded.last_seen,
			severity = excluded.severity,
			resolved_at = NULL
	`

	_, err := t.db.Exec(query,
		f.Fingerprint, f.IssueID, f.File, f.Line, f.Severity, f.Category, f.Message,
		f.FirstSeen, f.LastSeen)
	if err != nil {
		return fmt.Errorf("store finding %s: %w", f.Fingerprint[:12], err)
	}

	return nil
}

// Get retrieves a finding by fingerprint
func (t *TrackingDB) Get(fingerprint string) (*Finding, error) {
	query := `
		SELECT fingerprint, issue_id, file, line, severity, category, message, first_seen, last_seen, resolved_at
		FROM findings
		WHERE fingerprint = ?
	`

	var f Finding
	var resolvedAt sql.NullTime

	err := t.db.QueryRow(query, fingerprint).Scan(
		&f.Fingerprint, &f.IssueID, &f.File, &f.Line, &f.Severity, &f.Category, &f.Message,
		&f.FirstSeen, &f.LastSeen, &resolvedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get finding %s: %w", fingerprint[:12], err)
	}

	if resolvedAt.Valid {
		f.ResolvedAt = &resolvedAt.Time
	}

	return &f, nil
}

// GetByIssueID retrieves a finding by Beads issue ID
func (t *TrackingDB) GetByIssueID(issueID string) (*Finding, error) {
	query := `
		SELECT fingerprint, issue_id, file, line, severity, category, message, first_seen, last_seen, resolved_at
		FROM findings
		WHERE issue_id = ?
	`

	var f Finding
	var resolvedAt sql.NullTime

	err := t.db.QueryRow(query, issueID).Scan(
		&f.Fingerprint, &f.IssueID, &f.File, &f.Line, &f.Severity, &f.Category, &f.Message,
		&f.FirstSeen, &f.LastSeen, &resolvedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get finding by issue %s: %w", issueID, err)
	}

	if resolvedAt.Valid {
		f.ResolvedAt = &resolvedAt.Time
	}

	return &f, nil
}

// GetUnresolved retrieves all unresolved findings
func (t *TrackingDB) GetUnresolved() ([]*Finding, error) {
	query := `
		SELECT fingerprint, issue_id, file, line, severity, category, message, first_seen, last_seen, resolved_at
		FROM findings
		WHERE resolved_at IS NULL
		ORDER BY last_seen DESC
	`

	return t.queryFindings(query)
}

// GetAll retrieves all findings (for debugging)
func (t *TrackingDB) GetAll() ([]*Finding, error) {
	query := `
		SELECT fingerprint, issue_id, file, line, severity, category, message, first_seen, last_seen, resolved_at
		FROM findings
		ORDER BY last_seen DESC
	`

	return t.queryFindings(query)
}

func (t *TrackingDB) queryFindings(query string) ([]*Finding, error) {
	rows, err := t.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query findings: %w", err)
	}
	defer rows.Close()

	var findings []*Finding
	for rows.Next() {
		var f Finding
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&f.Fingerprint, &f.IssueID, &f.File, &f.Line, &f.Severity, &f.Category, &f.Message,
			&f.FirstSeen, &f.LastSeen, &resolvedAt)
		if err != nil {
			return nil, fmt.Errorf("scan finding: %w", err)
		}

		if resolvedAt.Valid {
			f.ResolvedAt = &resolvedAt.Time
		}

		findings = append(findings, &f)
	}

	return findings, rows.Err()
}

// MarkResolved marks a finding as resolved
func (t *TrackingDB) MarkResolved(fingerprint string, resolvedAt time.Time) error {
	query := `UPDATE findings SET resolved_at = ? WHERE fingerprint = ?`
	result, err := t.db.Exec(query, resolvedAt, fingerprint)
	if err != nil {
		return fmt.Errorf("mark resolved %s: %w", fingerprint[:12], err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("finding %s not found", fingerprint[:12])
	}

	return nil
}

// LogOperation records an operation attempt
func (t *TrackingDB) LogOperation(op *Operation) (int64, error) {
	query := `
		INSERT INTO operation_log (operation, fingerprint, issue_id, status, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := t.db.Exec(query,
		op.Operation, op.Fingerprint, op.IssueID, op.Status, op.Error, op.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("log operation: %w", err)
	}

	return result.LastInsertId()
}

// UpdateOperationStatus updates operation status
func (t *TrackingDB) UpdateOperationStatus(id int64, status, errorMsg string) error {
	query := `UPDATE operation_log SET status = ?, error = ? WHERE id = ?`
	_, err := t.db.Exec(query, status, errorMsg, id)
	if err != nil {
		return fmt.Errorf("update operation %d: %w", id, err)
	}
	return nil
}

// GetPendingOperations returns all pending operations
func (t *TrackingDB) GetPendingOperations() ([]*Operation, error) {
	query := `
		SELECT id, operation, fingerprint, issue_id, status, error, created_at
		FROM operation_log
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`

	rows, err := t.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get pending operations: %w", err)
	}
	defer rows.Close()

	var ops []*Operation
	for rows.Next() {
		var op Operation
		var issueID, errMsg sql.NullString

		err := rows.Scan(&op.ID, &op.Operation, &op.Fingerprint, &issueID,
			&op.Status, &errMsg, &op.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan operation: %w", err)
		}

		if issueID.Valid {
			op.IssueID = issueID.String
		}
		if errMsg.Valid {
			op.Error = errMsg.String
		}

		ops = append(ops, &op)
	}

	return ops, rows.Err()
}

// GetOrphanedIssues finds issues created but not in findings table
func (t *TrackingDB) GetOrphanedIssues() ([]string, error) {
	query := `
		SELECT DISTINCT issue_id
		FROM operation_log
		WHERE operation = 'create' AND status = 'completed'
		  AND issue_id NOT IN (SELECT issue_id FROM findings)
		ORDER BY created_at DESC
	`

	rows, err := t.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get orphaned issues: %w", err)
	}
	defer rows.Close()

	var issueIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan orphan: %w", err)
		}
		issueIDs = append(issueIDs, id)
	}

	return issueIDs, rows.Err()
}

// Stats returns database statistics
func (t *TrackingDB) Stats() (total, unresolved, resolved int, err error) {
	err = t.db.QueryRow("SELECT COUNT(*) FROM findings").Scan(&total)
	if err != nil {
		return
	}
	err = t.db.QueryRow("SELECT COUNT(*) FROM findings WHERE resolved_at IS NULL").Scan(&unresolved)
	if err != nil {
		return
	}
	resolved = total - unresolved
	return
}
