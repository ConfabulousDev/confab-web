# services/

API client and business logic services. All HTTP communication with the backend goes through this layer.

## Files

| File | Role |
|------|------|
| `api.ts` | Centralized API client with Zod-validated endpoints, error classes, and auth handling |
| `transcriptService.ts` | Transcript fetching, JSONL parsing, validation, caching, and incremental updates |
| `messageParser.ts` | Extracts display-ready data from raw transcript messages |

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

#### Error classes

```typescript
class APIError extends Error { status: number; statusText: string; data?: unknown }
class AuthenticationError extends APIError { /* always status 401 */ }
class NetworkError extends Error { /* fetch TypeError */ }
```

### transcriptService.ts -- Transcript Processing

Handles the full lifecycle of transcript data:

1. **Fetching**: `fetchTranscriptContent()` retrieves JSONL via `syncFilesAPI.getContent()`
2. **Parsing**: `parseJSONL()` splits on newlines, validates each line with Zod, skips `progress` messages
3. **Caching**: In-memory cache keyed by `sessionId-fileName`, with `skipCache` option for fresh loads
4. **Incremental updates**: `fetchNewTranscriptMessages()` fetches only lines after a given offset
5. **Error reporting**: Validation errors are reported to `/api/v1/client-errors` (fire-and-forget, deduplicated per session)

Key exports:
- `fetchParsedTranscript(sessionId, fileName, skipCache?)` -- Full transcript with metadata
- `fetchNewTranscriptMessages(sessionId, fileName, currentLineCount)` -- Incremental fetch
- `parseJSONL(jsonl)` -- Parse JSONL string into validated `TranscriptLine[]`

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
- `messageParser.test.ts` -- Message parsing for all message types, text extraction

## Dependencies

- `zod` (runtime response validation)
- `@/schemas/api` (response schemas and types)
- `@/schemas/transcript` (transcript line schemas)
- `@/utils/sessionErrors` (401 redirect skip list)
