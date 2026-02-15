import type { Meta, StoryObj } from '@storybook/react';
import FilterChipsBar from './FilterChipsBar';
import type { SessionFilterOptions } from '@/schemas/api';

const sampleFilterOptions: SessionFilterOptions = {
  repos: [
    { value: 'confab-web', count: 85 },
    { value: 'confab-cli', count: 42 },
    { value: 'backend-api', count: 23 },
  ],
  branches: [
    { value: 'main', count: 60 },
    { value: 'feature/auth', count: 20 },
    { value: 'fix/pagination', count: 10 },
  ],
  owners: [
    { value: 'alice@example.com', count: 70 },
    { value: 'bob@example.com', count: 45 },
    { value: 'carol@example.com', count: 35 },
  ],
  total: 150,
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
