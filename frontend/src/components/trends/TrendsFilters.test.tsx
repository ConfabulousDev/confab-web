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
    owners: [],
  };
  const noOwners: string[] = [];
  return {
    repos: ['confab-web', 'other-repo'],
    owners: noOwners,
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
          value: { dateRange: defaultDateRange, repos: [], includeNoRepo: true, providers: ['claude-code'], owners: [] },
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
            owners: [],
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
          value: { dateRange: defaultDateRange, repos: [], includeNoRepo: true, providers: ['claude-code'], owners: [] },
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
          value: { dateRange: defaultDateRange, repos: [], includeNoRepo: true, providers: ['claude-code'], owners: [] },
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

// CF-495: Owner multi-select dropdown — mirrors the Repo/Provider pattern
// with one extra behavior: self (selfEmail) is pinned to the top of the
// rendered list so it's one click for the dominant case.
describe('TrendsFilters Owner filter (CF-495)', () => {
  const ownersIn = ['bob@example.com', 'alice@example.com', 'charlie@example.com'];
  const selfEmail = 'alice@example.com';

  it('renders the Owner button after Repo (visibility narrowing is the finest cut)', () => {
    render(<TrendsFilters {...baseProps({ owners: ownersIn, selfEmail })} />);
    const buttons = screen.getAllByRole('button');
    const repoIdx = buttons.findIndex((b) => /repository/i.test(b.getAttribute('aria-label') || ''));
    const ownerIdx = buttons.findIndex((b) => /owner/i.test(b.getAttribute('aria-label') || ''));
    expect(ownerIdx).toBeGreaterThanOrEqual(0);
    expect(ownerIdx).toBeGreaterThan(repoIdx);
  });

  it('shows "All Owners" label when no owners are selected', () => {
    render(<TrendsFilters {...baseProps({ owners: ownersIn, selfEmail })} />);
    expect(screen.getByRole('button', { name: /owner/i })).toHaveTextContent(/all owners/i);
  });

  it('shows the owner email label when exactly one is selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          owners: ownersIn,
          selfEmail,
          value: {
            dateRange: defaultDateRange,
            repos: [],
            includeNoRepo: true,
            providers: [],
            owners: ['alice@example.com'],
          },
        })}
      />
    );
    expect(screen.getByRole('button', { name: /owner/i })).toHaveTextContent(/alice@example.com/);
  });

  it('shows "N owners" when more than one is selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          owners: ownersIn,
          selfEmail,
          value: {
            dateRange: defaultDateRange,
            repos: [],
            includeNoRepo: true,
            providers: [],
            owners: ['alice@example.com', 'bob@example.com'],
          },
        })}
      />
    );
    expect(screen.getByRole('button', { name: /owner/i })).toHaveTextContent(/2 owners/i);
  });

  it('dropdown lists owners with selfEmail pinned to the top', () => {
    render(<TrendsFilters {...baseProps({ owners: ownersIn, selfEmail })} />);
    fireEvent.click(screen.getByRole('button', { name: /owner/i }));

    const checkboxes = screen.getAllByRole('checkbox');
    // First checkbox in the owners dropdown should be selfEmail. There are
    // also includeNoRepo + provider checkboxes mounted; scope by label text.
    const labels = checkboxes
      .map((c) => c.closest('label')?.textContent ?? '')
      .filter((t) => /@example\.com/.test(t));
    expect(labels[0]).toContain('alice@example.com');
    // The remaining two should be the other emails, alphabetical.
    expect(labels.slice(1)).toEqual(
      expect.arrayContaining([expect.stringContaining('bob@example.com'), expect.stringContaining('charlie@example.com')])
    );
  });

  it('clicking an unselected owner row calls onChange with that owner added', () => {
    const onChange = vi.fn();
    render(<TrendsFilters {...baseProps({ owners: ownersIn, selfEmail, onChange })} />);
    fireEvent.click(screen.getByRole('button', { name: /owner/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /alice@example\.com/ }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ owners: ['alice@example.com'] })
    );
  });

  it('unchecking the last selected owner snaps state back to []', () => {
    const onChange = vi.fn();
    render(
      <TrendsFilters
        {...baseProps({
          owners: ownersIn,
          selfEmail,
          onChange,
          value: {
            dateRange: defaultDateRange,
            repos: [],
            includeNoRepo: true,
            providers: [],
            owners: ['alice@example.com'],
          },
        })}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: /owner/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /alice@example\.com/ }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ owners: [] }));
  });

  it('renders no Owner button when the owners list is empty (nothing to filter)', () => {
    render(<TrendsFilters {...baseProps({ owners: [], selfEmail })} />);
    // Owner button hidden — the page doesn't have any owners to narrow to.
    expect(screen.queryByRole('button', { name: /owner/i })).toBeNull();
  });
});

// CF-233: empty repos[] means "all repos" (matches CF-506 backend semantics).
// The button label and the dropdown's selection-toggle UX must reflect that.
describe('TrendsFilters Repo filter label & Clear (CF-233)', () => {
  const repos = ['ConfabulousDev/confab-web', 'jackie/notes', 'other/sandbox'];

  it('shows "All Repos" label when repos state is empty (post-CF-506 default)', () => {
    render(<TrendsFilters {...baseProps({ repos })} />);
    expect(screen.getByRole('button', { name: /repository/i })).toHaveTextContent(/all repos/i);
  });

  it('shows "All Repos" label when every available repo is explicitly selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          repos,
          value: { dateRange: defaultDateRange, repos, includeNoRepo: true, providers: [], owners: [] },
        })}
      />
    );
    expect(screen.getByRole('button', { name: /repository/i })).toHaveTextContent(/all repos/i);
  });

  it('shows "N repos" label when a subset is selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          repos,
          value: {
            dateRange: defaultDateRange,
            repos: ['ConfabulousDev/confab-web', 'jackie/notes'],
            includeNoRepo: true,
            providers: [],
            owners: [],
          },
        })}
      />
    );
    expect(screen.getByRole('button', { name: /repository/i })).toHaveTextContent(/2 repos/i);
  });

  it('does NOT show a "Select all" affordance in any state (empty=all makes it redundant)', () => {
    render(<TrendsFilters {...baseProps({ repos })} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    expect(screen.queryByText(/select all/i)).not.toBeInTheDocument();
  });

  it('hides the Clear button when repos state is empty (nothing to clear)', () => {
    render(<TrendsFilters {...baseProps({ repos })} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    expect(screen.queryByRole('button', { name: /^clear$/i })).toBeNull();
  });

  it('shows the Clear button when at least one repo is selected', () => {
    render(
      <TrendsFilters
        {...baseProps({
          repos,
          value: {
            dateRange: defaultDateRange,
            repos: ['ConfabulousDev/confab-web'],
            includeNoRepo: true,
            providers: [],
            owners: [],
          },
        })}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    expect(screen.getByRole('button', { name: /^clear$/i })).toBeInTheDocument();
  });

  it('clicking Clear resets repos to [] but leaves includeNoRepo untouched', () => {
    const onChange = vi.fn();
    render(
      <TrendsFilters
        {...baseProps({
          repos,
          onChange,
          value: {
            dateRange: defaultDateRange,
            repos: ['ConfabulousDev/confab-web', 'jackie/notes'],
            includeNoRepo: false,
            providers: [],
            owners: [],
          },
        })}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    fireEvent.click(screen.getByRole('button', { name: /^clear$/i }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ repos: [], includeNoRepo: false }),
    );
  });
});
