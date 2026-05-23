import type { Meta, StoryObj } from '@storybook/react-vite';
import { useEffect, type ReactNode } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import Header from './Header';

// Mock user for authenticated state
const mockUser = {
  id: 'user-123',
  name: 'Jane Developer',
  email: 'jane@example.com',
  avatar_url: 'https://avatars.githubusercontent.com/u/1?v=4',
};

// Create QueryClient with pre-populated auth data
function createQueryClient(user: typeof mockUser | null) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  });
  // Pre-populate the auth cache
  client.setQueryData(['auth', 'me'], user);
  return client;
}

const meta: Meta<typeof Header> = {
  title: 'Components/Header',
  component: Header,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof Header>;

export const LoggedOut: Story = {
  decorators: [
    (Story) => (
      <QueryClientProvider client={createQueryClient(null)}>
        <Story />
      </QueryClientProvider>
    ),
  ],
};

export const LoggedIn: Story = {
  decorators: [
    (Story) => (
      <QueryClientProvider client={createQueryClient(mockUser)}>
        <Story />
      </QueryClientProvider>
    ),
  ],
};

export const LoggedInNoAvatar: Story = {
  decorators: [
    (Story) => (
      <QueryClientProvider client={createQueryClient({ ...mockUser, avatar_url: '' })}>
        <Story />
      </QueryClientProvider>
    ),
  ],
};

// CF-483: when DEMO_IDENTITY_EMAIL is set, the backend injects
// window.__DEMO_IDENTITY__ and the header shows a "demo" badge next to
// the logo.
function WithDemoIdentity({ email, children }: { email: string; children: ReactNode }) {
  useEffect(() => {
    window.__DEMO_IDENTITY__ = email;
    return () => {
      delete window.__DEMO_IDENTITY__;
    };
  }, [email]);
  return <>{children}</>;
}

export const DemoModeLoggedIn: Story = {
  decorators: [
    (Story) => (
      <WithDemoIdentity email="demo@confabulous.dev">
        <QueryClientProvider client={createQueryClient({ ...mockUser, email: 'demo@confabulous.dev' })}>
          <Story />
        </QueryClientProvider>
      </WithDemoIdentity>
    ),
  ],
};

export const DemoModeLoggedOut: Story = {
  decorators: [
    (Story) => (
      <WithDemoIdentity email="demo@confabulous.dev">
        <QueryClientProvider client={createQueryClient(null)}>
          <Story />
        </QueryClientProvider>
      </WithDemoIdentity>
    ),
  ],
};
