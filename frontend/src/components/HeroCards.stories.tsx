import type { Meta, StoryObj } from '@storybook/react';
import HeroCards from './HeroCards';

const meta: Meta<typeof HeroCards> = {
  title: 'Components/HeroCards',
  component: HeroCards,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof HeroCards>;

export const Default: Story = {};
