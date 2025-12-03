# PowerShell version of setup-phase2-issues.sh
$ErrorActionPreference = "Stop"

Write-Host "=== Creating Phase 2 Beads Issues ===" -ForegroundColor Cyan

# Step 1: Create parent epic
Write-Host "Creating epic..."
$epicJson = bd create "Phase 2: Bidirectional sync and state tracking" -t epic -p 0 -d "Add SQLite tracking DB, update detection, and resolution handling" --json | ConvertFrom-Json
$EPIC_ID = $epicJson.id
Write-Host "Epic: $EPIC_ID" -ForegroundColor Green

# Step 2: Create all tasks
Write-Host "Creating tasks..."

$dbJson = bd create "Add SQLite tracking database" -t task -p 0 --parent "$EPIC_ID" -d "Create pkg/db/tracking.go with SQLite schema for finding-to-issue mapping" --design "Schema: findings(fingerprint PK, issue_id, file, line, severity, category, message, first_seen, last_seen, resolved_at). Fingerprint = sha256(file+category+message+code_context)" --acceptance "DB stores/retrieves findings, supports queries by fingerprint and issue_id, tests pass" --json | ConvertFrom-Json
$DB_ID = $dbJson.id
Write-Host "DB: $DB_ID" -ForegroundColor Green

$diffJson = bd create "Implement diff detection" -t task -p 0 --parent "$EPIC_ID" -d "Create pkg/sync/diff.go to detect new/changed/resolved findings" --design "Compare current UBS findings against tracking DB. Return 3 sets: new (not in DB), changed (severity differs), resolved (in DB but not in current scan)" --acceptance "Correctly identifies all three states from test fixtures" --json | ConvertFrom-Json
$DIFF_ID = $diffJson.id
Write-Host "Diff: $DIFF_ID" -ForegroundColor Green

$syncJson = bd create "Build sync command" -t task -p 1 --parent "$EPIC_ID" -d "Create cmd/strung/sync.go subcommand for incremental sync" --design "Read UBS JSON from stdin, compute diffs, create/update/close Beads issues via bd CLI. Flags: --db-path, --auto-close, --dry-run, --min-severity" --acceptance "Works: ubs --format=json | strung sync --db-path=.strung.db" --json | ConvertFrom-Json
$SYNC_ID = $syncJson.id
Write-Host "Sync: $SYNC_ID" -ForegroundColor Green

$resolutionJson = bd create "Add resolution detection" -t task -p 1 --parent "$EPIC_ID" -d "Implement auto-close for resolved findings" --design "When finding disappears from scan, optionally close corresponding Beads issue via bd close. Controlled by --auto-close flag" --acceptance "Resolved findings closed when --auto-close=true, logged when false" --json | ConvertFrom-Json
$RESOLUTION_ID = $resolutionJson.id
Write-Host "Resolution: $RESOLUTION_ID" -ForegroundColor Green

$enrichmentJson = bd create "Add metadata enrichment" -t task -p 2 --parent "$EPIC_ID" -d "Enhance Beads issues with tags and links" --design "Add UBS category as Beads tag. Generate repo file links (GitHub/GitLab). Include scan timestamp in description" --acceptance "Created issues have tags, contain clickable file links, show scan time" --json | ConvertFrom-Json
$ENRICHMENT_ID = $enrichmentJson.id
Write-Host "Enrichment: $ENRICHMENT_ID" -ForegroundColor Green

$testsJson = bd create "Add integration tests for sync workflow" -t task -p 2 --parent "$EPIC_ID" -d "Create testdata/sync-scenarios/ with multi-scan test cases" --acceptance "Tests cover: first sync, subsequent sync with changes, resolution detection. Coverage > 80%" --json | ConvertFrom-Json
$TESTS_ID = $testsJson.id
Write-Host "Tests: $TESTS_ID" -ForegroundColor Green

$docsJson = bd create "Update documentation for Phase 2" -t task -p 2 --parent "$EPIC_ID" -d "Update README.md, add SYNC.md usage guide, update CLI help text" --design "README: sync workflow section. SYNC.md: comprehensive guide. CLI: --help for sync subcommand" --acceptance "README has sync examples, SYNC.md exists, sync --help shows usage" --json | ConvertFrom-Json
$DOCS_ID = $docsJson.id
Write-Host "Docs: $DOCS_ID" -ForegroundColor Green

$errorsJson = bd create "Add error handling and transaction safety" -t task -p 1 --parent "$EPIC_ID" -d "Implement rollback/recovery for partial failures during sync" --design "Operation log table tracks actions. File locking prevents concurrent corruption. --recover flag fixes inconsistencies. Retry logic for transient failures" --acceptance "Partial failures don't leave orphaned issues, --recover fixes inconsistencies" --json | ConvertFrom-Json
$ERRORS_ID = $errorsJson.id
Write-Host "Errors: $ERRORS_ID" -ForegroundColor Green

# Step 3: Add dependencies
Write-Host "Adding dependencies..."
bd dep add "$DIFF_ID" "$DB_ID" --type blocks | Out-Null
bd dep add "$SYNC_ID" "$DIFF_ID" --type blocks | Out-Null
bd dep add "$SYNC_ID" "$DB_ID" --type blocks | Out-Null
bd dep add "$RESOLUTION_ID" "$SYNC_ID" --type blocks | Out-Null
bd dep add "$ENRICHMENT_ID" "$SYNC_ID" --type blocks | Out-Null
bd dep add "$TESTS_ID" "$RESOLUTION_ID" --type blocks | Out-Null
bd dep add "$TESTS_ID" "$ENRICHMENT_ID" --type blocks | Out-Null
bd dep add "$DOCS_ID" "$SYNC_ID" --type blocks | Out-Null
bd dep add "$DOCS_ID" "$RESOLUTION_ID" --type blocks | Out-Null
bd dep add "$DOCS_ID" "$ENRICHMENT_ID" --type blocks | Out-Null
bd dep add "$ERRORS_ID" "$SYNC_ID" --type blocks | Out-Null
bd dep add "$ERRORS_ID" "$DB_ID" --type blocks | Out-Null
bd dep add "$TESTS_ID" "$ERRORS_ID" --type blocks | Out-Null
Write-Host "Dependencies configured" -ForegroundColor Green

# Step 4: Save IDs
$envContent = @"
EPIC_ID=$EPIC_ID
DB_ID=$DB_ID
DIFF_ID=$DIFF_ID
SYNC_ID=$SYNC_ID
RESOLUTION_ID=$RESOLUTION_ID
ENRICHMENT_ID=$ENRICHMENT_ID
TESTS_ID=$TESTS_ID
DOCS_ID=$DOCS_ID
ERRORS_ID=$ERRORS_ID
"@
Set-Content -Path ".phase2-issue-ids.env" -Value $envContent
Write-Host "Issue IDs saved to .phase2-issue-ids.env" -ForegroundColor Green

# Step 5: Verification
Write-Host ""
Write-Host "=== Verification ===" -ForegroundColor Cyan
bd ready
Write-Host ""
Write-Host "Dependency tree:" -ForegroundColor Cyan
bd dep tree "$TESTS_ID"
Write-Host ""
Write-Host "=== Ready to Start ===" -ForegroundColor Green
Write-Host "Load IDs from: .phase2-issue-ids.env"
Write-Host "First task: bd show $DB_ID"
