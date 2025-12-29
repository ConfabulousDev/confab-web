import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CompactionCard } from './CompactionCard';

describe('CompactionCard', () => {
  it('renders compaction stats', () => {
    render(
      <CompactionCard
        data={{ auto: 2, manual: 1, avg_time_ms: 48500 }}
        loading={false}
      />
    );

    expect(screen.getByText('Compaction')).toBeInTheDocument();
    expect(screen.getByText('Auto')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('Manual')).toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('Avg time (auto)')).toBeInTheDocument();
    expect(screen.getByText('48.5s')).toBeInTheDocument();
  });

  it('shows dash for null avg_time_ms', () => {
    render(
      <CompactionCard
        data={{ auto: 0, manual: 0, avg_time_ms: null }}
        loading={false}
      />
    );

    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('shows loading state when loading with no data', () => {
    render(<CompactionCard data={null} loading={true} />);

    expect(screen.getByText('Compaction')).toBeInTheDocument();
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('returns null when not loading and no data', () => {
    const { container } = render(<CompactionCard data={null} loading={false} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('shows data even while loading (optimistic update)', () => {
    render(
      <CompactionCard
        data={{ auto: 3, manual: 0, avg_time_ms: 30000 }}
        loading={true}
      />
    );

    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
  });
});
