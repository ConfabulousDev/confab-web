import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AdminUsersPage from './AdminUsersPage';
import { AppConfigContext, type AppConfig } from '@/contexts/AppConfigContext';
import { defaultVersionInfo } from '@/contexts/appConfigDefaults';
import type { AdminUserListResponse, User } from '@/schemas/api';

// kyrr: the destructive user-action modals (deactivate/delete) require the admin to
// re-type the target email before the confirm button enables. These tests pin that
// disabled-until-match gating; the network mutation itself is never fired.

const appConfig: AppConfig = {
  sharesEnabled: true,
  saasFooterEnabled: false,
  saasTermlyEnabled: false,
  orgAnalyticsEnabled: false,
  passwordAuthEnabled: true,
  smartRecapEnabled: false,
  supportEmail: '',
  version: defaultVersionInfo,
};

const currentUser: User = { email: 'me@example.com', name: 'Me', is_admin: true };

type AdminUser = AdminUserListResponse['users'][number];

function adminUser(over: Partial<AdminUser> & { id: number; email: string }): AdminUser {
  return {
    name: over.email.split('@')[0] ?? over.email,
    status: 'active',
    session_count: 1,
    recap_cache_count: 0,
    recaps_this_month: 0,
    last_api_key_used: null,
    last_logged_in: null,
    created_at: new Date('2026-01-01T00:00:00Z').toISOString(),
    is_admin: false,
    is_super_admin: false,
    ...over,
  };
}

const usersResponse: AdminUserListResponse = {
  users: [
    adminUser({ id: 2, email: 'me@example.com', is_admin: true }),
    adminUser({ id: 4, email: 'target@example.com' }),
  ],
  totals: { total_sessions: 2, non_empty_sessions: 1, sessions_with_cache: 0, computations_this_month: 0 },
};

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  client.setQueryData(['admin', 'users'], usersResponse);
  client.setQueryData(['auth', 'me'], currentUser);
  return render(
    <AppConfigContext.Provider value={appConfig}>
      <QueryClientProvider client={client}>
        <AdminUsersPage />
      </QueryClientProvider>
    </AppConfigContext.Provider>,
  );
}

// openDeleteModal clicks Delete on the target row and returns the modal dialog.
async function openDeleteModal(user: ReturnType<typeof userEvent.setup>): Promise<HTMLElement> {
  const row = screen.getByRole('row', { name: /target@example\.com/ });
  await user.click(within(row).getByRole('button', { name: 'Delete' }));
  return screen.getByRole('dialog');
}

describe('AdminUsersPage destructive-action confirmation (kyrr)', () => {
  it('disables the delete confirm button until the target email is typed', async () => {
    const user = userEvent.setup();
    renderPage();
    const dialog = await openDeleteModal(user);

    const confirmBtn = within(dialog).getByRole('button', { name: 'Delete' });
    expect(confirmBtn).toBeDisabled();

    const input = within(dialog).getByPlaceholderText('target@example.com');
    await user.type(input, 'wrong@example.com');
    expect(confirmBtn).toBeDisabled();

    await user.clear(input);
    await user.type(input, 'target@example.com');
    expect(confirmBtn).toBeEnabled();
  });

  it('matches the target email case-insensitively', async () => {
    const user = userEvent.setup();
    renderPage();
    const dialog = await openDeleteModal(user);

    const confirmBtn = within(dialog).getByRole('button', { name: 'Delete' });
    await user.type(within(dialog).getByPlaceholderText('target@example.com'), '  Target@Example.COM  ');
    expect(confirmBtn).toBeEnabled();
  });

  it('requires the email echo on deactivate too', async () => {
    const user = userEvent.setup();
    renderPage();
    const row = screen.getByRole('row', { name: /target@example\.com/ });
    await user.click(within(row).getByRole('button', { name: 'Deactivate' }));

    const dialog = screen.getByRole('dialog');
    const confirmBtn = within(dialog).getByRole('button', { name: 'Deactivate' });
    expect(confirmBtn).toBeDisabled();
    await user.type(within(dialog).getByPlaceholderText('target@example.com'), 'target@example.com');
    expect(confirmBtn).toBeEnabled();
  });
});
