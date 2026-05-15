# services/

API client and business logic services. All HTTP communication with the backend goes through this layer.

## Files

| File | Role |
|------|------|
| `api.ts` | Centralized API client with Zod-validated endpoints, error classes, and auth handling |
| `transcriptService.ts` | Claude Code transcript: fetching, JSONL parsing, validation, caching, incremental updates |
| `codexTranscriptService.ts` | Codex rollout transcript: parses the Codex JSONL shape, normalizes raw lines into render items, and mirrors the Claude fetch / poll surface |
| `messageParser.ts` | Extracts display-ready data from raw Claude transcript messages |

## Key Components

### api.ts -- API Client

A singleton `APIClient` class that wraps `fetch` with:

- **Zod validation**: All responses are validated at runtime. Methods like `getValidated()`, `postValidated()`, `patchValidated()` parse responses through Zod schemas from `@/schemas/api.ts`. Additional helpers: `deleteVoid()` for DELETE operations, `getString()` for plain text responses.
- **Auth handling**: 401 responses trigger `handleAuthFailure()` (redirect to `/`) unless the endpoint is in the skip list (e.g., `/me`, `/sessions/:id`).
- **Error classes**: `APIError`, `AuthenticationError`, `NetworkError` with status codes and backend error message extraction.
- **Credential management**: All requests include `credentials: 'include'` for cookie-based auth.

#### Exported API namespaces

| Namespace | Methods | Description |
|-----------|---------|-------------|
| `sessionsAPI` | `list`, `get`, `updateTitle`, `getShares`, `createShare`, `revokeShare` | Session CRUD and sharing |
| `authAPI` | `me` | Current user info |
| `syncFilesAPI` | `getContent` | File content retrieval with optional `line_offset` |
| `keysAPI` | `list`, `create`, `delete` | API key management |
| `sharesAPI` | `list` | List user's share links |
| `githubLinksAPI` | `list`, `create`, `delete` | GitHub link management |
| `analyticsAPI` | `get`, `regenerateSmartRecap` | Session analytics with 304 support |
| `trendsAPI` | `get` | Aggregated trends with epoch-based date params |
| `orgAnalyticsAPI` | `get` | Organization-level analytics |
| `tilsAPI` | `list`, `listForSession`, `create`, `update`, `delete` | TIL management |

#### Error classes

```typescript
class APIError extends Error { status: number; statusText: string; data?: unknown }
class AuthenticationError extends APIError { /* always status 401 */ }
class NetworkError extends Error { /* fetch TypeError */ }
```

### transcriptService.ts -- Transcript Processing

Handles the full lifecycle of transcript data:

1. **Fetching**: `fetchTranscriptContent()` retrieves JSONL via `syncFilesAPI.getContent()`
2. **Parsing**: `parseJSONL()` splits on newlines, validates each line with Zod, and pre-filters metadata-only records listed in `SKIPPED_MESSAGE_TYPES` (currently `progress`, `permission-mode`, `ai-title`, `last-prompt`). `attachment` rows are no longer pre-skipped — they parse via `AttachmentMessageSchema` and the categorizer hides noisy subtypes (CF-346)
3. **Caching**: In-memory cache keyed by `sessionId-fileName`, with `skipCache` option for fresh loads
4. **Incremental updates**: `fetchNewTranscriptMessages()` fetches only lines after a given offset
5. **Error reporting**: Validation errors are reported to `/api/v1/client-errors` (fire-and-forget, deduplicated per session)

Key exports:
- `fetchParsedTranscript(sessionId, fileName, skipCache?)` -- Full transcript with metadata
- `fetchNewTranscriptMessages(sessionId, fileName, currentLineCount)` -- Incremental fetch
- `parseJSONL(jsonl)` -- Parse JSONL string into validated `TranscriptLine[]`

### codexTranscriptService.ts -- Codex Transcript Processing

Provider-specific parser for the Codex rollout JSONL (`{ timestamp, type, payload }`
envelopes with nested `payload.type` discriminators). Splits the work into two
passes so the React layer can re-derive items from raw lines in `useMemo`
without re-fetching:

1. **`parseCodexJSONL(jsonl)`** -- Validates each line against
   `RawCodexLineSchema` from `@/schemas/codexTranscript`. Returns
   `{ rawLines, errors, totalLines, successCount, errorCount }`. Bad lines are
   recorded but do not abort the parse. `totalLines` reflects non-empty input
   lines so the line-offset incremental fetch stays in sync.
2. **`normalizeCodexLines(rawLines)`** -- Pure synchronous transform that
   collapses the rich, partially-redundant Codex stream into a clean
   `CodexRenderItem[]` for the timeline. Responsibilities:
   - Drop noise: `session_meta`, `turn_context`, `event_msg.token_count`,
     `event_msg.task_started`, `event_msg.user_message`,
     `event_msg.agent_message`, `response_item.message[role=developer]`.
   - Strip `<environment_context>…</environment_context>` blocks from user
     message text.
   - Strip the Codex exec output preamble (`Chunk ID:`, `Wall time:`,
     `Process exited with code:`) and surface the parsed exit code + wall
     time as `execMetadata`.
   - Pair `function_call` ↔ `function_call_output` by `call_id`.
   - Pair `custom_tool_call` ↔ `custom_tool_call_output` ↔
     `event_msg.patch_apply_end` (merges structured patch info into the
     existing draft).
   - Track the running model from `session_meta.model` and
     `task_started.model`; attach it to each assistant render item.
   - Emit `CodexReasoningHiddenItem` placeholders for encrypted reasoning
     lines, `CodexTurnSeparatorItem` per `task_complete` (with
     `durationMs` + `timeToFirstTokenMs`), and `CodexCompactedItem` per
     `compacted` line.
   - Fall back to `CodexUnknownItem` for any top-level or nested type that
     isn't recognized, so forward-compat additions are visible instead of
     silently dropped.
   - **CF-360 — `lineId`**: stamps every emitted item with `String(idx)`,
     where `idx` is the item's position in the input `rawLines` array. The
     id is monotonic, unique per item, and stable across re-renders of the
     same append-only `rawLines` stream — the contract the `?msg=<lineId>`
     deep-link relies on. Multi-line items (tool_call + output pairing,
     compaction) keep the `lineId` of the line that *created* them; output
     mutations preserve `lineId` through the existing spread-update pattern.
     See `@/types/codexRenderItem` for the full invariant doc.

Key exports:
- `fetchParsedCodexTranscript(sessionId, fileName, skipCache?)` -- Initial
  load with in-memory cache (mirrors `fetchParsedTranscript`).
- `fetchNewCodexLines(sessionId, fileName, currentLineCount)` -- Incremental
  poll, returns `{ newRawLines, newTotalLineCount }`. Callers append raw lines
  to their accumulated state and re-derive items via `useMemo`.
- `extractCodexModel(rawLines)` -- Pure helper (CF-386). Walks parsed rollout
  lines and returns the first non-empty `payload.model` from a `session_meta`
  or `turn_context` envelope, matching the backend Codex parser's fallback
  chain (`backend/internal/codex/parser.go:170-177`). Used by `SessionViewer`
  to derive the model name for the session header on both providers.
  Returns `undefined` when no envelope carries a non-empty string model.
  Replaces CF-383's `fetchCodexSessionMeta`, which read only the first line
  and missed the canonical `turn_context` source.
- `parseCodexJSONL`, `normalizeCodexLines` -- Used directly by tests and
  Storybook stories; exposed so consumers don't need to re-fetch to re-derive.
- `reportCodexTranscriptErrors(sessionId, errors)` -- Sends to
  `/api/v1/client-errors` with `category: 'codex_transcript_validation'`
  (separate from Claude's `transcript_validation` for independent triage).

### messageParser.ts -- Message Display

Transforms raw `TranscriptLine` objects into display-ready `ParsedMessageData`:
- Determines role (`user`, `assistant`, `system`, `unknown`)
- Extracts content blocks, timestamp, model name
- Classifies message subtypes (tool result, thinking, tool use)
- Handles all message types: user, assistant, system, summary, file-history-snapshot, queue-operation, pr-link, unknown

Key exports:
- `parseMessage(message)` -- Returns `ParsedMessageData`
- `extractTextContent(content)` -- Plain text extraction for search indexing and clipboard
- `getRoleLabel(role, isToolResult)` -- Display label for message role

## How to Extend

### Adding a new API endpoint
1. Add the Zod schema to `@/schemas/api.ts`
2. Add the endpoint method to the appropriate namespace in `api.ts`
3. Use `getValidated()`, `postValidated()`, or `patchValidated()` for type-safe responses, or `deleteVoid()` for delete operations
4. For endpoints needing custom behavior (e.g., 304 handling), use the `fetchRaw()` helper

### Adding a new message type
1. Add the schema to `@/schemas/transcript.ts`
2. Add a type guard in the same file
3. Add a rendering branch in `messageParser.ts`'s `parseMessage()` function
4. Update `extractTextContent()` if the new type has searchable text

## Invariants / Conventions

- All API responses are Zod-validated at runtime -- schema mismatches throw a Zod `ZodError` via `validateResponse()`
- The API client is a singleton (`const api = new APIClient()`)
- 401 handling is centralized: all endpoints redirect to `/` on 401 unless explicitly skipped
- Transcript `line_offset` tracking uses total JSONL lines (not parsed message count) to stay in sync with the backend
- Backend error messages follow `{"error": "message"}` format and are extracted by `APIError`

## Design Decisions

- **Zod-validated responses**: Every API response is parsed through a Zod schema. This catches backend contract changes at runtime rather than letting invalid data silently corrupt the UI.
- **Conditional analytics requests**: `analyticsAPI.get()` sends `as_of_line` to get 304 Not Modified when data hasn't changed, reducing bandwidth for polling.
- **Fire-and-forget error reporting**: Transcript validation errors are reported to the backend for observability but never block the user. The UI gracefully skips invalid lines.
- **Epoch-based date parameters**: `trendsAPI` and `orgAnalyticsAPI` convert local dates to epoch seconds with timezone offset to ensure correct daily grouping regardless of server timezone.

## Testing

- `api.test.ts` -- API client error handling, auth flow, response validation
- `transcriptService.test.ts` -- JSONL parsing, validation error handling, incremental fetch
- `codexTranscriptService.test.ts` -- Codex JSONL parsing + normalization, fixture-driven (`src/test-fixtures/codex-rollout.jsonl`)
- `messageParser.test.ts` -- Message parsing for all message types, text extraction

## Dependencies

- `zod` (runtime response validation)
- `@/schemas/api` (response schemas and types)
- `@/schemas/transcript` (transcript line schemas)
- `@/schemas/codexTranscript` (Codex rollout schemas + `isKnown*` type predicates)
- `@/types/codexRenderItem` (Codex render-item types)
- `@/utils/sessionErrors` (401 redirect skip list)
- `@/utils/utils` (`isRecord` for safe shape inspection without `as` casts)
