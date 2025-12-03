package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/TheEditor/strung/pkg/beads"
)

// BeadsCreateResponse is the JSON response from bd create
type BeadsCreateResponse struct {
	ID string `json:"id"`
}

// createBeadsIssue creates an issue via bd CLI, returns assigned issue ID
func createBeadsIssue(issue *beads.Issue) (string, error) {
	args := []string{
		"create",
		issue.Title,
		"-t", issue.Type,
		"-p", fmt.Sprintf("%d", issue.Priority),
		"-d", issue.Description,
		"--json",
	}

	if issue.Design != "" {
		args = append(args, "--design", issue.Design)
	}
	if issue.Acceptance != "" {
		args = append(args, "--acceptance", issue.Acceptance)
	}
	for _, tag := range issue.Tags {
		args = append(args, "--tag", tag)
	}

	cmd := exec.Command("bd", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("bd create failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse JSON response to extract assigned ID
	var result BeadsCreateResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return "", fmt.Errorf("parse bd output: %w\noutput: %s", err, stdout.String())
	}

	if result.ID == "" {
		return "", fmt.Errorf("bd create returned empty ID\noutput: %s", stdout.String())
	}

	return result.ID, nil
}

// updateBeadsPriority updates issue priority
func updateBeadsPriority(issueID string, priority int) error {
	cmd := exec.Command("bd", "update", issueID, "-p", fmt.Sprintf("%d", priority))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bd update %s failed: %w\nstderr: %s", issueID, err, stderr.String())
	}

	return nil
}

// closeBeadsIssue closes an issue
func closeBeadsIssue(issueID string) error {
	cmd := exec.Command("bd", "close", issueID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bd close %s failed: %w\nstderr: %s", issueID, err, stderr.String())
	}

	return nil
}

// checkBeadsCLI verifies bd CLI is available
func checkBeadsCLI() error {
	cmd := exec.Command("bd", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bd CLI not found or not working: %w\nInstall: https://github.com/steveyegge/beads", err)
	}
	return nil
}
