import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import TrendsFilters, { type TrendsFiltersValue } from './TrendsFilters';

const meta: Meta<typeof TrendsFilters> = {
  title: 'Trends/TrendsFilters',
  component: TrendsFilters,
  parameters: {
    layout: 'centered',
  },
};

export default meta;
type Story = StoryObj<typeof TrendsFilters>;

const sampleRepos = ['org/confab-web', 'org/confab-cli', 'org/marketing'];

const defaultDateRange = {
  startDate: '2025-01-08',
  endDate: '2025-01-14',
  label: 'Last 7 Days',
};

function Interactive({ initial }: { initial: TrendsFiltersValue }) {
  const [value, setValue] = useState<TrendsFiltersValue>(initial);
  return <TrendsFilters repos={sampleRepos} value={value} onChange={setValue} />;
}

// CF-424: empty providers state = "All Providers" label, both checkboxes unchecked.
export const Default: Story = {
  render: () => (
    <Interactive
      initial={{
        dateRange: defaultDateRange,
        repos: [],
        includeNoRepo: true,
        providers: [],
      }}
    />
  ),
};

// CF-424: one provider selected — trigger button shows the provider's display label
// (via providerLabel), and the corresponding row checkbox is checked.
export const OneProviderSelected: Story = {
  render: () => (
    <Interactive
      initial={{
        dateRange: defaultDateRange,
        repos: [],
        includeNoRepo: true,
        providers: ['claude-code'],
      }}
    />
  ),
};

// CF-424: both providers selected — kept as a distinct state from [] per the
// interview decision. Trigger label is "2 providers", both rows checked.
export const BothProvidersSelected: Story = {
  render: () => (
    <Interactive
      initial={{
        dateRange: defaultDateRange,
        repos: [],
        includeNoRepo: true,
        providers: ['claude-code', 'codex'],
      }}
    />
  ),
};
