# transcript/

Rendering components for transcript content. Claude Code components live at the
top level (code blocks, bash output, timeline navigation bars, the main content
dispatcher, and the `attachments/` renderers for side-channel `attachment.*`
rows + `system.away_summary` blurbs). Codex components live under `codex/` and
consume a separate render-item model produced by `services/codexTranscriptService`.

## Files

| File | Role |
|------|------|
| `ContentBlock.tsx` | Dispatcher that renders content blocks by type (text, thinking, tool_use, tool_result, image, unknown). Uses the shared `renderMarkdownToHtml` helper from `@/utils/markdown` |
| `CodeBlock.tsx` | Syntax-highlighted code with Prism.js, copy button, line truncation, and search highlighting |
| `BashOutput.tsx` | Terminal-style bash command output with error styling |
| `CostBar.tsx` | Vertical cost heatmap bar alongside the transcript (intensity = cost per API call) |
| `TimelineBar.tsx` | Vertical timeline bar showing user/assistant turn segments with duration tooltips |
| `timelineSegments.ts` | Shared segment computation and layout hook (`useSegmentLayout`) for both bars |
| `attachments/` | Renderers for `attachment.*` subtypes (hook output, edited files, queued commands, tool deltas) and `system.away_summary`. See `attachments/README.md` |
| `codex/` | Codex transcript renderers (user, assistant, tool call, turn separator, reasoning placeholder, compaction divider, unknown fallback, virtualized timeline). See "Codex transcript renderers" below |

## Key Components

### ContentBlock

The central dispatcher for rendering transcript content. Routes each block type to the appropriate renderer:

- **text** -- Renders markdown via `marked` + `DOMPurify`, or pretty-prints JSON if content is valid JSON
- **thinking** -- Collapsible thinking block with icon header
- **tool_use** -- Tool name header + JSON input rendered via `CodeBlock`
- **tool_result** -- Success/error indicator + content (dispatches to `BashOutput` for Bash tool results, `CodeBlock` for others, or recurses for nested content blocks)
- **image** -- Renders base64 or URL images
- **unknown** -- Forward-compatibility fallback with best-effort text extraction

All block types support optional `searchQuery` and `isCurrentSearchMatch` props for transcript search highlighting.

### CodeBlock

Syntax-highlighted code rendering with:
- **Prism.js** for syntax highlighting (bash, typescript, javascript, json, python, go, markdown, yaml, sql, css, html/xml)
- **Language alias mapping** (e.g., `ts` -> `typescript`, `sh` -> `bash`)
- **Line truncation** with "Show all" toggle (auto-expands when search query matches hidden content)
- **Copy to clipboard** button
- **Search highlighting** layered on top of syntax highlighting

### Codex transcript renderers (`codex/`)

The Codex JSONL has its own shape (`{ timestamp, type, payload }` envelopes
with nested `payload.type` discriminators) and gets a parallel set of
components. They consume the normalized `CodexRenderItem` union from
`@/types/codexRenderItem`, produced by `normalizeCodexLines()` in the service
layer.

| File | Role |
|------|------|
| `CodexMessageTimeline.tsx` | Virtualized timeline (TanStack Virtual). Dispatches each item to its renderer by `kind` |
| `CodexUserMessage.tsx` | Plain user prompt in a chat row, with timestamp |
| `CodexAssistantMessage.tsx` | Assistant text. Lighter styling + "(commentary)" label when `phase === 'commentary'`; model badge per message |
| `CodexToolCallBlock.tsx` | Paired tool call + output. Dispatches by `toolName` to `ExecCommandBody` (command + output + exit-code badge, 100-line soft cap), `ApplyPatchBody` (file-list summary + raw expand), `WebSearchBody` (query chips), or a generic `details`-based fallback |
| `CodexTurnSeparator.tsx` | `Turn N — duration · TTFT` divider between `task_complete` boundaries |
| `CodexReasoningHidden.tsx` | "(reasoning hidden)" marker for encrypted `reasoning` lines |
| `CodexCompactedDivider.tsx` | Divider for `compacted` rows, with the count of replaced messages |
| `CodexUnknownItem.tsx` | Forward-compat fallback. Click-to-expand `details` showing raw JSON for any line whose top-level `type` or nested `payload.type` doesn't match a known schema |
| `codexFormat.ts` | Shared formatters: `formatCodexTimestamp`, `formatDurationMs`, `leafFileName`, `stringifyForDisplay` |
| `CodexMessage.module.css` | Shared chat-row chrome for user / assistant messages |
| `CodexToolCallBlock.module.css` | Tool-call card chrome (header, status badge, command/output blocks, file list, query chips) |
| `CodexDividers.module.css` | Shared styles for turn separator, reasoning placeholder, compaction divider, unknown fallback |
| `CodexMessageTimeline.module.css` | Scroll container + virtualizer placement styles |

The Codex renderers intentionally do **not** participate in cost mode, filter
chips, or transcript search. CF-349 ships v1 as read-only display; analytics
(CF-350) and search will arrive in follow-ups.

### TimelineBar / CostBar

Vertical navigation bars displayed alongside the transcript:
- **TimelineBar** shows user (blue) and assistant (purple) turn segments sized by a blended time+message-count metric. Clicking a segment scrolls to those messages.
- **CostBar** shows cost density as green intensity (per-API-call cost, not total segment cost). Only visible in cost mode.
- Both share layout computation via `useSegmentLayout` to ensure identical segment sizing and position indicator placement.

## Key Types

```typescript
// From timelineSegments.ts
interface TimelineSegment {
  speaker: 'user' | 'assistant';
  durationMs: number;
  startIndex: number;   // Index into messages array
  endIndex: number;
  messageCount: number;
}

interface SegmentLayout {
  segments: TimelineSegment[];
  heightPercents: number[];     // Visual height per segment
  totalSize: number;
  indicatorPosition: number;   // Current position as percentage
  findSegmentForIndex: (messageIndex: number) => { segment; segmentIndex } | null;
}
```

## How to Extend

### Adding a new content block type
1. Add the block schema to `@/schemas/transcript.ts`
2. Add a type guard (e.g., `isNewBlock`) in the same file
3. Add a rendering branch in `ContentBlock.tsx` before the unknown-block fallback
4. Update the `KNOWN_BLOCK_TYPES` list in `transcript.ts` to suppress schema drift warnings

### Adding a new Codex tool renderer

1. Identify the new tool name as it appears in `function_call.name` /
   `custom_tool_call.name` on the JSONL line.
2. Add a body component alongside `ExecCommandBody` / `ApplyPatchBody` /
   `WebSearchBody` in `codex/CodexToolCallBlock.tsx`.
3. Add a `case` in `renderBody()` and an entry in `TOOL_NAME_LABELS` so the
   header shows a friendly name instead of the generic `"Tool: <name>"`.
4. If the tool needs a per-status badge (e.g., `exit N` for exec), extend
   `renderStatusBadge()` similarly.
5. If the tool emits a paired `event_msg.*` enrichment (like
   `event_msg.patch_apply_end` does for `apply_patch`), add a case in
   `handleEventMsg()` in `services/codexTranscriptService.ts` that merges
   into the existing draft via `callIdToDraft`.

### Adding a new Codex top-level line type

1. Add the schema branch to `@/schemas/codexTranscript.ts` and include it in
   `KnownCodexLineSchema`, `KNOWN_LINE_TYPES`, and the type predicate
   `isKnownCodexLine`.
2. Add a `case` in `normalizeCodexLines()`'s top-level switch to produce a
   render item (or extend an existing item type).
3. Add a new `CodexRenderItem` variant in `@/types/codexRenderItem.ts` if
   the new line needs its own render shape. The exhaustiveness check in
   `CodexMessageTimeline.renderItem()` will force you to wire up the
   renderer.

### Adding a new syntax language to CodeBlock
1. Add the Prism.js language import at the top of `CodeBlock.tsx`
2. Add any aliases to the `languageMap` object

## Invariants / Conventions

- **Forward compatibility**: The unknown-block fallback renders any block type that doesn't match a known schema. `warnIfKnownTypeCaughtByCatchall()` logs a console warning when a known type falls through (indicates schema drift).
- **ANSI stripping**: All text content is passed through `stripAnsi()` before rendering, since Claude Code transcripts may contain terminal escape codes.
- **Search highlighting is HTML-aware**: `highlightTextInHtml()` only wraps matches in text nodes, never inside HTML tags or attributes.
- **Segment sizing blends time and message count**: Pure time-based sizing would make short assistant turns invisible; the blend (60% time, 40% message count) ensures every segment is clickable.

## Design Decisions

- **Prism.js over alternatives**: Synchronous highlighting via `useMemo` (no layout shift). Languages loaded statically rather than dynamically to keep bundle predictable.
- **Shared segment layout**: `useSegmentLayout` is a custom hook rather than a utility function because it uses `useMemo` and `useCallback` internally. Both `TimelineBar` and `CostBar` consume it to guarantee identical segment boundaries.
- **Truncation with search auto-expand**: `CodeBlock` auto-expands truncated content when a search query matches only in the hidden portion, using React's "adjust state during render" pattern.

## Testing

Content block rendering is tested indirectly through `TimelineMessage.test.tsx` and Storybook stories (`ContentBlock.stories.tsx`, `CostBar.stories.tsx`, `TimelineBar.stories.tsx`).

## Dependencies

- `marked` + `dompurify` (markdown rendering and XSS sanitization, accessed via `@/utils/markdown.renderMarkdownToHtml`)
- `prismjs` (syntax highlighting in CodeBlock)
- `@tanstack/react-virtual` (CodexMessageTimeline virtualization, same as MessageTimeline)
- `@/hooks/useCopyToClipboard` (copy button in CodeBlock and BashOutput)
- `@/utils/highlightSearch` (search match highlighting)
- `@/utils/utils` (`stripAnsi` for terminal escape code removal; `isRecord` for the Codex shape readers)
- `@/utils/markdown` (`renderMarkdownToHtml` — shared by ContentBlock, AwaySummary, QueuedCommand)
- `@/utils/tokenStats` (`formatCost` in CostBar)
- `@/types/codexRenderItem` (Codex render-item discriminated union)
- `@/schemas/codexTranscript` (Codex Zod schemas)
