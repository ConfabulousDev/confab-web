import type { Meta, StoryObj } from '@storybook/react';
import Chip from './Chip';
import { RepoIcon, BranchIcon, ComputerIcon, UserIcon, PRIcon, CommitIcon } from './icons';

const meta: Meta<typeof Chip> = {
  title: 'Components/Chip',
  component: Chip,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    variant: {
      control: 'select',
      options: ['neutral', 'blue', 'green', 'purple'],
    },
  },
};

export default meta;
type Story = StoryObj<typeof Chip>;

export const Neutral: Story = {
  args: {
    children: 'confab-web',
    variant: 'neutral',
    icon: RepoIcon,
  },
};

export const Blue: Story = {
  args: {
    children: 'main',
    variant: 'blue',
    icon: BranchIcon,
  },
};

export const Green: Story = {
  args: {
    children: 'macbook-pro.local',
    variant: 'green',
    icon: ComputerIcon,
  },
};

export const Purple: Story = {
  args: {
    children: 'sarah',
    variant: 'purple',
    icon: UserIcon,
  },
};

export const WithoutIcon: Story = {
  args: {
    children: 'plain chip',
    variant: 'neutral',
  },
};

export const LongText: Story = {
  args: {
    children: 'very-long-hostname-that-will-truncate.local',
    variant: 'green',
    icon: ComputerIcon,
  },
};

export const AllVariants: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
      <Chip icon={RepoIcon} variant="neutral">confab-web</Chip>
      <Chip icon={BranchIcon} variant="blue">main</Chip>
      <Chip icon={ComputerIcon} variant="green">macbook-pro</Chip>
      <Chip icon={UserIcon} variant="purple">sarah</Chip>
    </div>
  ),
};

export const SessionListExample: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '24px' }}>
      <div>
        <div style={{ fontSize: '11px', color: '#999', marginBottom: '4px', textTransform: 'uppercase' }}>Git</div>
        <div style={{ display: 'flex', gap: '4px' }}>
          <Chip icon={RepoIcon} variant="neutral">confab-web</Chip>
          <Chip icon={BranchIcon} variant="blue">main</Chip>
        </div>
      </div>
      <div>
        <div style={{ fontSize: '11px', color: '#999', marginBottom: '4px', textTransform: 'uppercase' }}>System</div>
        <div style={{ display: 'flex', gap: '4px' }}>
          <Chip icon={ComputerIcon} variant="green">macbook-pro.local</Chip>
          <Chip icon={UserIcon} variant="purple">sarah</Chip>
        </div>
      </div>
    </div>
  ),
};

export const Clickable: Story = {
  args: {
    children: 'Click to copy',
    variant: 'neutral',
    copyValue: 'https://github.com/example/repo',
  },
  parameters: {
    docs: {
      description: {
        story: 'Chips with `copyValue` show a border on hover and copy the value to clipboard when clicked.',
      },
    },
  },
};

export const ClickableVariants: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
      <Chip icon={RepoIcon} variant="neutral" copyValue="https://github.com/example/repo">example/repo</Chip>
      <Chip icon={BranchIcon} variant="blue" copyValue="https://github.com/example/repo/tree/main">main</Chip>
      <Chip icon={ComputerIcon} variant="green" copyValue="macbook-pro.local">macbook-pro</Chip>
      <Chip icon={UserIcon} variant="purple" copyValue="https://github.com/example/repo/pull/123">#123</Chip>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: 'All chip variants support the clickable copy behavior. Hover to see the border, click to copy.',
      },
    },
  },
};

export const LinkableChips: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
      <Chip icon={PRIcon} variant="purple" linkUrl="https://github.com/example/repo/pull/42">#42</Chip>
      <Chip icon={CommitIcon} variant="purple" linkUrl="https://github.com/example/repo/commit/abc1234">abc1234</Chip>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: 'Chips with `linkUrl` open the URL in a new tab when clicked instead of copying to clipboard.',
      },
    },
  },
};
