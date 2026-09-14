import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent } from 'storybook/test';
import Tooltip from './Tooltip';

const triggerStyle = {
  padding: '4px 10px',
  fontSize: 'var(--font-xs)',
  color: 'var(--color-text-primary)',
  background: 'var(--color-bg-primary)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius-md)',
  cursor: 'pointer',
};

const meta: Meta<typeof Tooltip> = {
  title: 'Components/Tooltip',
  component: Tooltip,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof Tooltip>;

/** Hover for ~300ms, or Tab onto the button (shown on load via keyboard focus). */
export const Default: Story = {
  render: () => (
    <Tooltip content="Explore the auth middleware">
      <button type="button" style={triggerStyle}>
        Hover or focus me
      </button>
    </Tooltip>
  ),
  play: async () => {
    await userEvent.tab();
  },
};

/** Long content wraps within 320px. */
export const LongContent: Story = {
  render: () => (
    <Tooltip
      content={
        <div>
          <strong>Reconcile token usage for provider batch 12 now</strong>
          <div>Status: running. Type: general-purpose. Model: claude-opus-5.</div>
          <div>Launched by Explore the auth middleware</div>
        </div>
      }
    >
      <button type="button" style={triggerStyle}>
        Long content
      </button>
    </Tooltip>
  ),
  play: async () => {
    await userEvent.tab();
  },
};

/** A trigger at the right edge: the tooltip clamps inside the viewport. */
export const NearViewportEdge: Story = {
  render: () => (
    <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
      <Tooltip content="This tooltip would overflow the right edge without clamping">
        <button type="button" style={triggerStyle}>
          Edge
        </button>
      </Tooltip>
    </div>
  ),
  play: async () => {
    await userEvent.tab();
  },
};
