# session/

Session viewer components for displaying session details, transcript timeline, analytics summary, and message filtering.

## Files

| File | Role |
|------|------|
| `SessionViewer.tsx` | Top-level session viewer with Summary/Transcript tab switching. Branches the Transcript pane on `session.provider` (`claude-code` vs `codex`). Owns Claude messages + Codex render items + both filter states + the `isCostMode` toggle (provider-agnostic since CF-362); renders provider-appropriate filter chips on the Transcript tab (CF-361). The Summary tab is provider-agnostic — both providers render `SessionSummaryPanel` (CF-364) |
| `SessionHeader.tsx` | Session header with title, metadata, share/delete actions, and provider-aware filter controls (Claude `FilterDropdown` or Codex `CodexFilterDropdown`, CF-361) |
| `SessionSummaryPanel.tsx` | Summary tab for both providers: renders analytics cards via card registry, GitHub links, smart recap actions. Codex sessions get cards from `ComputeFromCodexRollout` (CF-350); the on-demand cache-miss path calls the same adapter synchronously (CF-364) |
| `ClaudeTranscriptPane.tsx` | Transcript tab for Claude Code: thin wrapper around `MessageTimeline` that handles loading / error states (parent owns filter + cost state so the header can render chips) |
| `CodexTranscriptPane.tsx` | Transcript tab for Codex: presentational. Receives `items` (unfiltered, drives bar segments), `filteredItems` (drives the row list), `visibleIndices` (per CF-361, drives bar greying), `loading`, `error`, and `isCostMode` (CF-362) from `SessionViewer`. Accepts `targetLineId` (CF-360, the `?msg=` URL param reinterpreted as a stable `lineId` for Codex), forwarded unchanged to the timeline |
| `MessageTimeline.tsx` | Claude transcript: virtualized message list with search, timeline bar, cost bar |
| `TimelineMessage.tsx` | Single message row in the timeline (role badge, content blocks, cost, copy link) |
| `TranscriptSearchBar.tsx` | Cmd+F search bar with match count and prev/next navigation |
| `FilterDropdown.tsx` | Hierarchical dropdown for filtering Claude messages by category/subcategory. Imports `FilterDropdownShared.module.css` (shared chrome with `CodexFilterDropdown`) |
| `CodexFilterDropdown.tsx` | Hierarchical dropdown for filtering Codex transcripts (CF-361). Two hierarchical parents (`assistant`, `tool_call`) plus five flat categories (`user`, `reasoning_hidden`, `compacted`, `turn_separator`, `unknown`). Same visual chrome as Claude's dropdown via shared CSS module |
| `FilterDropdownShared.module.css` | CSS chrome shared by `FilterDropdown` (Claude) and `CodexFilterDropdown` (Codex) |
| `GitHubLinksCard.tsx` | Card displaying linked GitHub PRs and commits |
| `TILBadge.tsx` | Badge indicating a session has associated TIL entries |
| `GitInfoMeta.tsx` | Git branch/commit metadata display in session header |
| `MetaItem.tsx` | Small metadata item (icon + label + value) used in header |
| `messageCategories.ts` | Claude message categorization logic, filter state types, and filter matching |
| `codexCategories.ts` | Codex render-item categorization (CF-361): `CodexFilterState`, `CodexHierarchicalCounts`, `categorizeCodexToolCall`, `countCodexCategories`, `codexItemMatchesFilter`, `DEFAULT_CODEX_FILTER_STATE` |
| `index.ts` | Barrel export: `SessionViewer` component and `ViewTab` type |

## Key Types

```typescript
type ViewTab = 'summary' | 'transcript';

interface SessionViewerProps {
  session: SessionDetail;
  onShare?: () => void;
  onDelete?: () => void;
  onSessionUpdate?: (session: SessionDetail) => void;
  isOwner?: boolean;
  isShared?: boolean;
  activeTab?: ViewTab;           // Controlled mode
  onTabChange?: (tab: ViewTab) => void;
  targetMessageUuid?: string;    // Deep-link to specific message
  initialMessages?: TranscriptLine[];     // Storybook bypass
  initialAnalytics?: SessionAnalytics;    // Storybook bypass
  initialGithubLinks?: GitHubLink[];      // Storybook bypass
}

interface FilterState {
  user: { prompt: boolean; 'tool-result': boolean; skill: boolean };
  assistant: { text: boolean; 'tool-use': boolean; thinking: boolean };
  attachment: {
    hook: boolean;
    'file-edit': boolean;
    'queued-command': boolean;
    'deferred-tools': boolean;
    'mcp-instructions': boolean;
  };
  system: boolean;
  'file-history-snapshot': boolean;
  summary: boolean;
  'queue-operation': boolean;
  'pr-link': boolean;
  'away-summary': boolean;
  unknown: boolean;
}

// CF-361
interface CodexFilterState {
  user: boolean;
  assistant: { commentary: boolean; final: boolean };
  tool_call: {
    exec_command: boolean;
    apply_patch: boolean;
    web_search: boolean;
    generic: boolean;
  };
  reasoning_hidden: boolean;
  compacted: boolean;
  turn_separator: boolean;
  unknown: boolean;
}
```

## Key Components

- **SessionViewer** -- Orchestrates the entire session view. Supports controlled and uncontrolled tab modes. Owns parsed transcript state for BOTH providers (CF-386): Claude `messages` via `fetchParsedTranscript` / `fetchNewTranscriptMessages`, Codex `rawLines` via `fetchParsedCodexTranscript` / `fetchNewCodexLines`. A single provider-aware `loadTranscript()` and a single provider-aware poll useEffect branch on `isCodex`. The Summary tab routes to `SessionSummaryPanel` for both providers (CF-364); the Transcript tab routes to `ClaudeTranscriptPane` (Claude) or `CodexTranscriptPane` (Codex) — both are presentational and receive their data via props. Session header model is derived via `firstAssistant.message.model` (Claude) or `extractCodexModel(rawLines)` (Codex). Provider detection uses `isCodexProvider(session.provider)`, which matches the canonical `'codex'` value from CF-347.
- **ClaudeTranscriptPane** -- Stateless wrapper around `MessageTimeline` that handles the loading / error / timeline branching for Claude sessions. Filter and cost-mode state live in `SessionViewer` so the header can render the chips and toggle alongside the timeline.
- **CodexTranscriptPane** -- Presentational since CF-386. Receives `rawLines`, `loading`, `error`, and (CF-362) `isCostMode` from `SessionViewer` and re-derives render items via `useMemo`. Mirrors `ClaudeTranscriptPane`'s stateless shape so both providers have a single canonical owner (`SessionViewer`) for transcript data.
- **SessionSummaryPanel** -- Polls analytics via `useAnalyticsPolling`, renders ordered cards from the card registry, and provides smart recap regeneration. Provider-agnostic — Codex sessions display the same cards as Claude, with provider-specific shape captured in the backend adapter (`gpt-5` model strings, `cache_creation=0`, `files_read=0`, etc.).
- **MessageTimeline** -- Uses `@tanstack/react-virtual` for virtualized rendering of potentially thousands of messages. Integrates `TranscriptSearchBar`, `TimelineBar`, and `CostBar`.
- **FilterDropdown** -- Hierarchical filter with three top-level categories with subcategories (user, assistant, attachment) plus flat chips for system, away-summary, file-history-snapshot, summary, queue-operation, and pr-link. The attachment chip groups hook output, file edits, queued commands, deferred tools, and mcp instructions. Default state: only user + assistant + unknown are visible; everything else is opt-in.
- **CodexFilterDropdown** (CF-361) -- Codex parallel of `FilterDropdown`. Two hierarchical parents (`assistant` with `commentary`/`final`, `tool_call` with `exec_command`/`apply_patch`/`web_search`/`generic`) plus five flat chips (`user`, `reasoning_hidden`, `compacted`, `turn_separator`, `unknown`). Default state visible for everything except `reasoning_hidden` (opt-in). Imports `FilterDropdownShared.module.css` for visual parity with the Claude dropdown.

## How to Extend

### Adding a new message category filter (Claude)
1. Add the category to `MessageCategory` type in `messageCategories.ts`
2. Add default visibility to `DEFAULT_FILTER_STATE`
3. Update `countHierarchicalCategories()` and `messageMatchesFilter()`
4. Add the filter chip to `FilterDropdown.tsx`
5. Add the new path to `SUB_KEYS` / `FLAT_KEYS` and the `stateFromPaths` / `pathsFromState` round-trip in `@/hooks/useTranscriptFilters.ts` (so the chip persists in the `?hide=` URL param)
6. If the new category needs a custom body renderer (like attachments or away-summary), wire a dispatch branch in `TimelineMessage.tsx`'s content render block

### Adding a new Codex category filter (CF-361)
1. Add the new key to `CodexCategory` (or extend an existing sub union) in `codexCategories.ts`
2. Add default visibility to `DEFAULT_CODEX_FILTER_STATE`
3. Update `countCodexCategories()` and `codexItemMatchesFilter()`. If it's a `tool_call` sub, also update `categorizeCodexToolCall()` — that switch is the single source of truth both functions route through
4. Add the filter row to `CodexFilterDropdown.tsx`
5. Add the new path to `ASSISTANT_SUBS` / `TOOL_CALL_SUBS` / `FLAT_KEYS` and the `stateFromPaths` / `pathsFromState` round-trip in `@/hooks/useCodexTranscriptFilters.ts`
6. If the new category needs custom row chrome, wire a dispatch branch in `CodexMessageTimeline.tsx`'s `renderItem` switch

### Adding session header metadata
Add a new `MetaItem` component in `SessionHeader.tsx` with the appropriate icon.

## Invariants / Conventions

- **Transcript polling**: New transcript lines are fetched incrementally using `line_offset` to avoid re-downloading the entire transcript. The `lineCountRef` tracks total JSONL lines (not parsed messages) to stay in sync with the backend. Since CF-386 a single provider-aware poll useEffect in `SessionViewer` covers both Claude (via `fetchNewTranscriptMessages`) and Codex (via `fetchNewCodexLines`).
- **Provider branching**: `SessionViewer` dispatches on `session.provider` for the Transcript pane only — `'codex'` → `CodexTranscriptPane`, anything else (including the legacy `'Claude Code'` value backfilled by the API) → `ClaudeTranscriptPane`. The Summary tab uses `SessionSummaryPanel` for both providers (CF-364), backed by Codex analytics from `ComputeFromCodexRollout` (CF-350). TIL badges and smart-recap deep-links remain Claude-only because both anchor to message UUIDs that Codex messages don't carry.
- **Storybook bypass**: `SessionViewer` and `SessionSummaryPanel` accept `initial*` props to skip API calls in Storybook stories.
- **Deep linking**: When `targetMessageUuid` is set but the target message is hidden by filters, filters are automatically reset to make it visible. The same `?msg=` URL param doubles as a Codex deep-link target (CF-360): `SessionViewer` passes it to `CodexTranscriptPane` as `targetLineId`, which the Codex timeline interprets as a stable `rawLines` index. CF-361 wired the Codex parallel of the auto-reset: if the target's category is currently hidden, `setCodexFilterState({ ...DEFAULT_CODEX_FILTER_STATE, reasoning_hidden: target.kind === 'reasoning_hidden' }, { replace: true })` runs so the target becomes visible (the post-default override matters only for `reasoning_hidden` targets, since that's the only default-hidden Codex category).
- **URL filter grammar**: Claude and Codex filter hooks share the `?hide=` URL slot with provider-specific token grammars. Foreign tokens (e.g. `attachment.hook` on a Codex session) are silently ignored on read; a write from the Codex hook drops them. Cross-provider URL navigation degrades gracefully.

## Design Decisions

- **Virtualized timeline**: Messages are rendered with `@tanstack/react-virtual` because sessions can have thousands of transcript lines. Each message estimates its height based on content type.
- **Controlled/uncontrolled tabs**: `SessionViewer` supports both patterns so `SessionDetailPage` can control the tab (e.g., switching to transcript for deep links) while Storybook stories can use uncontrolled mode.
- **Filter state is hierarchical**: User and assistant categories have subcategories because a single transcript line can be a "user prompt" vs "user tool-result" vs "user skill expansion", and users need fine-grained control.

## Testing

- `SessionHeader.test.tsx` -- Title display, edit mode, metadata rendering
- `SessionSummaryPanel.test.tsx` -- Card rendering, analytics polling integration
- `SessionViewer.test.tsx` -- Summary-tab routing across providers (CF-364)
- `TimelineMessage.test.tsx` -- Message rendering by role, cost display
- `TranscriptSearchBar.test.tsx` -- Search open/close, match navigation
- `FilterDropdown.test.tsx` -- Open/close, tri-state rollup, subcategory expand, callback wiring
- `CodexFilterDropdown.test.tsx` -- Same surface, tuned to Codex categories
- `TILBadge.test.tsx` -- Label pluralization, popover open + link to /tils, click-propagation guard
- `messageCategories.test.ts` -- Message classification and filter matching logic
- `codexCategories.test.ts` -- Codex categorization rules + `codexItemMatchesFilter` contract (CF-361)
- `CodexTranscriptPane.test.tsx` -- Loading/error/empty prop contract after CF-361 lifted normalization to `SessionViewer`

## Dependencies

- `@tanstack/react-virtual` (MessageTimeline virtualization)
- `@/hooks/useAnalyticsPolling` (analytics data)
- `@/hooks/useTranscriptSearch` (transcript search)
- `@/hooks/useVisibility` (pause polling when tab hidden)
- `@/services/transcriptService` (Claude fetch/parse)
- `@/services/codexTranscriptService` (Codex fetch/parse/normalize)
- `@/components/transcript/` (TimelineBar, CostBar)
- `@/components/transcript/codex/` (Codex render components)
- `./cards/` (analytics card components and registry)
