import type { Meta, StoryObj } from '@storybook/react';
import TermsPage from './TermsPage';

const meta: Meta<typeof TermsPage> = {
  title: 'Pages/TermsPage',
  component: TermsPage,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      // Simulate the app's main element layout (flex: 1, min-height: 0)
      <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
        <div style={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <Story />
        </div>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof TermsPage>;

export const Default: Story = {};
