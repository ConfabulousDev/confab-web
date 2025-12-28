import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { MemoryRouter } from 'react-router-dom';
import SessionHeader from './SessionHeader';
import type { MessageCategory, MessageCategoryCounts } from './messageCategories';
import type { GitInfo } from '@/types';

// Sample data
const sampleCounts: MessageCategoryCounts = {
  user: 194,
  assistant: 271,
  system: 0,
  'file-history-snapshot': 39,
  summary: 0,
  'queue-operation': 6,
};

const defaultVisibleCategories = new Set<MessageCategory>([
  'user',
  'assistant',
  'system',
]);

const sampleGitInfo: GitInfo = {
  repo_url: 'https://github.com/ConfabulousDev/confab',
  branch: 'main',
  commit_sha: 'abc123',
};

// Interactive wrapper for filter state
function SessionHeaderInteractive(
  props: Omit<
    React.ComponentProps<typeof SessionHeader>,
    'categoryCounts' | 'visibleCategories' | 'onToggleCategory'
  > & {
    counts?: MessageCategoryCounts;
    initialVisible?: Set<MessageCategory>;
  }
) {
  const { counts = sampleCounts, initialVisible = defaultVisibleCategories, ...rest } = props;
  const [visibleCategories, setVisibleCategories] = useState(initialVisible);

  const handleToggle = (category: MessageCategory) => {
    setVisibleCategories((prev) => {
      const next = new Set(prev);
      if (next.has(category)) {
        next.delete(category);
      } else {
        next.add(category);
      }
      return next;
    });
  };

  return (
    <SessionHeader
      {...rest}
      categoryCounts={counts}
      visibleCategories={visibleCategories}
      onToggleCategory={handleToggle}
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
        <div style={{ background: '#fafafa', minHeight: '200px' }}>
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
    model: 'claude-sonnet-4-20250514',
    durationMs: 1800000, // 30 min
    sessionDate: new Date('2025-12-05T14:30:00'),
    gitInfo: { repo_url: 'https://github.com/user/project', branch: 'feature/dark-mode' },
    isOwner: false,
    isShared: true,
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
