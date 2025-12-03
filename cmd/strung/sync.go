package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/TheEditor/strung/pkg/db"
	"github.com/TheEditor/strung/pkg/parser"
	"github.com/TheEditor/strung/pkg/sync"
	"github.com/TheEditor/strung/pkg/transform"
)

// Exit codes for sync command
const (
	ExitSyncSuccess    = 0
	ExitSyncInputError = 1
	ExitSyncUsageError = 2
	ExitSyncError      = 3
)

type syncCmd struct {
	dbPath      string
	autoClose   bool
	dryRun      bool
	minSeverity string
	repoURL     string
	repoBranch  string
	verbose     bool
}

func newSyncCmd() *syncCmd {
	return &syncCmd{}
}

func (s *syncCmd) flags(fs *flag.FlagSet) {
	fs.StringVar(&s.dbPath, "db-path", ".strung.db", "Path to tracking database")
	fs.BoolVar(&s.autoClose, "auto-close", false, "Automatically close resolved issues")
	fs.BoolVar(&s.dryRun, "dry-run", false, "Show actions without executing")
	fs.StringVar(&s.minSeverity, "min-severity", "warning", "Minimum severity (critical, warning, info)")
	fs.StringVar(&s.repoURL, "repo-url", "", "Repository URL for file links (e.g., https://github.com/user/repo)")
	fs.StringVar(&s.repoBranch, "repo-branch", "main", "Repository branch for file links")
	fs.BoolVar(&s.verbose, "verbose", false, "Enable verbose output")
}

func (s *syncCmd) usage() {
	fmt.Fprintf(os.Stderr, `Usage: strung sync [flags]

Read UBS JSON from stdin, incrementally sync findings to Beads issues.

Flags:
  --db-path PATH        Path to tracking database (default: .strung.db)
  --auto-close          Automatically close resolved issues
  --dry-run             Show actions without executing
  --min-severity LEVEL  Minimum severity: critical, warning, info (default: warning)
  --repo-url URL        Repository URL for file links
  --repo-branch BRANCH  Repository branch (default: main)
  --verbose             Enable verbose output

Examples:
  # First sync
  ubs --format=json src/ | strung sync

  # With auto-close
  ubs --format=json src/ | strung sync --auto-close

  # Dry run preview
  ubs --format=json src/ | strung sync --dry-run

  # With GitHub links
  ubs --format=json src/ | strung sync --repo-url=https://github.com/user/repo

See docs/SYNC.md for complete documentation.
`)
}

func (s *syncCmd) run() int {
	// Verify bd CLI available (unless dry-run)
	if !s.dryRun {
		if err := checkBeadsCLI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return ExitSyncError
		}
	}

	// Validate severity
	validSeverities := map[string]bool{"critical": true, "warning": true, "info": true}
	if !validSeverities[s.minSeverity] {
		fmt.Fprintf(os.Stderr, "Error: invalid severity %q (use: critical, warning, info)\n", s.minSeverity)
		return ExitSyncUsageError
	}

	// Open/create tracking DB
	database, err := db.Open(s.dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitSyncError
	}
	defer database.Close()

	if s.verbose {
		fmt.Fprintf(os.Stderr, "Using database: %s\n", s.dbPath)
	}

	// Parse UBS JSON from stdin
	report, err := parser.ParseUBS(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitSyncInputError
	}

	if s.verbose {
		fmt.Fprintf(os.Stderr, "Parsed %d findings from %s\n", len(report.Findings), report.Project)
	}

	// Filter by severity
	findings := report.FilterBySeverity(s.minSeverity)
	if s.verbose {
		fmt.Fprintf(os.Stderr, "After severity filter (%s): %d findings\n", s.minSeverity, len(findings))
	}

	// Compute diff
	differ := sync.NewDiffer(database)
	diffResult, err := differ.Diff(findings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing diff: %v\n", err)
		return ExitSyncError
	}

	// Print summary
	fmt.Fprintf(os.Stderr, "Sync summary: %s\n", diffResult.Stats())

	if diffResult.IsEmpty() {
		fmt.Fprintf(os.Stderr, "No changes to sync.\n")
		return ExitSyncSuccess
	}

	// Execute actions
	exitCode := s.executeActions(database, diffResult)
	return exitCode
}

func (s *syncCmd) executeActions(database *db.TrackingDB, result *sync.DiffResult) int {
	transformer := transform.NewTransformer()
	now := time.Now()
	hasErrors := false

	// Create issues for new findings
	for _, finding := range result.New {
		issue, err := transformer.Transform(finding)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR transforming finding: %v\n", err)
			hasErrors = true
			continue
		}

		if s.dryRun {
			fmt.Fprintf(os.Stderr, "[DRY RUN] Would create: %s\n", issue.Title)
			continue
		}

		// Create via bd CLI
		issueID, err := createBeadsIssue(issue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR creating issue: %v\n", err)
			hasErrors = true
			continue
		}

		// Record in DB
		fp := db.ComputeFingerprint(finding.File, finding.Category, finding.Message, finding.CodeSnippet, finding.Line)
		dbFinding := &db.Finding{
			Fingerprint: fp,
			IssueID:     issueID,
			File:        finding.File,
			Line:        finding.Line,
			Severity:    finding.Severity,
			Category:    finding.Category,
			Message:     finding.Message,
			FirstSeen:   now,
			LastSeen:    now,
		}
		if err := database.Store(dbFinding); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR storing finding: %v (issue: %s)\n", err, issueID)
			hasErrors = true
			continue
		}

		fmt.Fprintf(os.Stderr, "Created: %s → %s\n", issue.Title, issueID)
	}

	// Update changed findings
	for _, change := range result.Changed {
		if s.dryRun {
			fmt.Fprintf(os.Stderr, "[DRY RUN] Would update: %s (severity %s → %s)\n",
				change.Previous.IssueID, change.Previous.Severity, change.Current.Severity)
			continue
		}

		// Update priority in Beads
		newPriority := transformer.SeverityToPriority(change.Current.Severity)
		if err := updateBeadsPriority(change.Previous.IssueID, newPriority); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR updating %s: %v\n", change.Previous.IssueID, err)
			hasErrors = true
			continue
		}

		// Update in DB
		fp := db.ComputeFingerprint(change.Current.File, change.Current.Category,
			change.Current.Message, change.Current.CodeSnippet, change.Current.Line)
		dbFinding := &db.Finding{
			Fingerprint: fp,
			IssueID:     change.Previous.IssueID,
			File:        change.Current.File,
			Line:        change.Current.Line,
			Severity:    change.Current.Severity,
			Category:    change.Current.Category,
			Message:     change.Current.Message,
			FirstSeen:   change.Previous.FirstSeen,
			LastSeen:    now,
		}
		if err := database.Store(dbFinding); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR updating DB: %v\n", err)
			hasErrors = true
		}

		fmt.Fprintf(os.Stderr, "Updated: %s (priority %d)\n", change.Previous.IssueID, newPriority)
	}

	// Handle resolved findings
	if s.autoClose {
		for _, resolved := range result.Resolved {
			if s.dryRun {
				fmt.Fprintf(os.Stderr, "[DRY RUN] Would close: %s\n", resolved.IssueID)
				continue
			}

			if err := closeBeadsIssue(resolved.IssueID); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR closing %s: %v\n", resolved.IssueID, err)
				hasErrors = true
				continue
			}

			if err := database.MarkResolved(resolved.Fingerprint, now); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR marking resolved: %v\n", err)
				hasErrors = true
			}

			fmt.Fprintf(os.Stderr, "Closed: %s\n", resolved.IssueID)
		}
	} else if len(result.Resolved) > 0 {
		fmt.Fprintf(os.Stderr, "Note: %d resolved findings (use --auto-close to close)\n", len(result.Resolved))
	}

	if hasErrors {
		return ExitSyncError
	}
	return ExitSyncSuccess
}
