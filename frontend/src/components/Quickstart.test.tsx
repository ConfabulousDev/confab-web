import type { ReactElement } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
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
const GITHUB_INSTALL_DOCS =
  'https://github.com/ConfabulousDev/confab?tab=readme-ov-file#installation';

function renderQuickstart(ui: ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

beforeEach(() => {
  mockCopy.mockClear();
  mockCopied = false;
});

describe('Quickstart', () => {
  it('landing variant renders install and setup commands', () => {
    renderQuickstart(<Quickstart variant="landing" />);
    expect(screen.getByText(INSTALL_CMD)).toBeInTheDocument();
    expect(screen.getByText(/confab setup --backend-url/)).toBeInTheDocument();
    expect(screen.getByText(/Work in your coding sessions as usual/)).toBeInTheDocument();
  });

  it('landing variant links "here" to the session listing', () => {
    renderQuickstart(<Quickstart variant="landing" />);
    expect(screen.getByRole('link', { name: 'here' })).toHaveAttribute('href', '/sessions');
  });

  it('landing variant omits the bottom quickstart-guide link', () => {
    renderQuickstart(<Quickstart variant="landing" />);
    expect(screen.queryByRole('link', { name: /quickstart guide/i })).not.toBeInTheDocument();
  });

  it('embedded variant keeps the GitHub installation docs link', () => {
    renderQuickstart(<Quickstart />);
    const link = screen.getByRole('link', { name: /view installation docs/i });
    expect(link).toHaveAttribute('href', GITHUB_INSTALL_DOCS);
  });

  it('copy button invokes clipboard copy with the install command', async () => {
    const user = userEvent.setup();
    renderQuickstart(<Quickstart variant="landing" />);
    const copyButtons = screen.getAllByRole('button', { name: 'Copy to clipboard' });
    expect(copyButtons.length).toBeGreaterThan(0);
    await user.click(copyButtons[0]!);
    expect(mockCopy).toHaveBeenCalledWith(INSTALL_CMD);
  });
});
