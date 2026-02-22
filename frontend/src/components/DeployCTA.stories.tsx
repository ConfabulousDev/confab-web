import type { Meta, StoryObj } from '@storybook/react';
import DeployCTA from './DeployCTA';

const meta: Meta<typeof DeployCTA> = {
  title: 'Components/DeployCTA',
  component: DeployCTA,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '780px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof DeployCTA>;

export const Default: Story = {};
