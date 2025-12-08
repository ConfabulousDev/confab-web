import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import SessionsFilterDropdown from './SessionsFilterDropdown';

// Sample data for stories
const sampleRepos = ['confab-web', 'confab-cli', 'my-app'];
const sampleBranches = ['main', 'feature/auth', 'fix/bug-123', 'develop'];

const sampleRepoCounts: Record<string, number> = {
  'confab-web': 45,
  'confab-cli': 23,
  'my-app': 12,
};

const sampleBranchCounts: Record<string, number> = {
  main: 30,
  'feature/auth': 8,
  'fix/bug-123': 5,
  develop: 2,
};

// Interactive wrapper component
function SessionsFilterDropdownInteractive({
  repos = sampleRepos,
  branches = sampleBranches,
  repoCounts = sampleRepoCounts,
  branchCounts = sampleBranchCounts,
  totalCount = 80,
  initialRepo = null,
  initialBranch = null,
}: {
  repos?: string[];
  branches?: string[];
  repoCounts?: Record<string, number>;
  branchCounts?: Record<string, number>;
  totalCount?: number;
  initialRepo?: string | null;
  initialBranch?: string | null;
}) {
  const [selectedRepo, setSelectedRepo] = useState<string | null>(initialRepo);
  const [selectedBranch, setSelectedBranch] = useState<string | null>(initialBranch);

  const handleRepoClick = (repo: string | null) => {
    setSelectedRepo(repo);
    setSelectedBranch(null); // Reset branch when repo changes
  };

  const handleBranchClick = (branch: string | null) => {
    setSelectedBranch(branch);
  };

  return (
    <SessionsFilterDropdown
      repos={repos}
      branches={branches}
      selectedRepo={selectedRepo}
      selectedBranch={selectedBranch}
      repoCounts={repoCounts}
      branchCounts={branchCounts}
      totalCount={totalCount}
      onRepoClick={handleRepoClick}
      onBranchClick={handleBranchClick}
    />
  );
}

const meta: Meta<typeof SessionsFilterDropdownInteractive> = {
  title: 'Components/SessionsFilterDropdown',
  component: SessionsFilterDropdownInteractive,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ padding: '100px', background: '#fafafa' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof SessionsFilterDropdownInteractive>;

export const Default: Story = {
  args: {},
};

export const WithRepoSelected: Story = {
  args: {
    initialRepo: 'confab-web',
  },
};

export const WithRepoAndBranchSelected: Story = {
  args: {
    initialRepo: 'confab-web',
    initialBranch: 'main',
  },
};

export const SingleRepo: Story = {
  args: {
    repos: ['my-project'],
    repoCounts: { 'my-project': 15 },
    totalCount: 15,
  },
};

export const NoRepos: Story = {
  args: {
    repos: [],
    branches: [],
    repoCounts: {},
    branchCounts: {},
    totalCount: 25,
  },
};

export const ManyRepos: Story = {
  args: {
    repos: [
      'project-alpha',
      'project-beta',
      'project-gamma',
      'project-delta',
      'project-epsilon',
      'project-zeta',
    ],
    repoCounts: {
      'project-alpha': 50,
      'project-beta': 35,
      'project-gamma': 28,
      'project-delta': 15,
      'project-epsilon': 8,
      'project-zeta': 3,
    },
    totalCount: 139,
  },
};

export const LongRepoNames: Story = {
  args: {
    repos: [
      'very-long-repository-name-that-might-overflow',
      'another-extremely-long-repo-name-here',
    ],
    repoCounts: {
      'very-long-repository-name-that-might-overflow': 42,
      'another-extremely-long-repo-name-here': 18,
    },
    totalCount: 60,
  },
};
