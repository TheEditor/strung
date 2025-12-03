# Sync Workflow Documentation

## Overview

The sync command enables incremental tracking of UBS findings with state management. Instead of creating duplicate issues for every scan, strung maintains a SQLite database that tracks findings across time, detecting what's NEW, CHANGED, or RESOLVED.

### How It Works

1. **Fingerprinting**: Each finding is given a stable identifier based on file, category, message, and code context
2. **Diff Detection**: New scans are compared against the database to identify changes
3. **Issue Lifecycle**: Issues are created, updated for severity changes, and can be automatically closed when resolved
4. **State Persistence**: All state is stored locally in a SQLite database (default: `.strung.db`)

## Quick Start

### First Sync

```bash
# Scan your code and sync findings
ubs --format=json src/ | strung sync --db-path=.strung.db
```

Output:
```
Sync summary: New: 5, Changed: 0, Resolved: 0
Created: UBS: null-safety in vault.ts:42 → proj-014
Created: UBS: resource-lifecycle in crypto.ts:87 → proj-015
...
```

### Subsequent Syncs

After the initial sync, future scans will only show changes:

```bash
# Same code scanned again
ubs --format=json src/ | strung sync --db-path=.strung.db --auto-close
```

Output:
```
Sync summary: New: 1, Changed: 2, Resolved: 1
Created: UBS: memory-leak in handler.ts:156 → proj-019
Updated: proj-014 (priority 1)
Closed: proj-018
```

## Workflow Patterns

### Development Workflow

Track issues as you code:

```bash
# Initial scan when starting work
ubs --format=json src/ | strung sync --db-path=.strung.db --dry-run

# Before committing, update the database
ubs --format=json src/ | strung sync --db-path=.strung.db

# Next day, see what changed
ubs --format=json src/ | strung sync --db-path=.strung.db --auto-close
```

### CI/CD Integration

```bash
#!/bin/bash
set -e

# Run scan and sync
ubs --format=json src/ | strung sync \
  --db-path=.strung.db \
  --repo-url=$CI_REPOSITORY_URL \
  --repo-branch=$CI_COMMIT_BRANCH \
  --auto-close

# Fail if new critical issues found
ubs --format=json src/ | strung sync \
  --min-severity=critical \
  --db-path=.strung.db \
  --dry-run | grep -q "New: [1-9]" && exit 1
```

### Release Preparation

```bash
# Before release, ensure all issues are resolved
ubs --format=json src/ | strung sync --db-path=.strung.db

# Check for remaining issues
bd list --status open | wc -l
```

## Flags Reference

### Core Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db-path` | string | `.strung.db` | Path to tracking database |
| `--dry-run` | bool | false | Preview changes without executing |
| `--auto-close` | bool | false | Automatically close resolved issues |

### Filtering

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--min-severity` | string | `warning` | Minimum severity: critical, warning, info |

Valid values: `critical` (highest priority), `warning` (medium), `info` (lowest)

### Enrichment

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo-url` | string | - | Repository URL (GitHub/GitLab) |
| `--repo-branch` | string | `main` | Repository branch |

When `--repo-url` is provided, issue descriptions include clickable file links:
```
**Location:** [src/test.ts:42](https://github.com/user/repo/blob/main/src/test.ts#L42)
```

### Debugging

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` | bool | false | Enable verbose output |

## Understanding Output

### Summary Line

```
Sync summary: New: 5, Changed: 2, Resolved: 1
```

- **New**: Findings that didn't exist in the database
- **Changed**: Existing findings with different severity
- **Resolved**: Findings that disappeared (no longer in scan)

### Action Lines

```
Created: UBS: null-safety in vault.ts:42 → proj-014
```
- Issue created with auto-generated ID (proj-014)
- Issue title: `UBS: <category> in <file>:<line>`

```
Updated: proj-014 (priority 2)
```
- Existing issue updated (typically severity changed)
- Priority updated (0=critical, 1=high, 2=medium)

```
Closed: proj-014
```
- Issue closed (requires `--auto-close` flag)

### Dry Run

```
[DRY RUN] Would create: UBS: null-safety in vault.ts:42
[DRY RUN] Would update: proj-014 (severity critical → warning)
[DRY RUN] Would close: proj-014
```

No changes made to database or issue tracker when using `--dry-run`.

## Database

### Schema

The database stores findings with the following information:

```sql
SELECT * FROM findings;
```

| Column | Type | Description |
|--------|------|-------------|
| fingerprint | TEXT | SHA256(file+category+message+context) |
| issue_id | TEXT | Beads issue ID |
| file | TEXT | File path |
| line | INTEGER | Line number |
| severity | TEXT | critical, warning, info |
| category | TEXT | UBS category |
| message | TEXT | Finding message |
| first_seen | TIMESTAMP | When first detected |
| last_seen | TIMESTAMP | When last detected |
| status | TEXT | open, resolved |

### Viewing State

```bash
# List all tracked findings
sqlite3 .strung.db "SELECT file, category, severity, issue_id FROM findings ORDER BY file"

# Find findings in a specific file
sqlite3 .strung.db "SELECT * FROM findings WHERE file LIKE 'src/crypto.ts'"

# Count by severity
sqlite3 .strung.db "SELECT severity, COUNT(*) FROM findings GROUP BY severity"

# Find resolved findings
sqlite3 .strung.db "SELECT * FROM findings WHERE status = 'resolved'"
```

### Resetting State

```bash
# Remove database to start fresh
rm .strung.db

# Next sync will treat all findings as NEW
ubs --format=json src/ | strung sync --db-path=.strung.db
```

## Troubleshooting

### "bd CLI not found"

Error:
```
Error: bd CLI not found or not working
Install: https://github.com/steveyegge/beads
```

Solution: Install Beads issue tracker

```bash
# macOS/Linux
brew install steveyegge/tools/beads

# Or from source
go install github.com/steveyegge/beads@latest
```

Note: `--dry-run` works without bd CLI available.

### "Issues not closing with --auto-close"

Possible causes:

1. **Issues already closed**: Check Beads status
   ```bash
   bd list --status closed | grep proj-014
   ```

2. **No resolved findings detected**: Verify findings disappeared
   ```bash
   # Check what the last scan recorded
   sqlite3 .strung.db "SELECT * FROM findings WHERE status = 'open'"
   ```

3. **Issue ID mismatch**: Verify fingerprinting consistency
   ```bash
   # Compare finding hashes
   echo "src/test.ts:null-safety:Test message" | sha256sum
   ```

### "Duplicate issues created"

Likely cause: Database corruption or multiple syncs with different options

Solution:

```bash
# Verify fingerprints are consistent
sqlite3 .strung.db "SELECT COUNT(DISTINCT fingerprint), COUNT(*) FROM findings"

# If mismatched, remove database and resync
rm .strung.db
ubs --format=json src/ | strung sync --db-path=.strung.db
```

### "Database locked"

Error:
```
Error: database is locked
```

Cause: Another strung process is using the database

Solution:

```bash
# Wait for other process to finish
# Or use different database path
ubs --format=json src/ | strung sync --db-path=.strung-$(date +%s).db
```

### "Sync says 'No changes' but findings are different"

Possible causes:

1. **Severity filter hiding changes**: Try with `--min-severity=info`
   ```bash
   ubs --format=json src/ | strung sync --min-severity=info --db-path=.strung.db --dry-run
   ```

2. **Code context hashing**: Same message from different context creates different fingerprint
   ```bash
   # Check if code snippet changed
   sqlite3 .strung.db "SELECT file, message, code_snippet FROM findings"
   ```

## Best Practices

### 1. Use a VCS-tracked Database

Commit `.strung.db` to version control for team visibility:

```bash
git add .strung.db
git commit -m "Update UBS findings state"
```

### 2. Run Before Every Commit

Hook into your Git workflow:

```bash
#!/bin/bash
# .git/hooks/pre-commit

if ! ubs --format=json src/ | strung sync --db-path=.strung.db --dry-run | grep -q "No changes"; then
    echo "Found new UBS findings. Run sync to update."
    exit 1
fi
```

### 3. Review Resolved Issues

Ensure findings are actually fixed:

```bash
# See what would be closed
ubs --format=json src/ | strung sync --db-path=.strung.db --dry-run | grep "Would close"

# Verify in Beads before auto-closing
bd list | grep proj-014
```

### 4. Use Repo URL for Context

Always provide repository context:

```bash
ubs --format=json src/ | strung sync \
  --repo-url=$GITHUB_REPO_URL \
  --repo-branch=$GITHUB_BRANCH \
  --db-path=.strung.db
```

### 5. Track in Monitoring

Include in CI/CD dashboards:

```bash
# Count unresolved findings
sqlite3 .strung.db "SELECT COUNT(*) FROM findings WHERE status = 'open'"

# Export for metrics
sqlite3 .strung.db ".mode csv" "SELECT severity, COUNT(*) FROM findings WHERE status = 'open' GROUP BY severity"
```

## Examples

### Example 1: Team Workflow

```bash
# Developer 1: Initial scan
ubs --format=json . | strung sync --db-path=.strung.db --repo-url=$REPO_URL
git add .strung.db
git commit -m "Add UBS baseline (5 critical, 12 warnings)"

# Developer 2: Fix one issue, rescan
vim src/crypto.ts  # Fix the issue
ubs --format=json . | strung sync --db-path=.strung.db --repo-url=$REPO_URL
# Output: Sync summary: New: 0, Changed: 0, Resolved: 1
git add .strung.db
git commit -m "Fix crypto.ts null-safety issue"

# Developer 3: Scan at release time
ubs --format=json . | strung sync --db-path=.strung.db --auto-close
# Closes resolved issues, reports remaining
```

### Example 2: CI/CD

```yaml
# GitHub Actions
- name: Sync UBS findings
  run: |
    ubs --format=json . | strung sync \
      --db-path=.strung.db \
      --repo-url=${{ github.server_url }}/${{ github.repository }} \
      --repo-branch=${{ github.ref_name }} \
      --auto-close

- name: Check for new critical issues
  run: |
    CRITICAL=$(sqlite3 .strung.db "SELECT COUNT(*) FROM findings WHERE severity='critical' AND status='open'")
    if [ $CRITICAL -gt 0 ]; then
      echo "Found $CRITICAL critical issues"
      exit 1
    fi
```

### Example 3: Severity Escalation

```bash
# Start by tracking everything
ubs --format=json . | strung sync --min-severity=info --db-path=.strung.db

# Later, only care about high priority
ubs --format=json . | strung sync --min-severity=critical --db-path=.strung.db

# Findings filtered out are not affected by sync
```

## See Also

- [strung README](../README.md) - Overview and installation
- [UBS Documentation](https://github.com/Dicklesworthstone/ultimate_bug_scanner) - Finding details
- [Beads Issue Tracker](https://github.com/steveyegge/beads) - Issue management
