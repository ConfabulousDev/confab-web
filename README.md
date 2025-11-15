# Confab

Archive and query your Claude Code sessions.

Confab automatically captures Claude Code session transcripts and agent sidechains to local SQLite storage for retrieval, search, and analytics.

## Installation

```bash
# Clone the repository
git clone https://github.com/santaclaude2025/confab.git
cd confab

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
5. Returns success response to Claude Code

## Database

**Location:** `~/.confab/sessions.db`

**Schema:**
- `sessions` - Session metadata (ID, timestamp, file count, size, working directory, end reason)
- `files` - Individual files per session (transcript and agent sidechains)

## Development

```bash
# Build
make build

# Clean
make clean

# Test with sample input
make test
```

## Future Features

- [ ] Cloud upload and sync
- [ ] Full-text search across sessions
- [ ] Session analytics and insights
- [ ] Export to various formats
- [ ] Compression (tar.zstd)

## License

MIT
