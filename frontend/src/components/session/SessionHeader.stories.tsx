import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { MemoryRouter } from 'react-router-dom';
import SessionHeader from './SessionHeader';
import type {
  MessageCategory,
  UserSubcategory,
  AssistantSubcategory,
  HierarchicalCounts,
  FilterState,
} from './messageCategories';
import { DEFAULT_FILTER_STATE } from './messageCategories';
import type { GitInfo } from '@/types';

// Sample hierarchical counts
const sampleCounts: HierarchicalCounts = {
  user: { total: 194, prompt: 40, 'tool-result': 152, skill: 2 },
  assistant: { total: 271, text: 50, 'tool-use': 180, thinking: 41 },
  system: 0,
  'file-history-snapshot': 39,
  summary: 0,
  'queue-operation': 6,
};

const sampleGitInfo: GitInfo = {
  repo_url: 'https://github.com/ConfabulousDev/confab',
  branch: 'main',
  commit_sha: 'abc123',
};

// Interactive wrapper for filter state
function SessionHeaderInteractive(
  props: Omit<
    React.ComponentProps<typeof SessionHeader>,
    'categoryCounts' | 'filterState' | 'onToggleCategory' | 'onToggleUserSubcategory' | 'onToggleAssistantSubcategory'
  > & {
    counts?: HierarchicalCounts;
    initialFilterState?: FilterState;
  }
) {
  const { counts = sampleCounts, initialFilterState = DEFAULT_FILTER_STATE, ...rest } = props;
  const [filterState, setFilterState] = useState(initialFilterState);

  const handleToggleCategory = (category: MessageCategory) => {
    setFilterState((prev) => {
      const next = { ...prev };
      if (category === 'user') {
        const allVisible = prev.user.prompt && prev.user['tool-result'] && prev.user.skill;
        next.user = { prompt: !allVisible, 'tool-result': !allVisible, skill: !allVisible };
      } else if (category === 'assistant') {
        const allVisible = prev.assistant.text && prev.assistant['tool-use'] && prev.assistant.thinking;
        next.assistant = { text: !allVisible, 'tool-use': !allVisible, thinking: !allVisible };
      } else {
        next[category] = !prev[category];
      }
      return next;
    });
  };

  const handleToggleUserSubcategory = (subcategory: UserSubcategory) => {
    setFilterState((prev) => ({
      ...prev,
      user: { ...prev.user, [subcategory]: !prev.user[subcategory] },
    }));
  };

  const handleToggleAssistantSubcategory = (subcategory: AssistantSubcategory) => {
    setFilterState((prev) => ({
      ...prev,
      assistant: { ...prev.assistant, [subcategory]: !prev.assistant[subcategory] },
    }));
  };

  return (
    <SessionHeader
      {...rest}
      categoryCounts={counts}
      filterState={filterState}
      onToggleCategory={handleToggleCategory}
      onToggleUserSubcategory={handleToggleUserSubcategory}
      onToggleAssistantSubcategory={handleToggleAssistantSubcategory}
    />
  );
}

const meta: Meta<typeof SessionHeaderInteractive> = {
  title: 'Session/SessionHeader',
  component: SessionHeaderInteractive,
  parameters: {
    layout: 'fullscreen',
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
    durationMs: 4980000, // ~1h 23m
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
    durationMs: 1800000, // 30 min
    sessionDate: new Date('2025-12-05T14:30:00'),
    gitInfo: { repo_url: 'https://github.com/user/project', branch: 'feature/dark-mode' },
    isOwner: false,
    isShared: true,
    sharedByEmail: 'alice@example.com',
  },
};

// Shared session without email (legacy shares or edge case)
export const SharedSessionWithoutEmail: Story = {
  args: {
    sessionId: 'session-456-legacy',
    title: 'Legacy Shared Session',
    hasCustomTitle: false,
    autoTitle: 'Legacy Shared Session',
    externalId: 'legacy789abc',
    ownerEmail: 'bob@example.com',
    model: 'claude-sonnet-4-20250514',
    durationMs: 1200000, // 20 min
    sessionDate: new Date('2025-12-04T09:00:00'),
    gitInfo: { repo_url: 'https://github.com/user/project', branch: 'main' },
    isOwner: false,
    isShared: true,
    sharedByEmail: null, // Falls back to "Shared Session"
  },
};

// Owner viewing their own share link - indicator is clickable
export const OwnerViewingShareLink: Story = {
  args: {
    sessionId: 'session-owner-share',
    title: 'API Authentication Implementation',
    hasCustomTitle: false,
    autoTitle: 'API Authentication Implementation',
    externalId: 'owner123share456',
    ownerEmail: 'developer@example.com',
    model: 'claude-opus-4-5-20251101',
    durationMs: 3600000, // 1 hour
    sessionDate: new Date('2025-12-06T10:00:00'),
    gitInfo: { repo_url: 'https://github.com/user/project', branch: 'feature/auth' },
    isOwner: true,
    isShared: true, // Owner viewing via share link
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
    durationMs: 300000, // 5 min
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
    durationMs: 7200000, // 2 hours
    sessionDate: new Date('2025-12-01T09:00:00'),
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

// Non-interactive story showing header without filter (Analytics tab view)
type DirectStory = StoryObj<typeof SessionHeader>;

export const WithoutFilter: DirectStory = {
  render: () => (
    <SessionHeader
      sessionId="session-analytics"
      title="Viewing Analytics Tab"
      hasCustomTitle={false}
      autoTitle="Viewing Analytics Tab"
      externalId="analytics123"
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
      // No filter props - simulates Analytics tab view
    />
  ),
};
