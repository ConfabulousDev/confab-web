import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { PrecomputeFailuresResponse } from '@/schemas/api';

vi.mock('@/services/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/services/api')>();
  return {
    ...actual,
    adminAPI: {
      listPrecomputeFailures: vi.fn(),
      releasePrecomputeFailure: vi.fn(),
      releaseAllPrecomputeFailures: vi.fn(),
    },
  };
});

import { adminAPI } from '@/services/api';
import AdminPrecomputeFailuresPage, {
  AdminPrecomputeFailuresPageContent,
  type AdminPrecomputeFailuresPageContentProps,
} from './AdminPrecomputeFailuresPage';

const SESSION_A = '11111111-1111-4111-8111-111111111111';
const SESSION_B = '22222222-2222-4222-8222-222222222222';

const twoRows: PrecomputeFailuresResponse = {
  failures: [
    {
      session_id: SESSION_A,
      user_id: 1,
      external_id: 'ext-a',
      provider: 'codex',
      bucket: 'regular_cards',
      attempts: 5,
      last_error: 'parse rollout: line too long',
      quarantined_at: '2026-10-19T12:00:00Z',
    },
    {
      session_id: SESSION_B,
      user_id: 2,
      external_id: 'ext-b',
      provider: 'claude-code',
      bucket: 'smart_recap',
      attempts: 6,
      last_error: 'worker exited during attempt',
      quarantined_at: '2026-10-18T08:00:00Z',
    },
  ],
};

const noop = () => {};
const base: AdminPrecomputeFailuresPageContentProps = {
  data: { failures: [] },
  isLoading: false,
  error: null,
  feedback: null,
  onFeedbackClose: noop,
  onRelease: noop,
  onReleaseAll: noop,
  releasingKey: null,
  isReleasingAll: false,
};

function renderContent(props: Partial<AdminPrecomputeFailuresPageContentProps>) {
  return render(
    <MemoryRouter>
      <AdminPrecomputeFailuresPageContent {...base} {...props} />
    </MemoryRouter>,
  );
}

describe('AdminPrecomputeFailuresPageContent', () => {
  it('renders a row per quarantined session with link, provider, bucket, attempts and error', () => {
    renderContent({ data: twoRows });

    const rows = screen.getAllByRole('row').slice(1); // skip header
    expect(rows).toHaveLength(2);

    const first = within(rows[0]!);
    expect(first.getByRole('link', { name: 'ext-a' })).toHaveAttribute('href', `/sessions/${SESSION_A}`);
    expect(first.getByText('Codex')).toBeInTheDocument();
    expect(first.getByText('Regular cards')).toBeInTheDocument();
    expect(first.getByText('5')).toBeInTheDocument();
    const err = first.getByText('parse rollout: line too long');
    expect(err).toHaveAttribute('title', 'parse rollout: line too long');

    const second = within(rows[1]!);
    expect(second.getByText('Smart recap')).toBeInTheDocument();
    expect(second.getByText('Claude Code')).toBeInTheDocument();
  });

  it('shows an empty state and hides Release all when nothing is quarantined', () => {
    renderContent({ data: { failures: [] } });
    expect(screen.getByText(/No quarantined sessions/i)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Release all/i })).not.toBeInTheDocument();
  });

  it('calls onRelease with the clicked row', async () => {
    const onRelease = vi.fn();
    renderContent({ data: twoRows, onRelease });
    const rows = screen.getAllByRole('row').slice(1);
    await userEvent.click(within(rows[1]!).getByRole('button', { name: 'Release' }));
    expect(onRelease).toHaveBeenCalledTimes(1);
    expect(onRelease).toHaveBeenCalledWith(twoRows.failures[1]);
  });

  it('marks the pending row and disables every release button while one release is in flight', () => {
    const { unmount } = renderContent({ data: twoRows, releasingKey: `${SESSION_A} regular_cards` });
    const rows = screen.getAllByRole('row').slice(1);
    expect(within(rows[0]!).getByRole('button', { name: 'Releasing...' })).toBeDisabled();
    expect(within(rows[1]!).getByRole('button', { name: 'Release' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Release all' })).toBeDisabled();
    unmount();

    renderContent({ data: twoRows, isReleasingAll: true });
    expect(screen.getByRole('button', { name: /Releasing all/i })).toBeDisabled();
  });

  it('shows loading and error states', () => {
    const { unmount } = renderContent({ data: null, isLoading: true });
    expect(screen.getByText('Loading...')).toBeInTheDocument();
    unmount();
    renderContent({ data: null, error: 'Failed to load precompute failures.' });
    expect(screen.getByText('Failed to load precompute failures.')).toBeInTheDocument();
  });
});

describe('AdminPrecomputeFailuresPage', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.listPrecomputeFailures).mockReset().mockResolvedValue(twoRows);
    vi.mocked(adminAPI.releasePrecomputeFailure).mockReset().mockResolvedValue({ released: 1 });
    vi.mocked(adminAPI.releaseAllPrecomputeFailures).mockReset().mockResolvedValue({ released: 2 });
  });

  function renderPage() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return render(
      <MemoryRouter>
        <QueryClientProvider client={client}>
          <AdminPrecomputeFailuresPage />
        </QueryClientProvider>
      </MemoryRouter>,
    );
  }

  it('Release calls the release endpoint with the row session and bucket, then refetches', async () => {
    renderPage();
    const link = await screen.findByRole('link', { name: 'ext-a' });
    const row = link.closest('tr')!;
    await userEvent.click(within(row).getByRole('button', { name: 'Release' }));

    await waitFor(() =>
      expect(adminAPI.releasePrecomputeFailure).toHaveBeenCalledWith({ session_id: SESSION_A, bucket: 'regular_cards' }),
    );
    await waitFor(() => expect(adminAPI.listPrecomputeFailures).toHaveBeenCalledTimes(2));
    expect(await screen.findByText(/Released 1 quarantine/i)).toBeInTheDocument();
  });

  it('Release all calls the release-all endpoint', async () => {
    renderPage();
    await userEvent.click(await screen.findByRole('button', { name: 'Release all' }));
    await waitFor(() => expect(adminAPI.releaseAllPrecomputeFailures).toHaveBeenCalledTimes(1));
    expect(await screen.findByText(/Released 2 quarantines/i)).toBeInTheDocument();
  });
});
