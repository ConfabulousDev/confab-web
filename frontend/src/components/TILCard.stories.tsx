import type { Meta, StoryObj } from '@storybook/react-vite';
import TILCard from './TILCard';
import type { TILWithSession } from '@/schemas/api';

const baseTIL: TILWithSession = {
  id: 1,
  title: 'PostgreSQL EXISTS subqueries need qualified column names',
  summary: 'When writing EXISTS (SELECT 1 FROM table WHERE table.session_id = id), PostgreSQL resolves bare "id" to the FROM table\'s own column, not the outer query. Always qualify with the outer alias.',
  session_id: 'abc-123',
  message_uuid: 'msg-456',
  created_at: new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString(),
  session_title: 'Fix session linking bug',
  git_repo: 'confab-web',
  git_branch: 'main',
  owner_email: 'jackie@confab.dev',
  is_owner: true,
  access_type: 'owner',
};

const meta: Meta<typeof TILCard> = {
  title: 'Components/TILCard',
  component: TILCard,
  parameters: { layout: 'centered' },
  decorators: [
    (Story) => (
      <div style={{ width: 320, padding: '1rem' }}>
        <Story />
      </div>
    ),
  ],
  args: {
    onNavigate: () => console.log('navigate'),
    onDelete: () => console.log('delete'),
  },
};

export default meta;
type Story = StoryObj<typeof TILCard>;

export const Default: Story = {
  args: { til: baseTIL },
};

export const ShortContent: Story = {
  args: {
    til: {
      ...baseTIL,
      id: 2,
      title: 'Use -short for quick tests',
      summary: 'Skips integration tests.',
      session_title: null,
      git_repo: null,
      git_branch: null,
    },
  },
};

export const LongContent: Story = {
  args: {
    til: {
      ...baseTIL,
      id: 3,
      title: 'Integration tests require Docker containers for Postgres and MinIO to be running',
      summary: 'Use testutil.SetupTestEnvironment(t) which spins up containerized Postgres and MinIO. Sessions need total_lines > 0 AND (summary IS NOT NULL OR first_user_message IS NOT NULL) to be visible in the paginated list endpoint. Use testutil.CreateTestSessionFull() instead of CreateTestSession() when testing list endpoints.',
      session_title: 'Refactor test infrastructure for CI pipeline',
      git_branch: 'feature/ci-containers',
    },
  },
};

export const NonOwner: Story = {
  args: {
    til: {
      ...baseTIL,
      id: 4,
      is_owner: false,
      access_type: 'shared',
      owner_email: 'teammate@confab.dev',
    },
  },
};

export const MinimalMetadata: Story = {
  args: {
    til: {
      ...baseTIL,
      id: 5,
      session_title: null,
      git_repo: null,
      git_branch: null,
    },
  },
};

export const LongChipValues: Story = {
  args: {
    til: {
      ...baseTIL,
      id: 7,
      session_title: 'Refactoring the authentication middleware to support OAuth2 PKCE flow',
      owner_email: 'alexandra.richardson@engineering.confab.dev',
      git_repo: 'confabulous-monorepo-infrastructure',
      git_branch: 'feature/oauth2-pkce-token-exchange-implementation',
    },
  },
};

export const RecentlyCreated: Story = {
  args: {
    til: {
      ...baseTIL,
      id: 6,
      created_at: new Date(Date.now() - 30 * 1000).toISOString(),
    },
  },
};
