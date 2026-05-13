import type { Meta, StoryObj } from '@storybook/react-vite';
import EditedFileSnippet from './EditedFileSnippet';

const meta: Meta<typeof EditedFileSnippet> = {
  title: 'Transcript/Attachments/EditedFileSnippet',
  component: EditedFileSnippet,
  parameters: { layout: 'padded' },
};
export default meta;

type Story = StoryObj<typeof EditedFileSnippet>;

export const TypeScript: Story = {
  args: {
    attachment: {
      type: 'edited_text_file',
      filename: '/home/user/project/src/example.ts',
      snippet:
        '     1\timport { useState } from \'react\';\n' +
        '     2\t\n' +
        '     3\texport function Counter() {\n' +
        '     4\t  const [n, setN] = useState(0);\n' +
        '     5\t  return <button onClick={() => setN(n + 1)}>{n}</button>;\n' +
        '     6\t}\n',
    },
  },
};

export const Markdown: Story = {
  args: {
    attachment: {
      type: 'edited_text_file',
      filename: '/home/user/project/README.md',
      snippet:
        '     1\t# Example Project\n' +
        '     2\t\n' +
        '     3\tA short description of the example project.\n' +
        '     4\t\n' +
        '     5\t## Getting started\n' +
        '     6\t\n' +
        '     7\t1. Clone the repo\n' +
        '     8\t2. Install dependencies\n' +
        '     9\t3. Run the dev server\n',
    },
  },
};
