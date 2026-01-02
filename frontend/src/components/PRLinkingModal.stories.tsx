import type { Meta, StoryObj } from '@storybook/react';
import PRLinkingModal from './PRLinkingModal';

const meta: Meta<typeof PRLinkingModal> = {
  title: 'Components/PRLinkingModal',
  component: PRLinkingModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof PRLinkingModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
