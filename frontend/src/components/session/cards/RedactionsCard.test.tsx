import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { RedactionsCard } from './RedactionsCard';
import type { RedactionsCardData } from '@/schemas/api';

describe('RedactionsCard', () => {
  it('returns null when total_redactions is 0', () => {
    const { container } = render(
      <RedactionsCard
        data={{ total_redactions: 0, redaction_counts: {} }}
        loading={false}
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders rows sorted by count descending', () => {
    const data: RedactionsCardData = {
      total_redactions: 8,
      redaction_counts: { jwt: 2, api_key: 5, password: 1 },
    };
    const { container } = render(<RedactionsCard data={data} loading={false} />);
    const labels = Array.from(container.querySelectorAll('[class*="statLabel"]'))
      .map((el) => el.textContent?.trim());
    expect(labels).toEqual(['api_key', 'jwt', 'password']);
  });

  it('renders pluralized tooltip text on each row', () => {
    const data: RedactionsCardData = {
      total_redactions: 5,
      redaction_counts: { api_key: 5 },
    };
    const { container } = render(<RedactionsCard data={data} loading={false} />);
    const row = container.querySelector('[title*="occurrences of [REDACTED:api_key]"]');
    expect(row).not.toBeNull();
  });

  it('renders singular "occurrence" when count is 1', () => {
    const data: RedactionsCardData = {
      total_redactions: 1,
      redaction_counts: { jwt: 1 },
    };
    const { container } = render(<RedactionsCard data={data} loading={false} />);
    const row = container.querySelector('[title="1 occurrence of [REDACTED:jwt]"]');
    expect(row).not.toBeNull();
  });

  it('renders subtitle "N total"', () => {
    const data: RedactionsCardData = {
      total_redactions: 7,
      redaction_counts: { api_key: 7 },
    };
    const { getByText } = render(<RedactionsCard data={data} loading={false} />);
    expect(getByText('7 total')).toBeInTheDocument();
  });

  it('renders loading state', () => {
    const { getByText } = render(<RedactionsCard data={null} loading={true} />);
    expect(getByText('Redactions')).toBeInTheDocument();
    expect(getByText('Loading...')).toBeInTheDocument();
  });

  it('renders CardError', () => {
    const { getByText } = render(
      <RedactionsCard data={null} loading={false} error="oops" />
    );
    expect(getByText(/Failed to compute: oops/)).toBeInTheDocument();
  });
});
