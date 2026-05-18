import type { Meta, StoryObj } from '@storybook/react-vite';
import CodexMessageImages from './CodexMessageImages';

const meta: Meta<typeof CodexMessageImages> = {
  title: 'Transcript/Codex/CodexMessageImages',
  component: CodexMessageImages,
};

export default meta;
type Story = StoryObj<typeof CodexMessageImages>;

// 80x80 checkerboard PNG inlined as base64 — same shape Codex writes to the
// rollout JSONL (`response_item.message.content[input_image]`), just small
// enough to keep the story file readable.
const CHECKERBOARD_PNG =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAFAAAABQCAYAAACOEfKtAAAA6ElEQVR4nO3XwQnDMAAEQdeT/iE9uKmkBcM+7EtGoO8ize+O1/v8XLlXz7/1jqc/8Ok9gAABTvcAAgQ43QMI8GbAX/nIXT2AsQcw9gDGHsDYAxh7AGMPYOwBjD2AsWfKxR5AgACnewABApzuAQQIcLpnysUewNgDGHsAYw9g7AGMPYCxBzD2AMYewNgz5WIPIECA0z2AAAFO9wACBDjdM+ViD2DsAYw9gLEHMPYAxh7A2AMYewBjD2DsmXKxBxAgwOkeQIAAp3sAAQKc7plysQcw9gDGHsDYAxh7AGMPYOwBjD2AsQcw9r7VCh/edp941wAAAABJRU5ErkJggg==';

// CF-388: user-attached images. The `altPrefix` distinction is load-bearing
// for screen readers — alt-text reads as "User-attached image #1", etc.
export const UserAttached: Story = {
  args: {
    images: [CHECKERBOARD_PNG, CHECKERBOARD_PNG, CHECKERBOARD_PNG],
    altPrefix: 'User-attached image',
  },
};

// CF-388: assistant-generated images flip the alt-prefix so the
// screen-reader text reads as "Assistant-generated image #1".
export const AssistantGenerated: Story = {
  args: {
    images: [CHECKERBOARD_PNG],
    altPrefix: 'Assistant-generated image',
  },
};
