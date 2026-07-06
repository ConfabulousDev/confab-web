import type { Meta, StoryObj } from '@storybook/react-vite';
import { TimeSeparator } from './TimeSeparator';

const meta: Meta<typeof TimeSeparator> = {
  title: 'Transcript/TimeSeparator',
  component: TimeSeparator,
};

export default meta;
type Story = StoryObj<typeof TimeSeparator>;

// Idle-gap divider (>5min, same day) — the pre-existing behavior this ticket
// (6h7m) preserves.
export const IdleGap: Story = {
  args: {
    label: '6:00 PM',
  },
};

// Day-boundary divider — the new behavior: full "Weekday, Month Day" text,
// shown even when the gap itself was small (e.g. 11:59pm → 12:01am).
export const DayBoundary: Story = {
  args: {
    label: 'Tuesday, July 7',
  },
};

// Cursor's estimated-timestamp treatment (Decision 8): muted `~` prefix +
// tooltip, reusing the same convention as Cursor's per-row time markers.
export const EstimatedDayBoundary: Story = {
  args: {
    label: 'Tuesday, July 7',
    estimated: true,
  },
};
