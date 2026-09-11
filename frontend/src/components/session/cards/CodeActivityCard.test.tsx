import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { CodeActivityCard } from './CodeActivityCard';
import type { CodeActivityCardData } from '@/schemas/api';
import { PROVIDER_METADATA } from '@/utils/providers';

const CODEX_TOOLTIPS = PROVIDER_METADATA.codex.cardTooltips?.codeActivity;

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
    // m2ky: the Files-read row used to be hidden for Codex because the value
    // was structurally always 0. Codex >=0.149.1 reports it for real, so the
    // row is shown for every provider and a tooltip explains the older-era 0.
    it('shows Files read row with an era tooltip when provider is codex', () => {
      const { getByText } = render(
        <CodeActivityCard
          data={makeData({ files_read: 17, files_modified: 5 })}
          loading={false}
          provider="codex"
        />
      );
      expect(getByText('Files read')).toBeInTheDocument();
      expect(getByText('17')).toBeInTheDocument();
      expect(getByText('Files read').closest('[title]')).toHaveAttribute(
        'title',
        CODEX_TOOLTIPS?.filesRead
      );
      // Other rows still render.
      expect(getByText('Files modified')).toBeInTheDocument();
      expect(getByText('Lines added')).toBeInTheDocument();
      expect(getByText('Lines removed')).toBeInTheDocument();
      expect(getByText('Searches')).toBeInTheDocument();
    });

    // The copy is the user's only explanation for a zero on an old session, so
    // pin the one fact it must carry rather than the whole sentence.
    it('names the Codex version that starts recording both figures', () => {
      expect(CODEX_TOOLTIPS?.filesRead).toContain('0.149.1');
      expect(CODEX_TOOLTIPS?.searches).toContain('0.149.1');
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

    it('sets the Codex tooltip on the Searches row when provider is codex', () => {
      const { getByText } = render(
        <CodeActivityCard
          data={makeData({ files_modified: 5, search_count: 0 })}
          loading={false}
          provider="codex"
        />
      );
      // The StatRow places `title` on its outer wrapper (`.statRow`).
      const row = getByText('Searches').closest('[title]');
      expect(row).toHaveAttribute('title', CODEX_TOOLTIPS?.searches);
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
