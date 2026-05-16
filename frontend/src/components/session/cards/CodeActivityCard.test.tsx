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
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders all stat rows', () => {
    const { getByText } = render(<CodeActivityCard data={makeData()} loading={false} />);
    expect(getByText('Files read')).toBeInTheDocument();
    expect(getByText('Files modified')).toBeInTheDocument();
    expect(getByText('Lines added')).toBeInTheDocument();
    expect(getByText('Lines removed')).toBeInTheDocument();
    expect(getByText('Searches')).toBeInTheDocument();
  });

  it('shows File extensions section and chart when language_breakdown is non-empty', () => {
    const { getByText, getAllByTestId } = render(
      <CodeActivityCard data={makeData()} loading={false} />
    );
    expect(getByText('File extensions')).toBeInTheDocument();
    expect(getAllByTestId('recharts-stub').length).toBeGreaterThan(0);
  });

  it('omits chart when language_breakdown is empty', () => {
    const { queryByText, queryByTestId } = render(
      <CodeActivityCard
        data={makeData({ language_breakdown: {} })}
        loading={false}
      />
    );
    expect(queryByText('File extensions')).toBeNull();
    expect(queryByTestId('recharts-stub')).toBeNull();
  });

  it('renders loading state', () => {
    const { getByText } = render(<CodeActivityCard data={null} loading={true} />);
    expect(getByText('Code Activity')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError', () => {
    const { getByText } = render(
      <CodeActivityCard data={null} loading={false} error="bad" />
    );
    expect(getByText(/Failed to compute: bad/)).toBeInTheDocument();
  });
});
