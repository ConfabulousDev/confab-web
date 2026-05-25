import type { Meta, StoryObj } from '@storybook/react';
import MultiProviderModal from './MultiProviderModal';

const meta: Meta<typeof MultiProviderModal> = {
  title: 'Components/MultiProviderModal',
  component: MultiProviderModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof MultiProviderModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
