import type { Meta, StoryObj } from '@storybook/react-vite';
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
