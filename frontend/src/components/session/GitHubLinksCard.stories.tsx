import type { Meta, StoryObj } from '@storybook/react-vite';
import type { GitHubLink } from '@/services/api';
import GitHubLinksCard from './GitHubLinksCard';

const sampleLinks: GitHubLink[] = [
  {
    id: 1,
    session_id: 'session-1',
    link_type: 'pull_request',
    url: 'https://github.com/acme/widgets/pull/42',
    owner: 'acme',
    repo: 'widgets',
    ref: '42',
    title: 'Add widget polling',
    source: 'cli_hook',
    created_at: '2026-01-02T10:00:00Z',
  },
  {
    id: 2,
    session_id: 'session-1',
    link_type: 'commit',
    url: 'https://github.com/acme/widgets/commit/9f8e7d6c5b4a',
    owner: 'acme',
    repo: 'widgets',
    ref: '9f8e7d6c5b4a',
    title: null,
    source: 'manual',
    created_at: '2026-01-02T09:00:00Z',
  },
];

const meta: Meta<typeof GitHubLinksCard> = {
  title: 'Session/GitHubLinksCard',
  component: GitHubLinksCard,
  parameters: { layout: 'centered' },
  args: { sessionId: 'session-1' },
};

export default meta;
type Story = StoryObj<typeof GitHubLinksCard>;

export const OwnerWithLinks: Story = {
  args: { isOwner: true, forceShow: true, initialLinks: sampleLinks },
};

/** Revealed via the menu with no links: the add form opens automatically. */
export const OwnerEmptyAutoOpensAddForm: Story = {
  args: { isOwner: true, forceShow: true, initialLinks: [] },
};

export const ViewerWithLinks: Story = {
  args: { isOwner: false, initialLinks: sampleLinks },
};
