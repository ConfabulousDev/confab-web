import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { RelativeTime } from './RelativeTime';

// Mock useVisibility
vi.mock('@/hooks/useVisibility', () => ({
  useVisibility: vi.fn(() => true),
}));

describe('RelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2025-01-15T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('renders formatted relative time', () => {
    // 30 seconds ago
    render(<RelativeTime date="2025-01-15T11:59:30Z" />);
    expect(screen.getByText('30s ago')).toBeInTheDocument();
  });

  it('updates over time', () => {
    // 30 seconds ago
    render(<RelativeTime date="2025-01-15T11:59:30Z" />);
    expect(screen.getByText('30s ago')).toBeInTheDocument();

    // Advance 10 seconds
    act(() => {
      vi.advanceTimersByTime(10000);
    });

    expect(screen.getByText('40s ago')).toBeInTheDocument();
  });

  it('renders different time units correctly', () => {
    const { rerender } = render(<RelativeTime date="2025-01-15T11:59:30Z" />);
    expect(screen.getByText('30s ago')).toBeInTheDocument();

    // 5 minutes ago
    rerender(<RelativeTime date="2025-01-15T11:55:00Z" />);
    expect(screen.getByText('5m ago')).toBeInTheDocument();

    // 2 hours ago
    rerender(<RelativeTime date="2025-01-15T10:00:00Z" />);
    expect(screen.getByText('2h ago')).toBeInTheDocument();

    // 1 day ago
    rerender(<RelativeTime date="2025-01-14T12:00:00Z" />);
    expect(screen.getByText('1d ago')).toBeInTheDocument();
  });
});
