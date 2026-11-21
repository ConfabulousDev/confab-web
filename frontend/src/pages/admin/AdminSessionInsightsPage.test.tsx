import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import type { ReactNode } from 'react';
import {
  AdminSessionInsightsPageContent,
  type AdminSessionInsightsPageContentProps,
} from './AdminSessionInsightsPage';
import { fixturePoint, fixturePointsResponse, fixtureShare } from './sessionInsightsFixtures';

// Per-file recharts mock: renders each scatter datum as a clickable node carrying
// its y value, each bar series as a node, and the y-axis's formatted ticks, so
// the chart wiring is assertable without real geometry.
vi.mock('recharts', () => {
  type Datum = { y: number; point: { id: string } };
  const Passthrough = ({ children }: { children?: ReactNode }) => <div>{children}</div>;
  return {
    ResponsiveContainer: Passthrough,
    BarChart: Passthrough,
    ScatterChart: Passthrough,
    CartesianGrid: () => null,
    Tooltip: () => null,
    XAxis: () => null,
    YAxis: ({ tickFormatter }: { tickFormatter?: (v: number) => string }) => (
      <div data-testid="y-ticks">{tickFormatter ? [0, 50, 100].map(tickFormatter).join('|') : ''}</div>
    ),
    Bar: ({ dataKey }: { dataKey: string }) => <div data-testid="bar-series">{dataKey}</div>,
    Scatter: ({
      name,
      data,
      onClick,
    }: {
      name: string;
      data: Datum[];
      onClick: (entry: { payload: Datum }) => void;
    }) => (
      <div data-testid="scatter-series" data-name={name}>
        {data.map((d) => (
          <button key={d.point.id} data-testid="scatter-point" data-y={d.y} onClick={() => onClick({ payload: d })}>
            {d.point.id}
          </button>
        ))}
      </div>
    ),
  };
});

const points = [
  fixturePoint({ id: 'a', provider: 'claude-code', lines: 100, cost_usd: 2 }),
  fixturePoint({ id: 'b', provider: 'codex', lines: 300, cost_usd: null }),
];

function renderPage(overrides: Partial<AdminSessionInsightsPageContentProps> = {}) {
  const props: AdminSessionInsightsPageContentProps = {
    filters: { range: '30d', providers: [] },
    onFiltersChange: vi.fn(),
    users: [{ id: 7, email: 'ada@example.com' }],
    share: fixtureShare,
    points: fixturePointsResponse(points),
    isLoading: false,
    error: null,
    onPointClick: vi.fn(),
    ...overrides,
  };
  render(<AdminSessionInsightsPageContent {...props} />);
  return props;
}

describe('AdminSessionInsightsPageContent', () => {
  it('renders both charts from fixtures with a series per provider', () => {
    renderPage();
    expect(screen.getByTestId('provider-share-chart')).toBeInTheDocument();
    expect(screen.getAllByTestId('bar-series').map((n) => n.textContent)).toEqual([
      'claude-code',
      'codex',
      'opencode',
      'cursor',
    ]);
    expect(screen.getAllByTestId('scatter-series').map((n) => n.dataset.name)).toEqual(['Claude Code', 'Codex']);
  });

  it('shows whole-range totals with shares in the table', () => {
    renderPage();
    const table = screen.getByRole('table');
    const codexRow = within(table).getByText('Codex').closest('tr')!;
    expect(within(codexRow).getByText('5')).toBeInTheDocument();
    expect(within(codexRow).getByText('23.8%')).toBeInTheDocument();
  });

  it('the %/count toggle switches the share axis to percent', () => {
    renderPage();
    const shareCard = screen.getByTestId('provider-share-chart');
    expect(within(shareCard).getByTestId('y-ticks').textContent).toBe('0|50|100');
    fireEvent.click(screen.getByRole('button', { name: '%' }));
    expect(within(shareCard).getByTestId('y-ticks').textContent).toBe('0%|50%|100%');
  });

  it('switching the size metric changes the y-values without new data', () => {
    const props = renderPage();
    expect(screen.getAllByTestId('scatter-point').map((n) => n.dataset.y)).toEqual(['100', '300']);
    fireEvent.change(screen.getByLabelText('Size'), { target: { value: 'cost_usd' } });
    expect(screen.getAllByTestId('scatter-point').map((n) => n.dataset.y)).toEqual(['2']);
    expect(screen.getByText('1 session has no cost data.')).toBeInTheDocument();
    expect(props.onFiltersChange).not.toHaveBeenCalled();
  });

  it('clicking a point opens that session', () => {
    const props = renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'b' }));
    expect(props.onPointClick).toHaveBeenCalledWith('b');
  });

  it('shows the capped note when the response is a sample', () => {
    renderPage({ points: fixturePointsResponse(points, 12_000) });
    expect(screen.getByRole('status')).toHaveTextContent('Showing 2 of 12,000 sessions');
  });

  it('omits the capped note when every session is returned', () => {
    renderPage();
    expect(screen.queryByText(/Showing .* of/)).not.toBeInTheDocument();
  });

  it('shows empty states, not charts, for an empty range', () => {
    renderPage({
      share: { ...fixtureShare, buckets: [], totals: {} },
      points: fixturePointsResponse([]),
    });
    expect(screen.getAllByText('No sessions in this range.')).toHaveLength(2);
    expect(screen.queryByTestId('provider-share-chart')).not.toBeInTheDocument();
    expect(screen.queryByTestId('size-distribution-chart')).not.toBeInTheDocument();
  });

  it('range, provider and owner controls report filter changes', () => {
    const props = renderPage();
    fireEvent.click(screen.getByRole('button', { name: '7d' }));
    expect(props.onFiltersChange).toHaveBeenLastCalledWith({ range: '7d', providers: [] });
    fireEvent.click(within(screen.getByRole('group', { name: 'Providers' })).getByRole('button', { name: 'Codex' }));
    expect(props.onFiltersChange).toHaveBeenLastCalledWith({ range: '30d', providers: ['codex'] });
    fireEvent.change(screen.getByLabelText('Owner'), { target: { value: '7' } });
    expect(props.onFiltersChange).toHaveBeenLastCalledWith({ range: '30d', providers: [], userId: 7 });
  });

  it('shows an error alert', () => {
    renderPage({ share: null, points: null, error: 'Failed to load session insights.' });
    expect(screen.getByText('Failed to load session insights.')).toBeInTheDocument();
  });
});
