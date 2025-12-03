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

var versionStr = "0.2.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "transform":
		runTransform(os.Args[2:])

	case "sync":
		fs := flag.NewFlagSet("sync", flag.ExitOnError)
		syncCmd := newSyncCmd()
		syncCmd.flags(fs)

		// Check for help flag
		for _, arg := range os.Args[2:] {
			if arg == "-h" || arg == "--help" || arg == "-help" {
				syncCmd.usage()
				os.Exit(0)
			}
		}

		fs.Parse(os.Args[2:])
		os.Exit(syncCmd.run())

	case "version", "--version", "-v":
		fmt.Printf("strung v%s\n", versionStr)
		os.Exit(0)

	case "help", "--help", "-h":
		printUsage()
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Run 'strung help' for usage.\n")
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Print(`strung - Transform UBS findings to Beads issues

Usage: strung <command> [flags]

Commands:
  transform   One-way transform: UBS JSON → Beads JSONL (stdin → stdout)
  sync        Incremental sync with state tracking (bidirectional)
  version     Print version
  help        Show this help

Transform Examples:
  ubs --format=json src/ | strung transform
  ubs --format=json src/ | strung transform --min-severity=critical

Sync Examples:
  ubs --format=json src/ | strung sync --db-path=.strung.db
  ubs --format=json src/ | strung sync --auto-close

Run 'strung <command> --help' for command-specific help.
`)
}

func runTransform(args []string) {
	fs := flag.NewFlagSet("transform", flag.ExitOnError)
	minSeverity := fs.String("min-severity", "warning", "Minimum severity (critical, warning, info)")
	verbose := fs.Bool("verbose", false, "Enable verbose output")
	fs.Parse(args)

	// Validate
	validSeverities := map[string]bool{"critical": true, "warning": true, "info": true}
	if !validSeverities[*minSeverity] {
		fmt.Fprintf(os.Stderr, "Error: invalid severity %q\n", *minSeverity)
		os.Exit(2)
	}

	// Configure logging
	log.SetOutput(os.Stderr)
	log.SetPrefix("[strung] ")
	if !*verbose {
		log.SetOutput(io.Discard)
	}

	// Parse
	report, err := parser.ParseUBS(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Parsed %d findings", len(report.Findings))

	// Filter
	findings := report.FilterBySeverity(*minSeverity)
	log.Printf("After filter: %d findings", len(findings))

	if len(findings) == 0 {
		os.Exit(0)
	}

	// Transform
	transformer := transform.NewTransformer()
	issues := transformer.TransformAll(findings)

	// Output
	for _, issue := range issues {
		jsonl, err := issue.ToJSONL()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(jsonl)
	}
}
