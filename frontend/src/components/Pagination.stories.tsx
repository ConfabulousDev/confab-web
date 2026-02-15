import type { Meta, StoryObj } from '@storybook/react';
import Pagination from './Pagination';

const meta: Meta<typeof Pagination> = {
  title: 'Components/Pagination',
  component: Pagination,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof Pagination>;

export const FirstPage: Story = {
  args: {
    hasMore: true,
    canGoPrev: false,
    onNext: () => {},
    onPrev: () => {},
  },
};

export const MiddlePage: Story = {
  args: {
    hasMore: true,
    canGoPrev: true,
    onNext: () => {},
    onPrev: () => {},
  },
};

export const LastPage: Story = {
  args: {
    hasMore: false,
    canGoPrev: true,
    onNext: () => {},
    onPrev: () => {},
  },
};

export const SinglePage: Story = {
  args: {
    hasMore: false,
    canGoPrev: false,
    onNext: () => {},
    onPrev: () => {},
  },
  parameters: {
    docs: {
      description: {
        story: 'When there is only one page, the pagination component is not rendered.',
      },
    },
  },
};
