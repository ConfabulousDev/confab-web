import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { MemoryRouter } from 'react-router-dom';
import SessionHeader from './SessionHeader';
import FilterDropdown from './FilterDropdown';
import CodexFilterDropdown from './CodexFilterDropdown';
import type {
  MessageCategory,
  UserSubcategory,
  AssistantSubcategory,
  AttachmentSubcategory,
  HierarchicalCounts,
  FilterState,
} from './messageCategories';
import { DEFAULT_FILTER_STATE } from './messageCategories';
import type { GitInfo } from '@/types';

// Sample hierarchical counts
const sampleCounts: HierarchicalCounts = {
  user: { total: 194, prompt: 40, 'tool-result': 152, skill: 2 },
  assistant: { total: 271, text: 50, 'tool-use': 180, thinking: 41 },
  attachment: { total: 0, hook: 0, 'file-edit': 0, 'queued-command': 0, 'deferred-tools': 0, 'mcp-instructions': 0 },
  system: 0,
  'file-history-snapshot': 39,
  summary: 0,
  'queue-operation': 6,
  'pr-link': 0,
  'away-summary': 0,
  unknown: 0,
};

const sampleGitInfo: GitInfo = {
  repo_url: 'https://github.com/ConfabulousDev/confab',
  branch: 'main',
  commit_sha: 'abc123',
};

// Composes the Claude FilterDropdown locally and drives its state via React.
// Used by stories that want an interactive filter chip. Mirrors what
// SessionViewer does at runtime via the claude provider adapter.
function useClaudeFilterSlot(
  counts: HierarchicalCounts = sampleCounts,
  initial: FilterState = DEFAULT_FILTER_STATE,
) {
  const [filterState, setFilterState] = useState(initial);

  const handleToggleCategory = (category: MessageCategory) => {
    setFilterState((prev) => {
      const next = { ...prev };
      if (category === 'user') {
        const allVisible = prev.user.prompt && prev.user['tool-result'] && prev.user.skill;
        next.user = { prompt: !allVisible, 'tool-result': !allVisible, skill: !allVisible };
      } else if (category === 'assistant') {
        const allVisible = prev.assistant.text && prev.assistant['tool-use'] && prev.assistant.thinking;
        next.assistant = { text: !allVisible, 'tool-use': !allVisible, thinking: !allVisible };
      } else if (category === 'attachment') {
        const a = prev.attachment;
        const allVisible = a.hook && a['file-edit'] && a['queued-command'] && a['deferred-tools'] && a['mcp-instructions'];
        next.attachment = {
          hook: !allVisible,
          'file-edit': !allVisible,
          'queued-command': !allVisible,
          'deferred-tools': !allVisible,
          'mcp-instructions': !allVisible,
        };
      } else {
        next[category] = !prev[category];
      }
      return next;
    });
  };

  const handleToggleUserSubcategory = (sub: UserSubcategory) =>
    setFilterState((prev) => ({ ...prev, user: { ...prev.user, [sub]: !prev.user[sub] } }));
  const handleToggleAssistantSubcategory = (sub: AssistantSubcategory) =>
    setFilterState((prev) => ({ ...prev, assistant: { ...prev.assistant, [sub]: !prev.assistant[sub] } }));
  const handleToggleAttachmentSubcategory = (sub: AttachmentSubcategory) =>
    setFilterState((prev) => ({ ...prev, attachment: { ...prev.attachment, [sub]: !prev.attachment[sub] } }));

  return (
    <FilterDropdown
      counts={counts}
      filterState={filterState}
      onToggleCategory={handleToggleCategory}
      onToggleUserSubcategory={handleToggleUserSubcategory}
      onToggleAssistantSubcategory={handleToggleAssistantSubcategory}
      onToggleAttachmentSubcategory={handleToggleAttachmentSubcategory}
    />
  );
}

// Interactive wrapper that mirrors how SessionViewer composes the header:
// builds the FilterDropdown locally and forwards it as `filterSlot`.
function SessionHeaderInteractive(
  props: Omit<React.ComponentProps<typeof SessionHeader>, 'filterSlot'> & {
    counts?: HierarchicalCounts;
    initialFilterState?: FilterState;
  },
) {
  const { counts, initialFilterState, ...rest } = props;
  const filterSlot = useClaudeFilterSlot(counts, initialFilterState);
  return <SessionHeader {...rest} filterSlot={filterSlot} />;
}

const meta: Meta<typeof SessionHeaderInteractive> = {
  title: 'Session/SessionHeader',
  component: SessionHeaderInteractive,
  parameters: {
    layout: 'fullscreen',
  },
  // Every story inherits a Claude provider by default; the CodexSession
  // story below overrides it.
  args: {
    provider: 'claude-code',
  },
  decorators: [
    (Story) => (
      <MemoryRouter>
        <div style={{ background: 'var(--color-bg)', minHeight: '200px' }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof SessionHeaderInteractive>;

export const Default: Story = {
  args: {
    sessionId: 'session-123',
    title: 'CLI Refactoring: Summary Linking & macOS Binary Fix',
    hasCustomTitle: false,
    autoTitle: 'CLI Refactoring: Summary Linking & macOS Binary Fix',
    externalId: 'abc123def456',
    ownerEmail: 'developer@example.com',
    model: 'claude-opus-4-5-20251101',
    durationMs: 4980000,
    sessionDate: new Date('2025-12-06T22:09:00'),
    gitInfo: sampleGitInfo,
    isOwner: true,
    isShared: false,
    onShare: () => alert('Share clicked'),
    onDelete: () => alert('Delete clicked'),
    onSessionUpdate: (session) => console.log('Session updated:', session),
  },
};

export const SharedSession: Story = {
  args: {
    sessionId: 'session-456',
    title: 'Implementing Dark Mode Toggle',
    hasCustomTitle: false,
    autoTitle: 'Implementing Dark Mode Toggle',
    externalId: 'xyz789abc123',
    ownerEmail: 'alice@example.com',
    model: 'claude-sonnet-4-20250514',
    durationMs: 1800000,
    sessionDate: new Date('2025-12-05T14:30:00'),
    gitInfo: { repo_url: 'https://github.com/user/project', branch: 'feature/dark-mode' },
    isOwner: false,
    isShared: true,
    sharedByEmail: 'alice@example.com',
  },
};

export const SharedSessionWithoutEmail: Story = {
  args: {
    sessionId: 'session-456-legacy',
    title: 'Legacy Shared Session',
    hasCustomTitle: false,
    autoTitle: 'Legacy Shared Session',
    externalId: 'legacy789abc',
    ownerEmail: 'bob@example.com',
    model: 'claude-sonnet-4-20250514',
    durationMs: 1200000,
    sessionDate: new Date('2025-12-04T09:00:00'),
    gitInfo: { repo_url: 'https://github.com/user/project', branch: 'main' },
    isOwner: false,
    isShared: true,
    sharedByEmail: null,
  },
};

export const OwnerViewingShareLink: Story = {
  args: {
    sessionId: 'session-owner-share',
    title: 'API Authentication Implementation',
    hasCustomTitle: false,
    autoTitle: 'API Authentication Implementation',
    externalId: 'owner123share456',
    ownerEmail: 'developer@example.com',
    model: 'claude-opus-4-5-20251101',
    durationMs: 3600000,
    sessionDate: new Date('2025-12-06T10:00:00'),
    gitInfo: { repo_url: 'https://github.com/user/project', branch: 'feature/auth' },
    isOwner: true,
    isShared: true,
  },
};

export const NoGitInfo: Story = {
  args: {
    sessionId: 'session-789',
    title: 'Quick debugging session',
    hasCustomTitle: false,
    autoTitle: 'Quick debugging session',
    externalId: 'def456ghi789',
    ownerEmail: 'developer@example.com',
    model: 'claude-haiku-3-5-20241022',
    durationMs: 300000,
    sessionDate: new Date(),
    isOwner: true,
    isShared: false,
    onShare: () => alert('Share clicked'),
    onDelete: () => alert('Delete clicked'),
    onSessionUpdate: (session) => console.log('Session updated:', session),
  },
};

export const LongTitle: Story = {
  args: {
    sessionId: 'session-long',
    ownerEmail: 'developer@example.com',
    title:
      'This is a very long session title that might need to wrap or be truncated depending on the available space in the header component',
    hasCustomTitle: false,
    autoTitle:
      'This is a very long session title that might need to wrap or be truncated depending on the available space in the header component',
    externalId: 'long123title456',
    model: 'claude-opus-4-5-20251101',
    durationMs: 7200000,
    sessionDate: new Date('2025-12-01T09:00:00'),
    gitInfo: sampleGitInfo,
    isOwner: true,
    isShared: false,
    onShare: () => alert('Share clicked'),
    onDelete: () => alert('Delete clicked'),
    onSessionUpdate: (session) => console.log('Session updated:', session),
  },
};

// Codex provider: copy-ID dropdown reads "Copy Codex ID" with "for codex resume".
export const CodexSession: Story = {
  args: {
    sessionId: 'session-codex',
    title: 'Investigate Codex rollout schema for transcript parser',
    hasCustomTitle: false,
    autoTitle: 'Investigate Codex rollout schema for transcript parser',
    externalId: '019e23cc-fixture-codex-rollout',
    provider: 'codex',
    ownerEmail: 'developer@example.com',
    model: 'gpt-5',
    durationMs: 1800000,
    sessionDate: new Date('2026-05-13T01:00:00'),
    gitInfo: sampleGitInfo,
    isOwner: true,
    isShared: false,
    onShare: () => alert('Share clicked'),
    onDelete: () => alert('Delete clicked'),
    onSessionUpdate: (session) => console.log('Session updated:', session),
  },
};

export const FallbackTitle: Story = {
  args: {
    sessionId: 'session-fallback',
    hasCustomTitle: false,
    externalId: 'fallback123456789',
    ownerEmail: 'developer@example.com',
    model: 'claude-sonnet-4-20250514',
    sessionDate: new Date(),
    isOwner: true,
    isShared: false,
    onShare: () => alert('Share clicked'),
    onDelete: () => alert('Delete clicked'),
    onSessionUpdate: (session) => console.log('Session updated:', session),
  },
};

// Interactive cost mode toggle
function CostModeDemo() {
  const [isCostMode, setIsCostMode] = useState(false);
  const filterSlot = useClaudeFilterSlot();

  return (
    <SessionHeader
      sessionId="session-cost"
      title="Cost Mode Demo"
      hasCustomTitle={false}
      autoTitle="Cost Mode Demo"
      externalId="cost123"
      provider="claude-code"
      ownerEmail="developer@example.com"
      model="claude-opus-4-5-20251101"
      durationMs={3600000}
      sessionDate={new Date('2025-12-06T10:00:00')}
      gitInfo={sampleGitInfo}
      isOwner={true}
      isShared={false}
      onShare={() => alert('Share clicked')}
      onDelete={() => alert('Delete clicked')}
      onSessionUpdate={(session) => console.log('Session updated:', session)}
      isCostMode={isCostMode}
      onToggleCostMode={() => setIsCostMode((prev) => !prev)}
      filterSlot={filterSlot}
    />
  );
}

/**
 * Header with cost mode toggle button. Click $ to toggle.
 */
type DirectStory = StoryObj<typeof SessionHeader>;

export const WithCostMode: DirectStory = {
  render: () => <CostModeDemo />,
};

// Non-interactive story showing header without filter (Analytics tab view)
export const WithoutFilter: DirectStory = {
  render: () => (
    <SessionHeader
      sessionId="session-analytics"
      title="Viewing Analytics Tab"
      hasCustomTitle={false}
      autoTitle="Viewing Analytics Tab"
      externalId="analytics123"
      provider="claude-code"
      ownerEmail="developer@example.com"
      model="claude-opus-4-5-20251101"
      durationMs={3600000}
      sessionDate={new Date('2025-12-06T10:00:00')}
      gitInfo={sampleGitInfo}
      isOwner={true}
      isShared={false}
      onShare={() => alert('Share clicked')}
      onDelete={() => alert('Delete clicked')}
      onSessionUpdate={(session) => console.log('Session updated:', session)}
      // No filterSlot - simulates Analytics tab view
    />
  ),
};

// CF-382: Two headers stacked with identical metadata except provider + model
// so reviewers can visually diff the Anthropic (orange) and OpenAI (teal) brand
// glyphs side-by-side in the model meta-item.
export const ProviderIconComparison: DirectStory = {
  render: () => {
    const sharedProps = {
      title: 'Survey Codex subagent rollout integration points for CF-354',
      hasCustomTitle: false,
      autoTitle: 'Survey Codex subagent rollout integration points for CF-354',
      ownerEmail: 'developer@example.com',
      durationMs: 1800000,
      sessionDate: new Date('2026-05-14T21:00:00'),
      gitInfo: sampleGitInfo,
      isOwner: true,
      isShared: false,
    };
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
        <SessionHeader
          {...sharedProps}
          sessionId="session-claude"
          externalId="019e23cc-fixture-claude-session"
          provider="claude-code"
          model="claude-opus-4-7"
        />
        <SessionHeader
          {...sharedProps}
          sessionId="session-codex"
          externalId="019e23cc-fixture-codex-session"
          provider="codex"
          model="gpt-5-codex"
        />
      </div>
    );
  },
};

// CF-383: Codex header rendered without a model — exercises the
// providerLabel fallback (icon + "Codex" text). This is what the Summary tab
// shows while the rollout session_meta is loading, or permanently if the
// rollout's first line lacks a model field.
export const CodexNoModel: DirectStory = {
  render: () => (
    <SessionHeader
      sessionId="session-codex-no-model"
      title="Ranked refactoring ideas for Go CLI sync tool"
      hasCustomTitle={false}
      autoTitle="Ranked refactoring ideas for Go CLI sync tool"
      externalId="019e-fixture-codex-no-model"
      provider="codex"
      ownerEmail="developer@example.com"
      model={undefined}
      durationMs={104000}
      sessionDate={new Date('2026-05-14T20:43:00')}
      gitInfo={sampleGitInfo}
      isOwner={true}
      isShared={false}
    />
  ),
};

// CF-361: Codex provider with the new hierarchical filter chip wired up.
// Use this story to inspect the dropdown's visual treatment of the Codex
// category model (assistant.commentary/final, tool_call.*, reasoning_hidden,
// compacted, turn_separator).
function CodexWithFiltersDemo() {
  type CodexState = {
    user: boolean;
    assistant: { commentary: boolean; final: boolean };
    tool_call: { exec_command: boolean; apply_patch: boolean; web_search: boolean; generic: boolean };
    reasoning_hidden: boolean;
    compacted: boolean;
    turn_separator: boolean;
    turn_aborted: boolean;
    unknown: boolean;
  };
  const initial: CodexState = {
    user: true,
    assistant: { commentary: true, final: true },
    tool_call: { exec_command: true, apply_patch: true, web_search: true, generic: true },
    reasoning_hidden: false,
    compacted: true,
    turn_separator: true,
    turn_aborted: true,
    unknown: true,
  };
  const [filterState, setFilterState] = useState<CodexState>(initial);

  const counts = {
    user: 12,
    assistant: { total: 21, commentary: 9, final: 12 },
    tool_call: { total: 17, exec_command: 11, apply_patch: 3, web_search: 1, generic: 2 },
    reasoning_hidden: 7,
    compacted: 1,
    turn_separator: 12,
    turn_aborted: 0,
    unknown: 0,
  };

  const filterSlot = (
    <CodexFilterDropdown
      counts={counts}
      filterState={filterState}
      onToggleCategory={(c) => {
        setFilterState((prev) => {
          const next: CodexState = { ...prev };
          if (c === 'assistant') {
            const all = prev.assistant.commentary && prev.assistant.final;
            next.assistant = { commentary: !all, final: !all };
          } else if (c === 'tool_call') {
            const tc = prev.tool_call;
            const all = tc.exec_command && tc.apply_patch && tc.web_search && tc.generic;
            next.tool_call = { exec_command: !all, apply_patch: !all, web_search: !all, generic: !all };
          } else {
            next[c] = !prev[c];
          }
          return next;
        });
      }}
      onToggleAssistantSubcategory={(sub) =>
        setFilterState((prev) => ({
          ...prev,
          assistant: { ...prev.assistant, [sub]: !prev.assistant[sub] },
        }))
      }
      onToggleToolCallSubcategory={(sub) =>
        setFilterState((prev) => ({
          ...prev,
          tool_call: { ...prev.tool_call, [sub]: !prev.tool_call[sub] },
        }))
      }
    />
  );

  return (
    <SessionHeader
      sessionId="session-codex-filters"
      title="Investigate Codex rollout schema for transcript parser"
      hasCustomTitle={false}
      autoTitle="Investigate Codex rollout schema for transcript parser"
      externalId="019e23cc-fixture-codex-rollout"
      provider="codex"
      ownerEmail="developer@example.com"
      model="gpt-5"
      durationMs={1800000}
      sessionDate={new Date('2026-05-13T01:00:00')}
      gitInfo={sampleGitInfo}
      isOwner={true}
      isShared={false}
      filterSlot={filterSlot}
    />
  );
}

export const CodexWithFilters: DirectStory = {
  render: () => <CodexWithFiltersDemo />,
};
