# strung

Transform [UBS (Ultimate Bug Scanner)](https://github.com/Dicklesworthstone/ultimate_bug_scanner) findings into [Beads](https://github.com/steveyegge/beads) issue tracker entries.

## Installation

```bash
go install github.com/TheEditor/strung/cmd/strung@latest
```

## Usage

```bash
# Basic transformation
ubs --format=json src/ | strung

# Filter by severity (critical, warning, info)
ubs --format=json src/ | strung --min-severity=critical

# Import directly to Beads
ubs --format=json src/ | strung | bd import

# Verbose output for debugging
ubs --format=json src/ | strung --verbose 2>&1 | head
```

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `--min-severity` | `warning` | Minimum severity to include |
| `--verbose` | `false` | Enable debug logging to stderr |
| `--version` | - | Print version and exit |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Input error (invalid JSON) |
| 2 | Usage error (invalid flags) |

## Why Strung?

UBS already has `--beads-jsonl` for basic export. Strung adds:
- Severity filtering (`--min-severity`)
- Future: bidirectional sync (Phase 2)
- Future: deduplication/fingerprinting (Phase 2)

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
