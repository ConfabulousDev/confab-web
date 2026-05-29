import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import ProviderFilter from './ProviderFilter';
import { PROVIDER_VALUES } from '@/utils/providers';

const meta: Meta<typeof ProviderFilter> = {
  title: 'Filters/ProviderFilter',
  component: ProviderFilter,
  parameters: {
    layout: 'centered',
  },
};

export default meta;
type Story = StoryObj<typeof ProviderFilter>;

// Stateful wrapper so the dropdown toggles and selections persist during
// interaction.
function Stateful({
  availableProviders = [...PROVIDER_VALUES],
  initialSelected = [],
}: {
  availableProviders?: string[];
  initialSelected?: string[];
}) {
  const [selected, setSelected] = useState<string[]>(initialSelected);
  return (
    <ProviderFilter
      availableProviders={availableProviders}
      selectedProviders={selected}
      onChange={setSelected}
    />
  );
}

// CF-424: empty providers state = "All Providers" label, both rows unchecked.
export const Default: Story = {
  render: () => <Stateful />,
};

// One provider selected — trigger shows the provider's display label.
export const OneProviderSelected: Story = {
  render: () => <Stateful initialSelected={['claude-code']} />,
};

// Both providers selected — trigger reads "2 providers".
export const BothProvidersSelected: Story = {
  render: () => <Stateful initialSelected={['claude-code', 'codex']} />,
};

// Org page with only one provider present narrows the dropdown to one row.
export const SingleProviderAvailable: Story = {
  render: () => <Stateful availableProviders={['claude-code']} />,
};
