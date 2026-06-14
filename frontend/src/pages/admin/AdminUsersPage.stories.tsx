import type { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import AdminUsersPage from './AdminUsersPage';
import { AppConfigContext, type AppConfig } from '@/contexts/AppConfigContext';
import { defaultVersionInfo } from '@/contexts/appConfigDefaults';
import type { AdminUserListResponse, User } from '@/schemas/api';

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

type AdminUser = AdminUserListResponse['users'][number];

function adminUser(over: Partial<AdminUser> & { id: number; email: string }): AdminUser {
  return {
    name: over.email.split('@')[0] ?? over.email,
    status: 'active',
    session_count: 3,
    recap_cache_count: 0,
    recaps_this_month: 0,
    last_api_key_used: null,
    last_logged_in: new Date(Date.now() - 1000 * 60 * 60).toISOString(),
    created_at: new Date(Date.now() - 1000 * 60 * 60 * 24 * 30).toISOString(),
    is_admin: false,
    is_super_admin: false,
    ...over,
  };
}

// A list with one of each: env super-admin (read-only indicator), column admin
// (Revoke admin), the current user as a column admin (self-demote → soft
// confirm), and a plain user (Make admin).
const users: AdminUser[] = [
  adminUser({ id: 1, email: 'env-super@example.com', is_super_admin: true }),
  adminUser({ id: 2, email: 'me@example.com', is_admin: true }),
  adminUser({ id: 3, email: 'col-admin@example.com', is_admin: true }),
  adminUser({ id: 4, email: 'plain@example.com' }),
];

const usersResponse: AdminUserListResponse = {
  users,
  totals: {
    total_sessions: 12,
    non_empty_sessions: 9,
    sessions_with_cache: 4,
    computations_this_month: 7,
  },
};

// Current user (for self-demote detection) is the column admin "me@example.com".
const currentUser: User = {
  email: 'me@example.com',
  name: 'Me',
  is_admin: true,
};

function createQueryClient(seed: AdminUserListResponse | undefined, me: User): QueryClient {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  if (seed) {
    client.setQueryData(['admin', 'users'], seed);
  }
  client.setQueryData(['auth', 'me'], me);
  return client;
}

interface DecoratorProps {
  seed?: AdminUserListResponse;
  me?: User;
  children: ReactNode;
}

function StoryProviders({ seed, me = currentUser, children }: DecoratorProps) {
  return (
    <AppConfigContext.Provider value={appConfig}>
      <QueryClientProvider client={createQueryClient(seed, me)}>{children}</QueryClientProvider>
    </AppConfigContext.Provider>
  );
}

const meta: Meta<typeof AdminUsersPage> = {
  title: 'Pages/Admin/AdminUsersPage',
  component: AdminUsersPage,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AdminUsersPage>;

// Mixed list: env super-admin, column admins (one is the current user → self
// demote soft-confirm), and a plain user (Make admin).
export const AdminRoles: Story = {
  decorators: [
    (Story) => (
      <StoryProviders seed={usersResponse}>
        <Story />
      </StoryProviders>
    ),
  ],
};

// All non-super-admin: shows the Make-admin / Revoke-admin toggle range.
export const ColumnAdminsOnly: Story = {
  decorators: [
    (Story) => (
      <StoryProviders
        seed={{
          ...usersResponse,
          users: users.filter((u) => !u.is_super_admin),
        }}
      >
        <Story />
      </StoryProviders>
    ),
  ],
};

// Only an env super-admin + a plain user — the env admin shows the read-only
// "Admin · env" indicator (no toggle).
export const EnvSuperAdminIndicator: Story = {
  decorators: [
    (Story) => (
      <StoryProviders
        seed={{
          ...usersResponse,
          users: users.filter((u) => u.is_super_admin || (!u.is_admin && !u.is_super_admin)),
        }}
        me={{ email: 'env-super@example.com', name: 'Env', is_admin: true }}
      >
        <Story />
      </StoryProviders>
    ),
  ],
};
