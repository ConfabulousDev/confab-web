import type { Meta, StoryObj } from '@storybook/react';
import PoliciesPage from './PoliciesPage';

const meta: Meta<typeof PoliciesPage> = {
  title: 'Pages/PoliciesPage',
  component: PoliciesPage,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof PoliciesPage>;

export const Default: Story = {};
