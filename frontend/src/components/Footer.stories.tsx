import type { Meta, StoryObj } from '@storybook/react-vite';
import Footer from './Footer';

// Footer reads useAppConfig via context; the default context value
// (no Termly, empty support email) is enough to render the link row.
const meta: Meta<typeof Footer> = {
  title: 'Components/Footer',
  component: Footer,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof Footer>;

export const Default: Story = {};
