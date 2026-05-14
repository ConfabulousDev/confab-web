import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import CopyIdDropdown from './CopyIdDropdown';

const writeTextMock = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  vi.clearAllMocks();
  Object.assign(navigator, {
    clipboard: { writeText: writeTextMock },
  });
});

const defaultProps = {
  confabId: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
  externalId: 'x9y8z7w6-v5u4-3210-fedc-ba0987654321',
};

describe('CopyIdDropdown', () => {
  describe('chip trigger (detail page)', () => {
    it('shows truncated UUID in chip', () => {
      render(<CopyIdDropdown {...defaultProps} showChip />);
      expect(screen.getByText('a1b2c3d4')).toBeInTheDocument();
    });

    it('opens dropdown on click', () => {
      render(<CopyIdDropdown {...defaultProps} showChip />);
      expect(screen.queryByText(/Copy Confab ID/)).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));

      expect(screen.getByText(/Copy Confab ID/)).toBeInTheDocument();
      expect(screen.getByText(/Copy Claude Code ID/)).toBeInTheDocument();
    });

    it('copies Confab ID when selected', () => {
      render(<CopyIdDropdown {...defaultProps} showChip />);
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));
      fireEvent.click(screen.getByText(/Copy Confab ID/));

      expect(writeTextMock).toHaveBeenCalledWith(defaultProps.confabId);
    });

    it('copies Claude Code ID when selected', () => {
      render(<CopyIdDropdown {...defaultProps} showChip />);
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));
      fireEvent.click(screen.getByText(/Copy Claude Code ID/));

      expect(writeTextMock).toHaveBeenCalledWith(defaultProps.externalId);
    });

    it('closes dropdown after copy with delay', () => {
      vi.useFakeTimers();
      render(<CopyIdDropdown {...defaultProps} showChip />);
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));
      fireEvent.click(screen.getByText(/Copy Confab ID/));

      // Still open briefly for confirmation
      expect(screen.getByText(/Copy Confab ID/)).toBeInTheDocument();

      act(() => { vi.advanceTimersByTime(700); });

      // Closed after delay
      expect(screen.queryByText(/Copy Confab ID/)).not.toBeInTheDocument();
      vi.useRealTimers();
    });
  });

  describe('icon trigger (list page)', () => {
    it('renders a copy icon button', () => {
      render(<CopyIdDropdown {...defaultProps} />);
      expect(screen.getByRole('button', { name: 'Copy session ID' })).toBeInTheDocument();
    });

    it('opens the same dropdown on click', () => {
      render(<CopyIdDropdown {...defaultProps} />);
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));

      expect(screen.getByText(/Copy Confab ID/)).toBeInTheDocument();
      expect(screen.getByText(/Copy Claude Code ID/)).toBeInTheDocument();
    });
  });

  describe('provider variants', () => {
    it('defaults to Claude Code label when provider is omitted', () => {
      render(<CopyIdDropdown {...defaultProps} showChip />);
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));

      expect(screen.getByText(/Copy Claude Code ID/)).toBeInTheDocument();
      expect(screen.getByText('for /resume')).toBeInTheDocument();
    });

    it('uses Codex label and hint when provider is codex', () => {
      render(<CopyIdDropdown {...defaultProps} provider="codex" showChip />);
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));

      expect(screen.getByText(/Copy Codex ID/)).toBeInTheDocument();
      expect(screen.getByText('for codex resume')).toBeInTheDocument();
      expect(screen.queryByText(/Copy Claude Code ID/)).not.toBeInTheDocument();
    });

    it('copies external ID under the codex label', () => {
      render(<CopyIdDropdown {...defaultProps} provider="codex" showChip />);
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));
      fireEvent.click(screen.getByText(/Copy Codex ID/));

      expect(writeTextMock).toHaveBeenCalledWith(defaultProps.externalId);
    });
  });

  describe('click outside', () => {
    it('closes dropdown when clicking outside', () => {
      render(
        <div>
          <span data-testid="outside">outside</span>
          <CopyIdDropdown {...defaultProps} showChip />
        </div>
      );
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));
      expect(screen.getByText(/Copy Confab ID/)).toBeInTheDocument();

      fireEvent.mouseDown(screen.getByTestId('outside'));
      expect(screen.queryByText(/Copy Confab ID/)).not.toBeInTheDocument();
    });
  });

  describe('event propagation', () => {
    it('stops propagation on chip click (prevents row navigation)', () => {
      const parentClick = vi.fn();
      render(
        <div onClick={parentClick}>
          <CopyIdDropdown {...defaultProps} showChip />
        </div>
      );
      fireEvent.click(screen.getByRole('button', { name: 'Copy session ID' }));
      expect(parentClick).not.toHaveBeenCalled();
    });
  });
});
