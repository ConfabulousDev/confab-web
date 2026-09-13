import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import type { GitHubLink } from '@/services/api';
import GitHubLinksCard from './GitHubLinksCard';

vi.mock('@/hooks', () => ({
  useVisibility: () => true,
}));

vi.mock('@/services/api', () => ({
  githubLinksAPI: {
    list: vi.fn(),
    create: vi.fn(),
    delete: vi.fn(),
  },
}));

import { githubLinksAPI } from '@/services/api';

const mockList = vi.mocked(githubLinksAPI.list);

const ADD_FORM_PLACEHOLDER = 'https://github.com/owner/repo/pull/123';

const prLink: GitHubLink = {
  id: 1,
  session_id: 'session-1',
  link_type: 'pull_request',
  url: 'https://github.com/acme/widgets/pull/42',
  owner: 'acme',
  repo: 'widgets',
  ref: '42',
  title: 'Add widgets',
  source: 'manual',
  created_at: '2026-01-01T00:00:00Z',
};

describe('GitHubLinksCard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('auto-opens the add form once loading finishes when forceShow is set and there are no links', async () => {
    mockList.mockResolvedValue({ links: [] });

    render(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);

    expect(await screen.findByPlaceholderText(ADD_FORM_PLACEHOLDER)).toBeInTheDocument();
  });

  it('does not show the add form while links are still loading', () => {
    mockList.mockReturnValue(new Promise(() => {}));

    render(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);

    expect(screen.getByText('Loading...')).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(ADD_FORM_PLACEHOLDER)).not.toBeInTheDocument();
  });

  it('does not auto-open the add form when links are present', async () => {
    mockList.mockResolvedValue({ links: [prLink] });

    render(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);

    expect(await screen.findByText('#42')).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(ADD_FORM_PLACEHOLDER)).not.toBeInTheDocument();
  });

  it('keeps the auto-opened add form closed after the user cancels it', async () => {
    mockList.mockResolvedValue({ links: [] });

    const { rerender } = render(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);
    await screen.findByPlaceholderText(ADD_FORM_PLACEHOLDER);

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(screen.queryByPlaceholderText(ADD_FORM_PLACEHOLDER)).not.toBeInTheDocument();
    expect(screen.getByText('No linked PRs or commits')).toBeInTheDocument();

    rerender(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);
    expect(screen.queryByPlaceholderText(ADD_FORM_PLACEHOLDER)).not.toBeInTheDocument();
  });

  it('reopens the add form when the user clicks "Add link" after cancelling', async () => {
    mockList.mockResolvedValue({ links: [] });

    render(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);
    await screen.findByPlaceholderText(ADD_FORM_PLACEHOLDER);

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Add link' }));

    expect(screen.getByPlaceholderText(ADD_FORM_PLACEHOLDER)).toBeInTheDocument();
  });

  it('auto-opens the add form again when the card is hidden and revealed after a dismissal', async () => {
    mockList.mockResolvedValue({ links: [] });

    const { rerender } = render(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);
    await screen.findByPlaceholderText(ADD_FORM_PLACEHOLDER);
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    rerender(<GitHubLinksCard sessionId="session-1" isOwner forceShow={false} />);
    rerender(<GitHubLinksCard sessionId="session-1" isOwner forceShow />);

    expect(screen.getByPlaceholderText(ADD_FORM_PLACEHOLDER)).toBeInTheDocument();
  });

  it('renders nothing for an owner when forceShow is off', async () => {
    mockList.mockResolvedValue({ links: [] });

    const { container } = render(<GitHubLinksCard sessionId="session-1" isOwner />);

    await waitFor(() => expect(mockList).toHaveBeenCalled());
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it('reports link availability through onHasLinksChange', async () => {
    const onHasLinksChange = vi.fn();
    mockList.mockResolvedValue({ links: [prLink] });

    render(
      <GitHubLinksCard sessionId="session-1" isOwner forceShow onHasLinksChange={onHasLinksChange} />,
    );

    await waitFor(() => expect(onHasLinksChange).toHaveBeenLastCalledWith(true));
  });

  it('reports no links through onHasLinksChange when the list is empty', async () => {
    const onHasLinksChange = vi.fn();
    mockList.mockResolvedValue({ links: [] });

    render(
      <GitHubLinksCard sessionId="session-1" isOwner forceShow onHasLinksChange={onHasLinksChange} />,
    );

    await waitFor(() => expect(onHasLinksChange).toHaveBeenLastCalledWith(false));
    expect(onHasLinksChange).not.toHaveBeenCalledWith(true);
  });
});
