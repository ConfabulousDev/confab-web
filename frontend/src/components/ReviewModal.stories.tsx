import type { Meta, StoryObj } from '@storybook/react';
import ReviewModal from './ReviewModal';

const meta: Meta<typeof ReviewModal> = {
  title: 'Components/ReviewModal',
  component: ReviewModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof ReviewModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
