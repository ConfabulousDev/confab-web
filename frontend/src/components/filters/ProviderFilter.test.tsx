import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ProviderFilter, { type ProviderFilterProps } from './ProviderFilter';
import { PROVIDER_VALUES } from '@/utils/providers';

function props(overrides: Partial<ProviderFilterProps> = {}): ProviderFilterProps {
  return {
    availableProviders: [...PROVIDER_VALUES],
    selectedProviders: [],
    onChange: vi.fn(),
    ...overrides,
  };
}

// CF-424: empty providers[] = "All Providers" (distinct from selecting every
// provider, but the label only distinguishes 0 / 1 / many).
describe('ProviderFilter label (CF-424)', () => {
  it('shows "All Providers" when none are selected', () => {
    render(<ProviderFilter {...props()} />);
    expect(screen.getByRole('button', { name: /provider/i })).toHaveTextContent(/all providers/i);
  });

  it('shows the provider label when exactly one is selected', () => {
    render(<ProviderFilter {...props({ selectedProviders: ['claude-code'] })} />);
    expect(screen.getByRole('button', { name: /provider/i })).toHaveTextContent(/claude code/i);
  });

  it('shows "N providers" when more than one is selected', () => {
    render(<ProviderFilter {...props({ selectedProviders: ['claude-code', 'codex'] })} />);
    expect(screen.getByRole('button', { name: /provider/i })).toHaveTextContent(/2 providers/i);
  });
});

describe('ProviderFilter button highlight', () => {
  it('is not highlighted when no provider is selected', () => {
    render(<ProviderFilter {...props()} />);
    expect(screen.getByRole('button', { name: /provider/i }).className).not.toMatch(/active/);
  });

  it('is highlighted when at least one provider is selected', () => {
    render(<ProviderFilter {...props({ selectedProviders: ['codex'] })} />);
    expect(screen.getByRole('button', { name: /provider/i }).className).toMatch(/active/);
  });
});

describe('ProviderFilter dropdown rows', () => {
  it('renders a checkbox row per available provider', () => {
    render(<ProviderFilter {...props()} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    expect(screen.getByRole('checkbox', { name: /claude code/i })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: /codex/i })).toBeInTheDocument();
  });

  it('narrows the rows to the provided availableProviders only', () => {
    render(<ProviderFilter {...props({ availableProviders: ['claude-code'] })} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    expect(screen.getByRole('checkbox', { name: /claude code/i })).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: /codex/i })).toBeNull();
  });

  it('reflects checked state for the selected providers', () => {
    render(<ProviderFilter {...props({ selectedProviders: ['claude-code'] })} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    expect(screen.getByRole('checkbox', { name: /claude code/i })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: /codex/i })).not.toBeChecked();
  });

  it('omits any Select-all / Deselect-all toggle', () => {
    render(<ProviderFilter {...props()} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    expect(screen.queryByText(/select all/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/deselect all/i)).not.toBeInTheDocument();
  });
});

describe('ProviderFilter onChange', () => {
  it('checking an unselected provider emits it added', () => {
    const onChange = vi.fn();
    render(<ProviderFilter {...props({ onChange })} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /claude code/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(['claude-code']);
  });

  it('unchecking the last selected provider snaps the selection back to []', () => {
    const onChange = vi.fn();
    render(<ProviderFilter {...props({ selectedProviders: ['claude-code'], onChange })} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /claude code/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith([]);
  });
});
