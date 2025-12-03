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

func TestCLIIntegration(t *testing.T) {
	// Use temp directory to avoid test pollution
	tmpDir := t.TempDir()
	binName := "strung-test"
	if runtime.GOOS == "windows" {
		binName = "strung-test.exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir, _ = os.Getwd()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, output)
	}

	t.Run("basic transformation", func(t *testing.T) {
		input := `{"project":"/test","files_scanned":1,"findings":[{"file":"test.ts","line":42,"severity":"critical","category":"null-safety","message":"Test message"}],"summary":{"critical":1,"warning":0,"info":0}}`

		cmd := exec.Command(binPath, "transform")
		cmd.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("Run failed: %v\nstderr: %s", err, stderr.String())
		}

		output := stdout.String()
		if !strings.Contains(output, "null-safety") {
			t.Error("Output missing category")
		}
		if !strings.Contains(output, "test.ts:42") {
			t.Error("Output missing location")
		}
	})

	t.Run("version flag", func(t *testing.T) {
		cmd := exec.Command(binPath, "version")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("Version failed: %v", err)
		}
		if !strings.Contains(string(output), "strung v") {
			t.Errorf("Version output incorrect: %s", output)
		}
	})

	t.Run("severity filter", func(t *testing.T) {
		input := `{"project":"/test","files_scanned":1,"findings":[
			{"file":"a.ts","line":1,"severity":"critical","category":"x","message":"m"},
			{"file":"b.ts","line":2,"severity":"warning","category":"x","message":"m"},
			{"file":"c.ts","line":3,"severity":"info","category":"x","message":"m"}
		],"summary":{"critical":1,"warning":1,"info":1}}`

		cmd := exec.Command(binPath, "transform", "--min-severity=critical")
		cmd.Stdin = strings.NewReader(input)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) != 1 {
			t.Errorf("Expected 1 issue (critical only), got %d", len(lines))
		}
	})

	t.Run("empty findings", func(t *testing.T) {
		input := `{"project":"/test","files_scanned":0,"findings":[],"summary":{}}`

		cmd := exec.Command(binPath, "transform")
		cmd.Stdin = strings.NewReader(input)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		if err := cmd.Run(); err != nil {
			t.Fatalf("Should succeed with empty findings: %v", err)
		}

		if stdout.Len() != 0 {
			t.Errorf("Expected no output for empty findings, got: %s", stdout.String())
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		cmd := exec.Command(binPath, "transform")
		cmd.Stdin = strings.NewReader("{invalid json")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			t.Error("Should fail on invalid JSON")
		}

		if !strings.Contains(stderr.String(), "Error") {
			t.Error("Should print error message")
		}
	})

	t.Run("invalid severity flag", func(t *testing.T) {
		cmd := exec.Command(binPath, "transform", "--min-severity=invalid")
		err := cmd.Run()
		if err == nil {
			t.Error("Should fail on invalid severity")
		}
	})
}
