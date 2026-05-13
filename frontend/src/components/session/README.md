# session/

Session viewer components for displaying session details, transcript timeline, analytics summary, and message filtering.

## Files

| File | Role |
|------|------|
| `SessionViewer.tsx` | Top-level session viewer with Summary/Transcript tab switching. Branches on `session.provider` (`claude-code` vs `codex`) to pick the right pane and to hide filter chips / cost toggle for Codex |
| `SessionHeader.tsx` | Session header with title, metadata, share/delete actions, and filter controls |
| `SessionSummaryPanel.tsx` | Summary tab for Claude Code sessions: renders analytics cards via card registry, GitHub links, smart recap actions |
| `CodexSummaryEmpty.tsx` | Summary tab for Codex sessions: placeholder explaining analytics aren't computed yet (CF-350) and pointing users to the Transcript tab |
| `ClaudeTranscriptPane.tsx` | Transcript tab for Claude Code: thin wrapper around `MessageTimeline` that handles loading / error states (parent owns filter + cost state so the header can render chips) |
| `CodexTranscriptPane.tsx` | Transcript tab for Codex: self-contained fetch + 15s poll + normalize → render `CodexMessageTimeline`. Mirrors the Claude line-offset polling pattern |
| `MessageTimeline.tsx` | Claude transcript: virtualized message list with search, timeline bar, cost bar |
| `TimelineMessage.tsx` | Single message row in the timeline (role badge, content blocks, cost, copy link) |
| `TranscriptSearchBar.tsx` | Cmd+F search bar with match count and prev/next navigation |
| `FilterDropdown.tsx` | Hierarchical dropdown for filtering messages by category/subcategory |
| `GitHubLinksCard.tsx` | Card displaying linked GitHub PRs and commits |
| `TILBadge.tsx` | Badge indicating a session has associated TIL entries |
| `GitInfoMeta.tsx` | Git branch/commit metadata display in session header |
| `MetaItem.tsx` | Small metadata item (icon + label + value) used in header |
| `messageCategories.ts` | Message categorization logic, filter state types, and filter matching |
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
```

## Key Components

- **SessionViewer** -- Orchestrates the entire session view. Supports controlled and uncontrolled tab modes. For Claude sessions: loads transcript, polls for new messages (15s interval), and manages filter/cost-mode state. For Codex sessions: skips the Claude-specific fetch/poll/TILs/filter UI; the `CodexTranscriptPane` owns its own data path. Provider detection uses `isCodexProvider(session.provider)`, which matches the canonical `'codex'` value from CF-347.
- **ClaudeTranscriptPane** -- Stateless wrapper around `MessageTimeline` that handles the loading / error / timeline branching for Claude sessions. Filter and cost-mode state live in `SessionViewer` so the header can render the chips and toggle alongside the timeline.
- **CodexTranscriptPane** -- Self-contained transcript view for Codex sessions. Owns its raw-line state, fetch (`fetchParsedCodexTranscript`), 15s poll (`fetchNewCodexLines`), and re-derives render items via `useMemo`. Hands the result to `CodexMessageTimeline`.
- **CodexSummaryEmpty** -- Placeholder rendered in the Summary tab for Codex sessions. Codex analytics aren't computed yet (CF-350); this stops the tab from looking broken.
- **SessionSummaryPanel** -- Polls analytics via `useAnalyticsPolling`, renders ordered cards from the card registry, and provides smart recap regeneration.
- **MessageTimeline** -- Uses `@tanstack/react-virtual` for virtualized rendering of potentially thousands of messages. Integrates `TranscriptSearchBar`, `TimelineBar`, and `CostBar`.
- **FilterDropdown** -- Hierarchical filter with three top-level categories with subcategories (user, assistant, attachment) plus flat chips for system, away-summary, file-history-snapshot, summary, queue-operation, and pr-link. The attachment chip groups hook output, file edits, queued commands, deferred tools, and mcp instructions. Default state: only user + assistant + unknown are visible; everything else is opt-in.

## How to Extend

### Adding a new message category filter
1. Add the category to `MessageCategory` type in `messageCategories.ts`
2. Add default visibility to `DEFAULT_FILTER_STATE`
3. Update `countHierarchicalCategories()` and `messageMatchesFilter()`
4. Add the filter chip to `FilterDropdown.tsx`
5. Add the new path to `SUB_KEYS` / `FLAT_KEYS` and the `stateFromPaths` / `pathsFromState` round-trip in `@/hooks/useTranscriptFilters.ts` (so the chip persists in the `?hide=` URL param)
6. If the new category needs a custom body renderer (like attachments or away-summary), wire a dispatch branch in `TimelineMessage.tsx`'s content render block

### Adding session header metadata
Add a new `MetaItem` component in `SessionHeader.tsx` with the appropriate icon.

## Invariants / Conventions

- **Transcript polling**: New messages are fetched incrementally using `line_offset` to avoid re-downloading the entire transcript. The `lineCountRef` tracks total JSONL lines (not parsed messages) to stay in sync with the backend. Both `ClaudeTranscriptPane` (via `SessionViewer`) and `CodexTranscriptPane` use the same pattern; the latter just re-derives its render items via `useMemo` after appending raw lines.
- **Provider branching**: `SessionViewer` dispatches on `session.provider`. `'codex'` → `CodexTranscriptPane` + `CodexSummaryEmpty`. Anything else (including the legacy `'Claude Code'` value backfilled by the API) → `ClaudeTranscriptPane` + `SessionSummaryPanel`.
- **Storybook bypass**: `SessionViewer` and `SessionSummaryPanel` accept `initial*` props to skip API calls in Storybook stories.
- **Deep linking**: When `targetMessageUuid` is set but the target message is hidden by filters, filters are automatically reset to make it visible.

## Design Decisions

- **Virtualized timeline**: Messages are rendered with `@tanstack/react-virtual` because sessions can have thousands of transcript lines. Each message estimates its height based on content type.
- **Controlled/uncontrolled tabs**: `SessionViewer` supports both patterns so `SessionDetailPage` can control the tab (e.g., switching to transcript for deep links) while Storybook stories can use uncontrolled mode.
- **Filter state is hierarchical**: User and assistant categories have subcategories because a single transcript line can be a "user prompt" vs "user tool-result" vs "user skill expansion", and users need fine-grained control.

## Testing

- `SessionHeader.test.tsx` -- Title display, edit mode, metadata rendering
- `SessionSummaryPanel.test.tsx` -- Card rendering, analytics polling integration
- `TimelineMessage.test.tsx` -- Message rendering by role, cost display
- `TranscriptSearchBar.test.tsx` -- Search open/close, match navigation
- `messageCategories.test.ts` -- Message classification and filter matching logic

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
