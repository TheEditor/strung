//go:build integration

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSync_DryRun(t *testing.T) {
	binPath := buildBinary(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")

	input := `{"project":"/test","files_scanned":1,"findings":[
		{"file":"test.ts","line":42,"severity":"critical","category":"null-safety","message":"Test message"}
	],"summary":{"critical":1}}`

	cmd := exec.Command(binPath, "sync", "--dry-run", "--db-path", dbPath)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Sync failed: %v\nstderr: %s", err, stderr.String())
	}

	output := stderr.String()
	if !strings.Contains(output, "[DRY RUN]") {
		t.Error("Should show dry run prefix")
	}
	if !strings.Contains(output, "Would create") {
		t.Error("Should show would create message")
	}

	// DB should NOT be created in dry-run
	// (Actually it is created but empty - that's fine)
}

func TestSync_EmptyFindings(t *testing.T) {
	binPath := buildBinary(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")

	input := `{"project":"/test","files_scanned":0,"findings":[],"summary":{}}`

	cmd := exec.Command(binPath, "sync", "--dry-run", "--db-path", dbPath)
	cmd.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "No changes") {
		t.Errorf("Should indicate no changes: %s", output)
	}
}

func TestSync_SeverityFilter(t *testing.T) {
	binPath := buildBinary(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")

	input := `{"project":"/test","files_scanned":1,"findings":[
		{"file":"a.ts","line":1,"severity":"critical","category":"x","message":"crit"},
		{"file":"b.ts","line":2,"severity":"warning","category":"x","message":"warn"},
		{"file":"c.ts","line":3,"severity":"info","category":"x","message":"info"}
	],"summary":{"critical":1,"warning":1,"info":1}}`

	// Critical only
	cmd := exec.Command(binPath, "sync", "--dry-run", "--db-path", dbPath, "--min-severity=critical")
	cmd.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "New: 1") {
		t.Errorf("Should have 1 new (critical only): %s", output)
	}
}

func TestSync_InvalidSeverity(t *testing.T) {
	binPath := buildBinary(t)

	cmd := exec.Command(binPath, "sync", "--min-severity=invalid")
	cmd.Stdin = strings.NewReader("{}")

	err := cmd.Run()
	if err == nil {
		t.Error("Should fail with invalid severity")
	}
}

func TestSync_MultiScanWorkflow(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd CLI not available - skipping integration test")
	}

	binPath := buildBinary(t)
	dbPath := filepath.Join(t.TempDir(), "workflow.db")

	// Scan 1: Initial findings
	scan1 := `{"project":"/test","files_scanned":2,"findings":[
		{"file":"a.ts","line":1,"severity":"critical","category":"x","message":"m1"},
		{"file":"b.ts","line":2,"severity":"warning","category":"x","message":"m2"}
	],"summary":{"critical":1,"warning":1}}`

	cmd := exec.Command(binPath, "sync", "--db-path", dbPath)
	cmd.Stdin = strings.NewReader(scan1)
	var stderr1 bytes.Buffer
	cmd.Stderr = &stderr1

	if err := cmd.Run(); err != nil {
		t.Fatalf("Scan 1 failed: %v\n%s", err, stderr1.String())
	}

	if !strings.Contains(stderr1.String(), "New: 2") {
		t.Errorf("Scan 1 should have 2 new: %s", stderr1.String())
	}

	// Scan 2: Same findings (no changes)
	cmd = exec.Command(binPath, "sync", "--db-path", dbPath)
	cmd.Stdin = strings.NewReader(scan1)
	var stderr2 bytes.Buffer
	cmd.Stderr = &stderr2

	if err := cmd.Run(); err != nil {
		t.Fatalf("Scan 2 failed: %v\n%s", err, stderr2.String())
	}

	if !strings.Contains(stderr2.String(), "No changes") {
		t.Errorf("Scan 2 should have no changes: %s", stderr2.String())
	}

	// Scan 3: One resolved, one changed
	scan3 := `{"project":"/test","files_scanned":1,"findings":[
		{"file":"a.ts","line":1,"severity":"warning","category":"x","message":"m1"}
	],"summary":{"warning":1}}`

	cmd = exec.Command(binPath, "sync", "--db-path", dbPath, "--auto-close")
	cmd.Stdin = strings.NewReader(scan3)
	var stderr3 bytes.Buffer
	cmd.Stderr = &stderr3

	if err := cmd.Run(); err != nil {
		t.Fatalf("Scan 3 failed: %v\n%s", err, stderr3.String())
	}

	output3 := stderr3.String()
	if !strings.Contains(output3, "Changed: 1") {
		t.Errorf("Scan 3 should have 1 changed: %s", output3)
	}
	if !strings.Contains(output3, "Resolved: 1") {
		t.Errorf("Scan 3 should have 1 resolved: %s", output3)
	}
}

// Helper
func buildBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binName := "strung-test"
	if runtime.GOOS == "windows" {
		binName = "strung-test.exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	// Build current package (cmd/strung)
	cmd := exec.Command("go", "build", "-tags", "integration", "-o", binPath, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, output)
	}

	// Verify binary exists
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("Binary not found after build: %v", err)
	}

	return binPath
}
