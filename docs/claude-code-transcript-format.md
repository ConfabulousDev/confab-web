# Claude Code Transcript Format Documentation

**Version:** 2.0.42 (as of Nov 2025)
**Format:** JSONL (JSON Lines) - one JSON object per line
**Location:** `~/.claude/projects/{project-path-encoded}/{session-id}.jsonl`
**Agent Location:** `~/.claude/projects/{project-path-encoded}/agent-{agent-id}.jsonl`

## Overview

Claude Code stores conversation transcripts as JSONL files where each line represents a message or event. Messages include user inputs, assistant responses, tool use/results, system events, and metadata snapshots.

## File Organization

```
~/.claude/projects/
├── -Users-santaclaude-dev-beta-confab/
│   ├── 8b24b5b1-4a2e-417d-ae4b-2e6519234433.jsonl  # Main session transcript
│   ├── agent-0da5686d.jsonl                        # Agent sidechain
│   ├── agent-136fa428.jsonl                        # Another agent
│   └── ...
```

- **Project directories**: Encoded by replacing `/` with `-` in path
- **Session files**: Named as `{session-uuid}.jsonl`
- **Agent files**: Named as `agent-{8-char-hex-id}.jsonl`

---

## Message Types

Claude Code transcripts contain 6 primary message types:

| Type | Description | Frequency | Visibility |
|------|-------------|-----------|------------|
| `user` | User messages | High | Always shown |
| `assistant` | Assistant responses | High | Always shown |
| `file-history-snapshot` | File tracking metadata | Medium | Hidden from user |
| `system` | System events (compaction, etc.) | Low | Shown as notices |
| `summary` | Conversation summaries | Very Low | Shown in UI |
| `queue-operation` | Queued user inputs | Low | Hidden |

---

## Message Structure by Type

### 1. User Message

**Purpose:** User input, either text or tool results

```json
{
  "type": "user",
  "uuid": "a7ae917c-eccb-4e61-9e82-39251b85fb3f",
  "parentUuid": "7cadf770-853a-4641-9312-155fd2869f7d",
  "timestamp": "2025-11-18T00:18:42.913Z",
  "isSidechain": false,
  "userType": "external",
  "cwd": "/Users/santaclaude/dev/beta/confab",
  "sessionId": "2e629759-ce43-4af3-a8f3-48c7c2ac72e2",
  "version": "2.0.42",
  "gitBranch": "main",
  "thinkingMetadata": {
    "level": "high",
    "disabled": false,
    "triggers": []
  },
  "message": {
    "role": "user",
    "content": "we recently added an api key validation endpoint. We want to rate limit on it"
  }
}
```

**Fields:**
- `uuid`: Unique message identifier
- `parentUuid`: UUID of message this responds to
- `timestamp`: ISO 8601 timestamp
- `isSidechain`: `false` for main transcript, `true` for agents
- `userType`: Usually `"external"`
- `cwd`: Current working directory
- `sessionId`: Session UUID
- `version`: Claude Code version
- `gitBranch`: Current git branch (if in repo)
- `thinkingMetadata`: Thinking block configuration
  - `level`: `"high"` | `"medium"` | `"low"` | `"off"`
  - `disabled`: boolean
  - `triggers`: array of trigger conditions
- `message`: The actual message content
  - `role`: `"user"`
  - `content`: String or array of content blocks

**User Message with Tool Results:**

```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {
        "type": "tool_result",
        "tool_use_id": "toolu_01NHDUYBGs52pSNJxuamKqsY",
        "content": "Found 4 files\ncli/cmd/status.go\nbackend/internal/api/server.go"
      }
    ]
  }
}
```

### 2. Assistant Message

**Purpose:** Claude's responses, including thinking, text, and tool use

```json
{
  "type": "assistant",
  "uuid": "7cadf770-853a-4641-9312-155fd2869f7d",
  "parentUuid": "a7ae917c-eccb-4e61-9e82-39251b85fb3f",
  "timestamp": "2025-11-18T00:18:46.680Z",
  "requestId": "req_011CVEPtecPZjEsmo1E5aAmF",
  "isSidechain": false,
  "userType": "external",
  "cwd": "/Users/santaclaude/dev/beta/confab",
  "sessionId": "2e629759-ce43-4af3-a8f3-48c7c2ac72e2",
  "version": "2.0.42",
  "gitBranch": "main",
  "message": {
    "model": "claude-sonnet-4-5-20250929",
    "id": "msg_01MWLUF1bbiqgxr6GBPbhurs",
    "type": "message",
    "role": "assistant",
    "content": [...],
    "stop_reason": "tool_use",
    "stop_sequence": null,
    "usage": {
      "input_tokens": 10,
      "cache_creation_input_tokens": 3291,
      "cache_read_input_tokens": 13077,
      "cache_creation": {
        "ephemeral_5m_input_tokens": 3291,
        "ephemeral_1h_input_tokens": 0
      },
      "output_tokens": 395,
      "service_tier": "standard"
    }
  }
}
```

**Fields:**
- `requestId`: API request ID
- `message.model`: Model identifier (e.g., `"claude-sonnet-4-5-20250929"`)
- `message.id`: API message ID
- `message.stop_reason`: Why generation stopped
  - `"end_turn"`: Natural completion
  - `"tool_use"`: Stopped to use a tool
  - `"max_tokens"`: Hit token limit
- `message.usage`: Token usage statistics
  - `input_tokens`: Input tokens
  - `cache_creation_input_tokens`: Tokens written to cache
  - `cache_read_input_tokens`: Tokens read from cache
  - `output_tokens`: Generated tokens
  - `service_tier`: API tier (`"standard"`)

**Content Blocks:**

Assistant `content` is always an array of content blocks:

#### Text Block
```json
{
  "type": "text",
  "text": "Let me search for the API key validation endpoint..."
}
```

#### Thinking Block
```json
{
  "type": "thinking",
  "thinking": "The user wants to add rate limiting...",
  "signature": "EtkECkYICRgCKkA3Ly7Dxk..." // Cryptographic signature
}
```

#### Tool Use Block
```json
{
  "type": "tool_use",
  "id": "toolu_01NHDUYBGs52pSNJxuamKqsY",
  "name": "Grep",
  "input": {
    "pattern": "api.*key.*validat",
    "output_mode": "files_with_matches",
    "-i": true
  }
}
```

**Common Tool Names:**
- `Bash` - Execute shell commands
- `Read` - Read file contents
- `Write` - Write to files
- `Edit` - Edit existing files
- `Grep` - Search file contents
- `Glob` - Find files by pattern
- `Task` - Spawn a sub-agent
- `WebFetch` - Fetch web content
- `WebSearch` - Search the web

### 3. Tool Result (in User Message)

**Purpose:** Results from tool executions

```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01NHDUYBGs52pSNJxuamKqsY",
  "content": "Found 4 files\ncli/cmd/status.go\nbackend/internal/api/server.go",
  "is_error": false
}
```

**With Error:**
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_014BniYkbBJ9QoKcC68TiGyU",
  "content": "[Request interrupted by user for tool use]",
  "is_error": true
}
```

**Agent Task Result:**
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01ABC123...",
  "content": [
    {
      "type": "text",
      "text": "Here's the comprehensive overview..."
    }
  ],
  "toolUseResult": {
    "status": "completed",
    "prompt": "Explore this codebase thoroughly...",
    "agentId": "0da5686d",
    "content": [...],
    "totalDurationMs": 100032,
    "totalTokens": 32927,
    "totalToolUseCount": 25,
    "usage": {
      "input_tokens": 7,
      "cache_creation_input_tokens": 416,
      "cache_read_input_tokens": 29067,
      "output_tokens": 3437,
      "service_tier": "standard"
    }
  }
}
```

**toolUseResult Fields:**
- `status`: `"completed"` | `"interrupted"` | `"error"`
- `prompt`: Original prompt given to agent
- `agentId`: The agent's 8-character hex ID
- `content`: Agent's final response
- `totalDurationMs`: Total execution time
- `totalTokens`: Total tokens used
- `totalToolUseCount`: Number of tools used
- `usage`: Token usage breakdown

### 4. File History Snapshot

**Purpose:** Tracks file changes for auto-revert feature

```json
{
  "type": "file-history-snapshot",
  "messageId": "7e36d7e5-a21c-4613-aa49-308d0787a1de",
  "isSnapshotUpdate": false,
  "snapshot": {
    "messageId": "7e36d7e5-a21c-4613-aa49-308d0787a1de",
    "timestamp": "2025-11-16T19:22:08.324Z",
    "trackedFileBackups": {
      "backend/cmd/server/main.go": {
        "backupFileName": "2ac9f4c88c39e759@v2",
        "version": 2,
        "backupTime": "2025-11-16T19:47:09.789Z"
      }
    }
  }
}
```

**Fields:**
- `trackedFileBackups`: Map of file path to backup metadata
  - `backupFileName`: Backup file identifier or `null`
  - `version`: Incremental version number
  - `backupTime`: When backup was created

### 5. System Message

**Purpose:** System events like conversation compaction

```json
{
  "type": "system",
  "uuid": "2009535a-02f4-438b-a72d-99d60b3fa0e1",
  "parentUuid": null,
  "logicalParentUuid": "bd65e37c-74e4-42ba-90e5-bf9677aad784",
  "timestamp": "2025-11-18T06:49:27.632Z",
  "isSidechain": false,
  "userType": "external",
  "cwd": "/Users/santaclaude/dev/beta/confab/backend",
  "sessionId": "2e629759-ce43-4af3-a8f3-48c7c2ac72e2",
  "version": "2.0.42",
  "gitBranch": "main",
  "subtype": "compact_boundary",
  "content": "Conversation compacted",
  "isMeta": false,
  "level": "info",
  "compactMetadata": {
    "trigger": "auto",
    "preTokens": 155341
  }
}
```

**Subtypes:**
- `"compact_boundary"`: Conversation was compacted to save tokens

**Fields:**
- `subtype`: Type of system event
- `content`: Human-readable description
- `level`: `"info"` | `"warning"` | `"error"`
- `compactMetadata`: Details about compaction
  - `trigger`: `"auto"` | `"manual"`
  - `preTokens`: Token count before compaction

### 6. Summary Message

**Purpose:** AI-generated summary of conversation segment

```json
{
  "type": "summary",
  "summary": "Fixed session metadata save database type error",
  "leafUuid": "9a86b37f-2241-459a-819b-e09640d740ee"
}
```

**Fields:**
- `summary`: Concise description of what was accomplished
- `leafUuid`: UUID of last message in summarized segment

### 7. Queue Operation Message

**Purpose:** Tracks queued user inputs (for rate limiting or batching)

```json
{
  "type": "queue-operation",
  "operation": "enqueue",
  "timestamp": "2025-11-18T01:17:46.302Z",
  "content": "let me about the go version",
  "sessionId": "2e629759-ce43-4af3-a8f3-48c7c2ac72e2"
}
```

**Fields:**
- `operation`: `"enqueue"` | `"dequeue"` (possibly others)
- `content`: The queued message text
- `timestamp`: When queued

---

## Agent Transcripts

Agent transcripts have the same structure as main transcripts but with:
- `isSidechain: true` (vs `false` in main)
- `agentId: "0da5686d"` (8-character hex identifier)
- Located in `agent-{agentId}.jsonl` files

**Linking Agents to Parent Messages:**

1. Find `tool_use` block with `name: "Task"` in main transcript
2. Extract the `tool_use_id`
3. Find corresponding `tool_result` with matching `tool_use_id`
4. Extract `toolUseResult.agentId`
5. Load `agent-{agentId}.jsonl` transcript
6. Display agent conversation inline or nested

**Example Flow:**
```
Main: Assistant uses Task tool (toolu_01ABC)
Main: User message with tool_result for toolu_01ABC
      → toolUseResult.agentId = "0da5686d"
Agent: Load agent-0da5686d.jsonl
Agent: Display agent messages
```

---

## Message Relationships

### Parent-Child Relationships

- `parentUuid`: Links to immediate parent message
- `logicalParentUuid`: Used for system messages (compaction preserves logical flow)
- `uuid`: Unique identifier for this message

**Conversation Thread:**
```
User (uuid: A, parentUuid: null)
  └─ Assistant (uuid: B, parentUuid: A)
       └─ User [tool result] (uuid: C, parentUuid: B)
            └─ Assistant (uuid: D, parentUuid: C)
```

### Agent Nesting

Agents can spawn sub-agents, creating arbitrary nesting depth:

```
Main Session
├─ Message 1 (user)
├─ Message 2 (assistant, uses Task -> agent-AAA)
│   └─ Agent AAA
│       ├─ Message 1 (assistant)
│       ├─ Message 2 (assistant, uses Task -> agent-BBB)
│       │   └─ Agent BBB
│       │       └─ ...
│       └─ Message 3 (assistant, response)
└─ Message 3 (user, tool result from agent-AAA)
```

---

## Common Patterns

### 1. Simple User-Assistant Exchange
```
User: "Fix the bug in auth.go"
Assistant: [thinking] "I need to find auth.go..." [tool_use: Read]
User: [tool_result: file contents]
Assistant: [text] "I found the issue. Let me fix it." [tool_use: Edit]
User: [tool_result: success]
Assistant: [text] "Fixed! The bug was..."
```

### 2. Agent Spawning
```
User: "Explore this codebase"
Assistant: [tool_use: Task, agentId will be assigned]
User: [tool_result with toolUseResult.agentId = "0da5686d"]
  → Agent 0da5686d runs independently
  → Returns comprehensive report
Assistant: "Based on the exploration..." [references agent findings]
```

### 3. Thinking Blocks

Thinking blocks appear before text or tool use:
```
Assistant message content: [
  { type: "thinking", thinking: "User wants X. I should..." },
  { type: "text", text: "I'll help with that..." },
  { type: "tool_use", name: "Bash", ... }
]
```

---

## Edge Cases & Special Scenarios

### Interrupted Tools
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_014BniYkbBJ9QoKcC68TiGyU",
  "content": "[Request interrupted by user for tool use]",
  "is_error": true
}
```

### Empty Transcripts
Some sessions may only contain `file-history-snapshot` messages if Claude Code exited immediately.

### Multiple Tool Uses in One Message
Assistant can request multiple tools simultaneously:
```json
{
  "content": [
    { "type": "tool_use", "id": "toolu_01A", "name": "Read", ... },
    { "type": "tool_use", "id": "toolu_01B", "name": "Grep", ... }
  ]
}
```

Results come back in separate user messages or combined in one.

### Tool Result with Multiple Content Blocks
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01ABC",
  "content": [
    { "type": "text", "text": "Result part 1" },
    { "type": "text", "text": "Result part 2" }
  ]
}
```

---

## Parsing Recommendations

### 1. JSONL Parsing
```typescript
function parseTranscript(jsonl: string): Message[] {
  return jsonl
    .split('\n')
    .filter(line => line.trim())
    .map(line => JSON.parse(line));
}
```

### 2. Type Detection
```typescript
if (msg.type === 'user') {
  // Check if it's a tool result
  const content = msg.message.content;
  if (Array.isArray(content) && content[0]?.type === 'tool_result') {
    // Handle tool result
  } else {
    // Handle user text
  }
}
```

### 3. Agent Linking
```typescript
// Find agent ID from tool result
const toolResult = userMessage.message.content[0];
if (toolResult.toolUseResult?.agentId) {
  const agentId = toolResult.toolUseResult.agentId;
  const agentFile = `agent-${agentId}.jsonl`;
  // Load and parse agent transcript
}
```

### 4. Content Block Handling
```typescript
assistant.message.content.forEach(block => {
  switch (block.type) {
    case 'thinking':
      renderThinking(block.thinking);
      break;
    case 'text':
      renderText(block.text);
      break;
    case 'tool_use':
      renderToolUse(block.name, block.input);
      break;
  }
});
```

---

## Known Limitations & Unknowns

### Unknowns
1. **Full list of system subtypes**: Only `compact_boundary` observed
2. **All possible stop_reason values**: Only seen `end_turn`, `tool_use`, `max_tokens`
3. **Queue operation types**: Only seen `enqueue`
4. **Thinking signature format**: Purpose and structure unclear
5. **Tool result content variations**: May support more formats than observed

### Format Stability
- **Likely stable**: Core message structure (`type`, `uuid`, `timestamp`, `message`)
- **May change**: `thinkingMetadata` structure, `usage` field additions
- **Could be added**: New message types, new tool names, new content block types

### Version Detection
Claude Code version is in every message (`version: "2.0.42"`), but there's no explicit transcript format version number. Monitor the `version` field for major changes (e.g., `3.0.0`).

---

## Testing Checklist

When parsing Claude Code transcripts, test against:

- [ ] Simple user-assistant text exchanges
- [ ] Messages with thinking blocks
- [ ] Tool use: Bash, Read, Write, Edit, Grep, Glob
- [ ] Tool errors (is_error: true)
- [ ] Agent spawning (Task tool)
- [ ] Nested agents (agents spawning sub-agents)
- [ ] Interrupted tools
- [ ] File history snapshots
- [ ] System messages (compaction)
- [ ] Summary messages
- [ ] Queue operations
- [ ] Empty/minimal transcripts
- [ ] Very large transcripts (1000+ messages)
- [ ] Unicode content (emojis, special characters)
- [ ] Code blocks in text
- [ ] Long tool outputs (truncation?)

---

## Appendix: Sample Messages

See example transcripts in:
- `~/.claude/projects/-Users-santaclaude-dev-beta-confab/2e629759-ce43-4af3-a8f3-48c7c2ac72e2.jsonl`
- Test data: `/Users/santaclaude/dev/beta/confab/test_session/`

---

**Last Updated:** 2025-11-19
**Based on:** Claude Code v2.0.42
**Transcript Samples:** 800+ messages analyzed across 6 sessions
