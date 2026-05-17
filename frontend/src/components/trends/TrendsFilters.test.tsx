import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TrendsFilters, { type TrendsFiltersValue } from './TrendsFilters';

const defaultDateRange = { startDate: '2025-01-01', endDate: '2025-01-31', label: 'Last 30 days' };

function baseProps(overrides: Partial<React.ComponentProps<typeof TrendsFilters>> = {}) {
  const value: TrendsFiltersValue = {
    dateRange: defaultDateRange,
    repos: [],
    includeNoRepo: true,
    providers: [],
  };
  return {
    repos: ['confab-web', 'other-repo'],
    value,
    onChange: vi.fn(),
    ...overrides,
  };
}

describe('TrendsFilters Provider filter (CF-424)', () => {
  it('renders the Provider button as the leftmost control', () => {
    render(<TrendsFilters {...baseProps()} />);
    const buttons = screen.getAllByRole('button');
    const providerIdx = buttons.findIndex((b) => /provider/i.test(b.getAttribute('aria-label') || ''));
    const dateIdx = buttons.findIndex((b) => /date/i.test(b.getAttribute('aria-label') || ''));
    expect(providerIdx).toBeGreaterThanOrEqual(0);
    expect(dateIdx).toBeGreaterThan(providerIdx);
  });

  it('shows "All Providers" label when providers state is empty', () => {
    render(<TrendsFilters {...baseProps()} />);
    expect(screen.getByRole('button', { name: /provider/i })).toHaveTextContent(/all providers/i);
  });

  it('shows the provider label when exactly one is selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          value: { dateRange: defaultDateRange, repos: [], includeNoRepo: true, providers: ['claude-code'] },
        })}
      />
    );
    expect(screen.getByRole('button', { name: /provider/i })).toHaveTextContent(/claude code/i);
  });

  it('shows "2 providers" when both are selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          value: {
            dateRange: defaultDateRange,
            repos: [],
            includeNoRepo: true,
            providers: ['claude-code', 'codex'],
          },
        })}
      />
    );
    expect(screen.getByRole('button', { name: /provider/i })).toHaveTextContent(/2 providers/i);
  });

  it('dropdown rows render unchecked when state is empty', () => {
    render(<TrendsFilters {...baseProps()} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));

    expect(screen.getByRole('checkbox', { name: /claude code/i })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: /codex/i })).not.toBeChecked();
  });

  it('dropdown row reflects checked state when one provider is selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          value: { dateRange: defaultDateRange, repos: [], includeNoRepo: true, providers: ['claude-code'] },
        })}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));

    expect(screen.getByRole('checkbox', { name: /claude code/i })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: /codex/i })).not.toBeChecked();
  });

  it('clicking an unselected provider row calls onChange with that provider', () => {
    const onChange = vi.fn();
    render(<TrendsFilters {...baseProps({ onChange })} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /claude code/i }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ providers: ['claude-code'] })
    );
  });

  it('unchecking the last selected provider snaps state back to []', () => {
    const onChange = vi.fn();
    render(
      <TrendsFilters
        {...baseProps({
          onChange,
          value: { dateRange: defaultDateRange, repos: [], includeNoRepo: true, providers: ['claude-code'] },
        })}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /claude code/i }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ providers: [] }));
  });

  it('omits the Select-all/Deselect-all toggle (only 2 options)', () => {
    render(<TrendsFilters {...baseProps()} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    expect(screen.queryByText(/select all/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/deselect all/i)).not.toBeInTheDocument();
  });
});
