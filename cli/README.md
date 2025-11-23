# Confab CLI

Command-line tool for archiving and querying your Claude Code sessions.

Confab automatically captures Claude Code session transcripts and agent sidechains to local SQLite storage for retrieval, search, and analytics.

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
4. Create the SQLite database at `~/.confab/sessions.db`

Make sure `~/.local/bin` is in your PATH.

## Usage

### View Status

```bash
confab status
```

Shows:
- Database location and session count
- Recent captured sessions
- Hook installation status

### Cloud Sync

Authenticate and sync sessions across devices:

```bash
# Interactive login (recommended)
confab login

# Or manually configure with API key
confab configure \
  --backend-url http://localhost:8080 \
  --api-key <your-api-key>

# Check cloud sync status
confab status

# Logout and disable cloud sync (clears API key)
confab logout
```

### Manual Capture (Testing)

```bash
# Normally the save command is called automatically by the SessionEnd hook
echo '{"session_id":"test","transcript_path":"/path/to/transcript.jsonl",...}' | confab save
```

### Uninstall Hook

```bash
confab uninstall
```

Removes the SessionEnd hook but preserves your database and sessions.

## How It Works

1. When you end a Claude Code session, the SessionEnd hook fires
2. Confab reads session metadata from stdin
3. Discovers the transcript file and any referenced agent sidechains (using regex pattern `agent-[a-f0-9]{8}`)
4. Stores metadata and file paths in SQLite
5. Optionally uploads to cloud backend (if configured)
6. Returns success response to Claude Code

## Local Database

**Location:** `~/.confab/sessions.db`

**Schema:**
- `sessions` - Unique session IDs with first seen timestamp
- `runs` - Individual executions/resumptions of sessions
- `files` - Files associated with each run (transcript and agent sidechains)

**Logs:** `~/.confab/logs/confab.log`

## Cloud Sync

When cloud sync is enabled, sessions are automatically uploaded to the backend after local storage succeeds. This enables:
- Session access across multiple devices
- Cloud-based search and analytics
- Backup and archival

See the [backend documentation](../backend/README.md) for running your own backend server.

## Development

```bash
# Build
make build

# Clean
make clean

# Test with sample input
make test

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run linter (requires golangci-lint)
golangci-lint run
```

## Environment Variables

Confab can be configured using the following environment variables:

### `CONFAB_CLAUDE_DIR`

Override the default Claude Code state directory location.

- **Default:** `~/.claude`
- **Usage:** Useful for testing, non-standard installations, or running multiple Claude Code instances
- **Example:**
  ```bash
  export CONFAB_CLAUDE_DIR=/custom/path/to/claude
  confab status
  ```

This affects:
- Settings file location: `$CONFAB_CLAUDE_DIR/settings.json`
- Projects directory: `$CONFAB_CLAUDE_DIR/projects/`
- Todos directory: `$CONFAB_CLAUDE_DIR/todos/`

## Configuration Files

Confab uses several configuration files:

### Cloud Sync Configuration
- **Location:** `~/.confab/config.json`
- **Contents:** Backend URL and API key
- **Created by:** `confab login` or `confab configure`

### Redaction Configuration
- **Location:** `~/.confab/redaction.json` (enabled) or `~/.confab/redaction.json.disabled` (disabled)
- **Contents:** Patterns for redacting sensitive data before upload
- **Managed by:** `confab redaction` commands

### Logs
- **Location:** `~/.confab/logs/confab.log`
- **Contents:** Detailed operation logs for debugging

## License

MIT
