import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import ErrorBoundary from './ErrorBoundary';

const GITHUB_ISSUES_URL = 'https://github.com/ConfabulousDev/confab-web/issues';

function Boom(): never {
  throw new Error('kaboom');
}

describe('ErrorBoundary report-issue link (CF-571)', () => {
  // The fallback logs the caught error; silence it to keep test output clean.
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders a Report an issue link in the fallback when a child throws', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>
    );
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'Report an issue' });
    expect(link).toHaveAttribute('href', GITHUB_ISSUES_URL);
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('does not render the report link when there is no error', () => {
    render(
      <ErrorBoundary>
        <div>all good</div>
      </ErrorBoundary>
    );
    expect(screen.getByText('all good')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Report an issue' })).toBeNull();
  });
});
