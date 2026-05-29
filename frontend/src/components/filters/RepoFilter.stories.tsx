import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import RepoFilter from './RepoFilter';

const meta: Meta<typeof RepoFilter> = {
  title: 'Filters/RepoFilter',
  component: RepoFilter,
  parameters: {
    layout: 'centered',
  },
};

export default meta;
type Story = StoryObj<typeof RepoFilter>;

const SAMPLE_REPOS = [
  'ConfabulousDev/confab-web',
  'ConfabulousDev/confab-cli',
  'ConfabulousDev/extensions',
];

// Stateful wrapper so the dropdown toggles and selections persist during
// interaction. Stories pass the initial state via props.
function Stateful({
  availableRepos = SAMPLE_REPOS,
  initialRepos = [],
  initialIncludeNoRepo = true,
}: {
  availableRepos?: string[];
  initialRepos?: string[];
  initialIncludeNoRepo?: boolean;
}) {
  const [repos, setRepos] = useState<string[]>(initialRepos);
  const [includeNoRepo, setIncludeNoRepo] = useState(initialIncludeNoRepo);
  return (
    <RepoFilter
      availableRepos={availableRepos}
      selectedRepos={repos}
      includeNoRepo={includeNoRepo}
      onChange={(next) => {
        setRepos(next.repos);
        setIncludeNoRepo(next.includeNoRepo);
      }}
    />
  );
}

// CF-233 / CF-506: empty repos[] is the "All Repos" default — the button label
// reads "All Repos" and no Clear affordance appears in the dropdown.
export const Default: Story = {
  render: () => <Stateful />,
};

// A subset is selected — label reads "N repos" and the Clear button appears.
export const RepoSubsetSelected: Story = {
  render: () => <Stateful initialRepos={['ConfabulousDev/confab-web']} />,
};

// No-repo sessions excluded — button is highlighted even with no repo subset.
export const IncludeNoRepoOff: Story = {
  render: () => <Stateful initialIncludeNoRepo={false} />,
};

// No repos available in range — only the Include-no-repo checkbox renders.
export const NoReposAvailable: Story = {
  render: () => <Stateful availableRepos={[]} />,
};
