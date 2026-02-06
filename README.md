# strung

Transform [UBS (Ultimate Bug Scanner)](https://github.com/Dicklesworthstone/ultimate_bug_scanner) findings into [Beads](https://github.com/Dicklesworthstone/beads_rust) issue tracker entries.

## Installation

```bash
go install github.com/TheEditor/strung/cmd/strung@latest
```

## Usage

### Transform (Phase 1)

```bash
# Basic transformation
ubs --format=json src/ | strung transform

# Filter by severity (critical, warning, info)
ubs --format=json src/ | strung transform --min-severity=critical

# Import directly to Beads

ubs --format=json src/ | strung transform | br sync --import-only
git add .beads/
git commit -m "sync beads"

# Verbose output for debugging
ubs --format=json src/ | strung transform --verbose 2>&1 | head
```

### Sync (Phase 2)

For incremental updates with state tracking:

```bash
# First sync - creates issues for all findings
ubs --format=json src/ | strung sync --db-path=.strung.db

# Subsequent syncs - only processes changes
ubs --format=json src/ | strung sync --db-path=.strung.db --auto-close

# Dry run (preview changes)
ubs --format=json src/ | strung sync --dry-run

# Only track critical issues
ubs --format=json src/ | strung sync --min-severity=critical

# With GitHub links
ubs --format=json src/ | strung sync \
  --repo-url=https://github.com/user/repo \
  --repo-branch=main
```

See [docs/SYNC.md](docs/SYNC.md) for complete sync documentation.

## Commands

| Command | Description |
|---------|-------------|
| `transform` | Convert UBS findings to Beads JSON (Phase 1) |
| `sync` | Incrementally sync findings with state tracking (Phase 2) |
| `help` | Show available commands |
| `version` | Print version and exit |

## Options

### transform

| Flag | Default | Description |
|------|---------|-------------|
| `--min-severity` | `warning` | Minimum severity: critical, warning, info |
| `--verbose` | `false` | Enable debug logging to stderr |

### sync

| Flag | Default | Description |
|------|---------|-------------|
| `--db-path` | `.strung.db` | Path to tracking database |
| `--auto-close` | `false` | Automatically close resolved issues |
| `--dry-run` | `false` | Show actions without executing |
| `--min-severity` | `warning` | Minimum severity: critical, warning, info |
| `--repo-url` | - | Repository URL for file links (GitHub/GitLab format) |
| `--repo-branch` | `main` | Repository branch for file links |
| `--verbose` | `false` | Enable verbose output |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Input error (invalid JSON) |
| 2 | Usage error (invalid flags) |

## Why Strung?

UBS already has `--beads-jsonl` for basic export. Strung adds:
- **Phase 1**: Severity filtering and transformation
- **Phase 2**: Incremental sync with state tracking
  - Fingerprinting for stable issue identification
  - Diff detection (NEW, CHANGED, RESOLVED)
  - Auto-close resolved issues
  - Enriched descriptions with timestamps and file links
  - Multi-tag support for filtering

## Development

```bash
# Build
make build

# Test
make test

# Test with coverage
make test-coverage

# Demo
make demo
```

## License

MIT