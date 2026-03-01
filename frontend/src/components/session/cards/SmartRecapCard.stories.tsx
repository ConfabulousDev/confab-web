import type { Meta, StoryObj } from '@storybook/react-vite';
import { SmartRecapCard } from './SmartRecapCard';

/** Shorthand to create an AnnotatedItem */
const a = (text: string, message_id?: string) => ({ text, message_id });

const meta: Meta<typeof SmartRecapCard> = {
  title: 'Session/Cards/SmartRecapCard',
  component: SmartRecapCard,
  parameters: { layout: 'centered' },
  decorators: [
    (Story) => (
      <div style={{ width: '560px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof SmartRecapCard>;

export const Default: Story = {
  args: {
    data: {
      recap:
        'The session involved implementing a new feature for user authentication. The user worked through several iterations to get the JWT token handling correct and added appropriate error handling.',
      went_well: [
        a('Clear problem description provided upfront', 'msg-uuid-1'),
        a('Good iteration based on feedback', 'msg-uuid-5'),
        a('Comprehensive test coverage added'),
      ],
      went_bad: [
        a('Initial requirements were ambiguous', 'msg-uuid-2'),
        a('Tests took a long time to run'),
      ],
      human_suggestions: [a('Include expected vs actual behavior in bug reports')],
      environment_suggestions: [a('Consider parallelizing test suite')],
      default_context_suggestions: [
        a('Add authentication patterns to CLAUDE.md'),
        a('Document preferred error handling approach'),
      ],
      computed_at: '2024-01-15T10:30:00Z',
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 3, limit: 20, exceeded: false },
    sessionId: 'demo-session-id',
  },
};

export const Refreshing: Story = {
  args: {
    data: {
      recap: 'Old recap content that will be replaced...',
      went_well: [a('Previous item')],
      went_bad: [],
      human_suggestions: [],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-15T10:30:00Z',
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 3, limit: 20, exceeded: false },
    isRefreshing: true,
  },
};

export const QuotaExceeded: Story = {
  args: {
    data: {
      recap: 'This recap was generated earlier. Monthly quota has been reached.',
      went_well: [a('Task was completed successfully')],
      went_bad: [],
      human_suggestions: [a('Be more specific with requirements')],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-10T14:00:00Z',
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 20, limit: 20, exceeded: true },
  },
};

export const MinimalData: Story = {
  args: {
    data: {
      recap: 'A simple debugging session to fix a typo in the configuration file.',
      went_well: [],
      went_bad: [],
      human_suggestions: [],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-15T10:30:00Z',
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 1, limit: 20, exceeded: false },
  },
};

export const AllSuggestions: Story = {
  args: {
    data: {
      recap:
        'Extended refactoring session covering multiple components with significant architectural changes.',
      went_well: [
        a('Clean separation of concerns achieved', 'msg-uuid-3'),
        a('Performance improved by 40%', 'msg-uuid-8'),
        a('All existing tests still pass'),
      ],
      went_bad: [
        a('Some components required multiple rewrites', 'msg-uuid-4'),
        a('Documentation was missing for legacy code'),
        a('Build times increased due to new dependencies'),
      ],
      human_suggestions: [
        a('Break large refactors into smaller PRs'),
        a('Write ADRs for architectural decisions'),
      ],
      environment_suggestions: [
        a('Enable incremental TypeScript compilation'),
        a('Add pre-commit hooks for linting'),
      ],
      default_context_suggestions: [
        a('Document the component architecture in CLAUDE.md'),
        a('Add refactoring guidelines'),
      ],
      computed_at: '2024-01-15T10:30:00Z',
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 5, limit: 20, exceeded: false },
    sessionId: 'demo-session-id',
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
    quota: null,
  },
};

export const UnlimitedQuota: Story = {
  args: {
    data: {
      recap: 'Recap without quota information displayed.',
      went_well: [a('Everything worked as expected')],
      went_bad: [],
      human_suggestions: [],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-15T10:30:00Z',
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: null,
  },
};

export const QuotaExceededNoData: Story = {
  args: {
    data: null,
    loading: false,
    quota: { used: 20, limit: 20, exceeded: true },
    missingReason: 'quota_exceeded',
  },
};

export const UnavailableNonOwner: Story = {
  args: {
    data: null,
    loading: false,
    missingReason: 'unavailable',
  },
};
