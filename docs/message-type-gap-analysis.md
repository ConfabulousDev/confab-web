# Claude Code Session JSONL Message Type Gap Analysis

**Date:** 2025-11-26
**Analyzed:** 27,754 messages across multiple session files
**Claude Code Version:** 2.0.42 - 2.0.51

## Executive Summary

The frontend session viewer is **missing rendering for image content blocks** and has **incomplete handling of system message subtypes and queue operations**. This report details all message types found in real JSONL files and compares them to the current TypeScript definitions and rendering implementation.

---

## Message Types Overview

### Top-Level Message Types

| Type | Count | In TypeScript | Rendered | Status |
|------|-------|---------------|----------|--------|
| `assistant` | 16,489 | Yes | Yes | OK |
| `user` | 9,462 | Yes | Yes | OK |
| `file-history-snapshot` | 1,619 | Yes | Yes | OK |
| `queue-operation` | 82 | Yes | Partial | NEEDS WORK |
| `summary` | 56 | Yes | Yes | OK |
| `system` | 46 | Yes | Partial | NEEDS WORK |

### Content Block Types (inside messages)

| Type | Count | In TypeScript | Rendered | Status |
|------|-------|---------------|----------|--------|
| `tool_use` | 8,499 | Yes | Yes | OK |
| `tool_result` | 8,495 | Yes | Yes | OK |
| `thinking` | 4,628 | Yes | Yes | OK |
| `text` | 3,493 | Yes | Yes | OK |
| **`image`** | **36** | **NO** | **NO** | **MISSING** |

---

## Gap #1: Image Content Blocks (CRITICAL)

### Problem
User messages can contain image content blocks when users paste screenshots into Claude Code. These are **not defined in TypeScript** and **not rendered** in the frontend.

### Occurrence
36 image blocks found in session files.

### Structure
```json
{
  "type": "image",
  "source": {
    "type": "base64",
    "media_type": "image/png",
    "data": "iVBORw0KGgoAAAANSUhEUgAAAfsAAAD3CAYAAAP..."
  }
}
```

### Files to Modify

#### 1. `frontend/src/types/transcript.ts`

Add ImageBlock interface:
```typescript
export interface ImageBlock {
  type: 'image';
  source: {
    type: 'base64' | 'url';
    media_type: string;
    data?: string;  // For base64
    url?: string;   // For URL type
  };
}
```

Update ContentBlock union:
```typescript
export type ContentBlock = TextBlock | ThinkingBlock | ToolUseBlock | ToolResultBlock | ImageBlock;
```

Add type guard:
```typescript
export function isImageBlock(block: ContentBlock): block is ImageBlock {
  return block.type === 'image';
}
```

#### 2. `frontend/src/components/transcript/ContentBlock.tsx`

Add image rendering:
```typescript
import { isImageBlock } from '@/types/transcript';

// In component:
if (isImageBlock(block)) {
  const src = block.source.type === 'base64'
    ? `data:${block.source.media_type};base64,${block.source.data}`
    : block.source.url;

  return (
    <div className={styles.imageBlock}>
      <img src={src} alt="User provided image" />
    </div>
  );
}
```

---

## Gap #2: System Message Subtypes (MODERATE)

### Problem
Only `compact_boundary` is documented, but other subtypes exist in production data.

### Subtypes Found

| Subtype | Count | Documented | Handled |
|---------|-------|------------|---------|
| `compact_boundary` | 43 | Yes | Yes |
| `local_command` | 2 | No | No |
| `api_error` | 1 | No | No |

### `api_error` Structure
```json
{
  "type": "system",
  "subtype": "api_error",
  "level": "error",
  "error": {
    "status": 529,
    "error": {
      "type": "overloaded_error",
      "message": "Overloaded"
    }
  },
  "retryInMs": 593.08,
  "retryAttempt": 1,
  "maxRetries": 10
}
```

### `local_command` Structure
```json
{
  "type": "system",
  "subtype": "local_command",
  "content": "<command-name>/resume</command-name>\n<command-message>resume</command-message>\n<command-args></command-args>",
  "level": "info"
}
```

### Files to Modify

#### 1. `frontend/src/types/transcript.ts`

Update SystemSubtype:
```typescript
export type SystemSubtype = 'compact_boundary' | 'local_command' | 'api_error' | string;
```

Add error metadata interface:
```typescript
export interface ApiErrorMetadata {
  status: number;
  error: {
    type: string;
    message: string;
  };
  retryInMs?: number;
  retryAttempt?: number;
  maxRetries?: number;
}

export interface SystemMessage extends BaseMessage {
  type: 'system';
  subtype: SystemSubtype;
  content: string;
  level: 'info' | 'warning' | 'error';
  error?: ApiErrorMetadata;  // Add this
  // ... rest of fields
}
```

#### 2. `frontend/src/services/messageParser.ts`

Improve system message parsing to show error details and local commands properly.

---

## Gap #3: Queue Operation Types (LOW)

### Problem
Only `enqueue` and `dequeue` are documented, but other operations exist.

### Operations Found

| Operation | Count | Documented | Handled |
|-----------|-------|------------|---------|
| `enqueue` | 41 | Yes | Yes |
| `remove` | 28 | No | No |
| `dequeue` | 11 | Yes | Yes |
| `popAll` | 2 | No | No |

### Files to Modify

#### 1. `frontend/src/types/transcript.ts`

Update operation type:
```typescript
export interface QueueOperationMessage {
  type: 'queue-operation';
  operation: 'enqueue' | 'dequeue' | 'remove' | 'popAll' | string;
  timestamp: string;
  content: string;
  sessionId: string;
}
```

#### 2. `frontend/src/services/messageParser.ts`

Update queue operation rendering:
```typescript
} else if (isQueueOperationMessage(message)) {
  role = 'system';
  timestamp = message.timestamp;
  const operationLabels: Record<string, { emoji: string; text: string }> = {
    enqueue: { emoji: '➕', text: 'Added to queue' },
    dequeue: { emoji: '➖', text: 'Processing from queue' },
    remove: { emoji: '🗑️', text: 'Removed from queue' },
    popAll: { emoji: '🧹', text: 'Cleared queue' },
  };
  const op = operationLabels[message.operation] || { emoji: '📋', text: message.operation };
  content = [{ type: 'text', text: `${op.emoji} ${op.text}: ${message.content}` }];
}
```

---

## Priority Ranking

1. **HIGH - Image blocks**: Users paste screenshots and they're invisible in the viewer
2. **MEDIUM - api_error system messages**: Useful for debugging session issues
3. **LOW - local_command system messages**: Shows slash commands used
4. **LOW - Queue operations**: Edge case but should be complete

---

## Testing Checklist

After implementation, verify rendering of:

- [ ] Image blocks (base64 encoded PNG/JPG)
- [ ] `api_error` system messages with retry information
- [ ] `local_command` system messages with command details
- [ ] `remove` queue operations
- [ ] `popAll` queue operations
- [ ] Unknown/future message types (fallback rendering)

---

## Appendix: Sample Files for Testing

Session files with image blocks:
- `~/.claude/projects/-Users-santaclaude-dev-beta-confab/1b3d0468-8fab-44ac-a8b6-b06e6ccea4d7.jsonl`

Session files with api_error:
- `~/.claude/projects/-Users-santaclaude-dev-beta-confab/9bfa5d06-d683-4a1e-8bad-2305f08b5288.jsonl`

Session files with local_command:
- `~/.claude/projects/-Users-santaclaude-dev-beta-confab/16133744-636e-4adf-a9dd-71ad2ab82df3.jsonl`
