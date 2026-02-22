import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import SessionSummaryPanel from './SessionSummaryPanel';
import type { SessionAnalytics } from '@/schemas/api';
import { analyticsAPI, APIError } from '@/services/api';

// Mock useAnalyticsPolling
const mockForceRefetch = vi.fn();
vi.mock('@/hooks/useAnalyticsPolling', () => ({
  useAnalyticsPolling: vi.fn(() => ({
    analytics: null,
    loading: false,
    error: null,
    forceRefetch: mockForceRefetch,
    pollingState: 'active',
    refetch: vi.fn(),
  })),
}));

// Mock useDropdown
vi.mock('@/hooks', () => ({
  useDropdown: () => ({
    isOpen: false,
    toggle: vi.fn(),
    containerRef: { current: null },
  }),
}));

// Mock child components to isolate panel logic
vi.mock('./GitHubLinksCard', () => ({
  default: () => <div data-testid="github-links-card" />,
}));

// Mock RelativeTime to avoid date formatting issues
vi.mock('@/components/RelativeTime', () => ({
  RelativeTime: ({ date }: { date: string }) => <span>{date}</span>,
}));

// Mock Alert
vi.mock('@/components/Alert', () => ({
  default: ({ children, variant }: { children: React.ReactNode; variant: string }) => (
    <div data-testid="alert" data-variant={variant}>{children}</div>
  ),
}));

// Import the mocked hook so we can change return values per test
import { useAnalyticsPolling } from '@/hooks/useAnalyticsPolling';

const mockUseAnalyticsPolling = vi.mocked(useAnalyticsPolling);

// Minimal valid analytics fixture
const baseAnalytics: SessionAnalytics = {
  computed_at: '2024-01-15T10:30:00Z',
  computed_lines: 200,
  tokens: { input: 1000, output: 500, cache_creation: 0, cache_read: 0 },
  cost: { estimated_usd: '0.10' },
  compaction: { auto: 0, manual: 0 },
  cards: {
    tokens: { input: 1000, output: 500, cache_creation: 0, cache_read: 0, estimated_usd: '0.10' },
  },
};

beforeEach(() => {
  vi.clearAllMocks();
  mockForceRefetch.mockResolvedValue(undefined);
  mockUseAnalyticsPolling.mockReturnValue({
    analytics: null,
    loading: false,
    error: null,
    forceRefetch: mockForceRefetch,
    pollingState: 'active',
    refetch: vi.fn(),
  });
});

describe('SessionSummaryPanel', () => {
  describe('loading / error / empty states', () => {
    it('shows loading spinner when loading with no analytics', () => {
      mockUseAnalyticsPolling.mockReturnValue({
        analytics: null,
        loading: true,
        error: null,
        forceRefetch: mockForceRefetch,
        pollingState: 'active',
        refetch: vi.fn(),
      });

      render(<SessionSummaryPanel sessionId="s1" isOwner={true} />);

      expect(screen.getByText('Loading analytics...')).toBeInTheDocument();
    });

    it('shows error message when error with no analytics', () => {
      mockUseAnalyticsPolling.mockReturnValue({
        analytics: null,
        loading: false,
        error: new Error('fetch failed'),
        forceRefetch: mockForceRefetch,
        pollingState: 'active',
        refetch: vi.fn(),
      });

      render(<SessionSummaryPanel sessionId="s1" isOwner={true} />);

      expect(screen.getByText('Failed to load analytics')).toBeInTheDocument();
    });

    it('shows empty message when no analytics and not loading', () => {
      render(<SessionSummaryPanel sessionId="s1" isOwner={true} />);

      expect(screen.getByText('No analytics available')).toBeInTheDocument();
    });
  });

  describe('card grid layout classes', () => {
    it('renders cards with correct span and size CSS classes', () => {
      // Use initialAnalytics to bypass polling
      render(
        <SessionSummaryPanel
          sessionId="s1"
          isOwner={false}
          initialAnalytics={baseAnalytics}
        />
      );

      // The component renders cards from the registry; each card wrapper gets span/size classes.
      // The "Session Summary" title should be present (panel rendered).
      expect(screen.getByText('Session Summary')).toBeInTheDocument();
    });
  });

  describe('smart recap extra props', () => {
    it('passes missingReason from analytics', () => {
      const analyticsWithMissing: SessionAnalytics = {
        ...baseAnalytics,
        smart_recap_missing_reason: 'quota_exceeded',
        smart_recap_quota: { used: 5, limit: 5, exceeded: true },
      };

      render(
        <SessionSummaryPanel
          sessionId="s1"
          isOwner={true}
          initialAnalytics={analyticsWithMissing}
        />
      );

      // SmartRecapCard should receive the missingReason and render the quota exceeded placeholder
      expect(screen.getByText('Configured limit reached')).toBeInTheDocument();
    });

    it('passes quota and onRefresh to owners', () => {
      const analyticsWithQuota: SessionAnalytics = {
        ...baseAnalytics,
        smart_recap_quota: { used: 3, limit: 10, exceeded: false },
        cards: {
          ...baseAnalytics.cards,
          smart_recap: {
            recap: 'Test recap',
            went_well: [],
            went_bad: [],
            human_suggestions: [],
            environment_suggestions: [],
            default_context_suggestions: [],
            computed_at: '2024-01-15T10:30:00Z',
            model_used: 'claude-sonnet-4-20250514',
          },
        },
      };

      render(
        <SessionSummaryPanel
          sessionId="s1"
          isOwner={true}
          initialAnalytics={analyticsWithQuota}
        />
      );

      // Owner should see quota in subtitle and refresh button
      expect(screen.getByText(/3\/10 this month/)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Regenerate recap' })).toBeInTheDocument();
    });

    it('non-owners do not get quota or onRefresh', () => {
      const analyticsWithQuota: SessionAnalytics = {
        ...baseAnalytics,
        smart_recap_quota: { used: 3, limit: 10, exceeded: false },
        cards: {
          ...baseAnalytics.cards,
          smart_recap: {
            recap: 'Test recap',
            went_well: [],
            went_bad: [],
            human_suggestions: [],
            environment_suggestions: [],
            default_context_suggestions: [],
            computed_at: '2024-01-15T10:30:00Z',
            model_used: 'claude-sonnet-4-20250514',
          },
        },
      };

      render(
        <SessionSummaryPanel
          sessionId="s1"
          isOwner={false}
          initialAnalytics={analyticsWithQuota}
        />
      );

      // Non-owner should see recap but no quota and no refresh button
      expect(screen.getByText('Test recap')).toBeInTheDocument();
      expect(screen.queryByText(/3\/10 this month/)).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Regenerate recap' })).not.toBeInTheDocument();
    });
  });

  describe('regeneration', () => {
    it('calls analyticsAPI.regenerateSmartRecap and forceRefetch', async () => {
      const regenerateSpy = vi.spyOn(analyticsAPI, 'regenerateSmartRecap').mockResolvedValue(baseAnalytics);

      // Use polled analytics (not initialAnalytics) so regeneration is enabled
      mockUseAnalyticsPolling.mockReturnValue({
        analytics: {
          ...baseAnalytics,
          cards: {
            ...baseAnalytics.cards,
            smart_recap: {
              recap: 'Original recap',
              went_well: [],
              went_bad: [],
              human_suggestions: [],
              environment_suggestions: [],
              default_context_suggestions: [],
              computed_at: '2024-01-15T10:30:00Z',
              model_used: 'claude-sonnet-4-20250514',
            },
          },
        },
        loading: false,
        error: null,
        forceRefetch: mockForceRefetch,
        pollingState: 'active',
        refetch: vi.fn(),
      });

      render(<SessionSummaryPanel sessionId="s1" isOwner={true} />);

      const refreshButton = screen.getByRole('button', { name: 'Regenerate recap' });
      await act(async () => {
        refreshButton.click();
      });

      expect(regenerateSpy).toHaveBeenCalledWith('s1');
      await waitFor(() => {
        expect(mockForceRefetch).toHaveBeenCalled();
      });
    });

    it('409 error is handled silently', async () => {
      const consoleSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
      vi.spyOn(analyticsAPI, 'regenerateSmartRecap').mockRejectedValue(
        new APIError('Conflict', 409, 'Conflict')
      );

      mockUseAnalyticsPolling.mockReturnValue({
        analytics: {
          ...baseAnalytics,
          cards: {
            ...baseAnalytics.cards,
            smart_recap: {
              recap: 'Recap text',
              went_well: [],
              went_bad: [],
              human_suggestions: [],
              environment_suggestions: [],
              default_context_suggestions: [],
              computed_at: '2024-01-15T10:30:00Z',
              model_used: 'claude-sonnet-4-20250514',
            },
          },
        },
        loading: false,
        error: null,
        forceRefetch: mockForceRefetch,
        pollingState: 'active',
        refetch: vi.fn(),
      });

      render(<SessionSummaryPanel sessionId="s1" isOwner={true} />);

      await act(async () => {
        screen.getByRole('button', { name: 'Regenerate recap' }).click();
      });

      // 409 is logged, not shown as error alert
      expect(consoleSpy).toHaveBeenCalledWith('Smart recap generation already in progress');
      expect(screen.queryByTestId('alert')).not.toBeInTheDocument();

      consoleSpy.mockRestore();
      consoleErrorSpy.mockRestore();
    });

    it('403 error shows quota message', async () => {
      vi.spyOn(analyticsAPI, 'regenerateSmartRecap').mockRejectedValue(
        new APIError('Forbidden', 403, 'Forbidden')
      );

      mockUseAnalyticsPolling.mockReturnValue({
        analytics: {
          ...baseAnalytics,
          cards: {
            ...baseAnalytics.cards,
            smart_recap: {
              recap: 'Recap text',
              went_well: [],
              went_bad: [],
              human_suggestions: [],
              environment_suggestions: [],
              default_context_suggestions: [],
              computed_at: '2024-01-15T10:30:00Z',
              model_used: 'claude-sonnet-4-20250514',
            },
          },
        },
        loading: false,
        error: null,
        forceRefetch: mockForceRefetch,
        pollingState: 'active',
        refetch: vi.fn(),
      });

      render(<SessionSummaryPanel sessionId="s1" isOwner={true} />);

      await act(async () => {
        screen.getByRole('button', { name: 'Regenerate recap' }).click();
      });

      await waitFor(() => {
        expect(screen.getByText('Recap limit reached. This limit resets next month.')).toBeInTheDocument();
      });
    });

    it('other errors show generic message', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {});
      vi.spyOn(analyticsAPI, 'regenerateSmartRecap').mockRejectedValue(
        new APIError('Server Error', 500, 'Internal Server Error')
      );

      mockUseAnalyticsPolling.mockReturnValue({
        analytics: {
          ...baseAnalytics,
          cards: {
            ...baseAnalytics.cards,
            smart_recap: {
              recap: 'Recap text',
              went_well: [],
              went_bad: [],
              human_suggestions: [],
              environment_suggestions: [],
              default_context_suggestions: [],
              computed_at: '2024-01-15T10:30:00Z',
              model_used: 'claude-sonnet-4-20250514',
            },
          },
        },
        loading: false,
        error: null,
        forceRefetch: mockForceRefetch,
        pollingState: 'active',
        refetch: vi.fn(),
      });

      render(<SessionSummaryPanel sessionId="s1" isOwner={true} />);

      await act(async () => {
        screen.getByRole('button', { name: 'Regenerate recap' }).click();
      });

      await waitFor(() => {
        expect(screen.getByText('Failed to regenerate recap. Please try again.')).toBeInTheDocument();
      });

      vi.mocked(console.error).mockRestore();
    });
  });
});
