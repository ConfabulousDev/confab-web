# Confab CLI

Command-line tool for capturing and uploading Claude Code sessions to cloud storage.

## Installation

```bash
# Clone the repository
git clone https://github.com/santaclaude2025/confab.git
cd confab/cli

# Run the install script
./install.sh
```

This will:
1. Build the `confab` binary
2. Install it to `~/.local/bin`
3. Set up the SessionEnd hook in `~/.claude/settings.json`

Make sure `~/.local/bin` is in your PATH.

## Usage

### Authentication

```bash
# Login using device code flow (works on any machine)
confab login

# Or use setup for login + hook installation in one step
confab setup

# Change backend URL (without re-authenticating)
confab configure --backend-url https://your-backend.com

# Check configuration status
confab status

# Logout (clears API key)
confab logout
```

### Redaction

Redact sensitive data before uploading:

```bash
# Enable redaction with default patterns
confab redaction enable

# View current configuration
confab redaction status

# Disable redaction
confab redaction disable
```

### Uninstall Hook

```bash
confab uninstall
```

Removes the SessionEnd hook from Claude Code settings.

## How It Works

1. When you end a Claude Code session, the SessionEnd hook fires
2. Confab reads session metadata from stdin
3. Discovers the transcript file and any referenced agent sidechains
4. Optionally redacts sensitive data (if enabled)
5. Uploads files to cloud backend (if API key configured)
6. Returns success response to Claude Code

**Note:** Sessions are only captured when the hook fires (on session end). There is no local database - all data is uploaded to the cloud backend.

## Configuration Files

- **Cloud config:** `~/.confab/config.json` - Backend URL and API key
- **Redaction config:** `~/.confab/redaction.json` - Redaction patterns
- **Logs:** `~/.confab/logs/` - Operation logs with automatic rotation
  - `confab.log` - Current log file (max 1MB)
  - `confab.log.1.gz`, `confab.log.2.gz`, etc. - Rotated logs (kept for 14 days)

## Log Management

Logs are automatically rotated and cleaned up:
- **Max size:** 1MB per log file
- **Retention:** 14 days (2 weeks)
- **Location:** `~/.confab/logs/`
- **Format:**
  - Active: `confab.log`
  - Rotated: `confab.log.1.gz`, `confab.log.2.gz`, etc.
- **Compression:** Old logs are compressed with gzip

To view logs:
```bash
# Current log
tail -f ~/.confab/logs/confab.log

# Rotated logs (compressed)
zcat ~/.confab/logs/confab.log.1.gz | tail -100

# All logs (last 100 lines)
for f in ~/.confab/logs/confab.log*; do
  if [[ $f == *.gz ]]; then
    zcat "$f"
  else
    cat "$f"
  fi
done | tail -100
```

## Development

```bash
# Build
make build

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## Environment Variables

### `CONFAB_CLAUDE_DIR`

Override the default Claude Code state directory.

- **Default:** `~/.claude`
- **Example:** `export CONFAB_CLAUDE_DIR=/custom/path/to/claude`

This affects:
- Settings file: `$CONFAB_CLAUDE_DIR/settings.json`
- Projects directory: `$CONFAB_CLAUDE_DIR/projects/`
- Todos directory: `$CONFAB_CLAUDE_DIR/todos/`

## Known Limitations

### Log Rotation Not Multi-Process Safe

The logger uses lumberjack for log rotation, which is not designed for multiple processes writing to the same log file simultaneously. Individual log writes are atomic (via OS-level `O_APPEND`), but the rotation logic can race when multiple CLI processes (e.g., `save` and `daemon`) both try to rotate at the same moment the file hits 1MB.

**Practical impact:** Low - this edge case requires heavy concurrent logging at the exact rotation threshold. Normal usage is unaffected.

**Relevant code:** `cli/pkg/logger/logger.go`

## License

MIT
