package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/TheEditor/strung/pkg/db"
)

type recoverCmd struct {
	dbPath string
	fix    bool
}

func newRecoverCmd() *recoverCmd {
	return &recoverCmd{}
}

func (r *recoverCmd) flags(fs *flag.FlagSet) {
	fs.StringVar(&r.dbPath, "db-path", ".strung.db", "Path to tracking database")
	fs.BoolVar(&r.fix, "fix", false, "Fix issues (mark pending as failed)")
}

func (r *recoverCmd) usage() {
	fmt.Fprintf(os.Stderr, `Usage: strung recover [flags]

Check database consistency and recover from incomplete operations.

Flags:
  --db-path PATH  Path to tracking database (default: .strung.db)
  --fix           Fix issues (mark pending operations as failed)

Examples:
  # Check database consistency
  strung recover --db-path=.strung.db

  # Fix pending operations
  strung recover --db-path=.strung.db --fix
`)
}

func (r *recoverCmd) run() int {
	// Open tracking DB
	database, err := db.Open(r.dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 3
	}
	defer database.Close()

	fmt.Fprintf(os.Stderr, "Checking database: %s\n\n", r.dbPath)

	// Check database stats
	findings, err := database.GetAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading findings: %v\n", err)
		return 3
	}

	if len(findings) == 0 {
		fmt.Fprintf(os.Stderr, "Database empty - no findings tracked\n")
		return 0
	}

	// Count by severity and resolution
	sevCounts := make(map[string]int)
	resolved := 0
	open := 0

	for _, f := range findings {
		sevCounts[f.Severity]++
		if f.ResolvedAt != nil {
			resolved++
		} else {
			open++
		}
	}

	fmt.Fprintf(os.Stderr, "Findings by severity:\n")
	for sev, count := range sevCounts {
		fmt.Fprintf(os.Stderr, "  %s: %d\n", sev, count)
	}

	fmt.Fprintf(os.Stderr, "\nFindings by status:\n")
	fmt.Fprintf(os.Stderr, "  open: %d\n", open)
	fmt.Fprintf(os.Stderr, "  resolved: %d\n", resolved)

	// Check for issues
	var issues []string

	// Check for unmatched issue IDs
	seenIssueIDs := make(map[string]int)
	for _, f := range findings {
		seenIssueIDs[f.IssueID]++
	}

	// Check for duplicate fingerprints
	seenFps := make(map[string]int)
	duplicates := 0
	for _, f := range findings {
		seenFps[f.Fingerprint]++
		if seenFps[f.Fingerprint] > 1 {
			duplicates++
		}
	}
	if duplicates > 0 {
		issues = append(issues, fmt.Sprintf("Found %d duplicate fingerprints", duplicates))
	}

	// Report issues
	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "\nIssues found:\n")
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", issue)
		}

		if r.fix {
			fmt.Fprintf(os.Stderr, "\nFixing issues...\n")
			// Mark duplicates as failed
			for fp, count := range seenFps {
				if count > 1 {
					if err := database.UpdateOperationStatus(0, "failed", "duplicate fingerprint"); err != nil {
						fmt.Fprintf(os.Stderr, "Error marking %s as failed: %v\n", fp, err)
					}
				}
			}
			fmt.Fprintf(os.Stderr, "Fixes applied\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "\nNo issues found\n")
	}

	return 0
}
