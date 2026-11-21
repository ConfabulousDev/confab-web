import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';
import {
  AdminSessionInsightsPageContent,
  type AdminSessionInsightsPageContentProps,
} from './AdminSessionInsightsPage';
import { fixturePoints, fixturePointsResponse, fixtureShare } from './sessionInsightsFixtures';

const baseProps: AdminSessionInsightsPageContentProps = {
  filters: { range: '7d', providers: [] },
  onFiltersChange: fn(),
  users: [
    { id: 1, email: 'ada@example.com' },
    { id: 2, email: 'grace@example.com' },
  ],
  share: fixtureShare,
  points: fixturePointsResponse(fixturePoints(120)),
  isLoading: false,
  error: null,
  onPointClick: fn(),
};

const meta: Meta<typeof AdminSessionInsightsPageContent> = {
  title: 'Pages/Admin/AdminSessionInsightsPage',
  component: AdminSessionInsightsPageContent,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AdminSessionInsightsPageContent>;

export const Default: Story = { args: baseProps };

export const Capped: Story = {
  args: { ...baseProps, points: fixturePointsResponse(fixturePoints(400), 18_250) },
};

export const Empty: Story = {
  args: {
    ...baseProps,
    share: { ...fixtureShare, buckets: [], totals: {} },
    points: fixturePointsResponse([]),
  },
};

const claudeOnly = fixturePoints(80).map((p) => ({ ...p, provider: 'claude-code' }));

export const SingleProvider: Story = {
  args: {
    ...baseProps,
    filters: { range: '7d', providers: ['claude-code'] },
    share: {
      ...fixtureShare,
      buckets: fixtureShare.buckets.map((b) => ({ ...b, counts: { 'claude-code': b.counts['claude-code'] ?? 0 } })),
      totals: { 'claude-code': 13 },
    },
    points: fixturePointsResponse(claudeOnly),
  },
};

export const Loading: Story = {
  args: { ...baseProps, share: null, points: null, isLoading: true },
};

export const ErrorState: Story = {
  args: { ...baseProps, share: null, points: null, error: 'Failed to load session insights.' },
};
