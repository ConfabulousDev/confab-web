import type { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import AdminSystemSharesPage from './AdminSystemSharesPage';
import { AppConfigContext, type AppConfig } from '@/contexts/AppConfigContext';
import { defaultVersionInfo } from '@/contexts/appConfigDefaults';
import type { AdminSystemSharesResponse } from '@/schemas/api';

const appConfig: AppConfig = {
  sharesEnabled: true,
  saasFooterEnabled: false,
  saasTermlyEnabled: false,
  orgAnalyticsEnabled: false,
  passwordAuthEnabled: false,
  smartRecapEnabled: false,
  supportEmail: '',
  version: defaultVersionInfo,
};

function createQueryClient(seed?: AdminSystemSharesResponse): QueryClient {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  if (seed) {
    client.setQueryData(['admin', 'system-shares'], seed);
  }
  return client;
}

interface DecoratorProps {
  seed?: AdminSystemSharesResponse;
  children: ReactNode;
}

function StoryProviders({ seed, children }: DecoratorProps) {
  return (
    <AppConfigContext.Provider value={appConfig}>
      <QueryClientProvider client={createQueryClient(seed)}>{children}</QueryClientProvider>
    </AppConfigContext.Provider>
  );
}

const populated: AdminSystemSharesResponse = {
  shares: [
    {
      id: 1,
      session_id: '01952a64-1111-2222-3333-444444444444',
      external_id: '01952a64-1111-2222-3333-444444444444',
      provider: 'claude-code',
      share_url: 'https://confab.example.com/sessions/01952a64-1111-2222-3333-444444444444',
      created_at: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
      last_accessed_at: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
      expires_at: null,
    },
    {
      id: 2,
      session_id: '7b3f1c2d-aaaa-bbbb-cccc-dddddddddddd',
      external_id: '7b3f1c2d-aaaa-bbbb-cccc-dddddddddddd',
      provider: 'codex',
      share_url: 'https://confab.example.com/sessions/7b3f1c2d-aaaa-bbbb-cccc-dddddddddddd',
      created_at: new Date(Date.now() - 1000 * 60 * 60 * 24 * 3).toISOString(),
      last_accessed_at: null,
      expires_at: new Date(Date.now() + 1000 * 60 * 60 * 24 * 7).toISOString(),
    },
  ],
};

const meta: Meta<typeof AdminSystemSharesPage> = {
  title: 'Pages/Admin/AdminSystemSharesPage',
  component: AdminSystemSharesPage,
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
type Story = StoryObj<typeof AdminSystemSharesPage>;

export const Empty: Story = {
  decorators: [
    (Story) => (
      <StoryProviders seed={{ shares: [] }}>
        <Story />
      </StoryProviders>
    ),
  ],
};

export const Populated: Story = {
  decorators: [
    (Story) => (
      <StoryProviders seed={populated}>
        <Story />
      </StoryProviders>
    ),
  ],
};

// Loading state: no seed → React Query stays in pending state because the
// underlying fetch can't reach a real backend from Storybook.
export const Loading: Story = {
  decorators: [
    (Story) => (
      <StoryProviders>
        <Story />
      </StoryProviders>
    ),
  ],
};
