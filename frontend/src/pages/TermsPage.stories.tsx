import type { Meta, StoryObj } from '@storybook/react';
import TermsPage from './TermsPage';

const meta: Meta<typeof TermsPage> = {
  title: 'Pages/TermsPage',
  component: TermsPage,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof TermsPage>;

export const Default: Story = {};
