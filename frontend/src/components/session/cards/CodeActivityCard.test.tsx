import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { CodeActivityCard } from './CodeActivityCard';
import type { CodeActivityCardData } from '@/schemas/api';

function makeData(overrides: Partial<CodeActivityCardData> = {}): CodeActivityCardData {
  return {
    files_read: 10,
    files_modified: 5,
    lines_added: 200,
    lines_removed: 50,
    search_count: 3,
    language_breakdown: { ts: 8, py: 4 },
    ...overrides,
  };
}

describe('CodeActivityCard', () => {
  it('returns null when no files and no searches', () => {
    const { container } = render(
      <CodeActivityCard
        data={makeData({
          files_read: 0,
          files_modified: 0,
          search_count: 0,
          language_breakdown: {},
        })}
        loading={false}
        provider="claude-code"
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders all stat rows', () => {
    const { getByText } = render(
      <CodeActivityCard data={makeData()} loading={false} provider="claude-code" />
    );
    expect(getByText('Files read')).toBeInTheDocument();
    expect(getByText('Files modified')).toBeInTheDocument();
    expect(getByText('Lines added')).toBeInTheDocument();
    expect(getByText('Lines removed')).toBeInTheDocument();
    expect(getByText('Searches')).toBeInTheDocument();
  });

  it('shows File extensions section and chart when language_breakdown is non-empty', () => {
    const { getByText, getAllByTestId } = render(
      <CodeActivityCard data={makeData()} loading={false} provider="claude-code" />
    );
    expect(getByText('File extensions')).toBeInTheDocument();
    expect(getAllByTestId('recharts-stub').length).toBeGreaterThan(0);
  });

  it('omits chart when language_breakdown is empty', () => {
    const { queryByText, queryByTestId } = render(
      <CodeActivityCard
        data={makeData({ language_breakdown: {} })}
        loading={false}
        provider="claude-code"
      />
    );
    expect(queryByText('File extensions')).toBeNull();
    expect(queryByTestId('recharts-stub')).toBeNull();
  });

  it('renders loading state', () => {
    const { getByText } = render(
      <CodeActivityCard data={null} loading={true} provider="claude-code" />
    );
    expect(getByText('Code Activity')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError', () => {
    const { getByText } = render(
      <CodeActivityCard data={null} loading={false} error="bad" provider="claude-code" />
    );
    expect(getByText(/Failed to compute: bad/)).toBeInTheDocument();
  });

  describe('provider-aware UX (CF-439)', () => {
    it('hides Files read row when provider is codex', () => {
      const { queryByText, getByText } = render(
        <CodeActivityCard
          data={makeData({ files_read: 0, files_modified: 5 })}
          loading={false}
          provider="codex"
        />
      );
      expect(queryByText('Files read')).toBeNull();
      // Other rows still render.
      expect(getByText('Files modified')).toBeInTheDocument();
      expect(getByText('Lines added')).toBeInTheDocument();
      expect(getByText('Lines removed')).toBeInTheDocument();
      expect(getByText('Searches')).toBeInTheDocument();
    });

    it('shows Files read row when provider is claude-code', () => {
      const { getByText } = render(
        <CodeActivityCard
          data={makeData({ files_read: 10, files_modified: 5 })}
          loading={false}
          provider="claude-code"
        />
      );
      expect(getByText('Files read')).toBeInTheDocument();
    });

    it('sets Codex web_search_call tooltip on Searches row when provider is codex', () => {
      const { getByText } = render(
        <CodeActivityCard
          data={makeData({ files_modified: 5, search_count: 0 })}
          loading={false}
          provider="codex"
        />
      );
      // The StatRow places `title` on its outer wrapper (`.statRow`).
      const row = getByText('Searches').closest('[title]');
      expect(row).toHaveAttribute(
        'title',
        "Codex's web_search_call is not counted as file search"
      );
    });

    it('does not set a Codex tooltip on Searches row when provider is claude-code', () => {
      const { getByText } = render(
        <CodeActivityCard
          data={makeData({ files_modified: 5, search_count: 3 })}
          loading={false}
          provider="claude-code"
        />
      );
      // Either no title attribute, or a falsy/empty title — no Codex-specific text.
      const title = getByText('Searches').closest('[title]')?.getAttribute('title') ?? '';
      expect(title).not.toMatch(/Codex/);
      expect(title).not.toMatch(/web_search_call/);
    });
  });
});
