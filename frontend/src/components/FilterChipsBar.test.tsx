import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import FilterChipsBar from './FilterChipsBar';
import type { SessionFilterOptions } from '@/schemas/api';

const sampleFilterOptions: SessionFilterOptions = {
  repos: ['confab-web'],
  branches: ['main'],
  owners: ['alice@co.com'],
  providers: ['claude-code', 'codex'],
};

function baseProps(overrides: Partial<React.ComponentProps<typeof FilterChipsBar>> = {}) {
  return {
    filters: { repos: [], branches: [], owners: [], providers: [], query: '' },
    filterOptions: sampleFilterOptions,
    currentUserEmail: 'alice@co.com',
    onToggleRepo: vi.fn(),
    onToggleBranch: vi.fn(),
    onToggleOwner: vi.fn(),
    onToggleProvider: vi.fn(),
    onQueryChange: vi.fn(),
    onClearAll: vi.fn(),
    ...overrides,
  };
}

describe('FilterChipsBar Provider filter (CF-393)', () => {
  it('renders a Provider dropdown trigger', () => {
    render(<FilterChipsBar {...baseProps()} />);
    expect(screen.getByRole('button', { name: /provider/i })).toBeInTheDocument();
  });

  it('shows both providers in the dropdown with display labels', () => {
    render(<FilterChipsBar {...baseProps()} />);
    fireEvent.click(screen.getByRole('button', { name: /provider/i }));

    expect(screen.getByText('Claude Code')).toBeInTheDocument();
    expect(screen.getByText('Codex')).toBeInTheDocument();
  });

  it('renders the Provider dropdown even when filterOptions is null', () => {
    // Provider options are static — they do not depend on backend data.
    render(<FilterChipsBar {...baseProps({ filterOptions: null })} />);
    expect(screen.getByRole('button', { name: /provider/i })).toBeInTheDocument();
  });

  it('Provider chip sits leftmost in the dimension row (before Repo)', () => {
    render(<FilterChipsBar {...baseProps()} />);
    const buttons = screen.getAllByRole('button');
    const providerIdx = buttons.findIndex((b) => /provider/i.test(b.textContent || ''));
    const repoIdx = buttons.findIndex((b) => /repo/i.test(b.textContent || ''));
    expect(providerIdx).toBeGreaterThanOrEqual(0);
    expect(repoIdx).toBeGreaterThan(providerIdx);
  });

  it('clicking a Provider option calls onToggleProvider with the canonical value', () => {
    const onToggleProvider = vi.fn();
    render(<FilterChipsBar {...baseProps({ onToggleProvider })} />);

    fireEvent.click(screen.getByRole('button', { name: /provider/i }));
    fireEvent.click(screen.getByText('Codex'));

    expect(onToggleProvider).toHaveBeenCalledWith('codex');
  });

  it('shows a numeric badge when one Provider is selected', () => {
    render(
      <FilterChipsBar {...baseProps({
        filters: { repos: [], branches: [], owners: [], providers: ['claude-code'], query: '' },
      })} />
    );
    // The dimension trigger has aria-expanded; the active-filter pill does not.
    const providerBtn = screen.getByRole('button', { name: /provider/i, expanded: false });
    expect(providerBtn.textContent).toMatch(/1/);
  });

  it('shows badge=2 when both providers are selected', () => {
    render(
      <FilterChipsBar {...baseProps({
        filters: { repos: [], branches: [], owners: [], providers: ['claude-code', 'codex'], query: '' },
      })} />
    );
    const providerBtn = screen.getByRole('button', { name: /provider/i, expanded: false });
    expect(providerBtn.textContent).toMatch(/2/);
  });

  it('renders an active-filter pill with the display label when a provider is selected', () => {
    render(
      <FilterChipsBar {...baseProps({
        filters: { repos: [], branches: [], owners: [], providers: ['codex'], query: '' },
      })} />
    );

    // The active-filter row shows `provider: Codex` (icon + text)
    const chip = screen.getByRole('button', { name: /provider:.*codex/i });
    expect(chip).toBeInTheDocument();
  });

  it('omits the Provider dropdown when showProviderFilter={false}', () => {
    render(<FilterChipsBar {...baseProps({ showProviderFilter: false })} />);
    expect(screen.queryByRole('button', { name: /provider/i })).not.toBeInTheDocument();
  });

  it('omits provider active pills when showProviderFilter={false} even if providers selected', () => {
    render(
      <FilterChipsBar {...baseProps({
        showProviderFilter: false,
        filters: { repos: [], branches: [], owners: [], providers: ['codex'], query: '' },
      })} />
    );
    expect(screen.queryByRole('button', { name: /provider:/i })).not.toBeInTheDocument();
  });

  it('clicking the active pill toggles the provider off', () => {
    const onToggleProvider = vi.fn();
    render(
      <FilterChipsBar {...baseProps({
        filters: { repos: [], branches: [], owners: [], providers: ['codex'], query: '' },
        onToggleProvider,
      })} />
    );

    const chip = screen.getByRole('button', { name: /provider:.*codex/i });
    fireEvent.click(chip);

    expect(onToggleProvider).toHaveBeenCalledWith('codex');
  });
});

// Six repos so the search input appears (options.length > 5).
const sixRepos = ['alpha', 'beta', 'confab-cli', 'confab-web', 'delta', 'epsilon'];

describe('DimensionDropdown divider (CF-511)', () => {
  it('shows a divider when both selected and unselected items are present', () => {
    render(
      <FilterChipsBar {...baseProps({
        filterOptions: { repos: sixRepos, branches: ['main'], owners: ['alice@co.com'] },
        filters: { repos: ['confab-web'], branches: [], owners: [], providers: [], query: '' },
      })} />
    );
    fireEvent.click(screen.getByRole('button', { name: /repo/i, expanded: false }));
    expect(screen.getByTestId('dimension-divider')).toBeInTheDocument();
  });

  it('does not show a divider when no items are selected', () => {
    render(
      <FilterChipsBar {...baseProps({
        filterOptions: { repos: sixRepos, branches: ['main'], owners: ['alice@co.com'] },
        filters: { repos: [], branches: [], owners: [], providers: [], query: '' },
      })} />
    );
    fireEvent.click(screen.getByRole('button', { name: /repo/i, expanded: false }));
    expect(screen.queryByTestId('dimension-divider')).not.toBeInTheDocument();
  });

  it('does not show a divider when all items are selected', () => {
    render(
      <FilterChipsBar {...baseProps({
        filterOptions: { repos: sixRepos, branches: ['main'], owners: ['alice@co.com'] },
        filters: { repos: [...sixRepos], branches: [], owners: [], providers: [], query: '' },
      })} />
    );
    fireEvent.click(screen.getByRole('button', { name: /repo/i, expanded: false }));
    expect(screen.queryByTestId('dimension-divider')).not.toBeInTheDocument();
  });

  it('keeps the divider during search when both groups remain visible', () => {
    // 'conf' matches confab-web (selected) and confab-cli (unselected) — both groups present
    render(
      <FilterChipsBar {...baseProps({
        filterOptions: { repos: sixRepos, branches: ['main'], owners: ['alice@co.com'] },
        filters: { repos: ['confab-web'], branches: [], owners: [], providers: [], query: '' },
      })} />
    );
    fireEvent.click(screen.getByRole('button', { name: /repo/i, expanded: false }));
    fireEvent.change(screen.getByPlaceholderText(/search repo/i), { target: { value: 'conf' } });
    expect(screen.getByTestId('dimension-divider')).toBeInTheDocument();
  });

  it('removes the divider during search when only unselected items match', () => {
    // 'cli' only matches confab-cli which is unselected — selected group disappears from view
    render(
      <FilterChipsBar {...baseProps({
        filterOptions: { repos: sixRepos, branches: ['main'], owners: ['alice@co.com'] },
        filters: { repos: ['confab-web'], branches: [], owners: [], providers: [], query: '' },
      })} />
    );
    fireEvent.click(screen.getByRole('button', { name: /repo/i, expanded: false }));
    fireEvent.change(screen.getByPlaceholderText(/search repo/i), { target: { value: 'cli' } });
    expect(screen.queryByTestId('dimension-divider')).not.toBeInTheDocument();
  });

  it('removes the divider during search when only selected items match', () => {
    // 'web' only matches confab-web which is selected — unselected group disappears from view
    render(
      <FilterChipsBar {...baseProps({
        filterOptions: { repos: sixRepos, branches: ['main'], owners: ['alice@co.com'] },
        filters: { repos: ['confab-web'], branches: [], owners: [], providers: [], query: '' },
      })} />
    );
    fireEvent.click(screen.getByRole('button', { name: /repo/i, expanded: false }));
    fireEvent.change(screen.getByPlaceholderText(/search repo/i), { target: { value: 'web' } });
    expect(screen.queryByTestId('dimension-divider')).not.toBeInTheDocument();
  });
});
