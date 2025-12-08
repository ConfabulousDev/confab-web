import type { Meta, StoryObj } from '@storybook/react-vite';
import SessionStatsSidebar from './SessionStatsSidebar';

const meta = {
  title: 'Session/SessionStatsSidebar',
  component: SessionStatsSidebar,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <div style={{ display: 'flex', height: '100vh', background: '#fafafa' }}>
        <Story />
        <div style={{ flex: 1, padding: '24px', color: '#666' }}>
          Main content area
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof SessionStatsSidebar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
