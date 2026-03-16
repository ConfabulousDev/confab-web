import type { Meta, StoryObj } from '@storybook/react';
import TILModal from './TILModal';

const meta: Meta<typeof TILModal> = {
  title: 'Components/TILModal',
  component: TILModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof TILModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
