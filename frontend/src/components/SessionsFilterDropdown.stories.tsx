import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import SessionsFilterDropdown from './SessionsFilterDropdown';

// Sample data for stories
const sampleRepos = ['confab-web', 'confab-cli', 'my-app'];
const sampleBranches = ['main', 'feature/auth', 'fix/bug-123', 'develop'];
const sampleHostnames = ['macbook-pro', 'desktop-linux', 'work-laptop'];
const samplePRs = ['142', '138', '125', '119'];

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

// Interactive wrapper component
function SessionsFilterDropdownInteractive({
  repos = sampleRepos,
  branches = sampleBranches,
  hostnames = sampleHostnames,
  prs = samplePRs,
  repoCounts = sampleRepoCounts,
  branchCounts = sampleBranchCounts,
  hostnameCounts = sampleHostnameCounts,
  prCounts = samplePRCounts,
  totalCount = 80,
  initialRepo = null,
  initialBranch = null,
  initialHostname = null,
  initialPR = null,
  initialCommitSearch = '',
  initialSearchQuery = '',
}: {
  repos?: string[];
  branches?: string[];
  hostnames?: string[];
  prs?: string[];
  repoCounts?: Record<string, number>;
  branchCounts?: Record<string, number>;
  hostnameCounts?: Record<string, number>;
  prCounts?: Record<string, number>;
  totalCount?: number;
  initialRepo?: string | null;
  initialBranch?: string | null;
  initialHostname?: string | null;
  initialPR?: string | null;
  initialCommitSearch?: string;
  initialSearchQuery?: string;
}) {
  const [selectedRepo, setSelectedRepo] = useState<string | null>(initialRepo);
  const [selectedBranch, setSelectedBranch] = useState<string | null>(initialBranch);
  const [selectedHostname, setSelectedHostname] = useState<string | null>(initialHostname);
  const [selectedPR, setSelectedPR] = useState<string | null>(initialPR);
  const [commitSearch, setCommitSearch] = useState<string>(initialCommitSearch);
  const [searchQuery, setSearchQuery] = useState<string>(initialSearchQuery);

  const handleRepoClick = (repo: string | null) => {
    setSelectedRepo(repo);
    // Reset repo-scoped filters when repo changes
    setSelectedBranch(null);
    setSelectedPR(null);
    setCommitSearch('');
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

  return (
    <SessionsFilterDropdown
      repos={repos}
      branches={branches}
      hostnames={hostnames}
      prs={prs}
      selectedRepo={selectedRepo}
      selectedBranch={selectedBranch}
      selectedHostname={selectedHostname}
      selectedPR={selectedPR}
      commitSearch={commitSearch}
      repoCounts={repoCounts}
      branchCounts={branchCounts}
      hostnameCounts={hostnameCounts}
      prCounts={prCounts}
      totalCount={totalCount}
      searchQuery={searchQuery}
      onRepoClick={handleRepoClick}
      onBranchClick={handleBranchClick}
      onHostnameClick={handleHostnameClick}
      onPRClick={handlePRClick}
      onCommitSearchChange={setCommitSearch}
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
      <div style={{ padding: '100px', background: 'var(--color-bg)' }}>
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
    initialCommitSearch: 'a1b2c3d',
    initialSearchQuery: 'test',
  },
};

export const WithPRSelected: Story = {
  args: {
    initialRepo: 'confab-web',
    initialPR: '142',
  },
};

export const WithCommitSearch: Story = {
  args: {
    initialRepo: 'confab-web',
    initialCommitSearch: 'a1b2c3d',
  },
};

export const WithPRAndCommitSearch: Story = {
  args: {
    initialRepo: 'confab-web',
    initialPR: '142',
    initialCommitSearch: 'a1b2c3d',
  },
};

export const NoPRs: Story = {
  args: {
    prs: [],
    prCounts: {},
    initialRepo: 'confab-web',
  },
};
