import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import RepoFilter, { type RepoFilterProps } from './RepoFilter';

const repos = ['ConfabulousDev/confab-web', 'jackie/notes', 'other/sandbox'];

function props(overrides: Partial<RepoFilterProps> = {}): RepoFilterProps {
  return {
    availableRepos: repos,
    selectedRepos: [],
    includeNoRepo: true,
    onChange: vi.fn(),
    ...overrides,
  };
}

// CF-233 / CF-506: empty repos[] means "all repos"; selecting every available
// repo is semantically the same, so the label collapses back to "All Repos".
describe('RepoFilter label (CF-233/CF-506)', () => {
  it('shows "All Repos" when selectedRepos is empty', () => {
    render(<RepoFilter {...props()} />);
    expect(screen.getByRole('button', { name: /repository/i })).toHaveTextContent(/all repos/i);
  });

  it('shows "All Repos" when every available repo is explicitly selected', () => {
    render(<RepoFilter {...props({ selectedRepos: repos })} />);
    expect(screen.getByRole('button', { name: /repository/i })).toHaveTextContent(/all repos/i);
  });

  it('shows "N repos" (plural) when a multi-repo subset is selected', () => {
    render(<RepoFilter {...props({ selectedRepos: repos.slice(0, 2) })} />);
    expect(screen.getByRole('button', { name: /repository/i })).toHaveTextContent(/2 repos/i);
  });

  it('shows singular "1 repo" when exactly one repo is selected', () => {
    render(<RepoFilter {...props({ selectedRepos: repos.slice(0, 1) })} />);
    expect(screen.getByRole('button', { name: /repository/i })).toHaveTextContent(/1 repo\b/i);
  });
});

describe('RepoFilter button highlight', () => {
  it('is not highlighted at the all-repos default (empty + includeNoRepo true)', () => {
    render(<RepoFilter {...props()} />);
    expect(screen.getByRole('button', { name: /repository/i }).className).not.toMatch(/active/);
  });

  it('is highlighted when a strict subset is selected', () => {
    render(<RepoFilter {...props({ selectedRepos: repos.slice(0, 1) })} />);
    expect(screen.getByRole('button', { name: /repository/i }).className).toMatch(/active/);
  });

  it('is highlighted when includeNoRepo is false even with no repo subset', () => {
    render(<RepoFilter {...props({ includeNoRepo: false })} />);
    expect(screen.getByRole('button', { name: /repository/i }).className).toMatch(/active/);
  });

  it('is NOT highlighted when every repo is selected and includeNoRepo is true', () => {
    render(<RepoFilter {...props({ selectedRepos: repos })} />);
    expect(screen.getByRole('button', { name: /repository/i }).className).not.toMatch(/active/);
  });
});

// CF-233: empty=all makes a "Select all" affordance redundant; Clear only
// appears when there's a selection to clear, and never touches includeNoRepo.
describe('RepoFilter Clear (CF-233)', () => {
  it('hides the Clear button when selectedRepos is empty', () => {
    render(<RepoFilter {...props()} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    expect(screen.queryByRole('button', { name: /^clear$/i })).toBeNull();
  });

  it('shows the Clear button when at least one repo is selected', () => {
    render(<RepoFilter {...props({ selectedRepos: repos.slice(0, 1) })} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    expect(screen.getByRole('button', { name: /^clear$/i })).toBeInTheDocument();
  });

  it('clicking Clear resets repos to [] but leaves includeNoRepo untouched', () => {
    const onChange = vi.fn();
    render(
      <RepoFilter {...props({ selectedRepos: repos.slice(0, 2), includeNoRepo: false, onChange })} />
    );
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    fireEvent.click(screen.getByRole('button', { name: /^clear$/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ repos: [], includeNoRepo: false });
  });

  it('does NOT render a "Select all" affordance in any state', () => {
    render(<RepoFilter {...props()} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    expect(screen.queryByText(/select all/i)).not.toBeInTheDocument();
  });
});

describe('RepoFilter toggles', () => {
  it('toggling include-no-repo emits includeNoRepo flipped with repos preserved', () => {
    const onChange = vi.fn();
    render(<RepoFilter {...props({ selectedRepos: repos.slice(0, 1), onChange })} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /include sessions without repo/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ repos: repos.slice(0, 1), includeNoRepo: false });
  });

  it('checking an unselected repo adds it to the selection', () => {
    const onChange = vi.fn();
    render(<RepoFilter {...props({ onChange })} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'jackie/notes' }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ repos: ['jackie/notes'], includeNoRepo: true });
  });

  it('unchecking a selected repo removes it from the selection', () => {
    const onChange = vi.fn();
    render(<RepoFilter {...props({ selectedRepos: ['jackie/notes'], onChange })} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'jackie/notes' }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ repos: [], includeNoRepo: true });
  });
});

describe('RepoFilter with no available repos', () => {
  it('renders only the include-no-repo checkbox and no "Filter by repo" list', () => {
    render(<RepoFilter {...props({ availableRepos: [] })} />);
    fireEvent.click(screen.getByRole('button', { name: /repository/i }));
    expect(
      screen.getByRole('checkbox', { name: /include sessions without repo/i })
    ).toBeInTheDocument();
    expect(screen.queryByText(/filter by repo/i)).not.toBeInTheDocument();
  });
});
