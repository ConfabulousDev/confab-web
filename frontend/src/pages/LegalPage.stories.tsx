import type { Meta, StoryObj } from '@storybook/react';
import LegalPage from './LegalPage';

const meta: Meta<typeof LegalPage> = {
  title: 'Pages/LegalPage',
  component: LegalPage,
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj<typeof LegalPage>;

export const Default: Story = {};
