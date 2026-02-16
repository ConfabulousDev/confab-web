import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
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
    ownerEmail: 'owner@example.com',
  };

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
});
