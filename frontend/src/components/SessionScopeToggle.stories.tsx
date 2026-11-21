import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import SessionScopeToggle from './SessionScopeToggle';
import type { SessionScope } from '@/hooks/useSessionFilters';

// xtrf: admin-only scope toggle on the Sessions page.
const meta: Meta<typeof SessionScopeToggle> = {
  title: 'Components/SessionScopeToggle',
  component: SessionScopeToggle,
  parameters: {
    layout: 'centered',
  },
  argTypes: {
    onScopeChange: { action: 'scopeChanged' },
  },
};

export default meta;
type Story = StoryObj<typeof SessionScopeToggle>;

export const MineAndShared: Story = {
  args: { scope: '' },
};

export const AllSessions: Story = {
  args: { scope: 'all' },
};

export const Disabled: Story = {
  args: { scope: '', disabled: true },
};

function InteractiveToggle() {
  const [scope, setScope] = useState<SessionScope>('');
  return <SessionScopeToggle scope={scope} onScopeChange={setScope} />;
}

export const Interactive: Story = {
  render: () => <InteractiveToggle />,
};
