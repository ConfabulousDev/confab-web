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
| `timelineFormat.ts` | `formatDuration` — shared `1h 15m` / `5m 30s` / `500ms` formatter for both Claude and Codex tooltip prose |
| `timelineSegments.ts` | Claude segment compute (`useSegmentLayout`) + generic `useBlendedSegmentLayout` hook (size + position math, also consumed by Codex's bar) |
| `timelineUtils.ts` | Provider-neutral helpers shared by Claude & Codex timelines: `formatTimeSeparator` (>5min idle-gap label), `retryOnAnimationFrame` (virtualizer scroll positioning), and `addCmdFListener` (Cmd/Ctrl+F intercept that opens the search bar — CF-359) |
| `attachments/` | Renderers for `attachment.*` subtypes (hook output, edited files, queued commands, tool deltas) and `system.away_summary`. See `attachments/README.md` |
| `codex/` | Codex transcript renderers (user, assistant, tool call, turn separator, reasoning placeholder, compaction divider, unknown fallback, virtualized timeline, turn-based timeline bar). See "Codex transcript renderers" below |

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
| `CodexMessageTimeline.tsx` | Virtualized timeline (TanStack Virtual). Owns selection state (`firstVisibleIndex`, `selectedIndex`), scroll buttons, the timeline bar, and time-separator injection. Dispatches each item to its renderer by `kind`, threading `isSelected` / `isNewSpeaker` / `isDeepLinkTarget` (CF-360) plus per-row `sessionId` and `onSkipToNext` / `onSkipToPrevious` callbacks (CF-360). Accepts a `targetLineId` prop for `?msg=<lineId>` deep linking and a `targetLineIdHidden` placeholder reserved for CF-361 filter-reset wiring |
| `CodexTimelineBar.tsx` | Vertical navigation rail. Each turn emits up to two clickable segments — a user thinking-gap stripe and an assistant body stripe — matching the Claude `TimelineBar` color language. Click-to-seek, hover tooltips (`User: 30s, 1 item` / `Codex: 5m, 12 items`), and a position indicator driven by the parent's `selectedIndex`. Consumes `useCodexSegmentLayout` |
| `codexTimelineSegments.ts` | `CodexSpeaker`, `CodexTimelineSegment`, `computeCodexSegments`, `useCodexSegmentLayout`. Per turn, emits a user segment sized by the wall-clock gap from the prior separator (synthetic 1s for turn 1 / zero-or-negative gaps) and an assistant body segment for the rest of the slice. Slices with no user item (compaction-only) collapse to one assistant segment. Layout math shared via `useBlendedSegmentLayout` |
| `codexVirtualItems.ts` | Pure `buildVirtualItems(items)` that injects `>5min` time separators and tags each item with `isNewSpeaker`. The speaker rule is Codex-specific: `tool_call` / `reasoning_hidden` / `turn_separator` / `compacted` / `unknown` items do NOT break user/assistant continuity, so `user → tool_call → user` is the same speaker. Also exports `skipNavKey` / `skipNavLabel` (CF-360) — the chain-key + aria-label mapping used for skip-to-next-of-same-kind navigation |
| `extractCodexItemText.ts` | CF-359 per-kind text projection (`(item: CodexRenderItem) => string`) consumed by the generic `useTranscriptSearch` hook. Returns user/assistant `text`, exec `cmd` + output, apply_patch diff + file paths, web_search queries, generic rawInput + rawOutput, the visible `compacted` label, and stringified `unknown` raw JSON. `turn_separator` and `reasoning_hidden` return `""` so they never match |
| `CodexRowActions.tsx` | CF-360 per-row chrome shared by all seven renderers. Renders, into the header-right slot, `[prev-skip?] [next-skip?] [copy-text?] [copy-link]`. Copy-link is always rendered; copy-text is hidden when `copyText` is empty/whitespace; skip buttons are hidden when their callbacks are absent. Copy-link URL format: `${origin}/sessions/${sessionId}?tab=transcript&msg=${lineId}` (matches Claude exactly) |
| `CodexUserMessage.tsx` | User prompt in a chat row, with timestamp. Body renders through `CodexMessageBody`. Accepts `isSelected` + `isNewSpeaker` for selection ring and speaker-change spacing, plus `isDeepLinkTarget`, `sessionId`, `onSkipToNext`, `onSkipToPrevious`, and `kindLabel` for the CF-360 row chrome |
| `CodexAssistantMessage.tsx` | Assistant text. Lighter styling + "(commentary)" label when `phase === 'commentary'`; model badge per message. Body renders through `CodexMessageBody`. Accepts the same chrome props as `CodexUserMessage` |
| `CodexMessageBody.tsx` | Shared rendering path for user + assistant text. JSON-shaped text pretty-prints via `CodeBlock` (`language="json"`); everything else flows through `renderMarkdownToHtml`. Mirrors `ContentBlock.tsx`'s text-block contract |
| `CodexMessageImages.tsx` | CF-388 shared image gallery for user + assistant messages. Renders one `<img loading="lazy">` per entry in `images`, parameterized by `altPrefix` (`User-attached image` vs `Assistant-generated image`). Same dimension caps as `ContentBlock`'s image render for cross-provider visual parity |
| `CodexToolCallBlock.tsx` | Paired tool call + output. Dispatches by `toolName` to `ExecCommandBody` (body-level `$ cmd` + `BashOutput` with terminal styling), `ApplyPatchBody` (file-list summary + `CodeBlock` `language="diff"`), `WebSearchBody` (query chips), or a generic body that renders rawInput/rawOutput as expanded `CodeBlock`s. Accepts the chrome props (`sessionId`, `isDeepLinkTarget`, `onSkipToNext` / `onSkipToPrevious`, `kindLabel`); `isNewSpeaker` is no-op per the Codex speaker rule |
| `codexToolCallHelpers.ts` | Pure helpers extracted from `CodexToolCallBlock.tsx` for testability: `buildToolCallCopyText` (per-tool copy-text composition for `CodexRowActions`) plus shape readers `readStringField`, `readPatchChanges`, `readWebSearchQueries`, `stringifyGenericInput` |
| `CodexTurnSeparator.tsx` | `Turn N — duration · TTFT` divider between `task_complete` boundaries. Accepts `isSelected` + `isDeepLinkTarget` + `sessionId`; the CF-360 row chrome renders a copy-link-only button strip |
| `CodexReasoningHidden.tsx` | "(reasoning hidden)" marker for encrypted `reasoning` lines. Accepts `isSelected` + `isDeepLinkTarget` + `sessionId`; copy-link-only chrome |
| `CodexCompactedDivider.tsx` | Divider for `compacted` rows, with the count of replaced messages. Accepts `isSelected` + `isDeepLinkTarget` + `sessionId`; copy-link-only chrome |
| `CodexUnknownItem.tsx` | Forward-compat fallback. Click-to-expand `details` showing raw JSON for any line whose top-level `type` or nested `payload.type` doesn't match a known schema. Accepts `isSelected` + `isDeepLinkTarget` + `sessionId`; copy-text uses `stringifyForDisplay(rawLine)` so users can dump the unknown payload |
| `codexFormat.ts` | Shared formatters: `formatCodexTimestamp`, `formatDurationMs`, `leafFileName`, `stringifyForDisplay` |
| `CodexMessage.module.css` | Shared chat-row chrome for user / assistant messages. Defines `.selected` (inset ring + tint), `.newSpeaker` (extra top margin), `.deepLinkTarget` (composes `deepLinkPulse` from `@/styles/animations.module.css`; accent ring overrides the grey selection ring on hover), and `.searchMatch` (amber ring — CF-359; source-ordered after `.selected` / `.deepLinkTarget` so it wins via the cascade) |
| `CodexToolCallBlock.module.css` | Tool-call card chrome (header, status badge, command/output blocks, file list, query chips). Defines `.selected`, `.deepLinkTarget`, and `.searchMatch` (CF-359) |
| `CodexDividers.module.css` | Shared styles for turn separator, reasoning placeholder, compaction divider, unknown fallback. Defines `.selected`, `.deepLinkTarget`, and `.searchMatch` (CF-359) |
| `CodexRowActions.module.css` | CF-360 button-strip chrome (icon button states, copy-success colour) |
| `CodexMessageTimeline.module.css` | Container (flex with bar on the right), virtualizer slot positioning, scroll chrome, time-separator divider |
| `CodexTimelineBar.module.css` | Vertical-bar chrome: segments, position indicator, hover tooltip |

CF-349 shipped v1 as read-only display. CF-358 brought rendering parity
(markdown, Prism, terminal output, diff highlighting) in line with Claude.
CF-357 added the navigation chrome — turn-based timeline bar, scroll-to-top/
bottom buttons, row hover → selection state, and `>5min` idle-gap time
separators. CF-360 added stable per-row `lineId`, `?msg=<lineId>` deep
linking (polling-aware), and per-row chrome — copy text, copy link, and
skip-to-next/prev same-kind — for user / assistant / tool_call rows; the
four divider variants get copy-link only. CF-388 added inline rendering of
`input_image` / `output_image` content blocks: image_url values surface on
`CodexUserItem.images` / `CodexAssistantItem.images` and render below the
message body, with Codex's own `<image name=[Image #N]>` / `</image>`
sentinel wrappers stripped from sibling `input_text` blocks. CF-359
landed Cmd-F transcript search: the generic `useTranscriptSearch` hook
is parameterized over a `(item) => string` extractor
(`extractCodexItemText` for Codex, `extractMessageText` for Claude), every
Codex renderer accepts `searchQuery` / `isCurrentSearchMatch` props,
matches highlight inline via `highlightTextInHtml` (markdown / raw JSON)
or `renderTextWithHighlight` (command line, file paths, query chips,
divider labels), `CodexUnknownItem` auto-opens its `<details>` when a
match lands inside, and `CodeBlock`'s search-driven auto-expand fires
on truncated `apply_patch` / generic tool blocks (now opt-in via
`truncateLines={100}`). Still deferred: filter chips (CF-361), and
cost mode (CF-362).

**Known gaps (deferred — see TODOs at the referenced sites):**
- Plaintext `reasoning` is not rendered — every reasoning line emits a
  `reasoning_hidden` item today. When plaintext arrives, extend
  `CodexReasoningHiddenItem` with a `{ decoded: true; text }` discriminator
  and render a 💭 collapsible block mirroring `ContentBlock`'s thinking
  treatment.
- Tool-call image outputs (`function_call_output.output` JSON carrying
  embedded `image_url`) are not surfaced. CF-388 covered message-content
  images only; a screenshot tool that returns a base64 image inside its
  unstructured `output` string needs its own decoder pass and is filed as a
  follow-up.

### TimelineBar / CostBar / CodexTimelineBar

Vertical navigation bars displayed alongside the transcript:
- **TimelineBar** (Claude) shows user (blue) and assistant (warm orange) turn segments derived from user-prompt boundaries, sized by a blended time+message-count metric. Clicking a segment scrolls to those messages.
- **CodexTimelineBar** (Codex) renders the same user (blue) + assistant (warm orange) palette per turn. The user segment is sized by the wall-clock gap from the prior separator to the current user prompt (synthetic 1s for turn 1 or zero-gap turns); the assistant segment is sized by the separator's `durationMs`. Tooltip wording is `User: <dur>, 1 item` / `Codex: <dur>, N items` — no TTFT, no turn index. Same sizing math and position indicator as Claude.
- **CostBar** (Claude only) shows cost density as green intensity (per-API-call cost, not total segment cost). Only visible in cost mode.
- All three share layout computation via `useBlendedSegmentLayout` (in `timelineSegments.ts`) to guarantee identical sizing and position-indicator placement. Each provider supplies its own segment-derivation function (`useSegmentLayout` / `useCodexSegmentLayout`).

## Key Types

```typescript
// From timelineSegments.ts — generic shape consumed by useBlendedSegmentLayout
interface BlendedSegment {
  durationMs: number;
  startIndex: number;
  endIndex: number;
  messageCount: number;
}

// Claude-specific (extends BlendedSegment)
interface TimelineSegment extends BlendedSegment {
  speaker: 'user' | 'assistant';
}

// Codex-specific (extends BlendedSegment) — from codex/codexTimelineSegments.ts
interface CodexTimelineSegment extends BlendedSegment {
  speaker: 'user' | 'assistant';
  turnIndex: number;
}

interface BlendedSegmentLayout<S extends BlendedSegment> {
  segments: S[];
  heightPercents: number[];     // Visual height per segment
  totalSize: number;
  indicatorPosition: number;   // Current position as percentage
  findSegmentForIndex: (messageIndex: number) => { segment: S; segmentIndex: number } | null;
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
- `prismjs` (syntax highlighting in CodeBlock; includes the `diff` grammar for `apply_patch` raw views and the standard set of language grammars)
- `@tanstack/react-virtual` (CodexMessageTimeline virtualization, same as MessageTimeline)
- `@/hooks/useCopyToClipboard` (copy button in CodeBlock and BashOutput)
- `@/utils/highlightSearch` (search match highlighting)
- `@/utils/utils` (`stripAnsi` for terminal escape code removal; `isRecord` for the Codex shape readers)
- `@/utils/markdown` (`renderMarkdownToHtml` — shared by ContentBlock, the Codex message renderers, AwaySummary, QueuedCommand; `tryParseAsJson` — shared JSON-fallback helper used before markdown rendering)
- `@/styles/markdown.module.css` (typography for rendered markdown — composed into `ContentBlock.module.css .textBlock` and `CodexMessage.module.css .markdown`)
- `@/utils/tokenStats` (`formatCost` in CostBar)
- `@/types/codexRenderItem` (Codex render-item discriminated union)
- `@/schemas/codexTranscript` (Codex Zod schemas)
