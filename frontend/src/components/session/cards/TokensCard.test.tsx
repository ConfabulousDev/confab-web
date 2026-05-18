import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TokensCard } from './TokensCard';
import { claudeAdapter } from '@/providers/claudeAdapter';
import { codexAdapter } from '@/providers/codexAdapter';

const mockData = {
  input: 110000,
  output: 20800,
  cache_creation: 23000,
  cache_read: 36000,
  estimated_usd: '4.25',
};

describe('TokensCard', () => {
  it('renders all token stats with formatted values', () => {
    render(<TokensCard data={mockData} loading={false} provider="claude-code" />);

    expect(screen.getByText('Tokens')).toBeInTheDocument();
    expect(screen.getByText('Input')).toBeInTheDocument();
    expect(screen.getByText('110.0k')).toBeInTheDocument();
    expect(screen.getByText('Output')).toBeInTheDocument();
    expect(screen.getByText('20.8k')).toBeInTheDocument();
    expect(screen.getByText('Cache created')).toBeInTheDocument();
    expect(screen.getByText('23.0k')).toBeInTheDocument();
    expect(screen.getByText('Cache read')).toBeInTheDocument();
    expect(screen.getByText('36.0k')).toBeInTheDocument();
    expect(screen.getByText('Estimated cost')).toBeInTheDocument();
    expect(screen.getByText('$4.25')).toBeInTheDocument();
  });

  it('shows loading state when loading with no data', () => {
    render(<TokensCard data={null} loading={true} provider="claude-code" />);

    expect(screen.getByText('Tokens')).toBeInTheDocument();
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('returns null when not loading and no data', () => {
    const { container } = render(<TokensCard data={null} loading={false} provider="claude-code" />);

    expect(container).toBeEmptyDOMElement();
  });

  it('shows data even while loading (optimistic update)', () => {
    render(<TokensCard data={mockData} loading={true} provider="claude-code" />);

    expect(screen.getByText('110.0k')).toBeInTheDocument();
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
  });

  it('shows warning tooltip when cost is zero', () => {
    const zeroCostData = { ...mockData, estimated_usd: '0.00' };
    render(<TokensCard data={zeroCostData} loading={false} provider="claude-code" />);

    expect(screen.getByText('$0.00')).toBeInTheDocument();
    const costRow = screen.getByText('Estimated cost').closest('div');
    expect(costRow).toHaveAttribute('title', 'Cost unavailable — session may use models not yet in the pricing table');
  });

  // CF-436: "Cache created" is meaningless for Codex (OpenAI doesn't charge
  // for cache writes) and for Claude sessions that didn't use prompt caching.
  // Hide the row when the value is exactly 0.
  describe('Cache created row hide rule (CF-436)', () => {
    it('hides "Cache created" row when cache_creation is 0', () => {
      const data = { ...mockData, cache_creation: 0 };
      render(<TokensCard data={data} loading={false} provider="claude-code" />);
      expect(screen.queryByText('Cache created')).not.toBeInTheDocument();
    });

    it('renders "Cache created" row when cache_creation > 0', () => {
      render(<TokensCard data={mockData} loading={false} provider="claude-code" />);
      expect(screen.getByText('Cache created')).toBeInTheDocument();
    });

    it('Codex-shaped data renders 4 rows: Estimated cost, Input, Output, Cache read', () => {
      const codexData = {
        input: 80_000,
        output: 5_000,
        cache_creation: 0,
        cache_read: 12_000,
        estimated_usd: '0.42',
      };
      render(<TokensCard data={codexData} loading={false} provider="codex" />);

      expect(screen.getByText('Estimated cost')).toBeInTheDocument();
      expect(screen.getByText('Input')).toBeInTheDocument();
      expect(screen.getByText('Output')).toBeInTheDocument();
      expect(screen.getByText('Cache read')).toBeInTheDocument();
      expect(screen.queryByText('Cache created')).not.toBeInTheDocument();
    });
  });

  // CF-436: Provider-specific copy lives on the ProviderAdapter (static
  // string fields). TokensCard resolves the adapter via `getAdapter(provider)`.
  describe('Cost tooltip provider-awareness (CF-436)', () => {
    it('uses Claude cost tooltip when provider is "claude-code"', () => {
      render(<TokensCard data={mockData} loading={false} provider="claude-code" />);
      const costRow = screen.getByText('Estimated cost').closest('div');
      expect(costRow).toHaveAttribute('title', claudeAdapter.tokensCostTooltip);
    });

    it('uses Codex cost tooltip when provider is "codex"', () => {
      render(<TokensCard data={mockData} loading={false} provider="codex" />);
      const costRow = screen.getByText('Estimated cost').closest('div');
      expect(costRow).toHaveAttribute('title', codexAdapter.tokensCostTooltip);
    });

    it('uses Anthropic-priority-tier wording for Claude fast-mode tooltip', () => {
      const data = { ...mockData, fast_turns: 5, fast_cost_usd: '1.10' };
      render(<TokensCard data={data} loading={false} provider="claude-code" />);
      const fastRow = screen.getByText('Fast mode').closest('div');
      expect(fastRow).toHaveAttribute('title', claudeAdapter.tokensFastTooltip!);
      expect(claudeAdapter.tokensFastTooltip).toMatch(/Anthropic priority tier/);
    });
  });
});
