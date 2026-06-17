# session/

Session viewer components for displaying session details, transcript timeline, analytics summary, and message filtering.

## Files

| File | Role |
|------|------|
| `SessionViewer.tsx` | Top-level session viewer with Summary/Transcript tab switching. Provider-agnostic (CF-417): dispatches transcript fetch / filter / rendering through a `ProviderAdapter` resolved via `getAdapter(session.provider)` (see `frontend/src/providers/`). Owns the `isCostMode` toggle and composes the active adapter's `ClaudeFilterDropdown` into `SessionHeader`'s `filterSlot`. The Summary tab is shared (`SessionSummaryPanel`, CF-364) |
| `SessionHeader.tsx` | Session header with title, metadata, share/delete actions, and a `filterSlot?: ReactNode` rendered into the actions row. SessionHeader is now provider-agnostic — the slot is filled by SessionViewer with the active adapter's filter dropdown (CF-417) |
| `SessionSummaryPanel.tsx` | Summary tab for both providers: renders analytics cards via card registry, GitHub links, smart recap actions. When `!isTokensMeasurable(provider)` (Cursor — st5f), shows a compact info callout above the card grid explaining unavailable token/cost/timing metrics. Codex sessions get cards from `ComputeFromCodexRollout` (CF-350); the on-demand cache-miss path calls the same adapter synchronously (CF-364). The hard "Failed to load analytics" state surfaces an actionable detail line via `describeAnalyticsError` (cd3z): an `APIValidationError` (Zod mismatch on a 200 body) reports the failing fields, an `APIError` reports the HTTP status, any other `Error` shows its message |
| `cursorAnalyticsFixture.ts` | Cursor analytics fixture for the `CursorSession` Storybook story and the cd3z schema regression test. Mirrors `ComputeFromCursorRollout` / the analytics wire shape: zero tokens + `"0"` cost (no usage in Cursor JSONL), no `tokens_v2` card (empty `by_provider` omitted from the wire), null durations (no timestamps), Cursor's own tool names, no `smart_recap`, and `session.models_used: []` (empty array, never null — Cursor has no per-line model in v1; y0kc). Sibling of `codexAnalyticsFixture.ts` |
| `ClaudeTranscriptPane.tsx` | Transcript tab for Claude Code: thin wrapper around `transcript/claude/ClaudeMessageTimeline` that handles loading / error states (parent owns filter + cost state so the header can render chips) |
| `CodexTranscriptPane.tsx` | Transcript tab for Codex: presentational. Receives `items` (unfiltered, drives bar segments), `filteredItems` (drives the row list), `visibleIndices` (per CF-361, drives bar greying), `loading`, `error`, and `isCostMode` (CF-362) from `SessionViewer`. Accepts `targetLineId` (CF-360, the `?msg=` URL param reinterpreted as a stable `lineId` for Codex), forwarded unchanged to the timeline |
| `OpenCodeTranscriptPane.tsx` | Transcript tab for OpenCode: presentational, virtualized list of user / assistant / tool / unknown render items with the shared `TimelineBar` minimap (ag2x), the green `CostBar` rail in cost mode (hfk7), and Cmd-F in-transcript search (5p9j). Search reuses the shared `useTranscriptSearch` toolkit driven by `extractOpenCodeItemText`; since OpenCode has no divider rows, the filtered index IS the virtual index (no Codex-style `itemIndexToVirtualIndex` map). The current-match effect scrolls the virtualizer to the match and brings its `<mark>` into view (finds matches in unmounted rows), and force-opens any collapsed reasoning / tool-output `<details>` holding the active match |
| `extractOpenCodeItemText.ts` | 5p9j per-kind text projection (`(item: OpenCodeRenderItem) => string`) consumed by the generic `useTranscriptSearch` hook. Module-level / stable reference so the hook's search index doesn't churn. Returns user `text`, assistant `reasoning` + `text`, tool `input` + `output`, and the unknown row's stringified raw line (via `stringifyUnknownRaw`, shared with `OpenCodeUnknownItem` so the index matches the rendered `<pre>`) |
| `CursorTranscriptPane.tsx` | Transcript tab for Cursor (18n2): presentational, virtualized list of user / assistant / tool render items with Cmd-F in-transcript search (shared `useTranscriptSearch` driven by `extractCursorItemText`), deep-link scroll-to, and the shared `transcript/TimelineBar` minimap (zztp). Leaner than the other panes in one way — **no** cost rail (no token/cost data). Row headers show an ESTIMATED per-row time (ce79): the pane runs `attachCursorTimestamps` over the full item stream using the `firstSeen`/`lastSyncAt` bounds threaded through the adapter, looks each row's time up by id, and renders it with a muted `~` prefix and a tooltip ("Estimated — Cursor transcripts have no per-message timestamps."); rows show nothing when bounds are unknown. The same stamped stream feeds `useCursorSegmentLayout` (`cursorTimelineSegments.ts`) for the minimap, so the bar self-hides when the bounds are unknown (empty layout) and its tooltip carries an `tooltipNote="Estimated times"` disclaimer; `visibleIndices` (derived from `items` vs `filteredItems`, CF-361) greys fully-filtered segments, and clicking a segment scrolls the virtualizer to the first visible row of that turn (`assistantLabel="Assistant"`, matching OpenCode). The user row renders the extracted prompt through `CursorMessageBody` (markdown, pt81) then renders any injected-context `sections` via `CursorContextSections` (0rcv); the assistant row renders its narrative text through `CursorMessageBody` too. Tool rows show the call (name + one-line input summary) as monospace `<pre>` with **no output** (Cursor records inputs only) — tool rows and context-section bodies stay plain text, only narrative rows get markdown. Every row's header carries the shared `transcript/RowActions` cluster (a9gr): copy-text (raw row payload via `buildCursorRowCopyText`, hidden when empty), copy-link (`?tab=transcript&msg=<item.id>` — the synthetic stable id, NOT the estimated timestamp), and same-kind prev/next skip nav (`buildCursorRowNav` over `filteredItems`, scrolling the virtualizer; buttons hide at chain ends). The pane derives its own greying from `items` vs `filteredItems`, so the adapter wrapper still accepts but ignores the contract's `visibleIndices` / `isCostMode` |
| `cursorTimelineSegments.ts` | zztp: `CursorTimelineSegment` (= the shared `SpeakerSegment`), `computeCursorSegments`, `useCursorSegmentLayout`. Mirrors `opencodeTimelineSegments.ts` — synthesizes turns from `user`-item transitions (Cursor has no separator rows), one user thinking-gap stripe + one folded assistant body stripe per turn, leading non-user items collapsing to one assistant segment. Unlike OpenCode's epoch-ms `timeCreated`, Cursor's per-row `timestamp` is an OPTIONAL ISO-8601 **string** (estimated, ce79), so durations come from `Date.parse` deltas; per ce79's "no bogus times" stance, if ANY item lacks a parseable timestamp the function returns `[]` → the shared `TimelineBar` self-hides. Layout math shared via `useBlendedSegmentLayout` |
| `CursorMessageBody.tsx` | pt81: shared rendering path for Cursor user-prompt + assistant **narrative** rows. Thin provider-named wrapper that delegates to the shared markdown + highlight utils (`renderMarkdownToHtml`, `tryParseAsJson`, `highlightTextInHtml`, `getHighlightClass`) — no re-implemented markdown/highlight logic. Mirrors `CodexMessageBody`'s JSON-or-markdown fallback: JSON-shaped text pretty-prints as a `CodeBlock`, everything else flows through the GFM markdown pipeline (bold / `###` headers / pipe tables / links), sanitized by DOMPurify. Cmd-F matches are wrapped in `<mark>` inside the rendered HTML (not on the raw markdown source) so scroll-to-`<mark>` keeps working. Text arrives pre-cleaned upstream (fa3h strips `[REDACTED]`; nfbe extracts `<user_query>`). Not used for tool rows or context-section bodies |
| `CursorContextSections.tsx` | 0rcv: renders a Cursor user row's injected-context `sections` (nfbe `CursorUserSection[]` — user rules, attached files, manually attached skills, system reminders, …) as native `<details>` disclosures, collapsed by default, one per parsed section in wire order (no grouping in v1). Returns null when there are no sections (the common case). Section bodies are plain preformatted text — rich rendering of attached-file contents (syntax highlight, image preview) is an explicit follow-up. Section content is opaque (a stray `<user_query>` inside renders as a literal text node, never a real element) |
| `cursorRowNav.ts` | a9gr pure helpers for the Cursor row-action cluster: `buildCursorRowNav` (same-kind prev/next skip-nav neighbor maps keyed by `filteredItems` index — user→user, assistant→assistant, tool→tool, flat kind set, no per-tool split), `cursorRowKindLabel` (aria-label/title), and `buildCursorRowCopyText` (raw copy payload — user/assistant `text`, tool `input`; `undefined` when empty so `RowActions` hides the button). Extracted from `CursorTranscriptPane` so the math is unit-testable without the virtualizer |
| `extractCursorItemText.ts` | Per-kind text projection (`(item: CursorRenderItem) => string`) for `useTranscriptSearch`. Module-level / stable reference. Returns user/assistant `text` (for user rows this is the extracted `<user_query>` prompt — the searchable text matches what renders, not the stripped envelope; nfbe) and tool `toolName` + `input` (no tool output exists) |
| `TranscriptSearchBar.tsx` | Cmd+F search bar with match count and prev/next navigation. Shared by the Claude, Codex, OpenCode, and Cursor transcript panes |
| `ProviderFilterDropdown.tsx` | Generic transcript filter dropdown (ew9f): owns all rendering — filter button, hierarchical groups (expand/collapse + tri-state parent) and flat chips — from a declarative `{ groups, flatItems }`. The per-provider dropdowns are thin wrappers that assemble chips and render this once |
| `filterChips.ts` | Shared `FilterChip`/`FilterChipGroup` types + `getColorValue` (SidebarItemColor → hex) for `ProviderFilterDropdown`; split out so the component file only exports a component (react-refresh) |
| `ClaudeFilterDropdown.tsx` | Thin wrapper: assembles Claude chips (3 hierarchical groups + flat) for `ProviderFilterDropdown` |
| `CodexFilterDropdown.tsx` | Thin wrapper: assembles Codex chips (CF-361 — `assistant`/`tool_call` hierarchical + 6 flat) for `ProviderFilterDropdown` |
| `OpenCodeFilterDropdown.tsx` | Thin wrapper: assembles OpenCode flat chips (`user`/`assistant`/`tool`/`unknown`) for `ProviderFilterDropdown` |
| `CursorFilterDropdown.tsx` | Thin wrapper: assembles Cursor flat chips (`user`/`assistant`/`tool`) for `ProviderFilterDropdown` |
| `FilterDropdownShared.module.css` | CSS chrome for `ProviderFilterDropdown` (shared across all providers) |
| `GitHubLinksCard.tsx` | Card displaying linked GitHub PRs and commits |
| `GitInfoMeta.tsx` | Git branch/commit metadata display in session header |
| `MetaItem.tsx` | Small metadata item (icon + label + value) used in header |
| `claudeCategories.ts` | Claude message categorization logic, filter state types, and filter matching. CF-574: `isUnknownClaudeMessage(message)` — precise predicate for the catch-all "unknown" bucket (excludes known-but-unlabeled types like `pr-link` that `getClaudeRoleLabel` also renders as "Unknown"); gates the transcript "Report bug" affordance. Exports the `isAwaySummaryMessage` / `isInformationalMessage` type guards (each narrows to `SystemMessage`) that `ClaudeTimelineMessage` dispatches custom bodies off. CF-419: `system.informational` rows get a `Notice` role label but stay bucketed under the `system` filter chip |
| `codexCategories.ts` | Codex render-item categorization (CF-361): `CodexFilterState`, `CodexHierarchicalCounts`, `categorizeCodexToolCall`, `countCodexCategories`, `codexItemMatchesFilter`, `DEFAULT_CODEX_FILTER_STATE` |
| `cursorCategories.ts` | Cursor render-item types + flat categorization (18n2): `CursorRenderItem` (`user`/`assistant`/`tool`, each with an optional `timestamp?` — an ESTIMATED ISO 8601 time interpolated frontend-side by `attachCursorTimestamps`, never from the wire, ce79), `CursorUserSection` (nfbe: one injected-context block parsed out of a user envelope — `{tag,label,content}` — carried on the user item's optional `sections` for the collapsible-context UI, 0rcv), `CursorFilterState`, `CursorHierarchicalCounts`, `countCursorCategories`, `cursorItemMatchesFilter`, `DEFAULT_CURSOR_FILTER_STATE`. No `unknown` kind — both wire shapes are fully handled |
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
  targetId?: string;             // Deep-link target (provider-opaque: Claude UUID, Codex lineId)
  initialMessages?: TranscriptLine[];     // Storybook bypass
  initialAnalytics?: SessionAnalytics;    // Storybook bypass
  initialGithubLinks?: GitHubLink[];      // Storybook bypass
}

interface ClaudeFilterState {
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

// CF-361 (extended in CF-368 with turn_aborted)
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
  turn_aborted: boolean;  // CF-368
  unknown: boolean;
}
```

## Key Components

- **SessionViewer** -- Orchestrates the entire session view. Supports controlled and uncontrolled tab modes. Provider-agnostic since CF-417: looks up an adapter via `getAdapter(session.provider)` and delegates transcript fetch + polling (`useTranscriptData`), filter state (`adapter.useFilters`), filter matching (`adapter.itemMatchesFilter`), deep-link reset (`adapter.useDeepLinkFilterReset`), model extraction (`adapter.extractModel`), session-meta computation (`adapter.computeMeta`), filter dropdown rendering (`<adapter.ClaudeFilterDropdown>`), and pane rendering (`<adapter.TranscriptPane>`). The Summary tab routes to `SessionSummaryPanel` for both providers (CF-364). All per-provider branching lives in `frontend/src/providers/` — `SessionViewer.tsx` has zero `isCodex` references.
- **ClaudeTranscriptPane** -- Stateless wrapper around `transcript/claude/ClaudeMessageTimeline` that handles the loading / error / timeline branching for Claude sessions. Filter and cost-mode state live in `SessionViewer` so the header can render the chips and toggle alongside the timeline.
- **CodexTranscriptPane** -- Presentational since CF-386. Receives `rawLines`, `loading`, `error`, and (CF-362) `isCostMode` from `SessionViewer` and re-derives render items via `useMemo`. Mirrors `ClaudeTranscriptPane`'s stateless shape so both providers have a single canonical owner (`SessionViewer`) for transcript data.
- **SessionSummaryPanel** -- Polls analytics via `useAnalyticsPolling`, renders ordered cards from the card registry, and provides smart recap regeneration. Provider-agnostic — Codex sessions display the same cards as Claude, with provider-specific shape captured in the backend adapter (`gpt-5` model strings, `cache_creation=0`, `files_read=0`, etc.).
- **ClaudeMessageTimeline** (in `transcript/claude/`) -- Uses `@tanstack/react-virtual` for virtualized rendering of potentially thousands of messages. Integrates `TranscriptSearchBar`, `TimelineBar`, and `CostBar`.
- **ClaudeFilterDropdown** -- Hierarchical filter with three top-level categories with subcategories (user, assistant, attachment) plus flat chips for system, away-summary, file-history-snapshot, summary, queue-operation, and pr-link. The attachment chip groups hook output, file edits, queued commands, deferred tools, and mcp instructions. Default state: only user + assistant + unknown are visible; everything else is opt-in.
- **CodexFilterDropdown** (CF-361) -- Codex parallel of `ClaudeFilterDropdown`. Two hierarchical parents (`assistant` with `commentary`/`final`, `tool_call` with `exec_command`/`apply_patch`/`web_search`/`generic`) plus six flat chips (`user`, `reasoning_hidden`, `compacted`, `turn_separator`, `turn_aborted` — added in CF-368 for the aborted-turn divider — `unknown`). Default state visible for everything except `reasoning_hidden` (opt-in). Imports `FilterDropdownShared.module.css` for visual parity with the Claude dropdown.

## How to Extend

### Adding a new message category filter (Claude)
1. Add the category to `ClaudeCategory` type in `claudeCategories.ts`
2. Add default visibility to `DEFAULT_CLAUDE_FILTER_STATE`
3. Update `countClaudeCategories()` and `claudeItemMatchesFilter()`
4. Add the filter chip to `ClaudeFilterDropdown.tsx`
5. Add the new path to `SUB_KEYS` / `FLAT_KEYS` and the `stateFromPaths` / `pathsFromState` round-trip in `@/hooks/useClaudeTranscriptFilters.ts` (so the chip persists in the `?hide=` URL param)
6. If the new category needs a custom body renderer (like attachments or away-summary), wire a dispatch branch in `transcript/claude/ClaudeTimelineMessage.tsx`'s content render block

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
- **Provider branching**: `SessionViewer` dispatches on `session.provider` for the Transcript pane only — `'codex'` → `CodexTranscriptPane`, anything else (including the legacy `'Claude Code'` value backfilled by the API) → `ClaudeTranscriptPane`. The Summary tab uses `SessionSummaryPanel` for both providers (CF-364), backed by Codex analytics from `ComputeFromCodexRollout` (CF-350).
- **Storybook bypass**: `SessionViewer` and `SessionSummaryPanel` accept `initial*` props to skip API calls in Storybook stories.
- **Deep linking**: When `targetId` is set but the target message is hidden by filters, filters are automatically reset to make it visible. The shell-level prop is provider-opaque (CF-367): Claude interprets it as a message UUID, Codex as a stable `lineId` (CF-360). `SessionViewer` forwards it to the active provider's adapter, which passes it through to the pane's provider-internal prop (`targetMessageUuid` for Claude, `targetLineId` for Codex). CF-361 wired the Codex parallel of the auto-reset: if the target's category is currently hidden, `setCodexFilterState({ ...DEFAULT_CODEX_FILTER_STATE, reasoning_hidden: target.kind === 'reasoning_hidden' }, { replace: true })` runs so the target becomes visible (the post-default override matters only for `reasoning_hidden` targets, since that's the only default-hidden Codex category).
- **URL filter grammar**: Claude and Codex filter hooks share the `?hide=` URL slot with provider-specific token grammars. Foreign tokens (e.g. `attachment.hook` on a Codex session) are silently ignored on read; a write from the Codex hook drops them. Cross-provider URL navigation degrades gracefully.

## Design Decisions

- **Virtualized timeline**: Messages are rendered with `@tanstack/react-virtual` because sessions can have thousands of transcript lines. Each message estimates its height based on content type.
- **Controlled/uncontrolled tabs**: `SessionViewer` supports both patterns so `SessionDetailPage` can control the tab (e.g., switching to transcript for deep links) while Storybook stories can use uncontrolled mode.
- **Filter state is hierarchical**: User and assistant categories have subcategories because a single transcript line can be a "user prompt" vs "user tool-result" vs "user skill expansion", and users need fine-grained control.

## Testing

- `SessionHeader.test.tsx` -- Title display, edit mode, metadata rendering
- `SessionSummaryPanel.test.tsx` -- Card rendering, analytics polling integration
- `SessionViewer.test.tsx` -- Summary-tab routing across providers (CF-364)
- `TranscriptSearchBar.test.tsx` -- Search open/close, match navigation
- `ProviderFilterDropdown.test.tsx` -- Generic dropdown: flat/disabled chips, tri-state parent, expand, parent + chip callback wiring, active-button state
- `ClaudeFilterDropdown.test.tsx` -- Open/close, tri-state rollup, subcategory expand, callback wiring (through the generic)
- `CodexFilterDropdown.test.tsx` -- Same surface, tuned to Codex categories
- `claudeCategories.test.ts` -- Message classification and filter matching logic
- `codexCategories.test.ts` -- Codex categorization rules + `codexItemMatchesFilter` contract (CF-361)
- `CodexTranscriptPane.test.tsx` -- Loading/error/empty prop contract after CF-361 lifted normalization to `SessionViewer`

## Dependencies

- `@tanstack/react-virtual` (ClaudeMessageTimeline virtualization)
- `@/hooks/useAnalyticsPolling` (analytics data)
- `@/hooks/useTranscriptSearch` (transcript search)
- `@/hooks/useVisibility` (pause polling when tab hidden)
- `@/services/claudeTranscriptService` (Claude fetch/parse)
- `@/services/codexTranscriptService` (Codex fetch/parse/normalize)
- `@/components/transcript/` (TimelineBar, CostBar)
- `@/components/transcript/codex/` (Codex render components)
- `./cards/` (analytics card components and registry)
