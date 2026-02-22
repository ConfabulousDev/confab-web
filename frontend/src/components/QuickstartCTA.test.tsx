import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import QuickstartCTA from './QuickstartCTA';

const DISMISS_KEY = 'quickstart-cta-dismissed';

// Mock localStorage since jsdom may not provide a fully functional implementation
function createMockLocalStorage() {
  const store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value;
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key];
    }),
    clear: vi.fn(() => {
      Object.keys(store).forEach((key) => delete store[key]);
    }),
    get length() {
      return Object.keys(store).length;
    },
    key: vi.fn((index: number) => Object.keys(store)[index] ?? null),
  };
}

describe('QuickstartCTA', () => {
  let mockStorage: ReturnType<typeof createMockLocalStorage>;

  beforeEach(() => {
    mockStorage = createMockLocalStorage();
    vi.stubGlobal('localStorage', mockStorage);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders banner when show is true and not dismissed', () => {
    render(<QuickstartCTA show={true} />);
    expect(
      screen.getByText('Set up session syncing to track your own Claude Code sessions.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Get started' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Dismiss' })).toBeInTheDocument();
  });

  it('does not render when show is false', () => {
    render(<QuickstartCTA show={false} />);
    expect(
      screen.queryByText('Set up session syncing to track your own Claude Code sessions.')
    ).not.toBeInTheDocument();
  });

  it('does not render when show is true but localStorage dismiss key is already set', () => {
    mockStorage.setItem(DISMISS_KEY, 'true');
    render(<QuickstartCTA show={true} />);
    expect(
      screen.queryByText('Set up session syncing to track your own Claude Code sessions.')
    ).not.toBeInTheDocument();
  });

  it('dismisses banner and sets localStorage when dismiss button is clicked', async () => {
    const user = userEvent.setup();
    render(<QuickstartCTA show={true} />);

    expect(
      screen.getByText('Set up session syncing to track your own Claude Code sessions.')
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Dismiss' }));

    expect(
      screen.queryByText('Set up session syncing to track your own Claude Code sessions.')
    ).not.toBeInTheDocument();
    expect(mockStorage.setItem).toHaveBeenCalledWith(DISMISS_KEY, 'true');
  });

  it('opens QuickstartModal when "Get started" button is clicked', async () => {
    const user = userEvent.setup();
    render(<QuickstartCTA show={true} />);

    // Modal should not be visible initially
    expect(screen.queryByRole('dialog', { name: 'Quickstart' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Get started' }));

    // Modal should now be open
    expect(screen.getByRole('dialog', { name: 'Quickstart' })).toBeInTheDocument();
  });
});
