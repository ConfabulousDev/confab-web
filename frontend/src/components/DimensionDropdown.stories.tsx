import type { Meta, StoryObj } from '@storybook/react';
import { DimensionDropdown } from './FilterChipsBar';
import { RepoIcon } from './icons';

const meta: Meta<typeof DimensionDropdown> = {
  title: 'Components/DimensionDropdown',
  component: DimensionDropdown,
  parameters: { layout: 'padded' },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof DimensionDropdown>;

const sixRepos = ['alpha', 'beta', 'confab-cli', 'confab-web', 'delta', 'epsilon'];

// CF-511: divider visible between selected and unselected groups.
export const OpenWithDivider: Story = {
  args: {
    label: 'Repo',
    icon: RepoIcon,
    options: sixRepos,
    selected: ['confab-web'],
    onToggle: () => {},
    initialOpen: true,
  },
};

export const OpenNoneSelected: Story = {
  args: {
    label: 'Repo',
    icon: RepoIcon,
    options: sixRepos,
    selected: [],
    onToggle: () => {},
    initialOpen: true,
  },
};

export const OpenAllSelected: Story = {
  args: {
    label: 'Repo',
    icon: RepoIcon,
    options: sixRepos,
    selected: [...sixRepos],
    onToggle: () => {},
    initialOpen: true,
  },
};

export const OpenMultipleSelected: Story = {
  args: {
    label: 'Repo',
    icon: RepoIcon,
    options: sixRepos,
    selected: ['alpha', 'confab-web', 'delta'],
    onToggle: () => {},
    initialOpen: true,
  },
};
