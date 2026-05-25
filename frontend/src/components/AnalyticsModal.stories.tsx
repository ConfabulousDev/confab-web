import type { Meta, StoryObj } from '@storybook/react';
import AnalyticsModal from './AnalyticsModal';

const meta: Meta<typeof AnalyticsModal> = {
  title: 'Components/AnalyticsModal',
  component: AnalyticsModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof AnalyticsModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
