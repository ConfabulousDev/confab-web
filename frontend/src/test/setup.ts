import { afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { setPricingTable } from '@/utils/tokenStats';
import { PRICING_FIXTURE } from './pricingFixture';

// The frontend bundles no price data (CF-515); install a frozen table so cost
// arithmetic is deterministic across the suite without a backend fetch.
setPricingTable(PRICING_FIXTURE);

// jsdom doesn't implement ResizeObserver, but components like ScrollNavButtons
// instantiate one on mount. Provide a no-op stub so renders complete.
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class {
    observe(): void { /* no-op */ }
    unobserve(): void { /* no-op */ }
    disconnect(): void { /* no-op */ }
  };
}

// Recharts touches DOM-measurement APIs jsdom doesn't implement and spams
// console output about 0px widths. Stub with passthroughs that also invoke
// the inline callbacks cards pass to Tooltip/XAxis/YAxis so per-card
// CustomTooltip/tickFormatter logic is exercised under the global mock.
// Tests that need real chart geometry should `vi.unmock('recharts')` per-file.
vi.mock('recharts', async () => {
  const React = await import('react');
  type AnyProps = { children?: React.ReactNode };
  const Passthrough = ({ children }: AnyProps) =>
    React.createElement('div', { 'data-testid': 'recharts-stub' }, children);

  // Synthetic payload covers both card CustomTooltip shapes (`success`/`errors`
  // dataKeys, plus the per-row `payload.payload` with name/displayName/type).
  const tooltipPayload = [
    {
      value: 1,
      dataKey: 'success',
      name: 'success',
      color: '#000',
      payload: {
        name: 'sample',
        displayName: 'sample',
        success: 1,
        errors: 0,
        total: 1,
        type: 'agent',
        extension: 'ts',
        count: 1,
        fullName: 'sample',
        value: 1,
      },
    },
    {
      value: 0,
      dataKey: 'errors',
      name: 'errors',
      color: '#000',
      payload: {
        name: 'sample',
        displayName: 'sample',
        success: 1,
        errors: 0,
        total: 1,
        type: 'agent',
        extension: 'ts',
        count: 1,
        fullName: 'sample',
        value: 1,
      },
    },
  ];

  type AxisProps = {
    tickFormatter?: (value: unknown) => unknown;
    tick?: unknown;
    'data-testid'?: string;
  };

  // Sample tick values used to exercise (and surface) the axis tickFormatter
  // without a real recharts render. Spans zero, a small count, and a value in
  // the thousands so $/count abbreviation (e.g. '$5.0K') is observable in tests.
  const AXIS_SAMPLE_TICKS = [0, 5, 5000];

  const Axis = ({ tickFormatter, 'data-testid': testId }: AxisProps) => {
    // Render the formatted sample ticks into the DOM so a card's inline
    // tickFormatter is both exercised and assertable (the real recharts axis is
    // mocked away). Each formatted tick is its own <text> element.
    const ticks = AXIS_SAMPLE_TICKS.map((v, i) => {
      let label: unknown = v;
      try { label = tickFormatter ? tickFormatter(v) : v; } catch { /* swallow */ }
      return React.createElement('text', { key: i }, String(label));
    });
    return React.createElement('g', { 'data-testid': testId }, ...ticks);
  };

  type TooltipProps = { content?: React.ReactElement<Record<string, unknown>> };
  const Tooltip = ({ content }: TooltipProps) => {
    if (!content || !React.isValidElement(content)) return null;
    return React.cloneElement(content, { active: true, payload: tooltipPayload });
  };

  return {
    ResponsiveContainer: Passthrough,
    BarChart: Passthrough,
    Bar: () => null,
    AreaChart: Passthrough,
    Area: () => null,
    XAxis: (props: AxisProps) => Axis({ ...props, 'data-testid': 'recharts-xaxis' }),
    YAxis: (props: AxisProps) => Axis({ ...props, 'data-testid': 'recharts-yaxis' }),
    CartesianGrid: () => null,
    ReferenceLine: () => null,
    Tooltip,
    Cell: () => null,
  };
});

afterEach(() => {
  cleanup();
});
