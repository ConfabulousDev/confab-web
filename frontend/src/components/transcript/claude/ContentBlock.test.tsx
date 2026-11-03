import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import ContentBlock from './ContentBlock';
import type { ContentBlock as ContentBlockType } from '@/types';

vi.mock('../CodeBlock', () => ({
  default: ({ code, language }: { code: string; language: string }) => (
    <pre data-testid="codeblock" data-language={language}>
      {code}
    </pre>
  ),
}));

vi.mock('../BashOutput', () => ({
  default: ({ output, interrupted, returnCodeInterpretation, persistedOutputPath }: {
    output: string;
    interrupted?: boolean;
    returnCodeInterpretation?: string;
    persistedOutputPath?: string;
  }) => (
    <pre
      data-testid="bash-output"
      data-interrupted={String(!!interrupted)}
      data-rci={returnCodeInterpretation ?? ''}
      data-persisted-path={persistedOutputPath ?? ''}
    >
      {output}
    </pre>
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

  it('threads toolUseResult metadata into BashOutput for a Bash result', () => {
    const block: ContentBlockType = {
      type: 'tool_result',
      tool_use_id: 't1',
      content: 'no matches',
    };
    const { getByTestId } = render(
      <ContentBlock
        block={block}
        toolName="Bash"
        toolUseResult={{
          stdout: 'no matches',
          interrupted: true,
          returnCodeInterpretation: 'No matches found',
          persistedOutputPath: '/tmp/p/tool-results/abc.txt',
          persistedOutputSize: 1024,
        }}
      />
    );
    const out = getByTestId('bash-output');
    expect(out.getAttribute('data-interrupted')).toBe('true');
    expect(out.getAttribute('data-rci')).toBe('No matches found');
    expect(out.getAttribute('data-persisted-path')).toBe('/tmp/p/tool-results/abc.txt');
  });

  it('ignores toolUseResult metadata for a non-Bash result', () => {
    const block: ContentBlockType = {
      type: 'tool_result',
      tool_use_id: 't1',
      content: 'plain output line',
    };
    const { queryByTestId } = render(
      <ContentBlock
        block={block}
        toolName="Read"
        toolUseResult={{ interrupted: true }}
      />
    );
    // Non-Bash results route to CodeBlock, not BashOutput.
    expect(queryByTestId('bash-output')).toBeNull();
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

  describe('Artifact tool', () => {
    const publishUse: ContentBlockType = {
      type: 'tool_use',
      id: 'toolu_art',
      name: 'Artifact',
      input: {
        file_path: '/Users/me/dev/resume/resume.html',
        description: 'My resume',
        icon: 'document',
        edits: [{ old_str: 'a', new_str: 'b' }, { old_str: 'c', new_str: 'd' }],
        capabilities: { comments: {} },
      },
    };

    it('renders a publish call as a summary card with the raw input collapsed', () => {
      const { container, getByText } = render(<ContentBlock block={publishUse} />);
      expect(getByText('Publish')).toBeInTheDocument();
      expect(getByText('/Users/me/dev/resume/resume.html')).toBeInTheDocument();
      expect(getByText('My resume')).toBeInTheDocument();
      expect(getByText('2 edits')).toBeInTheDocument();
      expect(getByText('capabilities: comments')).toBeInTheDocument();
      const details = container.querySelector('details');
      expect(details?.open).toBe(false);
      expect(details?.querySelector('[data-testid="codeblock"]')?.getAttribute('data-language')).toBe('json');
    });

    it('auto-opens the raw input when the row is the current search match', () => {
      const { container } = render(<ContentBlock block={publishUse} isCurrentSearchMatch />);
      expect(container.querySelector('details')?.open).toBe(true);
    });

    it('labels non-publish actions and links claude.ai URLs', () => {
      const block: ContentBlockType = {
        type: 'tool_use',
        id: 'toolu_read',
        name: 'Artifact',
        input: { action: 'read', url: 'https://claude.ai/artifact/abc' },
      };
      const { getByText } = render(<ContentBlock block={block} />);
      expect(getByText('Read')).toBeInTheDocument();
      expect(getByText('https://claude.ai/artifact/abc').closest('a')?.getAttribute('href')).toBe('https://claude.ai/artifact/abc');
    });

    it('labels asset uploads and type-based creates', () => {
      const asset: ContentBlockType = {
        type: 'tool_use', id: 't1', name: 'Artifact',
        input: { url: 'https://claude.ai/artifact/abc', file_path: '/x.png', asset: true },
      };
      expect(render(<ContentBlock block={asset} />).getByText('Upload asset')).toBeInTheDocument();
      const fromType: ContentBlockType = {
        type: 'tool_use', id: 't2', name: 'Artifact',
        input: { type_url: 'https://claude.ai/artifact/type', title: 'Deck' },
      };
      expect(render(<ContentBlock block={fromType} />).getByText('Create from type')).toBeInTheDocument();
    });

    it('links any https URL', () => {
      const block: ContentBlockType = {
        type: 'tool_use',
        id: 'toolu_https',
        name: 'Artifact',
        input: { action: 'read', url: 'https://example.com/x' },
      };
      const link = render(<ContentBlock block={block} />).getByText('https://example.com/x').closest('a');
      expect(link?.getAttribute('href')).toBe('https://example.com/x');
      expect(link?.getAttribute('target')).toBe('_blank');
      expect(link?.getAttribute('rel')).toBe('noopener noreferrer');
    });

    it.each([
      'javascript:alert(1)',
      'http://example.com/x',
      'data:text/html,<script>alert(1)</script>',
      'not a url',
    ])('never turns a non-https URL into a link: %s', (url) => {
      const block: ContentBlockType = {
        type: 'tool_use',
        id: 'toolu_bad',
        name: 'Artifact',
        input: { action: 'open', url, type_url: url, from_url: url },
      };
      const { container, getAllByText } = render(<ContentBlock block={block} />);
      expect(getAllByText(url)).toHaveLength(3);
      expect(container.querySelector('a')).toBeNull();
    });

    it('renders a non-https result link as plain text', () => {
      const block: ContentBlockType = { type: 'tool_result', tool_use_id: 't', content: 'Published ...' };
      const { container, getByText } = render(
        <ContentBlock block={block} toolName="Artifact" artifactToolUseResult={{ url: 'javascript:alert(1)', title: 'Evil' }} />,
      );
      expect(getByText('Evil')).toBeInTheDocument();
      expect(container.querySelector('a')).toBeNull();
    });

    it('shows unknown Artifact fields as rows and chips', () => {
      const block: ContentBlockType = {
        type: 'tool_use',
        id: 'toolu_unknown',
        name: 'Artifact',
        input: {
          file_path: '/x.html',
          contract: '0.2.96',
          limit: 50,
          overwrite_unread: ['a.css', 'b.js'],
          root: '',
          extra: null,
        },
      };
      const { getByText, queryByText } = render(<ContentBlock block={block} />);
      expect(getByText('contract').nextElementSibling?.textContent).toBe('0.2.96');
      expect(getByText('limit').nextElementSibling?.textContent).toBe('50');
      expect(getByText('overwrite_unread: 2 items')).toBeInTheDocument();
      expect(queryByText('root')).toBeNull();
      expect(queryByText('extra')).toBeNull();
      // Hand-labeled fields are not repeated as generic rows.
      expect(queryByText('file_path')).toBeNull();
    });

    it('capitalizes an unknown action verbatim', () => {
      const block: ContentBlockType = { type: 'tool_use', id: 't', name: 'Artifact', input: { action: 'archive' } };
      expect(render(<ContentBlock block={block} />).getByText('Archive')).toBeInTheDocument();
    });

    it('leads a successful result with the artifact link and version', () => {
      const block: ContentBlockType = {
        type: 'tool_result',
        tool_use_id: 'toolu_art',
        content: 'Published /x at https://claude.ai/artifact/abc (Version 3) ... long guidance ...',
      };
      const { container, getByText } = render(
        <ContentBlock
          block={block}
          toolName="Artifact"
          artifactToolUseResult={{ url: 'https://claude.ai/artifact/abc', title: 'My Resume', seq: 3 }}
        />,
      );
      expect(getByText('My Resume').closest('a')?.getAttribute('href')).toBe('https://claude.ai/artifact/abc');
      expect(getByText('Version 3')).toBeInTheDocument();
      expect(container.querySelector('details')?.open).toBe(false);
    });

    it('reads the link from a read result', () => {
      const block: ContentBlockType = { type: 'tool_result', tool_use_id: 't', content: '[Artifact ...] raw HTML' };
      const { getByText } = render(
        <ContentBlock
          block={block}
          toolName="Artifact"
          artifactToolUseResult={{ read: { url: 'https://claude.ai/artifact/abc', title: 'Page' } }}
        />,
      );
      expect(getByText('Page').closest('a')).not.toBeNull();
    });

    it('keeps the plain rendering for errors and results without a link', () => {
      const err: ContentBlockType = { type: 'tool_result', tool_use_id: 't', content: 'Publish refused', is_error: true };
      const r1 = render(
        <ContentBlock block={err} toolName="Artifact" artifactToolUseResult={{ url: 'https://claude.ai/artifact/abc' }} />,
      );
      expect(r1.container.querySelector('details')).toBeNull();
      expect(r1.container.querySelector('a')).toBeNull();
      const quickstart: ContentBlockType = { type: 'tool_result', tool_use_id: 't', content: 'Quickstart. ...' };
      const r2 = render(<ContentBlock block={quickstart} toolName="Artifact" artifactToolUseResult={{}} />);
      expect(r2.container.querySelector('details')).toBeNull();
    });
  });

  describe('ArtifactComments / ArtifactData tools', () => {
    it('renders an ArtifactComments resolve call as a card', () => {
      const block: ContentBlockType = {
        type: 'tool_use',
        id: 'toolu_c1',
        name: 'ArtifactComments',
        input: { action: 'resolve', url: 'https://claude.ai/artifact/abc', thread_id: 'th_1' },
      };
      const { container, getByText } = render(<ContentBlock block={block} />);
      expect(getByText('Artifact comments')).toBeInTheDocument();
      expect(getByText('Resolve')).toBeInTheDocument();
      expect(getByText('https://claude.ai/artifact/abc').closest('a')?.getAttribute('href')).toBe('https://claude.ai/artifact/abc');
      expect(getByText('thread_id').nextElementSibling?.textContent).toBe('th_1');
      const details = container.querySelector('details');
      expect(details?.open).toBe(false);
      expect(details?.querySelector('[data-testid="codeblock"]')).not.toBeNull();
    });

    it('renders an ArtifactComments watch call with no link', () => {
      const block: ContentBlockType = { type: 'tool_use', id: 'toolu_c2', name: 'ArtifactComments', input: { action: 'watch' } };
      const { container, getByText } = render(<ContentBlock block={block} />);
      expect(getByText('Watch')).toBeInTheDocument();
      expect(container.querySelector('a')).toBeNull();
    });

    it('omits the action label when the call has none', () => {
      const block: ContentBlockType = { type: 'tool_use', id: 't', name: 'ArtifactData', input: { url: 'https://claude.ai/artifact/abc' } };
      const { container, getByText } = render(<ContentBlock block={block} />);
      expect(getByText('Artifact data')).toBeInTheDocument();
      expect(container.textContent).not.toContain('Publish');
    });

    it('renders an ArtifactData list call with a collection row and a query chip', () => {
      const block: ContentBlockType = {
        type: 'tool_use',
        id: 'toolu_d1',
        name: 'ArtifactData',
        input: { action: 'list', url: 'https://claude.ai/artifact/abc', collection: 'votes', query: { limit: 500 } },
      };
      const { getByText } = render(<ContentBlock block={block} />);
      expect(getByText('Artifact data')).toBeInTheDocument();
      expect(getByText('List')).toBeInTheDocument();
      expect(getByText('collection').nextElementSibling?.textContent).toBe('votes');
      expect(getByText('query: 1 key')).toBeInTheDocument();
    });

    it('renders an unseen ArtifactData action with count chips', () => {
      const block: ContentBlockType = {
        type: 'tool_use',
        id: 'toolu_d2',
        name: 'ArtifactData',
        input: { action: 'batch', url: 'https://claude.ai/artifact/abc', batch: [{}, {}, {}, {}], dry_run: false },
      };
      const { getByText } = render(<ContentBlock block={block} />);
      expect(getByText('Batch')).toBeInTheDocument();
      expect(getByText('batch: 4 items')).toBeInTheDocument();
      expect(getByText('dry_run').nextElementSibling?.textContent).toBe('false');
    });

    it('keeps sibling tool_results unchanged', () => {
      const block: ContentBlockType = { type: 'tool_result', tool_use_id: 't', content: 'No documents matched.' };
      for (const toolName of ['ArtifactComments', 'ArtifactData']) {
        const { container, unmount } = render(
          <ContentBlock block={block} toolName={toolName} artifactToolUseResult={{ url: 'https://claude.ai/artifact/abc' }} />,
        );
        expect(container.querySelector('details')).toBeNull();
        expect(container.querySelector('a')).toBeNull();
        expect(container.querySelector('[data-testid="codeblock"]')?.textContent).toBe('No documents matched.');
        unmount();
      }
    });
  });
});
