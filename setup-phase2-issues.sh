#!/bin/bash
# setup-phase2-issues.sh
set -e

echo "=== Creating Phase 2 Beads Issues ==="

# Step 1: Create parent epic
echo "Creating epic..."
EPIC_JSON=$(br create "Phase 2: Bidirectional sync and state tracking" \
  -t epic -p 0 \
  -d "Add SQLite tracking DB, update detection, and resolution handling" \
  --json)
EPIC_ID=$(echo "$EPIC_JSON" | jq -r '.id')
echo "✓ Epic: $EPIC_ID"

# Step 2: Create all tasks
echo "Creating tasks..."

DB_JSON=$(br create "Add SQLite tracking database" \
  -t task -p 0 --parent "$EPIC_ID" \
  -d "Create pkg/db/tracking.go with SQLite schema for finding-to-issue mapping" \
  --design "Schema: findings(fingerprint PK, issue_id, file, line, severity, category, message, first_seen, last_seen, resolved_at). Fingerprint = sha256(file+category+message+code_context)" \
  --acceptance "DB stores/retrieves findings, supports queries by fingerprint and issue_id, tests pass" \
  --json)
DB_ID=$(echo "$DB_JSON" | jq -r '.id')
echo "✓ DB: $DB_ID"

DIFF_JSON=$(br create "Implement diff detection" \
  -t task -p 0 --parent "$EPIC_ID" \
  -d "Create pkg/sync/diff.go to detect new/changed/resolved findings" \
  --design "Compare current UBS findings against tracking DB. Return 3 sets: new (not in DB), changed (severity differs), resolved (in DB but not in current scan)" \
  --acceptance "Correctly identifies all three states from test fixtures" \
  --json)
DIFF_ID=$(echo "$DIFF_JSON" | jq -r '.id')
echo "✓ Diff: $DIFF_ID"

SYNC_JSON=$(br create "Build sync command" \
  -t task -p 1 --parent "$EPIC_ID" \
  -d "Create cmd/strung/sync.go subcommand for incremental sync" \
  --design "Read UBS JSON from stdin, compute diffs, create/update/close Beads issues via br CLI. Flags: --db-path, --auto-close, --dry-run, --min-severity" \
  --acceptance "Works: ubs --format=json | strung sync --db-path=.strung.db" \
  --json)
SYNC_ID=$(echo "$SYNC_JSON" | jq -r '.id')
echo "✓ Sync: $SYNC_ID"

RESOLUTION_JSON=$(br create "Add resolution detection" \
  -t task -p 1 --parent "$EPIC_ID" \
  -d "Implement auto-close for resolved findings" \
  --design "When finding disappears from scan, optionally close corresponding Beads issue via br close. Controlled by --auto-close flag" \
  --acceptance "Resolved findings closed when --auto-close=true, logged when false" \
  --json)
RESOLUTION_ID=$(echo "$RESOLUTION_JSON" | jq -r '.id')
echo "✓ Resolution: $RESOLUTION_ID"

ENRICHMENT_JSON=$(br create "Add metadata enrichment" \
  -t task -p 2 --parent "$EPIC_ID" \
  -d "Enhance Beads issues with tags and links" \
  --design "Add UBS category as Beads tag. Generate repo file links (GitHub/GitLab). Include scan timestamp in description" \
  --acceptance "Created issues have tags, contain clickable file links, show scan time" \
  --json)
ENRICHMENT_ID=$(echo "$ENRICHMENT_JSON" | jq -r '.id')
echo "✓ Enrichment: $ENRICHMENT_ID"

TESTS_JSON=$(br create "Add integration tests for sync workflow" \
  -t task -p 2 --parent "$EPIC_ID" \
  -d "Create testdata/sync-scenarios/ with multi-scan test cases" \
  --acceptance "Tests cover: first sync, subsequent sync with changes, resolution detection. Coverage > 80%" \
  --json)
TESTS_ID=$(echo "$TESTS_JSON" | jq -r '.id')
echo "✓ Tests: $TESTS_ID"

DOCS_JSON=$(br create "Update documentation for Phase 2" \
  -t task -p 2 --parent "$EPIC_ID" \
  -d "Update README.md, add SYNC.md usage guide, update CLI help text" \
  --design "README: sync workflow section. SYNC.md: comprehensive guide. CLI: --help for sync subcommand" \
  --acceptance "README has sync examples, SYNC.md exists, sync --help shows usage" \
  --json)
DOCS_ID=$(echo "$DOCS_JSON" | jq -r '.id')
echo "✓ Docs: $DOCS_ID"

ERRORS_JSON=$(br create "Add error handling and transaction safety" \
  -t task -p 1 --parent "$EPIC_ID" \
  -d "Implement rollback/recovery for partial failures during sync" \
  --design "Operation log table tracks actions. File locking prevents concurrent corruption. --recover flag fixes inconsistencies. Retry logic for transient failures" \
  --acceptance "Partial failures don't leave orphaned issues, --recover fixes inconsistencies" \
  --json)
ERRORS_ID=$(echo "$ERRORS_JSON" | jq -r '.id')
echo "✓ Errors: $ERRORS_ID"

# Step 3: Add dependencies
echo "Adding dependencies..."
br dep add "$DIFF_ID" "$DB_ID" --type blocks
br dep add "$SYNC_ID" "$DIFF_ID" --type blocks
br dep add "$SYNC_ID" "$DB_ID" --type blocks
br dep add "$RESOLUTION_ID" "$SYNC_ID" --type blocks
br dep add "$ENRICHMENT_ID" "$SYNC_ID" --type blocks
br dep add "$TESTS_ID" "$RESOLUTION_ID" --type blocks
br dep add "$TESTS_ID" "$ENRICHMENT_ID" --type blocks
br dep add "$DOCS_ID" "$SYNC_ID" --type blocks
br dep add "$DOCS_ID" "$RESOLUTION_ID" --type blocks
br dep add "$DOCS_ID" "$ENRICHMENT_ID" --type blocks
br dep add "$ERRORS_ID" "$SYNC_ID" --type blocks
br dep add "$ERRORS_ID" "$DB_ID" --type blocks
br dep add "$TESTS_ID" "$ERRORS_ID" --type blocks
echo "✓ Dependencies configured"

# Step 4: Save IDs
cat > .phase2-issue-ids.env << EOF
EPIC_ID=$EPIC_ID
DB_ID=$DB_ID
DIFF_ID=$DIFF_ID
SYNC_ID=$SYNC_ID
RESOLUTION_ID=$RESOLUTION_ID
ENRICHMENT_ID=$ENRICHMENT_ID
TESTS_ID=$TESTS_ID
DOCS_ID=$DOCS_ID
ERRORS_ID=$ERRORS_ID
EOF
echo "✓ Issue IDs saved to .phase2-issue-ids.env"

# Step 5: Verification
echo ""
echo "=== Verification ==="
br ready --json
echo ""
echo "Dependency tree:"
br dep tree "$TESTS_ID"
echo ""
echo "=== Ready to Start ==="
echo "Load IDs: source .phase2-issue-ids.env"
echo "First task: br show \$DB_ID"