import type { Meta, StoryObj } from '@storybook/react';
import HowItWorksModal from './HowItWorksModal';

const meta: Meta<typeof HowItWorksModal> = {
  title: 'Components/HowItWorksModal',
  component: HowItWorksModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof HowItWorksModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
