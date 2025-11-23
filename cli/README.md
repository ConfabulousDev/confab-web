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
# Interactive login (recommended - opens browser for GitHub OAuth)
confab login

# Or manually configure with API key
confab configure \
  --backend-url https://your-backend.com \
  --api-key <your-api-key>

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
- **Logs:** `~/.confab/logs/confab.log` - Operation logs

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

## License

MIT
