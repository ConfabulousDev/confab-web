import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import OrgFilters, { type OrgFiltersValue } from './OrgFilters';
import { PROVIDER_VALUES } from '@/utils/providers';

const meta: Meta<typeof OrgFilters> = {
  title: 'Org/OrgFilters',
  component: OrgFilters,
  parameters: {
    layout: 'centered',
  },
};

export default meta;
type Story = StoryObj<typeof OrgFilters>;

const SAMPLE_REPOS = [
  'ConfabulousDev/confab-web',
  'ConfabulousDev/confab-cli',
  'ConfabulousDev/extensions',
];

const DEFAULT_DATE_RANGE = {
  startDate: '2026-05-01',
  endDate: '2026-05-18',
  label: 'Last 7 days',
};

// Stateful wrapper so the dropdowns actually toggle and selections persist
// during interaction. Stories pass the initial value via `initialValue`.
function Stateful({
  initialValue,
  availableProviders,
  availableRepos,
}: {
  initialValue: OrgFiltersValue;
  availableProviders: string[];
  availableRepos: string[];
}) {
  const [value, setValue] = useState<OrgFiltersValue>(initialValue);
  return (
    <OrgFilters
      value={value}
      onChange={setValue}
      availableProviders={availableProviders}
      availableRepos={availableRepos}
    />
  );
}

export const Default: Story = {
  render: () => (
    <Stateful
      availableProviders={[...PROVIDER_VALUES]}
      availableRepos={SAMPLE_REPOS}
      initialValue={{
        dateRange: DEFAULT_DATE_RANGE,
        providers: [],
        repos: SAMPLE_REPOS,
        includeNoRepo: true,
      }}
    />
  ),
};

export const ProviderSelected: Story = {
  render: () => (
    <Stateful
      availableProviders={[...PROVIDER_VALUES]}
      availableRepos={SAMPLE_REPOS}
      initialValue={{
        dateRange: DEFAULT_DATE_RANGE,
        providers: ['codex'],
        repos: SAMPLE_REPOS,
        includeNoRepo: true,
      }}
    />
  ),
};

export const BothProvidersSelected: Story = {
  render: () => (
    <Stateful
      availableProviders={[...PROVIDER_VALUES]}
      availableRepos={SAMPLE_REPOS}
      initialValue={{
        dateRange: DEFAULT_DATE_RANGE,
        providers: ['claude-code', 'codex'],
        repos: SAMPLE_REPOS,
        includeNoRepo: true,
      }}
    />
  ),
};

export const RepoSubset: Story = {
  render: () => (
    <Stateful
      availableProviders={[...PROVIDER_VALUES]}
      availableRepos={SAMPLE_REPOS}
      initialValue={{
        dateRange: DEFAULT_DATE_RANGE,
        providers: [],
        repos: ['ConfabulousDev/confab-web'],
        includeNoRepo: false,
      }}
    />
  ),
};

export const AllFiltersActive: Story = {
  render: () => (
    <Stateful
      availableProviders={[...PROVIDER_VALUES]}
      availableRepos={SAMPLE_REPOS}
      initialValue={{
        dateRange: DEFAULT_DATE_RANGE,
        providers: ['codex'],
        repos: ['ConfabulousDev/confab-web', 'ConfabulousDev/confab-cli'],
        includeNoRepo: false,
      }}
    />
  ),
};

// Org with only Claude data — dropdown narrows to a single provider.
export const SingleProviderAvailable: Story = {
  render: () => (
    <Stateful
      availableProviders={['claude-code']}
      availableRepos={SAMPLE_REPOS}
      initialValue={{
        dateRange: DEFAULT_DATE_RANGE,
        providers: [],
        repos: SAMPLE_REPOS,
        includeNoRepo: true,
      }}
    />
  ),
};
