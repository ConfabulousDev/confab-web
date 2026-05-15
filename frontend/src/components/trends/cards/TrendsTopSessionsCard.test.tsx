import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { TrendsTopSessionsCard } from './TrendsTopSessionsCard';
import type { TrendsTopSessionsCard as TrendsTopSessionsCardData } from '@/schemas/api';

// CF-381 pins per-row provider icons in the Costliest Sessions card.
// Unlike the app-wide getProviderIcon (which defaults to Claude for unknown
// values), this card surfaces a neutral ChatIcon for empty/unknown providers
// so we don't claim Claude identity for unidentified rows.
describe('TrendsTopSessionsCard provider icons', () => {
  const makeData = (provider: string): TrendsTopSessionsCardData => ({
    sessions: [
      {
        id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        title: 'Some session',
        provider,
        estimated_cost_usd: '5.00',
        duration_ms: 60000,
        git_repo: 'org/repo',
      },
    ],
  });

  it('renders CodexIcon for codex sessions', () => {
    render(<TrendsTopSessionsCard data={makeData('codex')} />);
    const row = screen.getByText('Some session').closest('a');
    expect(row).not.toBeNull();
    expect(within(row!).getByTestId('icon-codex')).toBeInTheDocument();
    expect(within(row!).queryByTestId('icon-claude')).not.toBeInTheDocument();
    expect(within(row!).queryByTestId('icon-chat')).not.toBeInTheDocument();
  });

  it('renders ClaudeCodeIcon for claude-code sessions', () => {
    render(<TrendsTopSessionsCard data={makeData('claude-code')} />);
    const row = screen.getByText('Some session').closest('a');
    expect(row).not.toBeNull();
    expect(within(row!).getByTestId('icon-claude')).toBeInTheDocument();
    expect(within(row!).queryByTestId('icon-codex')).not.toBeInTheDocument();
    expect(within(row!).queryByTestId('icon-chat')).not.toBeInTheDocument();
  });

  it('renders ChatIcon for unknown providers (not Claude — divergent fallback)', () => {
    render(<TrendsTopSessionsCard data={makeData('windsurf')} />);
    const row = screen.getByText('Some session').closest('a');
    expect(row).not.toBeNull();
    expect(within(row!).getByTestId('icon-chat')).toBeInTheDocument();
    expect(within(row!).queryByTestId('icon-claude')).not.toBeInTheDocument();
    expect(within(row!).queryByTestId('icon-codex')).not.toBeInTheDocument();
  });
});
