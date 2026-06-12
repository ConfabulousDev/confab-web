import type { Meta, StoryObj } from '@storybook/react-vite';
import { CostAmount } from './CostAmount';

const meta: Meta<typeof CostAmount> = {
  title: 'Components/CostAmount',
  component: CostAmount,
  parameters: {
    layout: 'centered',
  },
  argTypes: {
    usd: { control: 'number' },
  },
};

export default meta;
type Story = StoryObj<typeof CostAmount>;

export const Normal: Story = {
  args: { usd: 1.23 },
};

export const Large: Story = {
  args: { usd: 1234.5 },
};

export const Tiny: Story = {
  args: { usd: 0.004 },
};

export const Zero: Story = {
  args: { usd: 0 },
};

export const WithClassName: Story = {
  args: { usd: 42.0, className: 'demoBold' },
  decorators: [
    (Story) => (
      <>
        <style>{`.demoBold { font-weight: 700; }`}</style>
        <Story />
      </>
    ),
  ],
};
