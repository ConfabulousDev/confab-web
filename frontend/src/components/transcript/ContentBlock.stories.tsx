import type { Meta, StoryObj } from '@storybook/react';
import ContentBlock from './ContentBlock';
import type { ContentBlock as ContentBlockType } from '@/types';

const meta: Meta<typeof ContentBlock> = {
  title: 'Transcript/ContentBlock',
  component: ContentBlock,
  parameters: {
    layout: 'padded',
    backgrounds: {
      default: 'app',
      values: [
        { name: 'app', value: 'var(--color-bg)' },
        { name: 'card', value: 'var(--color-bg-primary)' },
      ],
    },
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '800px', padding: '16px', background: 'var(--color-bg-primary)', borderRadius: '8px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof ContentBlock>;

// Text blocks
export const TextBlock: Story = {
  args: {
    block: {
      type: 'text',
      text: 'I\'ll help you implement that feature. Let me start by reading the relevant files.',
    },
  },
};

export const TextBlockWithMarkdown: Story = {
  args: {
    block: {
      type: 'text',
      text: `Here's what I found:

1. The **main component** is in \`src/components/App.tsx\`
2. The styles are in \`src/styles/main.css\`

\`\`\`typescript
function greet(name: string): string {
  return \`Hello, \${name}!\`;
}
\`\`\`

Let me know if you need more details.`,
    },
  },
};

// Tool use blocks
export const ToolUseRead: Story = {
  args: {
    block: {
      type: 'tool_use',
      id: 'tool_123',
      name: 'Read',
      input: {
        file_path: '/Users/santaclaude/dev/advent-of-code-2025/01_input.txt',
      },
    },
  },
};

export const ToolUseWrite: Story = {
  args: {
    block: {
      type: 'tool_use',
      id: 'tool_456',
      name: 'Write',
      input: {
        file_path: '/Users/santaclaude/dev/project/src/utils.ts',
        content: `export function formatDate(date: Date): string {
  return date.toISOString().split('T')[0];
}

export function parseDate(str: string): Date {
  return new Date(str);
}`,
      },
    },
  },
};

export const ToolUseBash: Story = {
  args: {
    block: {
      type: 'tool_use',
      id: 'tool_789',
      name: 'Bash',
      input: {
        command: 'npm run build && npm test',
        description: 'Build and test the project',
      },
    },
  },
};

export const ToolUseWebFetch: Story = {
  args: {
    block: {
      type: 'tool_use',
      id: 'tool_web',
      name: 'WebFetch',
      input: {
        url: 'https://api.github.com/repos/anthropics/claude-code',
        prompt: 'Get repository information',
      },
    },
  },
};

// Tool result blocks
export const ToolResultSuccess: Story = {
  args: {
    block: {
      type: 'tool_result',
      tool_use_id: 'tool_123',
      content: `{
  "file_path": "/Users/santaclaude/dev/advent-of-code-2025/01_input.txt"
}`,
      is_error: false,
    },
    toolName: 'Read',
  },
};

export const ToolResultWithCode: Story = {
  args: {
    block: {
      type: 'tool_result',
      tool_use_id: 'tool_456',
      content: `     1→import { useState } from 'react';
     2→import styles from './App.module.css';
     3→
     4→interface Props {
     5→  title: string;
     6→  count: number;
     7→}
     8→
     9→function App({ title, count }: Props) {
    10→  const [value, setValue] = useState(0);
    11→
    12→  return (
    13→    <div className={styles.container}>
    14→      <h1>{title}</h1>
    15→      <p>Count: {count}</p>
    16→      <button onClick={() => setValue(v => v + 1)}>
    17→        Clicked {value} times
    18→      </button>
    19→    </div>
    20→  );
    21→}
    22→
    23→export default App;`,
      is_error: false,
    },
    toolName: 'Read',
  },
};

export const ToolResultError: Story = {
  args: {
    block: {
      type: 'tool_result',
      tool_use_id: 'tool_err',
      content: 'Request failed with status code 400',
      is_error: true,
    },
    toolName: 'WebFetch',
  },
};

export const ToolResultBashOutput: Story = {
  args: {
    block: {
      type: 'tool_result',
      tool_use_id: 'tool_bash',
      content: `$ npm run build

> project@1.0.0 build
> tsc && vite build

vite v5.0.0 building for production...
✓ 142 modules transformed.
dist/index.html                   0.46 kB
dist/assets/index-DiwrgTda.css    1.21 kB │ gzip:  0.67 kB
dist/assets/index-CdlXj82-.js   142.63 kB │ gzip: 45.89 kB
✓ built in 1.23s

$ npm test

 PASS  src/App.test.tsx
 PASS  src/utils.test.ts

Test Suites: 2 passed, 2 total
Tests:       8 passed, 8 total
Snapshots:   0 total
Time:        2.341 s`,
      is_error: false,
    },
    toolName: 'Bash',
  },
};

export const ToolResultJSON: Story = {
  args: {
    block: {
      type: 'tool_result',
      tool_use_id: 'tool_json',
      content: JSON.stringify({
        name: 'claude-code',
        version: '1.0.0',
        dependencies: {
          react: '^18.2.0',
          typescript: '^5.0.0',
          vite: '^5.0.0',
        },
        scripts: {
          build: 'tsc && vite build',
          test: 'vitest',
          lint: 'eslint .',
        },
      }, null, 2),
      is_error: false,
    },
    toolName: 'Read',
  },
};

// Thinking block
export const ThinkingBlock: Story = {
  args: {
    block: {
      type: 'thinking',
      thinking: `Let me analyze the user's request. They want to implement a dark mode feature.

I should:
1. Check if there's an existing theme system
2. Look at the CSS variables being used
3. Create a toggle mechanism

This seems straightforward - I'll start by reading the main CSS file.`,
    },
  },
};

// Combined realistic example
export const RealisticToolSequence: Story = {
  render: () => {
    const blocks: Array<{ block: ContentBlockType; toolName?: string }> = [
      {
        block: {
          type: 'text',
          text: 'Let me read the configuration file first.',
        },
      },
      {
        block: {
          type: 'tool_use',
          id: 'tool_1',
          name: 'Read',
          input: {
            file_path: '/Users/santaclaude/dev/project/config.json',
          },
        },
      },
      {
        block: {
          type: 'tool_result',
          tool_use_id: 'tool_1',
          content: JSON.stringify({
            theme: 'light',
            features: {
              darkMode: false,
              animations: true,
            },
            api: {
              baseUrl: 'https://api.example.com',
              timeout: 5000,
            },
          }, null, 2),
          is_error: false,
        },
        toolName: 'Read',
      },
      {
        block: {
          type: 'text',
          text: 'I can see dark mode is currently disabled. Let me update the config and add the necessary CSS.',
        },
      },
    ];

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
        {blocks.map((item, i) => (
          <ContentBlock key={i} block={item.block} toolName={item.toolName} />
        ))}
      </div>
    );
  },
};

// All variants for comparison
export const AllToolResults: Story = {
  render: () => {
    const results: Array<{ block: ContentBlockType; toolName: string; label: string }> = [
      {
        label: 'Simple JSON',
        toolName: 'Read',
        block: {
          type: 'tool_result',
          tool_use_id: '1',
          content: '{"file_path": "/path/to/file.txt"}',
          is_error: false,
        },
      },
      {
        label: 'Code with line numbers',
        toolName: 'Read',
        block: {
          type: 'tool_result',
          tool_use_id: '2',
          content: `     1→const x: number = 42;
     2→const y: string = "hello";
     3→console.log(x, y);`,
          is_error: false,
        },
      },
      {
        label: 'Bash output',
        toolName: 'Bash',
        block: {
          type: 'tool_result',
          tool_use_id: '3',
          content: `$ ls -la
total 24
drwxr-xr-x  5 user  staff   160 Jan  3 10:00 .
drwxr-xr-x  3 user  staff    96 Jan  3 09:00 ..
-rw-r--r--  1 user  staff  1234 Jan  3 10:00 file.txt`,
          is_error: false,
        },
      },
      {
        label: 'Error',
        toolName: 'WebFetch',
        block: {
          type: 'tool_result',
          tool_use_id: '4',
          content: 'Request failed with status code 404: Not Found',
          is_error: true,
        },
      },
    ];

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
        {results.map((item, i) => (
          <div key={i}>
            <div style={{
              fontSize: '12px',
              color: 'var(--color-text-muted)',
              marginBottom: '8px',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
            }}>
              {item.label}
            </div>
            <ContentBlock block={item.block} toolName={item.toolName} />
          </div>
        ))}
      </div>
    );
  },
};
