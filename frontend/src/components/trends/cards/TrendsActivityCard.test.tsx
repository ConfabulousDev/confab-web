import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TrendsActivityCard } from './TrendsActivityCard';
import type { TrendsActivityCard as TrendsActivityCardData } from '@/schemas/api';

// Files Read row has three-state behavior based on providersPresent:
//   - empty or no codex          → row renders, no caveat
//   - codex + >= 1 other provider → row renders + ⓘ tooltip
//   - codex only                 → row omitted entirely (mirrors CF-439)
// Other rows (Files Modified, Lines Added/Removed) are always present.

function makeData(overrides: Partial<TrendsActivityCardData> = {}): TrendsActivityCardData {
  return {
    total_files_read: 1234,
    total_files_modified: 410,
    total_lines_added: 12000,
    total_lines_removed: 5400,
    daily_session_counts: [
      { date: '2024-01-08', session_count: 5, per_provider: { 'claude-code': 5 } },
      { date: '2024-01-09', session_count: 7, per_provider: { 'claude-code': 5, codex: 2 } },
    ],
    ...overrides,
  };
}

describe('TrendsActivityCard Files Read row — providersPresent branches', () => {
  it('renders Files Read row with no caveat when providersPresent is empty', () => {
    render(<TrendsActivityCard data={makeData()} providersPresent={[]} />);
    expect(screen.getByText('Files Read')).toBeInTheDocument();
    expect(screen.queryByText(/excludes codex/i)).not.toBeInTheDocument();
  });

  it('renders Files Read row with no caveat when providersPresent is ["claude-code"]', () => {
    render(<TrendsActivityCard data={makeData()} providersPresent={['claude-code']} />);
    expect(screen.getByText('Files Read')).toBeInTheDocument();
    expect(screen.queryByText(/excludes codex/i)).not.toBeInTheDocument();
  });

  it('renders Files Read row plus a caveat ⓘ when providersPresent contains both claude-code and codex', () => {
    const { container } = render(
      <TrendsActivityCard data={makeData()} providersPresent={['claude-code', 'codex']} />,
    );
    expect(screen.getByText('Files Read')).toBeInTheDocument();
    // The ⓘ affordance carries a native title= attribute with the caveat copy.
    const caveatEl = container.querySelector('[title*="Excludes Codex sessions"]');
    expect(caveatEl).not.toBeNull();
  });

  // m2ky: the row used to be omitted for a Codex-only window because the total
  // was structurally 0. Codex >=0.149.1 reports file reads for real, so the row
  // renders with its real value and keeps the caveat for the older era.
  it('renders Files Read with its value when providersPresent is ["codex"]', () => {
    const { container } = render(
      <TrendsActivityCard
        data={makeData({ total_files_read: 412 })}
        providersPresent={['codex']}
      />,
    );
    expect(screen.getByText('Files Read')).toBeInTheDocument();
    expect(screen.getByText('412')).toBeInTheDocument();
    expect(container.querySelector('[title*="0.149.1"]')).not.toBeNull();
  });

  it('ⓘ title attribute names the Codex version, not a missing Read tool', () => {
    const { container } = render(
      <TrendsActivityCard data={makeData()} providersPresent={['claude-code', 'codex']} />,
    );
    expect(container.querySelector('[title*="0.149.1"]')).not.toBeNull();
    // The old copy claimed Codex has no Read tool at all; that is now false.
    expect(container.querySelector('[title*="no Read tool"]')).toBeNull();
  });
});

describe('TrendsActivityCard always-rendered rows', () => {
  const matrix: Array<{ name: string; providersPresent: string[] }> = [
    { name: 'empty providersPresent', providersPresent: [] },
    { name: 'claude-code only', providersPresent: ['claude-code'] },
    { name: 'mixed claude-code + codex', providersPresent: ['claude-code', 'codex'] },
    { name: 'codex only', providersPresent: ['codex'] },
  ];

  for (const { name, providersPresent } of matrix) {
    it(`renders Files Modified / Lines Added / Lines Removed for ${name}`, () => {
      render(<TrendsActivityCard data={makeData()} providersPresent={providersPresent} />);
      expect(screen.getByText('Files Modified')).toBeInTheDocument();
      expect(screen.getByText('Lines Added')).toBeInTheDocument();
      expect(screen.getByText('Lines Removed')).toBeInTheDocument();
    });
  }
});

describe('TrendsActivityCard null data', () => {
  it('returns null when data is null', () => {
    const { container } = render(<TrendsActivityCard data={null} providersPresent={[]} />);
    expect(container.firstChild).toBeNull();
  });
});
