import type { Meta, StoryObj } from '@storybook/react-vite';
import UpdateBadgeView from './UpdateBadgeView';

const meta: Meta<typeof UpdateBadgeView> = {
  title: 'Components/UpdateBadge',
  component: UpdateBadgeView,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof UpdateBadgeView>;

export const UpdateAvailable: Story = {
  args: {
    show: true,
    current: 'v0.4.1',
    latest: 'v0.4.3',
    latestUrl: 'https://github.com/ConfabulousDev/confab-web/releases/tag/v0.4.3',
    severity: 'available',
  },
  parameters: {
    docs: {
      description: {
        story: 'Patch behind (v0.4.1 → v0.4.3): the regular orange badge.',
      },
    },
  },
};

export const UpdateRecommended: Story = {
  args: {
    show: true,
    current: 'v0.4.1',
    latest: 'v0.5.0',
    latestUrl: 'https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0',
    severity: 'recommended',
  },
  parameters: {
    docs: {
      description: {
        story:
          'Minor or major behind (v0.4.1 → v0.5.0): the red "Update recommended" badge, signalling the self-hosted backend has likely been outrun by auto-updating CLIs.',
      },
    },
  },
};

export const Dev: Story = {
  args: {
    show: true,
    current: '',
    latest: 'v0.5.0',
    latestUrl: 'https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0',
  },
  parameters: {
    docs: {
      description: {
        story: 'Local-dev rendering: backend has no version (e.g. `go run`), tooltip reads `(dev) → vX.Y.Z`.',
      },
    },
  },
};

export const Hidden: Story = {
  args: {
    show: false,
    current: 'v0.5.0',
    latest: 'v0.5.0',
    latestUrl: 'https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0',
  },
  parameters: {
    docs: {
      description: {
        story: 'When up-to-date the component renders nothing.',
      },
    },
  },
};
