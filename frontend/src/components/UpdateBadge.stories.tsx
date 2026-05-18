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
    latest: 'v0.5.0',
    latestUrl: 'https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0',
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
