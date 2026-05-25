import type { Meta, StoryObj } from '@storybook/react';
import CTALinks from './CTALinks';

const meta: Meta<typeof CTALinks> = {
  title: 'Components/CTALinks',
  component: CTALinks,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof CTALinks>;

export const Default: Story = {};
