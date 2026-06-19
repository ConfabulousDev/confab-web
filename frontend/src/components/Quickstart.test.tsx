import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Quickstart from './Quickstart';

const mockCopy = vi.fn();
let mockCopied = false;

vi.mock('@/hooks', () => ({
  useCopyToClipboard: () => ({
    copy: mockCopy,
    copied: mockCopied,
  }),
}));

const INSTALL_CMD =
  'curl -fsSL https://raw.githubusercontent.com/ConfabulousDev/confab/main/install.sh | bash';
const END_USER_DOCS = 'https://docs.confabulous.dev/getting-started/end-user-quickstart/';
const GITHUB_INSTALL_DOCS =
  'https://github.com/ConfabulousDev/confab?tab=readme-ov-file#installation';

beforeEach(() => {
  mockCopy.mockClear();
  mockCopied = false;
});

describe('Quickstart', () => {
  it('landing variant renders install and setup commands', () => {
    render(<Quickstart variant="landing" />);
    expect(screen.getByText(INSTALL_CMD)).toBeInTheDocument();
    expect(screen.getByText(/confab setup --backend-url/)).toBeInTheDocument();
    expect(screen.getByText(/Use Claude Code or Codex as usual/)).toBeInTheDocument();
  });

  it('landing variant links to the end-user quickstart docs', () => {
    render(<Quickstart variant="landing" />);
    const link = screen.getByRole('link', { name: /full quickstart guide/i });
    expect(link).toHaveAttribute('href', END_USER_DOCS);
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('embedded variant keeps the GitHub installation docs link', () => {
    render(<Quickstart />);
    const link = screen.getByRole('link', { name: /view installation docs/i });
    expect(link).toHaveAttribute('href', GITHUB_INSTALL_DOCS);
  });

  it('copy button invokes clipboard copy with the install command', async () => {
    const user = userEvent.setup();
    render(<Quickstart variant="landing" />);
    const copyButtons = screen.getAllByRole('button', { name: 'Copy to clipboard' });
    expect(copyButtons.length).toBeGreaterThan(0);
    await user.click(copyButtons[0]!);
    expect(mockCopy).toHaveBeenCalledWith(INSTALL_CMD);
  });
});
