import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexSummaryEmpty from './CodexSummaryEmpty';

const meta: Meta<typeof CodexSummaryEmpty> = {
  title: 'Session/CodexSummaryEmpty',
  component: CodexSummaryEmpty,
};

export default meta;
type Story = StoryObj<typeof CodexSummaryEmpty>;

export const Default: Story = {};
