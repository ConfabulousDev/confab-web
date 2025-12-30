import type { Meta, StoryObj } from '@storybook/react-vite';
import { CodeActivityCard } from './CodeActivityCard';

const meta: Meta<typeof CodeActivityCard> = {
  title: 'Session/Cards/CodeActivityCard',
  component: CodeActivityCard,
  parameters: {
    layout: 'centered',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '280px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof CodeActivityCard>;

export const Default: Story = {
  args: {
    data: {
      files_read: 42,
      files_modified: 12,
      lines_added: 156,
      lines_removed: 23,
      search_count: 18,
      language_breakdown: {
        go: 28,
        ts: 18,
        css: 5,
        sql: 3,
      },
    },
    loading: false,
  },
};

export const ManyLanguages: Story = {
  args: {
    data: {
      files_read: 85,
      files_modified: 34,
      lines_added: 520,
      lines_removed: 180,
      search_count: 45,
      language_breakdown: {
        go: 45,
        ts: 25,
        tsx: 18,
        css: 12,
        sql: 8,
        md: 6,
        json: 4,
        yaml: 2,
      },
    },
    loading: false,
  },
};

export const SingleLanguage: Story = {
  args: {
    data: {
      files_read: 15,
      files_modified: 5,
      lines_added: 200,
      lines_removed: 50,
      search_count: 8,
      language_breakdown: {
        go: 20,
      },
    },
    loading: false,
  },
};

export const NoModifications: Story = {
  args: {
    data: {
      files_read: 25,
      files_modified: 0,
      lines_added: 0,
      lines_removed: 0,
      search_count: 12,
      language_breakdown: {
        ts: 15,
        tsx: 10,
      },
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Session where code was only read/searched, not modified.',
      },
    },
  },
};

export const HeavyModification: Story = {
  args: {
    data: {
      files_read: 8,
      files_modified: 45,
      lines_added: 1250,
      lines_removed: 890,
      search_count: 5,
      language_breakdown: {
        go: 35,
        sql: 10,
      },
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Session with heavy code modifications (refactoring).',
      },
    },
  },
};

export const SearchOnly: Story = {
  args: {
    data: {
      files_read: 0,
      files_modified: 0,
      lines_added: 0,
      lines_removed: 0,
      search_count: 25,
      language_breakdown: {},
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Session with only searches (Glob/Grep), no file reads or modifications.',
      },
    },
  },
};

export const NoActivity: Story = {
  args: {
    data: {
      files_read: 0,
      files_modified: 0,
      lines_added: 0,
      lines_removed: 0,
      search_count: 0,
      language_breakdown: {},
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'When no file activity, the card is not rendered (returns null).',
      },
    },
  },
};

export const Loading: Story = {
  args: {
    data: undefined,
    loading: true,
  },
};

export const NoLanguages: Story = {
  args: {
    data: {
      files_read: 5,
      files_modified: 2,
      lines_added: 50,
      lines_removed: 10,
      search_count: 3,
      language_breakdown: {},
    },
    loading: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'Files without extensions (e.g., Dockerfile, Makefile) - no language chart shown.',
      },
    },
  },
};
