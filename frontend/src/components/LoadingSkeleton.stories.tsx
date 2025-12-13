import type { Meta, StoryObj } from '@storybook/react-vite';
import LoadingSkeleton from './LoadingSkeleton';

const meta: Meta<typeof LoadingSkeleton> = {
  title: 'Components/LoadingSkeleton',
  component: LoadingSkeleton,
  parameters: {
    layout: 'padded',
  },
  argTypes: {
    variant: {
      control: 'select',
      options: ['card', 'text', 'list'],
    },
    count: {
      control: { type: 'number', min: 1, max: 10 },
    },
  },
};

export default meta;
type Story = StoryObj<typeof LoadingSkeleton>;

export const Card: Story = {
  args: {
    variant: 'card',
    count: 1,
  },
};

export const MultipleCards: Story = {
  args: {
    variant: 'card',
    count: 3,
  },
};

export const Text: Story = {
  args: {
    variant: 'text',
    count: 3,
  },
};

export const TextParagraph: Story = {
  args: {
    variant: 'text',
    count: 5,
  },
};

export const List: Story = {
  args: {
    variant: 'list',
    count: 4,
  },
};

export const AllVariants: Story = {
  render: () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '32px' }}>
      <div>
        <h3 style={{ marginBottom: '12px', color: '#666' }}>Card Skeleton</h3>
        <LoadingSkeleton variant="card" count={2} />
      </div>
      <div>
        <h3 style={{ marginBottom: '12px', color: '#666' }}>Text Skeleton</h3>
        <LoadingSkeleton variant="text" count={4} />
      </div>
      <div>
        <h3 style={{ marginBottom: '12px', color: '#666' }}>List Skeleton</h3>
        <LoadingSkeleton variant="list" count={3} />
      </div>
    </div>
  ),
};
