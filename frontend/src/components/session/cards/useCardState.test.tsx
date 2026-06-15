import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { useCardState } from './useCardState';

// useCardState is a pure element-returning helper; render its output directly.
function Harness<T>(props: {
  data: T | null;
  loading: boolean;
  error?: string;
}) {
  const guard = useCardState(props.data, props.loading, props.error, { title: 'Tokens' });
  return guard ?? <div data-testid="ready">ready</div>;
}

describe('useCardState', () => {
  it('renders CardError when there is an error and no data', () => {
    const { getByText } = render(<Harness data={null} loading={false} error="boom" />);
    expect(getByText(/Failed to compute: boom/)).toBeInTheDocument();
    expect(getByText('Tokens')).toBeInTheDocument();
  });

  it('renders the loading placeholder when loading and no data', () => {
    const { getByText, queryByText } = render(<Harness data={null} loading error={undefined} />);
    expect(getByText('Loading...')).toBeInTheDocument();
    expect(getByText('Tokens')).toBeInTheDocument();
    expect(queryByText(/Failed to compute/)).toBeNull();
  });

  it('returns null (card proceeds) once data is present', () => {
    const { getByTestId } = render(<Harness data={{ x: 1 }} loading={false} error={undefined} />);
    expect(getByTestId('ready')).toBeInTheDocument();
  });

  it('prefers existing data over a late loading flag (no flicker to placeholder)', () => {
    const { getByTestId, queryByText } = render(<Harness data={{ x: 1 }} loading error="boom" />);
    expect(getByTestId('ready')).toBeInTheDocument();
    expect(queryByText('Loading...')).toBeNull();
  });
});
