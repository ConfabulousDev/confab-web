import type { Meta, StoryObj } from '@storybook/react';
import ShareModal from './ShareModal';

const meta: Meta<typeof ShareModal> = {
  title: 'Components/ShareModal',
  component: ShareModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof ShareModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
