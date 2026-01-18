import type { Meta, StoryObj } from '@storybook/react-vite';
import { SmartRecapCard } from './SmartRecapCard';

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
        'Clear problem description provided upfront',
        'Good iteration based on feedback',
        'Comprehensive test coverage added',
      ],
      went_bad: [
        'Initial requirements were ambiguous',
        'Tests took a long time to run',
      ],
      human_suggestions: ['Include expected vs actual behavior in bug reports'],
      environment_suggestions: ['Consider parallelizing test suite'],
      default_context_suggestions: [
        'Add authentication patterns to CLAUDE.md',
        'Document preferred error handling approach',
      ],
      computed_at: '2024-01-15T10:30:00Z',
      is_stale: false,
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 3, limit: 20, exceeded: false },
  },
};

export const Refreshing: Story = {
  args: {
    data: {
      recap: 'Old recap content that will be replaced...',
      went_well: ['Previous item'],
      went_bad: [],
      human_suggestions: [],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-15T10:30:00Z',
      is_stale: true,
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 3, limit: 20, exceeded: false },
    isRefreshing: true,
  },
};

export const Stale: Story = {
  args: {
    data: {
      recap:
        'This is an older recap that may not reflect the latest changes in the session.',
      went_well: ['Initial setup was smooth'],
      went_bad: ['Some edge cases were missed'],
      human_suggestions: [],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-14T08:00:00Z',
      is_stale: true,
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 15, limit: 20, exceeded: false },
  },
};

export const QuotaExceeded: Story = {
  args: {
    data: {
      recap: 'This recap was generated earlier. Monthly quota has been reached.',
      went_well: ['Task was completed successfully'],
      went_bad: [],
      human_suggestions: ['Be more specific with requirements'],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-10T14:00:00Z',
      is_stale: true,
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
      is_stale: false,
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
        'Clean separation of concerns achieved',
        'Performance improved by 40%',
        'All existing tests still pass',
      ],
      went_bad: [
        'Some components required multiple rewrites',
        'Documentation was missing for legacy code',
        'Build times increased due to new dependencies',
      ],
      human_suggestions: [
        'Break large refactors into smaller PRs',
        'Write ADRs for architectural decisions',
        'Request code review earlier in the process',
      ],
      environment_suggestions: [
        'Enable incremental TypeScript compilation',
        'Add pre-commit hooks for linting',
        'Set up CI caching for dependencies',
      ],
      default_context_suggestions: [
        'Document the component architecture in CLAUDE.md',
        'Add refactoring guidelines',
        'Include performance targets and benchmarks',
      ],
      computed_at: '2024-01-15T10:30:00Z',
      is_stale: false,
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: { used: 5, limit: 20, exceeded: false },
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
    quota: null,
  },
};

export const NoQuotaInfo: Story = {
  args: {
    data: {
      recap: 'Recap without quota information displayed.',
      went_well: ['Everything worked as expected'],
      went_bad: [],
      human_suggestions: [],
      environment_suggestions: [],
      default_context_suggestions: [],
      computed_at: '2024-01-15T10:30:00Z',
      is_stale: false,
      model_used: 'claude-haiku-4-5-20251101',
    },
    loading: false,
    quota: null,
  },
};
