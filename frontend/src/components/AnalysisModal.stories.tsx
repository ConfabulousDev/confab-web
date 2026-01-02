import type { Meta, StoryObj } from '@storybook/react';
import AnalysisModal from './AnalysisModal';

const meta: Meta<typeof AnalysisModal> = {
  title: 'Components/AnalysisModal',
  component: AnalysisModal,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof AnalysisModal>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};
