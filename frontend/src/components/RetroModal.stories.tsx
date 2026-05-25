import type { Meta, StoryObj } from '@storybook/react';
import RetroModal from './RetroModal';

const meta: Meta<typeof RetroModal> = {
  title: 'Components/RetroModal',
  component: RetroModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof RetroModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
