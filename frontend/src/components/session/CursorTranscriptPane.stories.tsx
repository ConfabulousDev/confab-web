import type { Meta, StoryObj } from '@storybook/react-vite';
import { userEvent, within, waitFor, expect } from 'storybook/test';
import CursorTranscriptPane from './CursorTranscriptPane';
import type { CursorRenderItem } from './cursorCategories';
import {
  parseCursorJSONL,
  normalizeCursorLines,
} from '@/services/cursorTranscriptService';

const meta: Meta<typeof CursorTranscriptPane> = {
  title: 'Session/CursorTranscriptPane',
  component: CursorTranscriptPane,
  parameters: { layout: 'fullscreen' },
  decorators: [
    (Story) => (
      <div style={{ height: '600px', width: '720px', border: '1px solid var(--color-border)' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof CursorTranscriptPane>;

// Items mirror the real Cursor wire fixture: a user prompt, an assistant turn
// with text + tool calls (Read / Grep), then a follow-up. Tool rows show the
// call only (Cursor records inputs, never outputs).
const items: CursorRenderItem[] = [
  { kind: 'user', id: '0', text: 'Add input validation to the session handler and write a test for it.' },
  {
    kind: 'assistant',
    id: '1',
    text: 'Reading the handler and searching for the existing validation helpers.',
  },
  { kind: 'tool', id: '1-0', toolName: 'Read', input: 'internal/api/session_handler.go' },
  { kind: 'tool', id: '1-1', toolName: 'Grep', input: 'func validate' },
  {
    kind: 'assistant',
    id: '2',
    text: 'Found the validation seam. Editing the handler to reject empty session ids.',
  },
  { kind: 'tool', id: '2-0', toolName: 'StrReplace', input: 'internal/api/session_handler.go' },
];

export const Default: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
};

// Tool rows filtered out.
export const Filtered: Story = {
  args: {
    sessionId: 'demo',
    items,
    filteredItems: items.filter((it) => it.kind !== 'tool'),
    loading: false,
    error: null,
  },
};

export const SearchActive: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.keyboard('{Meta>}f{/Meta}');
    const input = await canvas.findByLabelText('Search transcript');
    await userEvent.type(input, 'validation');
    await waitFor(() => {
      if (canvasElement.querySelectorAll('mark').length === 0) {
        throw new Error('no highlight yet');
      }
    });
  },
  parameters: {
    docs: {
      description: {
        story:
          'Cmd-F opens the shared transcript search bar (useTranscriptSearch). Typing a query highlights matches inline across user / assistant / tool rows.',
      },
    },
  },
};

// fa3h: Cursor's on-disk JSONL appends a bare `[REDACTED]` to nearly every
// assistant turn — as a trailing suffix after narrative, or as the entire text
// block on tool-only turns. Normalizing these raw lines strips the placeholder:
// the first assistant row shows narrative only, and the second (text was only
// `[REDACTED]`) drops its assistant bubble while its Shell tool row still
// renders. A Confab CLI `[REDACTED:TYPE]` marker is a different contract and
// stays visible.
const redactedRawJSONL = [
  JSON.stringify({
    role: 'user',
    message: { content: [{ type: 'text', text: 'Check for open Dependabot alerts.' }] },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [
        {
          type: 'text',
          text: 'Checking the repo for open Dependabot alerts via the GitHub CLI.\n\n[REDACTED]',
        },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh api dependabot/alerts' } },
      ],
    },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: '[REDACTED]' },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh pr list --state open' } },
      ],
    },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [{ type: 'text', text: 'The token [REDACTED:GITHUB_TOKEN] is set in the env.' }],
    },
  }),
].join('\n');

const redactedItems = normalizeCursorLines(parseCursorJSONL(redactedRawJSONL).rawLines);

export const RedactedStripped: Story = {
  args: {
    sessionId: 'demo',
    items: redactedItems,
    filteredItems: redactedItems,
    loading: false,
    error: null,
  },
  parameters: {
    docs: {
      description: {
        story:
          "Cursor's native bare `[REDACTED]` is stripped during normalize (fa3h): trailing suffixes vanish, a `[REDACTED]`-only turn drops its assistant bubble but keeps its tool row, and a Confab CLI `[REDACTED:TYPE]` marker stays visible.",
      },
    },
  },
};

// nfbe: Cursor user `text` blocks arrive wrapped in a `<user_query>` envelope,
// often preceded by injected-context tags (here `<manually_attached_skills>`).
// Normalizing these raw lines extracts ONLY the human prompt into the user row
// — the envelope tags never render literally.
const envelopeRawJSONL = [
  JSON.stringify({
    role: 'user',
    message: {
      content: [
        {
          type: 'text',
          text: '<manually_attached_skills>\nThe user attached a skill with workflow instructions.\n</manually_attached_skills>\n<user_query>\ndoes gh repo have any outstanding dependabot alerts?\n</user_query>',
        },
      ],
    },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: 'Checking the repo for open Dependabot alerts.' },
        { type: 'tool_use', name: 'Shell', input: { command: 'gh api dependabot/alerts' } },
      ],
    },
  }),
].join('\n');

const envelopeItems = normalizeCursorLines(parseCursorJSONL(envelopeRawJSONL).rawLines);

export const UserQueryEnvelope: Story = {
  args: {
    sessionId: 'demo',
    items: envelopeItems,
    filteredItems: envelopeItems,
    loading: false,
    error: null,
  },
  parameters: {
    docs: {
      description: {
        story:
          'Cursor user messages arrive wrapped in a `<user_query>` envelope (often with injected-context tags like `<manually_attached_skills>`). Normalize extracts only the human prompt (nfbe) — the user row shows the prompt with no envelope tags.',
      },
    },
  },
};

// 0rcv: a Cursor user envelope often carries injected context (user rules,
// attached files, manually attached skills, system reminders) alongside the
// `<user_query>` prompt. nfbe parses those into `sections`; the user row shows
// the prompt prominently and folds each context block into a collapsed-by-
// default disclosure so the prompt reads cleanly while the audit context stays
// one click away.
const contextRawJSONL = [
  JSON.stringify({
    role: 'user',
    message: {
      content: [
        {
          type: 'text',
          text:
            '<user_rules>\nAlways prefer the latest stable versions of libraries.\nNever hardcode colors — use CSS custom properties.\n</user_rules>\n' +
            '<manually_attached_skills>\nThe user attached a skill with workflow instructions.\n</manually_attached_skills>\n' +
            '<attached_files>\nsrc/components/session/CursorTranscriptPane.tsx\nsrc/services/cursorTranscriptService.ts\n</attached_files>\n' +
            '<user_query>\nadd collapsible context sections to the Cursor user row\n</user_query>',
        },
      ],
    },
  }),
  JSON.stringify({
    role: 'assistant',
    message: {
      content: [
        { type: 'text', text: 'Reading the pane and the transcript service to find the user-row seam.' },
        { type: 'tool_use', name: 'Read', input: { path: 'src/components/session/CursorTranscriptPane.tsx' } },
      ],
    },
  }),
].join('\n');

const contextItems = normalizeCursorLines(parseCursorJSONL(contextRawJSONL).rawLines);

export const UserContextSections: Story = {
  args: {
    sessionId: 'demo',
    items: contextItems,
    filteredItems: contextItems,
    loading: false,
    error: null,
  },
  parameters: {
    docs: {
      description: {
        story:
          'The user row shows the extracted prompt prominently; injected-context blocks (user rules, manually attached skills, attached files) render as collapsed-by-default disclosures (0rcv). The `<user_query>` tag never appears inside a section body.',
      },
    },
  },
};

export const UserContextSectionsExpanded: Story = {
  args: {
    sessionId: 'demo',
    items: contextItems,
    filteredItems: contextItems,
    loading: false,
    error: null,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const toggle = await canvas.findByText('User rules');
    await userEvent.click(toggle);
    await waitFor(() => {
      if (!canvasElement.textContent?.includes('latest stable versions')) {
        throw new Error('section not expanded yet');
      }
    });
  },
  parameters: {
    docs: {
      description: {
        story:
          'Clicking a disclosure expands its injected-context block to show the raw preformatted body (0rcv). Bodies are plain text in v1 — rich rendering of attached-file contents is a follow-up.',
      },
    },
  },
};

// pt81: narrative rows (user prompt + assistant text) render through the
// shared markdown pipeline (CursorMessageBody). The assistant Dependabot
// final-response shows a rendered pipe table, bold repo name, `###` headers,
// and a clickable link instead of literal monospace markdown source. Tool rows
// and the user prompt's inline code render too; tool input summaries stay
// monospace `<pre>`.
const markdownItems: CursorRenderItem[] = [
  {
    kind: 'user',
    id: '0',
    text: 'Check for open Dependabot alerts and summarize by severity. Run `gh api`.',
  },
  {
    kind: 'assistant',
    id: '1',
    text: [
      'Yes — **ConfabulousDev/confab-web** has **14 open Dependabot alerts**:',
      '',
      '| Severity | Count |',
      '|----------|-------|',
      '| High     | 8     |',
      '| Moderate | 4     |',
      '| Low      | 2     |',
      '',
      '### Next steps',
      '',
      'Full list: [github.com/ConfabulousDev/confab-web/security/dependabot](https://github.com/ConfabulousDev/confab-web/security/dependabot)',
    ].join('\n'),
  },
  { kind: 'tool', id: '1-0', toolName: 'Shell', input: 'gh api dependabot/alerts' },
];

export const MarkdownNarrative: Story = {
  args: {
    sessionId: 'demo',
    items: markdownItems,
    filteredItems: markdownItems,
    loading: false,
    error: null,
  },
  parameters: {
    docs: {
      description: {
        story:
          'pt81: user/assistant narrative rows render markdown (bold, pipe tables, `###` headers, links) via the shared CursorMessageBody pipeline. Tool input summaries stay monospace `<pre>`.',
      },
    },
  },
};

// pt81: Cmd-F over a markdown-rendered assistant row still produces a `<mark>`
// — search runs against the rendered HTML (highlightTextInHtml), not the raw
// markdown source, so scroll-to-mark keeps working.
export const SearchActiveInMarkdown: Story = {
  args: {
    sessionId: 'demo',
    items: markdownItems,
    filteredItems: markdownItems,
    loading: false,
    error: null,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.keyboard('{Meta>}f{/Meta}');
    const input = await canvas.findByLabelText('Search transcript');
    await userEvent.type(input, 'Dependabot');
    await waitFor(() => {
      if (canvasElement.querySelectorAll('mark').length === 0) {
        throw new Error('no highlight yet');
      }
    });
  },
  parameters: {
    docs: {
      description: {
        story:
          'pt81: searching a term that appears in a markdown-rendered assistant row wraps the match in `<mark>` inside the rendered HTML, preserving Cmd-F and scroll-to-match.',
      },
    },
  },
};

// ce79: Cursor JSONL has no per-message time, so each row shows an ESTIMATED
// time interpolated over the session's [firstSeen, lastSyncAt] bounds. The first
// row sits at firstSeen, the last at lastSyncAt, the rest evenly between; tool
// rows inherit their parent assistant line's time. A muted `~` prefix and a
// tooltip flag the estimate. Passing the bounds drives the interpolation.
export const EstimatedTimestamps: Story = {
  args: {
    sessionId: 'demo',
    items,
    filteredItems: items,
    loading: false,
    error: null,
    firstSeen: '2026-06-17T10:00:00.000Z',
    lastSyncAt: '2026-06-17T10:06:00.000Z',
  },
  play: async ({ canvasElement }) => {
    await waitFor(() => {
      const times = canvasElement.querySelectorAll('[title^="Estimated"]');
      if (times.length === 0) throw new Error('no estimated times rendered yet');
    });
  },
  parameters: {
    docs: {
      description: {
        story:
          'ce79: per-row estimated times interpolated over the session bounds (firstSeen → lastSyncAt). Times increase monotonically down the transcript; the assistant row and its Read/Grep tool rows share one line time. The `~` prefix and the "Estimated — Cursor transcripts have no per-message timestamps." tooltip mark them as estimates, not real per-message timestamps.',
      },
    },
  },
};

// a9gr: every row carries a per-row action cluster in its header-right slot —
// copy-text (raw row payload) + copy-link (deep link to the row) + same-kind
// skip-prev/next. Copy-link writes `${origin}/sessions/<id>?tab=transcript&msg=
// <item.id>` (the synthetic stable id, NOT the estimated timestamp). Copy-text
// is hidden on rows with no payload; skip buttons hide at the ends of a chain.
export const RowActionsCluster: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // Every row renders a copy-link button.
    const copyLinks = await canvas.findAllByLabelText('Copy link to row');
    expect(copyLinks.length).toBeGreaterThan(0);
    // Rows with a payload render a copy-text button too.
    const copyTexts = canvas.getAllByLabelText('Copy text');
    expect(copyTexts.length).toBeGreaterThan(0);
    // Stub the clipboard so the copy click doesn't throw in the iframe, then
    // assert copy-link writes the deep-link URL addressed by the row id.
    const writes: string[] = [];
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: (t: string) => { writes.push(t); return Promise.resolve(); } },
      writable: true,
      configurable: true,
    });
    await userEvent.click(copyLinks[0]!);
    await waitFor(() => {
      if (!writes.some((w) => w.includes('tab=transcript&msg='))) {
        throw new Error('copy-link did not write a deep-link URL');
      }
    });
  },
  parameters: {
    docs: {
      description: {
        story:
          'a9gr: per-row action cluster (copy text / copy link / same-kind skip nav), shared with Codex via the provider-agnostic RowActions. Copy-link deep-links to the row by its synthetic stable id.',
      },
    },
  },
};

// a9gr: same-kind skip nav. Clicking a row's "Next assistant message" jumps to
// the next assistant row (over the filtered list); first/last-of-kind rows hide
// the corresponding button.
export const SkipNavigation: Story = {
  args: { sessionId: 'demo', items, filteredItems: items, loading: false, error: null },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const nextAssistant = await canvas.findAllByLabelText('Next assistant message');
    expect(nextAssistant.length).toBeGreaterThan(0);
    await userEvent.click(nextAssistant[0]!);
  },
  parameters: {
    docs: {
      description: {
        story:
          'a9gr: prev/next skip nav jumps between rows of the same kind (user→user, assistant→assistant, tool→tool) over the filtered list, scrolling via the virtualizer.',
      },
    },
  },
};

export const Loading: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: true, error: null },
};

export const Empty: Story = {
  args: { sessionId: 'demo', items: [], filteredItems: [], loading: false, error: null },
};
