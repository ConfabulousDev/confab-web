import type { Meta, StoryObj } from '@storybook/react';
import SelfHostedModal from './SelfHostedModal';

const meta: Meta<typeof SelfHostedModal> = {
  title: 'Components/SelfHostedModal',
  component: SelfHostedModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof SelfHostedModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
