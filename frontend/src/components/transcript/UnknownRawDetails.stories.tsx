import type { Meta, StoryObj } from '@storybook/react-vite';
import UnknownRawDetails from './UnknownRawDetails';
import ReportUnknownButton from './ReportUnknownButton';

const meta: Meta<typeof UnknownRawDetails> = {
  title: 'Transcript/UnknownRawDetails',
  component: UnknownRawDetails,
};

export default meta;
type Story = StoryObj<typeof UnknownRawDetails>;

const rawText = JSON.stringify(
  { type: 'future_top_level_type', payload: { some: 'shape', nested: { a: 1 } } },
  null,
  2,
);

export const Default: Story = {
  args: {
    label: 'Unrecognized line',
    rawText,
    summaryAside: <span>unrecognized top-level line type</span>,
    actions: (
      <ReportUnknownButton
        descriptor={{
          provider: 'codex',
          surface: 'line',
          type: 'future_top_level_type',
          reason: 'unrecognized top-level line type',
          keyFingerprint: ['type', 'payload', 'payload.some', 'payload.nested'],
        }}
      />
    ),
  },
};

export const Selected: Story = {
  args: { ...Default.args, isSelected: true },
};

export const DeepLinkTarget: Story = {
  args: { ...Default.args, isDeepLinkTarget: true },
};
