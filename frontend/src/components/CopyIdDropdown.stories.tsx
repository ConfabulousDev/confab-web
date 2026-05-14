import type { Meta, StoryObj } from '@storybook/react-vite';
import CopyIdDropdown from './CopyIdDropdown';

const meta: Meta<typeof CopyIdDropdown> = {
  title: 'Components/CopyIdDropdown',
  component: CopyIdDropdown,
  parameters: {
    layout: 'centered',
  },
  args: {
    confabId: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    externalId: 'x9y8z7w6-v5u4-3210-fedc-ba0987654321',
  },
};

export default meta;
type Story = StoryObj<typeof CopyIdDropdown>;

/** Chip trigger showing truncated UUID (used on session detail page) */
export const ChipTrigger: Story = {
  args: {
    showChip: true,
  },
};

/** Icon-only trigger (used on session list hover) */
export const IconTrigger: Story = {
  args: {
    showChip: false,
  },
};

/** Chip with a short ID */
export const ShortId: Story = {
  args: {
    showChip: true,
    confabId: 'abcd1234',
    externalId: 'efgh5678',
  },
};

/** Codex provider: dropdown reads "Copy Codex ID" with "for codex resume" hint */
export const CodexProvider: Story = {
  args: {
    showChip: true,
    provider: 'codex',
    externalId: '019e23cc-fixture-codex-rollout-uuid-aaaa',
  },
};
