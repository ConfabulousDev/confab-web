# schemas/

Zod validation schemas and the TypeScript type system for the Confab frontend. All external data (API responses, transcript files) is validated at runtime through these schemas.

## Files

| File | Role |
|------|------|
| `api.ts` | Zod schemas for all API responses: sessions, analytics, trends, org, auth, shares, GitHub links |
| `transcript.ts` | Zod schemas for Claude Code transcript JSONL: message types, content blocks, token usage |
| `validation.ts` | Zod schemas for form input validation: share forms, API key creation, email validation |

## Key Types

### api.ts -- API Response Types

All types are inferred from Zod schemas via `z.infer<>`:

**Session types:**
- `Session` -- List item (id, title, git info, cost, access type)
- `SessionDetail` -- Full detail (adds files, cwd, hostname, git_info)
- `SessionListResponse` -- Paginated list with `filter_options`
- `SessionFilterOptions` -- Available filter values (repos, branches, owners)

**Analytics types:**
- `SessionAnalytics` -- Full analytics response with cards, errors, quota
- `AnalyticsCards` -- Map of card key to card data (all optional)
- Card data types: `TokensCardData`, `SessionCardData`, `ConversationCardData`, `CodeActivityCardData`, `ToolsCardData`, `AgentsAndSkillsCardData`, `RedactionsCardData`, `SmartRecapCardData`
- `SmartRecapQuotaInfo` -- Quota tracking for AI recap generation
- `AnnotatedItem` -- List item with optional message deep-link reference

**Trends types:**
- `TrendsResponse` -- Aggregated analytics with date range and repo filters
- Card types: `TrendsOverviewCard`, `TrendsTokensCard`, `TrendsActivityCard`, `TrendsToolsCard`, `TrendsUtilizationCard`, `TrendsAgentsAndSkillsCard`, `TrendsTopSessionsCard`

**Org types:**
- `OrgAnalyticsResponse` -- Per-user analytics with date range
- `OrgUserAnalytics` -- Individual user metrics

**Other types:**
- `User` -- Current user (email, name, avatar)
- `SessionShare`, `APIKey`, `GitHubLink`, `GitInfo`

### transcript.ts -- Transcript Types

**Content blocks** (discriminated union on `type`):
- `TextBlock` -- `{ type: 'text', text: string }`
- `ThinkingBlock` -- `{ type: 'thinking', thinking: string }`
- `ToolUseBlock` -- `{ type: 'tool_use', id, name, input }`
- Tool result block -- `{ type: 'tool_result', tool_use_id, content: string | ContentBlock[], is_error? }`
- `ImageBlock` -- `{ type: 'image', source: { type, media_type, data?, url? } }`
- `UnknownBlock` -- Forward-compatibility catch-all

**Message types** (discriminated union on `type`):
- `UserMessage` -- User prompts and tool results
- `AssistantMessage` -- Claude responses with token usage
- `SystemMessage` -- System events (compact_boundary, api_error, turn_duration, etc.)
- `SummaryMessage`, `PRLinkMessage`, `QueueOperationMessage`, `FileHistorySnapshot`
- Unknown message -- Forward-compatibility catch-all

**Utility functions:**
- Type guards: `isUserMessage()`, `isAssistantMessage()`, `isTextBlock()`, `isToolUseBlock()`, etc.
- Content helpers: `hasThinking()`, `usesTools()`, `isToolResultMessage()`, `isSkillExpansionMessage()`
- Command expansion: `isCommandExpansionMessage()`, `getCommandExpansionSkillName()`, `stripCommandExpansionTags()`
- Validation: `validateParsedTranscriptLine()`, `parseTranscriptLineWithError()`
- Schema drift detection: `warnIfKnownTypeCaughtByCatchall()`

### validation.ts -- Form Validation

- `emailSchema` -- Trimmed, non-empty, valid email, max 255 chars
- `shareFormSchema` -- Share form with public/private mode, recipients, expiration (1-365 days)
- `apiKeyNameSchema` -- Alphanumeric + spaces/hyphens/underscores, max 100 chars
- `createAPIKeySchema` -- Wraps `apiKeyNameSchema`
- `validateForm()` -- Generic schema validator returning typed success/error result
- `getFieldError()` -- Extract first error message for a field

## How to Extend

### Adding a new API response type
1. Define the Zod schema in `api.ts`
2. Export the schema (for use in `api.ts` service) and the inferred type (for consumers)
3. Add to the appropriate parent schema if it's part of a larger response (e.g., add to `AnalyticsCardsSchema`)

### Adding a new transcript message type
1. Define the message schema in `transcript.ts`
2. Add it to the `TranscriptLineSchema` union **before** `UnknownMessageSchema`
3. Add the type string to `KNOWN_MESSAGE_TYPES`
4. Export a type guard function
5. Update `messageParser.ts` to handle the new type

### Adding a new form validation schema
1. Define the Zod schema in `validation.ts`
2. Export both the schema and the inferred `z.infer<>` type
3. Use `validateForm()` in the consuming hook/component

## Invariants / Conventions

- **Schemas validate external data**: Schemas are designed to validate data we don't control (API responses, transcript files). They use `.passthrough()` and `z.string()` (instead of `z.enum()`) for forward compatibility with new field values.
- **Union ordering matters**: In discriminated unions (`TranscriptLineSchema`, `ContentBlockSchema`), the catch-all (`UnknownMessageSchema`, `UnknownBlockSchema`) must be last. Zod tries branches in order and returns the first match.
- **Type guards re-exported from `@/types`**: The `src/types/index.ts` file re-exports all type guards and types from `schemas/transcript.ts`. Components import from `@/types`, not directly from schemas.
- **Schema drift detection**: `warnIfKnownTypeCaughtByCatchall()` logs a console warning when a message with a known `type` string falls through to the catch-all schema, indicating the specific schema needs updating.
- **`AnnotatedItem` backward compatibility**: Accepts both plain strings (legacy) and `{ text, message_id? }` objects, normalizing strings to objects via `.transform()`.

## Design Decisions

- **Zod over TypeScript-only types**: Runtime validation catches backend contract changes immediately rather than letting corrupt data flow through the app silently. Every API call and transcript line is validated.
- **Forward-compatible schemas**: New message types and content block types from future Claude Code versions won't crash the app. Unknown types render with fallback UI and trigger console warnings for developer visibility.
- **Inferred types**: Types are derived from schemas via `z.infer<>` rather than defined separately. This eliminates the possibility of types and schemas drifting apart.
- **Separated validation concerns**: `api.ts` validates server responses, `transcript.ts` validates external JSONL data, `validation.ts` validates user input. Different trust levels require different schema styles.

## Testing

- `transcript.test.ts` -- Transcript line parsing, validation error formatting, type guards
- `validation.test.ts` -- Share form validation, email validation, API key name validation

## Dependencies

- `zod` (schema definition and validation)
