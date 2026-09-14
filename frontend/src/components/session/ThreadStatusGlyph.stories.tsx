import type { Meta, StoryObj } from '@storybook/react-vite';
import type { TranscriptThreadStatus } from '@/providers/types';
import ThreadStatusGlyph from './ThreadStatusGlyph';

const STATUSES: TranscriptThreadStatus[] = ['running', 'completed', 'failed', 'stopped', 'unknown'];

const meta: Meta<typeof ThreadStatusGlyph> = {
  title: 'Session/ThreadStatusGlyph',
  component: ThreadStatusGlyph,
  args: { status: 'running' },
  argTypes: { status: { control: 'select', options: STATUSES } },
};
export default meta;

type Story = StoryObj<typeof ThreadStatusGlyph>;

/** The running arc rotates; with the OS "reduce motion" setting on it stays a static ring. */
export const Running: Story = {};

/** Every status side by side, in both themes via the toolbar. Shapes differ so color is never the only cue. */
export const AllStatuses: Story = {
  render: () => (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 24, fontSize: 'var(--font-sm)', color: 'var(--color-text-secondary)' }}>
      {STATUSES.map((status) => (
        <span key={status} style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <ThreadStatusGlyph status={status} />
          {status}
        </span>
      ))}
    </div>
  ),
};
