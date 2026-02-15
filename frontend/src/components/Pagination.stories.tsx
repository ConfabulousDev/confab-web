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

export const ManyPages: Story = {
  args: {
    page: 8,
    pageSize: 50,
    total: 1200,
    onPageChange: () => {},
  },
};

export const LastOfManyPages: Story = {
  args: {
    page: 24,
    pageSize: 50,
    total: 1200,
    onPageChange: () => {},
  },
};

export const FewPages: Story = {
  args: {
    page: 2,
    pageSize: 50,
    total: 120,
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
