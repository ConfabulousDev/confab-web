# transcript/

Rendering components for Claude Code transcript content: code blocks with syntax highlighting, bash output, timeline navigation bars, and the main content block dispatcher.

## Files

| File | Role |
|------|------|
| `ContentBlock.tsx` | Dispatcher that renders content blocks by type (text, thinking, tool_use, tool_result, image, unknown) |
| `CodeBlock.tsx` | Syntax-highlighted code with Prism.js, copy button, line truncation, and search highlighting |
| `BashOutput.tsx` | Terminal-style bash command output with error styling |
| `CostBar.tsx` | Vertical cost heatmap bar alongside the transcript (intensity = cost per API call) |
| `TimelineBar.tsx` | Vertical timeline bar showing user/assistant turn segments with duration tooltips |
| `timelineSegments.ts` | Shared segment computation and layout hook (`useSegmentLayout`) for both bars |

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

- `marked` + `dompurify` (markdown rendering and XSS sanitization in ContentBlock)
- `prismjs` (syntax highlighting in CodeBlock)
- `@/hooks/useCopyToClipboard` (copy button in CodeBlock and BashOutput)
- `@/utils/highlightSearch` (search match highlighting)
- `@/utils/utils` (`stripAnsi` for terminal escape code removal)
- `@/utils/tokenStats` (`formatCost` in CostBar)
