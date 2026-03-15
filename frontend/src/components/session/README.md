# session/

Session viewer components for displaying session details, transcript timeline, analytics summary, and message filtering.

## Files

| File | Role |
|------|------|
| `SessionViewer.tsx` | Top-level session viewer with Summary/Transcript tab switching |
| `SessionHeader.tsx` | Session header with title, metadata, share/delete actions, and filter controls |
| `SessionSummaryPanel.tsx` | Summary tab: renders analytics cards via card registry, GitHub links, smart recap actions |
| `MessageTimeline.tsx` | Transcript tab: virtualized message list with search, timeline bar, cost bar |
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
  system: boolean;
  'file-history-snapshot': boolean;
  summary: boolean;
  'queue-operation': boolean;
  'pr-link': boolean;
  unknown: boolean;
}
```

## Key Components

- **SessionViewer** -- Orchestrates the entire session view. Supports controlled and uncontrolled tab modes. Loads transcript, polls for new messages (15s interval), and manages filter/cost-mode state.
- **SessionSummaryPanel** -- Polls analytics via `useAnalyticsPolling`, renders ordered cards from the card registry, and provides smart recap regeneration.
- **MessageTimeline** -- Uses `@tanstack/react-virtual` for virtualized rendering of potentially thousands of messages. Integrates `TranscriptSearchBar`, `TimelineBar`, and `CostBar`.
- **FilterDropdown** -- Hierarchical filter with top-level categories (user, assistant, system) and subcategories (prompt, tool-result, skill, text, tool-use, thinking).

## How to Extend

### Adding a new message category filter
1. Add the category to `MessageCategory` type in `messageCategories.ts`
2. Add default visibility to `DEFAULT_FILTER_STATE`
3. Update `classifyMessage()` and `messageMatchesFilter()`
4. Add the filter chip to `FilterDropdown.tsx`

### Adding session header metadata
Add a new `MetaItem` component in `SessionHeader.tsx` with the appropriate icon.

## Invariants / Conventions

- **Transcript polling**: New messages are fetched incrementally using `line_offset` to avoid re-downloading the entire transcript. The `lineCountRef` tracks total JSONL lines (not parsed messages) to stay in sync with the backend.
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
- `@/services/transcriptService` (fetch/parse transcripts)
- `@/components/transcript/` (TimelineBar, CostBar)
- `./cards/` (analytics card components and registry)
