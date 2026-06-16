import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import {
  AdminUnpricedModelsPageContent,
  type AdminUnpricedModelsPageContentProps,
} from './AdminUnpricedModelsPage';

const base: AdminUnpricedModelsPageContentProps = {
  data: { models: [] },
  isLoading: false,
  error: null,
};

describe('AdminUnpricedModelsPageContent', () => {
  it('renders a row per unpriced model with provider label, family, and count', () => {
    render(
      <AdminUnpricedModelsPageContent
        {...base}
        data={{
          models: [
            { provider: 'claude-code', family: 'opus-9', session_count: 42, last_seen: '2026-06-16T12:00:00Z' },
            { provider: 'codex', family: 'gpt-9', session_count: 7, last_seen: '2026-06-15T09:30:00Z' },
          ],
        }}
      />,
    );

    // providerLabel maps claude-code → "Claude Code"; the raw family is shown verbatim.
    expect(screen.getByText('Claude Code')).toBeInTheDocument();
    expect(screen.getByText('opus-9')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
    expect(screen.getByText('gpt-9')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
  });

  it('shows an empty state when there are no unpriced models', () => {
    render(<AdminUnpricedModelsPageContent {...base} data={{ models: [] }} />);
    expect(screen.getByText(/No unpriced models/i)).toBeInTheDocument();
  });

  it('shows a loading state while fetching', () => {
    render(<AdminUnpricedModelsPageContent {...base} data={null} isLoading />);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('shows an error alert when the fetch failed', () => {
    render(
      <AdminUnpricedModelsPageContent {...base} data={null} error="Failed to load unpriced models." />,
    );
    expect(screen.getByText('Failed to load unpriced models.')).toBeInTheDocument();
  });
});
