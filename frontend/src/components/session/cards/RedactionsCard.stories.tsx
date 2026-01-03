import type { Meta, StoryObj } from '@storybook/react-vite';
import { RedactionsCard } from './RedactionsCard';

const meta: Meta<typeof RedactionsCard> = {
  title: 'Session/Cards/RedactionsCard',
  component: RedactionsCard,
  parameters: { layout: 'centered' },
  decorators: [
    (Story) => (
      <div style={{ width: '280px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof RedactionsCard>;

export const Default: Story = {
  args: {
    data: {
      total_redactions: 8,
      redaction_counts: {
        GITHUB_TOKEN: 3,
        API_KEY: 2,
        PASSWORD: 2,
        AWS_SECRET: 1,
      },
    },
    loading: false,
  },
};

export const SingleType: Story = {
  args: {
    data: {
      total_redactions: 5,
      redaction_counts: {
        GITHUB_TOKEN: 5,
      },
    },
    loading: false,
  },
};

export const ManyTypes: Story = {
  args: {
    data: {
      total_redactions: 25,
      redaction_counts: {
        GITHUB_TOKEN: 8,
        AWS_ACCESS_KEY_ID: 5,
        AWS_SECRET_ACCESS_KEY: 4,
        API_KEY: 3,
        PASSWORD: 2,
        DATABASE_URL: 2,
        STRIPE_SECRET_KEY: 1,
      },
    },
    loading: false,
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
  },
};

export const Empty: Story = {
  args: {
    data: {
      total_redactions: 0,
      redaction_counts: {},
    },
    loading: false,
  },
};
