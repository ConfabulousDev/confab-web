import type { Meta, StoryObj } from '@storybook/react';
import OrgCostMetricsModal from './OrgCostMetricsModal';

const meta: Meta<typeof OrgCostMetricsModal> = {
  title: 'Components/OrgCostMetricsModal',
  component: OrgCostMetricsModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof OrgCostMetricsModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
