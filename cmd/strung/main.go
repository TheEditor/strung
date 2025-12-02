package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/TheEditor/strung/pkg/parser"
	"github.com/TheEditor/strung/pkg/transform"
)

var (
	minSeverity = flag.String("min-severity", "warning", "Minimum severity (critical, warning, info)")
	verbose     = flag.Bool("verbose", false, "Enable verbose output")
	showVersion = flag.Bool("version", false, "Print version and exit")
)

// Set via ldflags
var versionStr = "0.1.0-dev"

// Exit codes
const (
	ExitSuccess    = 0
	ExitInputError = 1
	ExitUsageError = 2
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `strung - Transform UBS findings to Beads issues

Usage: strung [options]

Reads UBS JSON from stdin, writes Beads JSONL to stdout.

Examples:
  ubs --format=json src/ | strung
  ubs --format=json src/ | strung --min-severity=critical
  ubs --format=json src/ | strung | bd import

Options:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("strung v%s\n", versionStr)
		os.Exit(ExitSuccess)
	}

	// Validate flags
	validSeverities := map[string]bool{"critical": true, "warning": true, "info": true}
	if !validSeverities[*minSeverity] {
		fmt.Fprintf(os.Stderr, "Error: invalid severity %q (use: critical, warning, info)\n", *minSeverity)
		os.Exit(ExitUsageError)
	}

	// Configure logging
	log.SetOutput(os.Stderr)
	log.SetPrefix("[strung] ")
	if !*verbose {
		log.SetOutput(io.Discard)
	}

	log.Println("Parsing UBS input from stdin...")

	// Parse UBS JSON from stdin
	report, err := parser.ParseUBS(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitInputError)
	}

	log.Printf("Parsed %d findings from %s", len(report.Findings), report.Project)

	// Filter by severity
	findings := report.FilterBySeverity(*minSeverity)
	log.Printf("After severity filter (%s): %d findings", *minSeverity, len(findings))

	if len(findings) == 0 {
		log.Println("No findings match filter, exiting successfully")
		os.Exit(ExitSuccess)
	}

	// Transform to Beads issues
	transformer := transform.NewTransformer()
	transformer.Verbose = *verbose
	issues := transformer.TransformAll(findings)

	log.Printf("Transformed %d issues", len(issues))

	// Output JSONL to stdout
	for _, issue := range issues {
		jsonl, err := issue.ToJSONL()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing issue: %v\n", err)
			os.Exit(ExitInputError)
		}
		fmt.Println(jsonl)
	}

	log.Println("Done")
}
