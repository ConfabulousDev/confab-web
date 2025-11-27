# Confab Sync Daemon Implementation Plan

**Status: Core Implementation Complete**

## Overview

Implement a daemon process that incrementally syncs session data to the backend during active Claude Code sessions, rather than only uploading at session end.

## Current Architecture

- **SessionEnd hook** triggers `confab save` which uploads complete session files
- **Problem**: If session end fails (crash, kill, etc.), data is lost
- **Problem**: Large sessions accumulate data that all uploads at once

## Proposed Architecture

```
SessionStart hook
    ↓
confab sync start (spawns daemon)
    ↓
Daemon: watches transcript, uploads chunks periodically
    ↓
SessionEnd hook
    ↓
confab sync stop (final sync + shutdown daemon)
```

## Implementation Steps

### Phase 1: New CLI Commands

#### 1.1 Add `confab sync start` command
- **File**: `cmd/sync.go` (new)
- **Input**: Receives `HookInput` from stdin (same as save)
- **Behavior**:
  1. Return `HookResponse` immediately (don't block Claude)
  2. Call backend `POST /api/v1/sync/init` to get session_id and resume state
  3. Spawn daemon process with session context
  4. Parent exits

#### 1.2 Add `confab sync stop` command
- **File**: `cmd/sync.go`
- **Input**: Receives `HookInput` from stdin
- **Behavior**:
  1. Signal daemon to do final sync and shutdown
  2. Wait briefly for clean shutdown
  3. Return `HookResponse`
  4. Fallback: if daemon not running, do full upload (like current save)

#### 1.3 Add `confab sync status` command
- **File**: `cmd/sync.go`
- **Behavior**: Show running daemons and their sync state

### Phase 2: Daemon Process

#### 2.1 Create daemon package
- **File**: `pkg/daemon/daemon.go` (new)
- **Responsibilities**:
  - Track sync state (last synced line per file)
  - Watch transcript file for changes
  - Upload new chunks periodically
  - Handle shutdown signal

#### 2.2 Daemon state management
- **File**: `pkg/daemon/state.go` (new)
- **State file**: `~/.confab/sync/{external_id}.json`
- **Contents**:
  ```json
  {
    "external_id": "abc123",
    "session_id": "uuid-from-backend",
    "pid": 12345,
    "started_at": "2025-01-15T10:00:00Z",
    "files": {
      "transcript.jsonl": {"last_synced_line": 150}
    }
  }
  ```

#### 2.3 File watching strategy
- **Option A**: Poll-based (simpler, cross-platform)
  - Check file mtime every N seconds
  - Read new lines from last position
- **Option B**: fsnotify (more efficient)
  - React to file modifications
  - Debounce rapid changes

**Recommendation**: Start with poll-based for simplicity

#### 2.4 Chunk upload logic
- **File**: `pkg/daemon/sync.go` (new)
- **Functions**:
  - `readNewLines(filePath, fromLine) ([]string, error)`
  - `uploadChunk(sessionID, fileName, fileType, firstLine, lines) error`
  - `syncFile(filePath, fileType, state) error`

### Phase 3: Backend API Integration

#### 3.1 Add sync client
- **File**: `pkg/sync/client.go` (new)
- **Functions**:
  ```go
  type SyncClient struct {
      httpClient *http.Client
      config     *config.UploadConfig
  }

  func (c *SyncClient) Init(externalID, transcriptPath, cwd string) (*SyncInitResponse, error)
  func (c *SyncClient) UploadChunk(sessionID, fileName, fileType string, firstLine int, lines []string) error
  ```

### Phase 4: Hook Installation

#### 4.1 Modify hook installer
- **File**: `pkg/config/config.go`
- **Changes**:
  - Add `InstallSyncHooks()` function
  - Installs both SessionStart and SessionEnd hooks
  - SessionStart → `confab sync start`
  - SessionEnd → `confab sync stop`

#### 4.2 Add migration path
- **File**: `cmd/init.go`
- **Changes**:
  - Detect old save-only hook
  - Offer to upgrade to sync hooks
  - Or add `--sync` flag to init

### Phase 5: Graceful Shutdown

#### 5.1 Signal handling
- **File**: `pkg/daemon/daemon.go`
- **Signals**: SIGTERM, SIGINT
- **Behavior**:
  1. Stop file watching
  2. Do final sync of all files
  3. Update state file
  4. Exit cleanly

#### 5.2 Shutdown via IPC
- **Options**:
  - Unix socket
  - PID file + signal
  - Named pipe

**Recommendation**: PID file + SIGTERM (simplest)

#### 5.3 Orphan detection
- Check if parent Claude process is still alive
- If not, do final sync and exit
- Prevents zombie daemons

### Phase 6: Error Handling & Recovery

#### 6.1 Backend unavailable
- Retry with exponential backoff
- Don't block file watching during retries
- Log failures

#### 6.2 Resume after crash
- On `sync start`, check for existing state file
- If daemon crashed, backend has high-water mark
- Resume from where backend says we left off

#### 6.3 Chunk upload failure
- Keep local state file as source of truth during session
- On init, merge with backend state
- Log any discrepancies

## File Structure

```
cli/
├── cmd/
│   ├── sync.go           # sync start/stop/status commands (NEW)
│   └── init.go           # Modified for sync hooks
├── pkg/
│   ├── daemon/           # NEW
│   │   ├── daemon.go     # Main daemon loop
│   │   ├── state.go      # State file management
│   │   ├── sync.go       # Chunk sync logic
│   │   └── watcher.go    # File watching
│   ├── sync/             # NEW
│   │   └── client.go     # Sync API client
│   └── config/
│       └── config.go     # Modified for sync hooks
```

## Configuration

### Sync interval
- Default: 30 seconds
- Configurable via `~/.confab/config.json`:
  ```json
  {
    "sync_interval": "30s"
  }
  ```

### Chunk size
- Default: 100 lines per chunk (or whatever's new)
- Upload whatever is available at each interval

## Testing Strategy

### Unit tests
- State file read/write
- Line counting and chunk extraction
- Signal handling

### Integration tests
- Full sync flow with test backend
- Resume after simulated crash
- Concurrent file modifications

### Manual testing
- Real Claude Code session
- Kill daemon mid-session
- Network interruption scenarios

## Migration Path

1. **Phase 1**: Ship sync commands, keep save as default
2. **Phase 2**: Add `confab init --sync` to opt-in
3. **Phase 3**: Make sync the default for new installs
4. **Phase 4**: Auto-migrate existing users (with notice)

## Design Decisions

1. **Watch all files or just transcript?**
   - **Decision**: Watch transcript + agent files (subtranscripts)
   - Transcript is primary, start syncing immediately
   - As transcript grows, scan for agent file references (`toolUseResult.agentId`)
   - When new agent file discovered, start syncing it too (same chunk logic)
   - TODOs: Handle later (small, change frequently, different format)

2. **What if transcript gets truncated/rotated?**
   - Claude Code doesn't do this, but should handle gracefully
   - Detect via file size decrease
   - Reset and re-sync from start?

3. **Multiple Claude Code sessions simultaneously?**
   - Each session gets its own daemon
   - State files keyed by external_id
   - No conflicts

## Implementation Status

| Component | Status | File |
|-----------|--------|------|
| Sync API client | ✅ Done | `pkg/sync/client.go` |
| State management | ✅ Done | `pkg/daemon/state.go` |
| File watcher | ✅ Done | `pkg/daemon/watcher.go` |
| Sync logic | ✅ Done | `pkg/daemon/sync.go` |
| Daemon main loop | ✅ Done | `pkg/daemon/daemon.go` |
| CLI commands | ✅ Done | `cmd/sync.go` |
| Hook installation | ✅ Done | `pkg/config/config.go` |
| Tests | ⏳ Pending | - |

## Usage

```bash
# Install sync hooks (replaces old save hook)
confab init --sync

# Check running daemons
confab sync status

# Manual daemon control (normally triggered by hooks)
confab sync start  # Start daemon (reads hook input from stdin)
confab sync stop   # Stop daemon (reads hook input from stdin)
```
