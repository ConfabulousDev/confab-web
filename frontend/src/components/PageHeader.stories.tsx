import type { Meta, StoryObj } from '@storybook/react';
import PageHeader from './PageHeader';
import Pagination from './Pagination';
import { RefreshIcon } from './icons';
import sessionsStyles from '@/pages/SessionsPage.module.css';

const meta: Meta<typeof PageHeader> = {
  title: 'Components/PageHeader',
  component: PageHeader,
  parameters: { layout: 'fullscreen' },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof PageHeader>;

export const TitleOnly: Story = {
  args: {
    leftContent: <h1 className={sessionsStyles.title}>Sessions</h1>,
  },
};

// Shows refresh + Pagination side-by-side so the matched
// `--control-height` (28px) is visually verifiable.
export const WithToolbarActions: Story = {
  args: {
    leftContent: <h1 className={sessionsStyles.title}>Sessions</h1>,
    actions: (
      <div className={sessionsStyles.toolbarActions}>
        <Pagination hasMore canGoPrev onNext={() => {}} onPrev={() => {}} />
        <button
          className={sessionsStyles.refreshBtn}
          aria-label="Refresh"
          title="Refresh"
        >
          {RefreshIcon}
        </button>
      </div>
    ),
  },
};

export const RefreshOnlyFirstPage: Story = {
  args: {
    leftContent: <h1 className={sessionsStyles.title}>Sessions</h1>,
    actions: (
      <div className={sessionsStyles.toolbarActions}>
        <Pagination hasMore canGoPrev={false} onNext={() => {}} onPrev={() => {}} />
        <button
          className={sessionsStyles.refreshBtn}
          aria-label="Refresh"
          title="Refresh"
        >
          {RefreshIcon}
        </button>
      </div>
    ),
  },
};

export const TitleAndSubtitle: Story = {
  args: {
    title: 'Trends',
    subtitle: 'Last 30 days',
  },
};
