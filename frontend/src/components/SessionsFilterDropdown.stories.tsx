import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import SessionsFilterDropdown from './SessionsFilterDropdown';

// Sample data for stories
const sampleRepos = ['confab-web', 'confab-cli', 'my-app'];
const sampleBranches = ['main', 'feature/auth', 'fix/bug-123', 'develop'];
const sampleHostnames = ['macbook-pro', 'desktop-linux', 'work-laptop'];
const samplePRs = ['142', '138', '125', '119'];
const sampleCommits = ['a1b2c3d4e5f6789', 'b2c3d4e5f6789a1', 'c3d4e5f6789a1b2'];

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

const sampleHostnameCounts: Record<string, number> = {
  'macbook-pro': 38,
  'desktop-linux': 25,
  'work-laptop': 17,
};

const samplePRCounts: Record<string, number> = {
  '142': 5,
  '138': 3,
  '125': 8,
  '119': 2,
};

const sampleCommitCounts: Record<string, number> = {
  'a1b2c3d4e5f6789': 4,
  'b2c3d4e5f6789a1': 3,
  'c3d4e5f6789a1b2': 2,
};

// Interactive wrapper component
function SessionsFilterDropdownInteractive({
  repos = sampleRepos,
  branches = sampleBranches,
  hostnames = sampleHostnames,
  prs = samplePRs,
  commits = sampleCommits,
  repoCounts = sampleRepoCounts,
  branchCounts = sampleBranchCounts,
  hostnameCounts = sampleHostnameCounts,
  prCounts = samplePRCounts,
  commitCounts = sampleCommitCounts,
  totalCount = 80,
  initialRepo = null,
  initialBranch = null,
  initialHostname = null,
  initialPR = null,
  initialCommit = null,
  initialSearchQuery = '',
}: {
  repos?: string[];
  branches?: string[];
  hostnames?: string[];
  prs?: string[];
  commits?: string[];
  repoCounts?: Record<string, number>;
  branchCounts?: Record<string, number>;
  hostnameCounts?: Record<string, number>;
  prCounts?: Record<string, number>;
  commitCounts?: Record<string, number>;
  totalCount?: number;
  initialRepo?: string | null;
  initialBranch?: string | null;
  initialHostname?: string | null;
  initialPR?: string | null;
  initialCommit?: string | null;
  initialSearchQuery?: string;
}) {
  const [selectedRepo, setSelectedRepo] = useState<string | null>(initialRepo);
  const [selectedBranch, setSelectedBranch] = useState<string | null>(initialBranch);
  const [selectedHostname, setSelectedHostname] = useState<string | null>(initialHostname);
  const [selectedPR, setSelectedPR] = useState<string | null>(initialPR);
  const [selectedCommit, setSelectedCommit] = useState<string | null>(initialCommit);
  const [searchQuery, setSearchQuery] = useState<string>(initialSearchQuery);

  const handleRepoClick = (repo: string | null) => {
    setSelectedRepo(repo);
    // Reset repo-scoped filters when repo changes
    setSelectedBranch(null);
    setSelectedPR(null);
    setSelectedCommit(null);
  };

  const handleBranchClick = (branch: string | null) => {
    setSelectedBranch(branch);
  };

  const handleHostnameClick = (hostname: string | null) => {
    setSelectedHostname(hostname);
  };

  const handlePRClick = (pr: string | null) => {
    setSelectedPR(pr);
  };

  const handleCommitClick = (commit: string | null) => {
    setSelectedCommit(commit);
  };

  return (
    <SessionsFilterDropdown
      repos={repos}
      branches={branches}
      hostnames={hostnames}
      prs={prs}
      commits={commits}
      selectedRepo={selectedRepo}
      selectedBranch={selectedBranch}
      selectedHostname={selectedHostname}
      selectedPR={selectedPR}
      selectedCommit={selectedCommit}
      repoCounts={repoCounts}
      branchCounts={branchCounts}
      hostnameCounts={hostnameCounts}
      prCounts={prCounts}
      commitCounts={commitCounts}
      totalCount={totalCount}
      searchQuery={searchQuery}
      onRepoClick={handleRepoClick}
      onBranchClick={handleBranchClick}
      onHostnameClick={handleHostnameClick}
      onPRClick={handlePRClick}
      onCommitClick={handleCommitClick}
      onSearchChange={setSearchQuery}
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

export const WithHostnameSelected: Story = {
  args: {
    initialHostname: 'macbook-pro',
  },
};

export const WithSearchQuery: Story = {
  args: {
    initialSearchQuery: 'auth',
  },
};

export const NoHostnames: Story = {
  args: {
    hostnames: [],
    hostnameCounts: {},
  },
};

export const AllFiltersActive: Story = {
  args: {
    initialRepo: 'confab-web',
    initialBranch: 'main',
    initialHostname: 'macbook-pro',
    initialPR: '142',
    initialCommit: 'a1b2c3d4e5f6789',
    initialSearchQuery: 'test',
  },
};

export const WithPRSelected: Story = {
  args: {
    initialRepo: 'confab-web',
    initialPR: '142',
  },
};

export const WithCommitSelected: Story = {
  args: {
    initialRepo: 'confab-web',
    initialCommit: 'a1b2c3d4e5f6789',
  },
};

export const WithPRAndCommitSelected: Story = {
  args: {
    initialRepo: 'confab-web',
    initialPR: '142',
    initialCommit: 'a1b2c3d4e5f6789',
  },
};

export const NoPRsOrCommits: Story = {
  args: {
    prs: [],
    commits: [],
    prCounts: {},
    commitCounts: {},
    initialRepo: 'confab-web',
  },
};
