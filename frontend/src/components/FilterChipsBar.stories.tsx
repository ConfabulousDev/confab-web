import type { Meta, StoryObj } from '@storybook/react';
import FilterChipsBar from './FilterChipsBar';
import type { SessionFilterOptions } from '@/schemas/api';

const sampleFilterOptions: SessionFilterOptions = {
  repos: ['backend-api', 'confab-cli', 'confab-web'],
  branches: ['feature/auth', 'fix/pagination', 'main'],
  owners: ['alice@example.com', 'bob@example.com', 'carol@example.com'],
  providers: ['claude-code', 'codex'],
};

const meta: Meta<typeof FilterChipsBar> = {
  title: 'Components/FilterChipsBar',
  component: FilterChipsBar,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof FilterChipsBar>;

const noopHandlers = {
  onToggleRepo: () => {},
  onToggleBranch: () => {},
  onToggleOwner: () => {},
  onToggleProvider: () => {},
  onQueryChange: () => {},
  onClearAll: () => {},
};

export const NoFilters: Story = {
  args: {
    filters: { repos: [], branches: [], owners: [], providers: [], query: '' },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

export const WithActiveFilters: Story = {
  args: {
    filters: {
      repos: ['confab-web'],
      branches: ['main'],
      owners: ['alice@example.com'],
      providers: [],
      query: '',
    },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

export const ManyFilters: Story = {
  args: {
    filters: {
      repos: ['confab-web', 'confab-cli'],
      branches: ['main', 'feature/auth'],
      owners: ['alice@example.com'],
      providers: [],
      query: 'fix auth',
    },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

// CF-393: Provider filter coverage.

export const ProviderFilterAvailable: Story = {
  args: {
    filters: { repos: [], branches: [], owners: [], providers: [], query: '' },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

export const OneProviderSelected: Story = {
  args: {
    filters: { repos: [], branches: [], owners: [], providers: ['claude-code'], query: '' },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

export const BothProvidersSelected: Story = {
  args: {
    filters: { repos: [], branches: [], owners: [], providers: ['claude-code', 'codex'], query: '' },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

export const ProviderPlusOtherDimensions: Story = {
  args: {
    filters: {
      repos: ['confab-web'],
      branches: ['main'],
      owners: ['alice@example.com'],
      providers: ['codex'],
      query: 'fix auth',
    },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

// CF-511: FilterChipsBar state that produces a mixed dropdown (some repos selected).
export const MixedDropdownSelection: Story = {
  args: {
    filters: {
      repos: ['confab-web'],
      branches: [],
      owners: [],
      providers: [],
      query: '',
    },
    filterOptions: {
      repos: ['alpha', 'beta', 'confab-cli', 'confab-web', 'delta', 'epsilon'],
      branches: ['main'],
      owners: ['alice@example.com'],
    },
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

// CF-491: when a user has worked on the same project through a fork
// (jackie/confab-web) and through the upstream (ConfabulousDev/confab-web),
// the repo chip list shows a single upstream-root entry rather than two
// chips. Collapsing happens server-side via session_repos.root_name; the
// frontend sees only the collapsed name.
export const CollapsedForks: Story = {
  args: {
    filters: { repos: [], branches: [], owners: [], providers: [], query: '' },
    filterOptions: {
      repos: ['ConfabulousDev/confab-web'],
      branches: ['main'],
      owners: ['alice@example.com'],
    },
    currentUserEmail: 'alice@example.com',
    ...noopHandlers,
  },
};

