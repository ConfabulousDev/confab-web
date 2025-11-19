# Claude Code Data Directory (~/.claude)

Comprehensive documentation of Claude Code's local data storage structure.

## Overview

Claude Code stores all local data in `~/.claude/`. This includes session history, file backups, settings, and various caches.

## Directory Structure

```
~/.claude/
├── backup-config.json      # Configuration backup
├── debug/                  # Debug logs per session
├── downloads/              # Downloaded files
├── file-history/           # File backups for undo (per session)
├── history.jsonl           # All user prompts (chronological)
├── projects/               # Per-project session data & snapshots
├── session-env/            # Environment variables per session
├── settings.json           # User settings/preferences
├── shell-snapshots/        # Shell environment snapshots
├── statsig/                # Analytics/feature flags
└── todos/                  # Todo lists per session
```

---

## history.jsonl

Chronological log of all user messages across all sessions.

### Location

`~/.claude/history.jsonl`

### Format

JSON Lines (JSONL) - one JSON object per line, sorted chronologically by timestamp.

### Schema

```json
{
  "display": "tell me more about file history",
  "pastedContents": {},
  "timestamp": 1763532074126,
  "project": "/Users/santaclaude/dev/beta/confab",
  "sessionId": "8b24b5b1-4a2e-417d-ae4b-2e6519234433"
}
```

### Fields

| Field | Description |
|-------|-------------|
| `display` | The user's message text (may be placeholder if content was pasted) |
| `pastedContents` | Map of pasted content blocks with their full text |
| `timestamp` | Unix timestamp in milliseconds |
| `project` | Working directory when message was sent |
| `sessionId` | UUID linking to session data in other directories |

### Pasted Content

When content is pasted into a message, it's stored separately in `pastedContents`:

```json
{
  "display": "[Pasted text #1 +8 lines]",
  "pastedContents": {
    "1": {
      "id": 1,
      "type": "text",
      "content": "santaclaude@equestria backend % go run cmd/server/main.go...\n..."
    }
  },
  "timestamp": 1762669139702,
  "project": "/Users/santaclaude/dev/github2",
  "sessionId": "7bc4fa71-c045-4266-b9b2-4a8586d0eee3"
}
```

### Characteristics

- **Append-only**: New messages are added to the end
- **Chronologically sorted**: Timestamps increase from first to last line
- **Cross-session**: Contains messages from all sessions, all projects
- **Global**: Single file for all Claude Code usage

### Use Cases

- **Prompt history**: Powers up-arrow history navigation in the CLI
- **Search**: Find previous prompts across sessions
- **Analytics**: Track usage patterns over time

---

## projects/

Per-project session data and conversation transcripts.

### Location

`~/.claude/projects/`

### Directory Structure

```
~/.claude/projects/
└── {encoded-project-path}/
    ├── {session-uuid}.jsonl        # Main session conversations
    └── agent-{short-id}.jsonl      # Sub-agent conversations
```

### Path Encoding

Project paths are encoded by replacing `/` with `-`:
- `/Users/santaclaude/dev/beta/confab` → `-Users-santaclaude-dev-beta-confab`

### File Types

#### Session Files (`{uuid}.jsonl`)

Full conversation transcripts for main Claude Code sessions.

Example: `8b24b5b1-4a2e-417d-ae4b-2e6519234433.jsonl`

#### Agent Files (`agent-{short-id}.jsonl`)

Sub-agent/task conversations spawned during sessions (e.g., from the Task tool).

Example: `agent-211dff5c.jsonl`

### Record Types

Each `.jsonl` file contains multiple record types:

| Type | Description |
|------|-------------|
| `user` | User messages and tool results |
| `assistant` | Claude's responses |
| `file-history-snapshot` | File state snapshots for undo |

### Message Schema

#### User Message

```json
{
  "type": "user",
  "uuid": "ae754b76-e8dc-4538-851e-140afbbe3b84",
  "parentUuid": null,
  "sessionId": "8b24b5b1-4a2e-417d-ae4b-2e6519234433",
  "userType": "external",
  "cwd": "/Users/santaclaude/dev/beta/confab",
  "gitBranch": "main",
  "timestamp": 1763531840393,
  "version": 1,
  "isSidechain": false,
  "message": {
    "role": "user",
    "content": "i want to understand what is stored in ~/.claude"
  }
}
```

#### Tool Result Message

When a tool is executed, the result is stored as a user message with structured content:

```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {
        "tool_use_id": "toolu_01N2ycMHLVzrhWuJePrytRP3",
        "type": "tool_result",
        "content": "... output ...",
        "is_error": false
      }
    ]
  }
}
```

### Key Fields

| Field | Description |
|-------|-------------|
| `type` | Record type: `user`, `assistant`, `file-history-snapshot` |
| `uuid` | Unique message identifier |
| `parentUuid` | Links to previous message (forms conversation chain) |
| `sessionId` | Session this message belongs to |
| `userType` | `external` (human) or internal system messages |
| `cwd` | Working directory when message was sent |
| `gitBranch` | Active git branch |
| `timestamp` | Unix timestamp in milliseconds |
| `isSidechain` | Whether this is part of a branched conversation |
| `message` | The actual message content |

### Conversation Structure

Messages are linked via `uuid` and `parentUuid` to form a conversation tree:

```
message-1 (parentUuid: null)
    └── message-2 (parentUuid: message-1)
        └── message-3 (parentUuid: message-2)
            └── ...
```

This enables:
- Linear conversation replay
- Branching conversations (sidechains)
- Finding conversation history at any point

### Use Cases

- **Session resume**: Reload previous conversations with `/resume`
- **Context recovery**: Restore full conversation state after restart
- **File undo**: Links to `file-history-snapshot` records for restoring files
- **Analytics**: Track conversation patterns and tool usage

---

## file-history/

File backups for undo/restore functionality.

### Location

`~/.claude/file-history/{session-uuid}/`

### Directory Structure

```
~/.claude/file-history/
└── {session-uuid}/
    ├── {file-hash}@v1
    ├── {file-hash}@v2
    └── ...
```

### Naming Convention

Files are stored as `{file-hash}@v{version}`:
- **file-hash**: Deterministic hash derived from the file path
- **version**: Incremental version number for each edit

### Content

Each file contains the **complete file contents** at that point in time (not diffs).

Example:
```
28c29f3c0404b032@v4
28c29f3c0404b032@v5
28c29f3c0404b032@v6
```

### File History Snapshots

Snapshots are stored in project `.jsonl` files and link conversation messages to file backups.

#### Schema

```json
{
  "type": "file-history-snapshot",
  "messageId": "3e6b047f-5d7d-482b-beab-f95f3a79b764",
  "snapshot": {
    "messageId": "3e6b047f-5d7d-482b-beab-f95f3a79b764",
    "trackedFileBackups": {
      "backend/internal/api/server.go": {
        "backupFileName": "c3135ea02233069d@v14",
        "version": 14,
        "backupTime": "2025-11-17T05:59:32.400Z"
      },
      "backend/.env.example": {
        "backupFileName": "28c29f3c0404b032@v4",
        "version": 4,
        "backupTime": "2025-11-16T23:17:17.987Z"
      }
    },
    "timestamp": "2025-11-17T05:59:32.398Z"
  },
  "isSnapshotUpdate": false
}
```

#### Fields

| Field | Description |
|-------|-------------|
| `type` | Always `"file-history-snapshot"` |
| `messageId` | UUID of the conversation message |
| `trackedFileBackups` | Map of file paths to backup metadata |
| `backupFileName` | Reference to file in `~/.claude/file-history/` (null if no backup exists) |
| `version` | Cumulative version number across the session |
| `backupTime` | ISO timestamp when backup was created |
| `isSnapshotUpdate` | Whether this updates an existing snapshot |

### How It Works

1. **File modification**: When Claude edits a file, the previous content is saved to `~/.claude/file-history/{session}/`

2. **Hash mapping**: File paths are hashed to create the backup filename (e.g., `backend/.env.example` → `28c29f3c0404b032`)

3. **Version tracking**: Each edit increments the version number (`@v4` → `@v5`)

4. **Snapshot creation**: After each message that modifies files, a snapshot is created containing the state of ALL tracked files

5. **Undo operation**: When `/undo` is called, Claude Code:
   - Finds the previous snapshot
   - Retrieves the backup file contents
   - Restores files to their prior state

### Key Characteristics

- **Full file storage**: Each version stores complete file contents (not diffs)
- **Cumulative snapshots**: Each snapshot includes ALL tracked files, not just changed ones
- **Per-session isolation**: File history is scoped to individual sessions
- **Null backups**: `backupFileName: null` indicates new files without prior versions

### Use Cases

- **Undo changes**: Restore files modified by Claude to previous versions
- **Session recovery**: Recover work if something goes wrong
- **Audit trail**: Track what was modified during a session

---

## shell-snapshots/

Shell environment snapshots for consistent command execution.

### Location

`~/.claude/shell-snapshots/`

### File Naming

```
snapshot-{shell}-{timestamp}-{random}.sh
```

- **shell**: Shell type (e.g., `zsh`, `bash`)
- **timestamp**: Unix timestamp in milliseconds
- **random**: Random suffix for uniqueness

Example: `snapshot-zsh-1763531799989-1l8wtj.sh`

### Contents

Each snapshot is a shell script that captures:

#### 1. Functions

Shell functions defined in your environment:

```bash
nodenv () {
    local command
    command="${1:-}"
    if [ "$#" -gt 0 ]
    then
        shift
    fi
    case "$command" in
        (rehash | shell) eval "$(nodenv "sh-$command" "$@")" ;;
        (*) command nodenv "$command" "$@" ;;
    esac
}
```

#### 2. Shell Options

Shell-specific settings:

```bash
# zsh options
setopt nohashdirs
setopt login
```

#### 3. Aliases

```bash
alias -- run-help=man
alias -- which-command=whence
```

#### 4. PATH and Environment

```bash
export PATH='/Users/santaclaude/.pyenv/shims:/usr/local/bin:...'
```

#### 5. Tool Availability Checks

Fallbacks for tools Claude Code needs:

```bash
# Check for rg availability
if ! command -v rg >/dev/null 2>&1; then
  alias rg='/Users/santaclaude/.local/share/claude/versions/2.0.42 --ripgrep'
fi
```

### Purpose

- **Consistent environment**: Commands run with same tools/config as your terminal
- **Version managers**: Access to pyenv, nodenv, rbenv, etc.
- **Custom functions**: Your shell functions are available
- **PATH access**: All your PATH entries are included

### When Created

New snapshots are created when:
- Starting a new Claude Code session
- Shell environment changes are detected

### Use Cases

- Ensures `python` resolves to correct pyenv version
- Makes custom shell functions available to Claude
- Preserves aliases and shell options
- Provides consistent command execution context

---

## todos/

Task lists created during sessions via the TodoWrite tool.

### Location

`~/.claude/todos/`

### File Naming

```
{parent-session-uuid}-agent-{agent-uuid}.json
```

- **parent-session-uuid**: The main Claude Code session
- **agent-uuid**: The agent that owns this todo list

#### Why "agent"?

Claude Code treats both the main session and sub-agents (spawned via Task tool) as "agents". Each agent can have its own todo list.

**Example - Session with multiple agents:**

```
8243eb68-...-agent-8243eb68-....json  # Main session's todos
8243eb68-...-agent-7afda32a-....json  # Sub-agent's todos
8243eb68-...-agent-be218e52-....json  # Another sub-agent's todos
```

When the parent and agent UUIDs match, it's the main session's todo list.

### Schema

```json
[
  {
    "content": "Add zstd compression to CLI upload",
    "status": "completed",
    "activeForm": "Adding zstd compression to CLI upload"
  },
  {
    "content": "Add zstd decompression middleware to backend",
    "status": "completed",
    "activeForm": "Adding zstd decompression middleware to backend"
  },
  {
    "content": "Test compression with actual session save",
    "status": "pending",
    "activeForm": "Testing compression with actual session save"
  }
]
```

### Fields

| Field | Description |
|-------|-------------|
| `content` | Task description in imperative form (e.g., "Run tests") |
| `status` | Task state: `pending`, `in_progress`, or `completed` |
| `activeForm` | Present continuous form shown during execution (e.g., "Running tests") |

### Task States

| State | Description |
|-------|-------------|
| `pending` | Task not yet started |
| `in_progress` | Currently being worked on |
| `completed` | Task finished successfully |

### Characteristics

- **JSON arrays**: Each file contains an array of todo items
- **Empty by default**: Most sessions don't use todos, resulting in `[]`
- **Per-agent isolation**: Each agent has its own independent todo list
- **Persistent**: Survives session restarts

### Use Cases

- **Progress tracking**: Visual indication of multi-step task progress
- **Task planning**: Break down complex work into manageable steps
- **User visibility**: Shows what Claude is working on
- **Session organization**: Keeps track of remaining work

---

## debug/

Debug logs for each Claude Code session.

### Location

`~/.claude/debug/`

### File Naming

```
{session-uuid}.txt
```

Example: `8b24b5b1-4a2e-417d-ae4b-2e6519234433.txt`

#### Latest Symlink

`latest` → symlink pointing to the current/most recent session's debug file

### Log Format

```
2025-11-19T05:56:39.703Z [DEBUG] Message content here
```

| Component | Description |
|-----------|-------------|
| Timestamp | ISO 8601 format with milliseconds |
| Log level | Primarily `[DEBUG]` |
| Message | Log message with optional component prefix |

### Common Log Categories

| Prefix | Description |
|--------|-------------|
| `Hooks:` | Hook execution, matching, and lifecycle |
| `LSP` / `[LSP MANAGER]` | Language server protocol operations |
| `Stream` | API response streaming events |
| `FileHistory:` | File backup and snapshot operations |
| `Skills` | Skills directory loading |
| `Slash` | Slash command loading |
| `AutoUpdaterWrapper:` | Update checking |
| `Writing` / `Renaming` / `File` | Atomic file write operations |
| `Permission` | Permission rule updates |
| `executePreToolHooks` | Pre-tool hook execution |

### Example Log Entries

#### Session Startup

```
2025-11-19T05:56:39.703Z [DEBUG] Watching for changes in setting files ~/.claude/settings.json, .claude/settings.local.json...
2025-11-19T05:56:39.787Z [DEBUG] [LSP MANAGER] initializeLspServerManager() called
2025-11-19T05:56:39.787Z [DEBUG] [LSP MANAGER] Created manager instance, state=pending
```

#### Permission Loading

```
2025-11-19T05:56:39.796Z [DEBUG] Applying permission update: Adding 24 allow rule(s) to destination 'localSettings': ["Bash(git add:*)","Bash(go build:*)"...]
```

#### Shell Snapshot Creation

```
2025-11-19T05:56:39.989Z [DEBUG] Creating shell snapshot for zsh (/bin/zsh)
2025-11-19T05:56:39.990Z [DEBUG] Creating snapshot at: ~/.claude/shell-snapshots/snapshot-zsh-1763531799989-1l8wtj.sh
2025-11-19T05:56:40.373Z [DEBUG] Shell snapshot created successfully (1404 bytes)
```

#### Atomic File Writes

```
2025-11-19T05:56:39.800Z [DEBUG] Writing to temp file: ~/.claude.json.tmp.2277.1763531799800
2025-11-19T05:56:39.801Z [DEBUG] Temp file written successfully, size: 43687 bytes
2025-11-19T05:56:39.801Z [DEBUG] Renaming ~/.claude.json.tmp... to ~/.claude.json
2025-11-19T05:56:39.801Z [DEBUG] File ~/.claude.json written atomically
```

### File Sizes

Debug files vary in size based on session activity:
- Short sessions: ~10-25 KB
- Long sessions: 100 KB - 500+ KB

### Use Cases

- **Troubleshooting**: Diagnose issues with hooks, LSP, permissions
- **Performance analysis**: Track timing of operations
- **Understanding internals**: See how Claude Code initializes and operates
- **Bug reporting**: Attach debug logs when reporting issues

### Accessing Logs

```bash
# View current session's log
cat ~/.claude/debug/latest

# Follow log in real-time
tail -f ~/.claude/debug/latest

# Search for errors
grep -i error ~/.claude/debug/latest
```

---

## statsig/

Feature flag and A/B testing cache from Statsig.

### Location

`~/.claude/statsig/`

### Files

| File | Description |
|------|-------------|
| `statsig.stable_id.*` | Persistent user ID for consistent experiment bucketing |
| `statsig.session_id.*` | Current session tracking with timestamps |
| `statsig.last_modified_time.evaluations` | Cache freshness timestamps |
| `statsig.cached.evaluations.*` | Cached feature flag and config results |

### Stable ID

```json
"f14c225f-fb9a-430f-b083-001a82036cbd"
```

Persistent UUID that ensures you stay in the same experiment groups across sessions.

### Session ID

```json
{
  "sessionID": "1dd18781-7542-4646-a0bc-f73ca179c7bb",
  "startTime": 1762149222792,
  "lastUpdate": 1762149392819
}
```

Tracks the current analytics session with timestamps.

### Cached Evaluations

The main cache file contains evaluated feature flags and configurations:

```json
{
  "data": {
    "feature_gates": {
      "1089434329": {
        "name": "1089434329",
        "value": false,
        "rule_id": "default",
        "id_type": "userID",
        "secondary_exposures": []
      }
    },
    "dynamic_configs": { ... },
    "layer_configs": { ... },
    "derived_fields": { ... }
  },
  "stableID": "f14c225f-...",
  "fullUserHash": "...",
  "receivedAt": 1762149387111,
  "source": "..."
}
```

#### Data Types

| Type | Count | Description |
|------|-------|-------------|
| `feature_gates` | ~43 | Boolean flags for enabling/disabling features |
| `dynamic_configs` | ~55 | Key-value configurations |
| `layer_configs` | - | Experiment layer configurations |

#### Note on Hashing

Feature gate and config names are hashed (e.g., `1089434329` instead of `enable_new_feature`). This:
- Obscures internal feature names
- Reduces payload size
- Makes reverse-engineering difficult

### Purpose

- **Feature flags**: Enable/disable features for specific users
- **A/B testing**: Run experiments with different user groups
- **Gradual rollouts**: Release features to a percentage of users
- **Remote configuration**: Adjust behavior without code changes

### Caching Benefits

- Faster startup (no network wait)
- Offline functionality
- Reduced API calls to Statsig servers
- Consistent behavior within a session

---

## Other Files and Directories

### settings.json

User settings and preferences for Claude Code.

### backup-config.json

Backup of configuration data.

### downloads/

Directory for downloaded files (typically empty).

### session-env/

Per-session environment variable storage. Contains empty directories named after session UUIDs - appears to be placeholder for future functionality.
