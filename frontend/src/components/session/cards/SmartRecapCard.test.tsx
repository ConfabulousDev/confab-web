import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { SmartRecapCard } from './SmartRecapCard';
import type { SmartRecapCardData, SmartRecapQuotaInfo } from '@/schemas/api';

/** Shorthand to create an AnnotatedItem */
const a = (text: string, message_id?: string) => ({ text, message_id });

const mockData: SmartRecapCardData = {
  recap: 'The user refactored the auth module and fixed a login bug.',
  went_well: [a('Clean separation of concerns'), a('Good test coverage', 'uuid-1')],
  went_bad: [a('Missed edge case in token refresh', 'uuid-2')],
  human_suggestions: [a('Add retry logic for network failures')],
  environment_suggestions: [a('Update Node.js to v20')],
  default_context_suggestions: [a('Add CLAUDE.md entry for auth patterns')],
  computed_at: '2024-01-15T10:30:00Z',
  model_used: 'claude-sonnet-4-20250514',
};

const mockQuota: SmartRecapQuotaInfo = {
  used: 3,
  limit: 10,
  exceeded: false,
};

describe('SmartRecapCard', () => {
  it('renders recap text and subtitle with model name trimmed of date suffix', () => {
    render(<SmartRecapCard data={mockData} loading={false} />);

    expect(screen.getByText('Smart Recap')).toBeInTheDocument();
    expect(screen.getByText(mockData.recap)).toBeInTheDocument();
    // Model name should have date suffix stripped: "claude-sonnet-4-20250514" -> "claude-sonnet-4"
    expect(screen.getByText(/claude-sonnet-4(?!\d)/)).toBeInTheDocument();
  });

  it('subtitle includes quota info when quota prop provided', () => {
    render(<SmartRecapCard data={mockData} loading={false} quota={mockQuota} />);

    expect(screen.getByText(/3\/10 this month/)).toBeInTheDocument();
  });

  it('subtitle excludes quota info when no quota', () => {
    render(<SmartRecapCard data={mockData} loading={false} />);

    expect(screen.queryByText(/this month$/)).not.toBeInTheDocument();
  });

  it('loading state shows "Loading..."', () => {
    render(<SmartRecapCard data={null} loading={true} />);

    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('refreshing state shows "Generating AI recap..."', () => {
    render(<SmartRecapCard data={null} loading={false} isRefreshing={true} />);

    expect(screen.getByText('Generating AI recap...')).toBeInTheDocument();
  });

  it('error state shows CardError', () => {
    render(<SmartRecapCard data={null} loading={false} error="compute failed" />);

    expect(screen.getByText(/Failed to compute: compute failed/)).toBeInTheDocument();
  });

  it('missingReason=quota_exceeded shows quota placeholder with limit text', () => {
    render(
      <SmartRecapCard
        data={null}
        loading={false}
        missingReason="quota_exceeded"
        quota={mockQuota}
      />
    );

    expect(screen.getByText('Configured limit reached')).toBeInTheDocument();
    expect(screen.getByText('The per-user monthly recap limit has been reached. This resets next month.')).toBeInTheDocument();
    expect(screen.getByText('3/10 this month')).toBeInTheDocument();
  });

  it('missingReason=unavailable shows unavailable placeholder', () => {
    render(
      <SmartRecapCard data={null} loading={false} missingReason="unavailable" />
    );

    expect(screen.getByText('No smart recap available for this session.')).toBeInTheDocument();
  });

  it('no data and no missingReason returns null', () => {
    const { container } = render(
      <SmartRecapCard data={null} loading={false} />
    );

    expect(container).toBeEmptyDOMElement();
  });

  it('renders went_well items', () => {
    render(<SmartRecapCard data={mockData} loading={false} />);

    expect(screen.getByText('Went Well')).toBeInTheDocument();
    expect(screen.getByText('Clean separation of concerns')).toBeInTheDocument();
    expect(screen.getByText('Good test coverage')).toBeInTheDocument();
  });

  it('renders went_bad items', () => {
    render(<SmartRecapCard data={mockData} loading={false} />);

    expect(screen.getByText('Needs Improvement')).toBeInTheDocument();
    expect(screen.getByText('Missed edge case in token refresh')).toBeInTheDocument();
  });

  it('renders suggestion items from all three categories', () => {
    render(<SmartRecapCard data={mockData} loading={false} />);

    expect(screen.getByText('Suggestions')).toBeInTheDocument();
    expect(screen.getByText('Add retry logic for network failures')).toBeInTheDocument();
    expect(screen.getByText('Update Node.js to v20')).toBeInTheDocument();
    expect(screen.getByText('Add CLAUDE.md entry for auth patterns')).toBeInTheDocument();
  });

  it('skips sections with empty arrays', () => {
    const minimalData: SmartRecapCardData = {
      ...mockData,
      went_well: [],
      went_bad: [],
      human_suggestions: [],
      environment_suggestions: [],
      default_context_suggestions: [],
    };
    render(<SmartRecapCard data={minimalData} loading={false} />);

    expect(screen.getByText(mockData.recap)).toBeInTheDocument();
    expect(screen.queryByText('Went Well')).not.toBeInTheDocument();
    expect(screen.queryByText('Needs Improvement')).not.toBeInTheDocument();
    expect(screen.queryByText('Suggestions')).not.toBeInTheDocument();
  });

  it('shows refresh button when onRefresh provided', () => {
    const onRefresh = vi.fn();
    render(<SmartRecapCard data={mockData} loading={false} onRefresh={onRefresh} />);

    const button = screen.getByRole('button', { name: 'Regenerate recap' });
    expect(button).toBeInTheDocument();
    expect(button).not.toBeDisabled();

    fireEvent.click(button);
    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it('refresh button disabled when quota.exceeded is true', () => {
    const exceededQuota: SmartRecapQuotaInfo = { used: 10, limit: 10, exceeded: true };
    render(
      <SmartRecapCard
        data={mockData}
        loading={false}
        quota={exceededQuota}
        onRefresh={() => {}}
      />
    );

    const button = screen.getByRole('button', { name: 'Regenerate recap' });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('title', 'Configured limit reached');
  });

  it('no refresh button when onRefresh not provided', () => {
    render(<SmartRecapCard data={mockData} loading={false} />);

    expect(screen.queryByRole('button', { name: 'Regenerate recap' })).not.toBeInTheDocument();
  });

  // =========================================================================
  // Message reference link tests
  // =========================================================================

  it('renders message link for items with message_id when sessionId provided', () => {
    render(<SmartRecapCard data={mockData} loading={false} sessionId="sess-123" />);

    // Items with message_id should have links
    const links = screen.getAllByRole('link', { name: 'View in transcript' });
    expect(links.length).toBe(2); // uuid-1 (went_well) + uuid-2 (went_bad)

    // Verify link href and target
    expect(links[0]).toHaveAttribute('href', '/sessions/sess-123?tab=transcript&msg=uuid-1');
    expect(links[0]).toHaveAttribute('target', '_blank');
    expect(links[1]).toHaveAttribute('href', '/sessions/sess-123?tab=transcript&msg=uuid-2');
  });

  it('does not render message links when sessionId is not provided', () => {
    render(<SmartRecapCard data={mockData} loading={false} />);

    expect(screen.queryByRole('link', { name: 'View in transcript' })).not.toBeInTheDocument();
  });

  it('does not render message link for items without message_id', () => {
    const dataWithNoRefs: SmartRecapCardData = {
      ...mockData,
      went_well: [a('No ref item 1'), a('No ref item 2')],
      went_bad: [],
    };
    render(<SmartRecapCard data={dataWithNoRefs} loading={false} sessionId="sess-123" />);

    expect(screen.queryByRole('link', { name: 'View in transcript' })).not.toBeInTheDocument();
  });
});
