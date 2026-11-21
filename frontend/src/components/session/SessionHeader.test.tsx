import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import SessionHeader from './SessionHeader';

// Wrap component with router since it uses Link
const renderWithRouter = (ui: React.ReactElement) => {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
};

describe('SessionHeader', () => {
  const defaultProps = {
    sessionId: '123e4567-e89b-12d3-a456-426614174000',
    externalId: 'abc12345-6789-0def-ghij-klmnopqrstuv',
    provider: 'claude-code',
    ownerEmail: 'owner@example.com',
  };

  // xtrf: admin viewing another user's session
  describe('viewing as admin', () => {
    it('shows the "Viewing as admin" badge with the owner email', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} isOwner={false} isShared={true} viewingAsAdmin={true} />
      );
      expect(screen.getByText('Viewing as admin')).toBeInTheDocument();
      // Owner email appears in the badge (it may also appear in the owner chip).
      const badge = screen.getByText('Viewing as admin').parentElement!;
      expect(within(badge).getByText('owner@example.com')).toBeInTheDocument();
      expect(screen.queryByText('Shared Session')).not.toBeInTheDocument();
    });

    it('hides owner-only controls for admin viewers', () => {
      renderWithRouter(
        <SessionHeader
          {...defaultProps}
          isOwner={false}
          isShared={true}
          viewingAsAdmin={true}
          onShare={() => {}}
          onDelete={() => {}}
        />
      );
      expect(screen.queryByRole('button', { name: 'Share' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument();
    });

    it('shows the regular shared indicator when not an admin view', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} isOwner={false} isShared={true} />
      );
      expect(screen.getByText('Shared Session')).toBeInTheDocument();
      expect(screen.queryByText('Viewing as admin')).not.toBeInTheDocument();
    });
  });

  describe('UUID chip', () => {
    it('renders truncated backend UUID as a chip', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} isOwner={true} />
      );
      // First 8 chars of the UUID
      expect(screen.getByText('123e4567')).toBeInTheDocument();
    });

    it('renders UUID chip for shared sessions too', () => {
      renderWithRouter(
        <SessionHeader
          {...defaultProps}
          isShared={true}
          isOwner={false}
          sharedByEmail="owner@example.com"
        />
      );
      expect(screen.getByText('123e4567')).toBeInTheDocument();
    });
  });

  describe('sharedByEmail', () => {
    it('renders "Shared Session" when sharedByEmail is provided and not owner', () => {
      renderWithRouter(
        <SessionHeader
          {...defaultProps}
          isShared={true}
          isOwner={false}
          sharedByEmail="owner@example.com"
        />
      );

      expect(screen.getByText('Shared Session')).toBeInTheDocument();
    });

    it('falls back to "Shared Session" when sharedByEmail is null', () => {
      renderWithRouter(
        <SessionHeader
          {...defaultProps}
          isShared={true}
          isOwner={false}
          sharedByEmail={null}
        />
      );

      expect(screen.getByText('Shared Session')).toBeInTheDocument();
    });

    it('falls back to "Shared Session" when sharedByEmail is undefined', () => {
      renderWithRouter(
        <SessionHeader
          {...defaultProps}
          isShared={true}
          isOwner={false}
        />
      );

      expect(screen.getByText('Shared Session')).toBeInTheDocument();
    });

    it('shows "Shared Session" for owner viewing shared session (clickable link)', () => {
      renderWithRouter(
        <SessionHeader
          {...defaultProps}
          isShared={true}
          isOwner={true}
          sharedByEmail="owner@example.com"
        />
      );

      // Owner sees "Shared Session" text (clickable to switch to owner view)
      expect(screen.getByText('Shared Session')).toBeInTheDocument();
      // And it should be a link
      expect(screen.getByRole('link')).toHaveAttribute('href', `/sessions/${defaultProps.sessionId}`);
    });

    it('does not show shared indicator when session is not shared', () => {
      renderWithRouter(
        <SessionHeader
          {...defaultProps}
          isShared={false}
          isOwner={true}
          onShare={vi.fn()}
        />
      );

      expect(screen.queryByText(/Shared/)).not.toBeInTheDocument();
      // Owner should see Share button instead
      expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument();
    });
  });

  describe('provider icon', () => {
    it('renders the Claude brand icon for claude-code sessions', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} provider="claude-code" model="claude-opus-4-7" />
      );
      expect(screen.getByTestId('icon-claude')).toBeInTheDocument();
      expect(screen.queryByTestId('icon-codex')).not.toBeInTheDocument();
    });

    it('renders the Codex brand icon for codex sessions', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} provider="codex" model="gpt-5-codex" />
      );
      expect(screen.getByTestId('icon-codex')).toBeInTheDocument();
      expect(screen.queryByTestId('icon-claude')).not.toBeInTheDocument();
    });
  });

  // CF-383: when `model` is missing the provider icon must still render and
  // the meta-item value falls back to the provider brandDisplayName from
  // PROVIDER_METADATA (Codex / Claude Code). Queries are scoped to the meta
  // row via `session-meta` data-testid so the assertions don't collide with
  // any title-row text that happens to contain "Codex" or "Claude Code".
  describe('provider meta-item — model fallback', () => {
    it('shows Codex brand icon for codex sessions even when model is missing', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} provider="codex" model={undefined} />
      );
      const meta = within(screen.getByTestId('session-meta'));
      expect(meta.getByTestId('icon-codex')).toBeInTheDocument();
      expect(meta.getByText('Codex')).toBeInTheDocument();
    });

    it('shows Claude Code brand icon for claude sessions even when model is missing', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} provider="claude-code" model={undefined} />
      );
      const meta = within(screen.getByTestId('session-meta'));
      expect(meta.getByTestId('icon-claude')).toBeInTheDocument();
      expect(meta.getByText('Claude Code')).toBeInTheDocument();
    });

    it('shows formatted model name for codex sessions when model is present', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} provider="codex" model="gpt-5-codex" />
      );
      const meta = within(screen.getByTestId('session-meta'));
      expect(meta.getByTestId('icon-codex')).toBeInTheDocument();
      expect(meta.getByText('gpt-5-codex')).toBeInTheDocument();
      expect(meta.queryByText('Codex')).not.toBeInTheDocument();
    });

    it('shows formatted model name for claude sessions when model is present', () => {
      renderWithRouter(
        <SessionHeader {...defaultProps} provider="claude-code" model="claude-opus-4-7" />
      );
      const meta = within(screen.getByTestId('session-meta'));
      expect(meta.getByTestId('icon-claude')).toBeInTheDocument();
      expect(meta.queryByText('Claude Code')).not.toBeInTheDocument();
    });
  });
});
