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
    page: 1,
    pageSize: 50,
    total: 234,
    onPageChange: () => {},
  },
};

export const MiddlePage: Story = {
  args: {
    page: 3,
    pageSize: 50,
    total: 234,
    onPageChange: () => {},
  },
};

export const LastPage: Story = {
  args: {
    page: 5,
    pageSize: 50,
    total: 234,
    onPageChange: () => {},
  },
};

export const SinglePage: Story = {
  args: {
    page: 1,
    pageSize: 50,
    total: 30,
    onPageChange: () => {},
  },
  parameters: {
    docs: {
      description: {
        story: 'When total fits in one page, the pagination component is not rendered.',
      },
    },
  },
};
