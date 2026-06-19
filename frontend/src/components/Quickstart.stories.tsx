import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router-dom';
import Quickstart from './Quickstart';

const meta: Meta<typeof Quickstart> = {
  title: 'Components/Quickstart',
  component: Quickstart,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof Quickstart>;

export const Embedded: Story = {
  decorators: [
    (Story) => (
      <div style={{ width: '600px', background: 'var(--color-bg-primary)', borderRadius: '8px' }}>
        <Story />
      </div>
    ),
  ],
};

export const Landing: Story = {
  args: { variant: 'landing' },
  decorators: [
    (Story) => (
      <div style={{ width: '1100px', background: 'var(--color-bg-secondary)', padding: '24px' }}>
        <Story />
      </div>
    ),
  ],
};
