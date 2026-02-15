import type { Meta, StoryObj } from '@storybook/react';
import FilterChipsBar from './FilterChipsBar';
import type { SessionFilterOptions } from '@/schemas/api';

const sampleFilterOptions: SessionFilterOptions = {
  repos: ['backend-api', 'confab-cli', 'confab-web'],
  branches: ['feature/auth', 'fix/pagination', 'main'],
  owners: ['alice@example.com', 'bob@example.com', 'carol@example.com'],
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

export const NoFilters: Story = {
  args: {
    filters: { repos: [], branches: [], owners: [], query: '' },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    onToggleRepo: () => {},
    onToggleBranch: () => {},
    onToggleOwner: () => {},
    onQueryChange: () => {},
    onClearAll: () => {},
  },
};

export const WithActiveFilters: Story = {
  args: {
    filters: {
      repos: ['confab-web'],
      branches: ['main'],
      owners: ['alice@example.com'],
      query: '',
    },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    onToggleRepo: () => {},
    onToggleBranch: () => {},
    onToggleOwner: () => {},
    onQueryChange: () => {},
    onClearAll: () => {},
  },
};

export const ManyFilters: Story = {
  args: {
    filters: {
      repos: ['confab-web', 'confab-cli'],
      branches: ['main', 'feature/auth'],
      owners: ['alice@example.com'],
      query: 'fix auth',
    },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@example.com',
    onToggleRepo: () => {},
    onToggleBranch: () => {},
    onToggleOwner: () => {},
    onQueryChange: () => {},
    onClearAll: () => {},
  },
};
