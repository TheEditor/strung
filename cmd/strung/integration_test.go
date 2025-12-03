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

func TestIntegration_MultiScanScenarios(t *testing.T) {
	binPath := buildTestBinary(t)
	dbPath := filepath.Join(t.TempDir(), "scenarios.db")

	// Get path to testdata relative to project root
	scanDir := filepath.Join("..", "..", "testdata", "sync-scenarios")

	// Scenario 1: Initial scan with 2 findings (dry-run to verify detection)
	scan1Data, err := os.ReadFile(filepath.Join(scanDir, "scan1.json"))
	if err != nil {
		t.Fatalf("Read scan1.json: %v", err)
	}

	cmd := exec.Command(binPath, "sync", "--dry-run", "--db-path", dbPath)
	cmd.Stdin = bytes.NewReader(scan1Data)
	var stderr1 bytes.Buffer
	cmd.Stderr = &stderr1
	if err := cmd.Run(); err != nil {
		t.Fatalf("Scan 1 failed: %v\n%s", err, stderr1.String())
	}

	output1 := stderr1.String()
	if !strings.Contains(output1, "New: 2") {
		t.Errorf("Scan 1 should have 2 new findings: %s", output1)
	}

	// Store findings in DB for next scans
	cmd = exec.Command(binPath, "sync", "--db-path", dbPath)
	cmd.Stdin = bytes.NewReader(scan1Data)
	if err := cmd.Run(); err != nil {
		t.Logf("Note: Scan 1 store failed (expected if bd CLI unavailable): %v", err)
	}

	// Scenario 2: Verify detection of severity change (dry-run)
	// scan2 has: vault.ts severity changed (critical→warning), crypto.ts unchanged, cli.ts (info - filtered out)
	scan2Data, err := os.ReadFile(filepath.Join(scanDir, "scan2.json"))
	if err != nil {
		t.Fatalf("Read scan2.json: %v", err)
	}

	cmd = exec.Command(binPath, "sync", "--dry-run", "--db-path", dbPath)
	cmd.Stdin = bytes.NewReader(scan2Data)
	var stderr2 bytes.Buffer
	cmd.Stderr = &stderr2
	if err := cmd.Run(); err != nil {
		t.Fatalf("Scan 2 failed: %v\n%s", err, stderr2.String())
	}

	output2 := stderr2.String()
	if !strings.Contains(output2, "Changed: 1") {
		t.Errorf("Scan 2 should have 1 changed (severity change): %s", output2)
	}

	// Store changes
	cmd = exec.Command(binPath, "sync", "--db-path", dbPath)
	cmd.Stdin = bytes.NewReader(scan2Data)
	if err := cmd.Run(); err != nil {
		t.Logf("Note: Scan 2 store failed (expected if bd CLI unavailable): %v", err)
	}

	// Scenario 3: Verify detection of resolutions (dry-run)
	// scan3 has only crypto.ts, so vault.ts is resolved
	scan3Data, err := os.ReadFile(filepath.Join(scanDir, "scan3.json"))
	if err != nil {
		t.Fatalf("Read scan3.json: %v", err)
	}

	cmd = exec.Command(binPath, "sync", "--dry-run", "--db-path", dbPath, "--auto-close")
	cmd.Stdin = bytes.NewReader(scan3Data)
	var stderr3 bytes.Buffer
	cmd.Stderr = &stderr3
	if err := cmd.Run(); err != nil {
		t.Fatalf("Scan 3 failed: %v\n%s", err, stderr3.String())
	}

	output3 := stderr3.String()
	if !strings.Contains(output3, "Resolved: 1") {
		t.Errorf("Scan 3 should have 1 resolved (vault.ts): %s", output3)
	}
}

func TestIntegration_DBPersistence(t *testing.T) {
	binPath := buildTestBinary(t)
	dbPath := filepath.Join(t.TempDir(), "persist.db")

	scan := `{"project":"/test","files_scanned":1,"findings":[
        {"file":"test.ts","line":1,"severity":"critical","category":"x","message":"Persist test"}
    ],"summary":{"critical":1}}`

	// First sync: creates entry
	cmd := exec.Command(binPath, "sync", "--db-path", dbPath)
	cmd.Stdin = strings.NewReader(scan)
	var stderr1 bytes.Buffer
	cmd.Stderr = &stderr1
	if err := cmd.Run(); err == nil {
		// First sync should succeed (would create real issues)
		if !strings.Contains(stderr1.String(), "New: 1") {
			t.Errorf("First sync should show 1 new: %s", stderr1.String())
		}
	}

	// Second sync in dry-run: same data, should show no changes since DB has it
	cmd = exec.Command(binPath, "sync", "--dry-run", "--db-path", dbPath)
	cmd.Stdin = strings.NewReader(scan)
	var stderr2 bytes.Buffer
	cmd.Stderr = &stderr2
	if err := cmd.Run(); err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}
	if !strings.Contains(stderr2.String(), "No changes") {
		t.Errorf("Second sync should show no changes: %s", stderr2.String())
	}
}

func TestIntegration_HelpCommands(t *testing.T) {
	binPath := buildTestBinary(t)

	tests := []struct {
		args     []string
		contains []string
	}{
		{[]string{"help"}, []string{"transform", "sync", "Commands"}},
		{[]string{"sync", "--help"}, []string{"--db-path", "--auto-close", "--dry-run"}},
		{[]string{"version"}, []string{"strung v"}},
	}

	for _, tt := range tests {
		name := strings.Join(tt.args, " ")
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(binPath, tt.args...)
			output, _ := cmd.CombinedOutput()

			for _, expected := range tt.contains {
				if !strings.Contains(string(output), expected) {
					t.Errorf("Missing %q in output: %s", expected, output)
				}
			}
		})
	}
}

func TestIntegration_ErrorCases(t *testing.T) {
	binPath := buildTestBinary(t)

	tests := []struct {
		name      string
		args      []string
		stdin     string
		expectErr bool
	}{
		{
			name:      "Invalid JSON",
			args:      []string{"sync", "--dry-run"},
			stdin:     "{invalid",
			expectErr: true,
		},
		{
			name:      "Invalid severity flag",
			args:      []string{"sync", "--min-severity=bad"},
			stdin:     "{}",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binPath, tt.args...)
			if tt.stdin != "" {
				cmd.Stdin = strings.NewReader(tt.stdin)
			}

			err := cmd.Run()
			if tt.expectErr && err == nil {
				t.Error("Expected error, got success")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}
		})
	}
}

// Helper
func buildTestBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binName := "strung-integration"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	cmd := exec.Command("go", "build", "-tags", "integration", "-o", binPath, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, output)
	}

	return binPath
}
