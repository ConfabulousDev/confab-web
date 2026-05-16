import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import ContentBlock from './ContentBlock';
import type { ContentBlock as ContentBlockType } from '@/types';

vi.mock('./CodeBlock', () => ({
  default: ({ code, language }: { code: string; language: string }) => (
    <pre data-testid="codeblock" data-language={language}>
      {code}
    </pre>
  ),
}));

vi.mock('./BashOutput', () => ({
  default: ({ output }: { output: string }) => (
    <pre data-testid="bash-output">{output}</pre>
  ),
}));

describe('ContentBlock', () => {
  it('renders text block as a rendered-html container with text content', () => {
    const block: ContentBlockType = { type: 'text', text: 'Hello **world**' };
    const { container, queryByTestId } = render(<ContentBlock block={block} />);
    expect(container.textContent).toContain('Hello');
    expect(queryByTestId('codeblock')).toBeNull();
  });

  it('routes JSON-parseable text to CodeBlock with language=json', () => {
    const block: ContentBlockType = {
      type: 'text',
      text: JSON.stringify({ ok: true, value: 42 }),
    };
    const { getByTestId } = render(<ContentBlock block={block} />);
    const cb = getByTestId('codeblock');
    expect(cb.getAttribute('data-language')).toBe('json');
  });

  it('returns null for thinking block whose content is whitespace-only', () => {
    const block: ContentBlockType = { type: 'thinking', thinking: '   \n  ' };
    const { container } = render(<ContentBlock block={block} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders header and content for non-empty thinking block', () => {
    const block: ContentBlockType = { type: 'thinking', thinking: 'pondering things' };
    const { container, getByText } = render(<ContentBlock block={block} />);
    expect(getByText('Thinking')).toBeInTheDocument();
    expect(container.textContent).toContain('pondering things');
  });

  it('renders tool_use with tool name and JSON-encoded input', () => {
    const block: ContentBlockType = {
      type: 'tool_use',
      id: 't1',
      name: 'Bash',
      input: { command: 'ls' },
    };
    const { getByText, getByTestId } = render(<ContentBlock block={block} />);
    expect(getByText('Bash')).toBeInTheDocument();
    const cb = getByTestId('codeblock');
    expect(cb.getAttribute('data-language')).toBe('json');
    expect(cb.textContent).toContain('"command"');
  });

  it('routes non-Bash string tool_result content to CodeBlock', () => {
    const block: ContentBlockType = {
      type: 'tool_result',
      tool_use_id: 't1',
      content: 'plain output line',
    };
    const { getByTestId, queryByTestId } = render(
      <ContentBlock block={block} toolName="Read" />
    );
    expect(getByTestId('codeblock')).toBeInTheDocument();
    expect(queryByTestId('bash-output')).toBeNull();
  });

  it('routes Bash tool_result content to BashOutput', () => {
    const block: ContentBlockType = {
      type: 'tool_result',
      tool_use_id: 't1',
      content: '$ ls\nfile.txt',
    };
    const { getByTestId } = render(<ContentBlock block={block} toolName="Bash" />);
    expect(getByTestId('bash-output')).toBeInTheDocument();
  });

  it('applies an error indicator on tool_result with is_error=true', () => {
    const block: ContentBlockType = {
      type: 'tool_result',
      tool_use_id: 't1',
      content: 'failed',
      is_error: true,
    };
    const { container } = render(<ContentBlock block={block} toolName="Read" />);
    expect(container.textContent).toContain('❌');
  });

  it('recursively renders array tool_result content', () => {
    const block: ContentBlockType = {
      type: 'tool_result',
      tool_use_id: 't1',
      content: [
        { type: 'text', text: 'first' },
        { type: 'text', text: 'second' },
      ],
    };
    const { container } = render(<ContentBlock block={block} toolName="Read" />);
    expect(container.textContent).toContain('first');
    expect(container.textContent).toContain('second');
  });

  it('renders image block with base64 src', () => {
    const block: ContentBlockType = {
      type: 'image',
      source: { type: 'base64', media_type: 'image/png', data: 'abc' },
    };
    const { container } = render(<ContentBlock block={block} />);
    const img = container.querySelector('img');
    expect(img?.getAttribute('src')).toBe('data:image/png;base64,abc');
  });

  it('renders image block with URL src', () => {
    const block: ContentBlockType = {
      type: 'image',
      source: {
        type: 'url',
        media_type: 'image/jpeg',
        url: 'https://example.com/x.jpg',
      },
    };
    const { container } = render(<ContentBlock block={block} />);
    const img = container.querySelector('img');
    expect(img?.getAttribute('src')).toBe('https://example.com/x.jpg');
  });

  it('renders tool_reference block with tool_name', () => {
    const block: ContentBlockType = { type: 'tool_reference', tool_name: 'WebFetch' };
    const { container, getByText } = render(<ContentBlock block={block} />);
    expect(getByText('WebFetch')).toBeInTheDocument();
    expect(container.textContent).toContain('🔗');
  });

  it('renders unknown block type with best-effort text fallback', () => {
    // Force an unrecognized discriminator to exercise the fallback branch.
    // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
    const block = { type: 'mystery_block', text: 'inner content' } as ContentBlockType;
    const { container } = render(<ContentBlock block={block} />);
    expect(container.textContent).toContain('Unknown content block: mystery_block');
    expect(container.textContent).toContain('inner content');
  });
});
