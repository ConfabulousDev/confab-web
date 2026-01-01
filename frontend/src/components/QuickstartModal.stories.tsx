import type { Meta, StoryObj } from '@storybook/react';
import QuickstartModal from './QuickstartModal';

const meta: Meta<typeof QuickstartModal> = {
  title: 'Components/QuickstartModal',
  component: QuickstartModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof QuickstartModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
