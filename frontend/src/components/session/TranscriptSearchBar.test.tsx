import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createRef } from 'react';
import TranscriptSearchBar from './TranscriptSearchBar';

function renderSearchBar(overrides: Partial<React.ComponentProps<typeof TranscriptSearchBar>> = {}) {
  const defaultProps: React.ComponentProps<typeof TranscriptSearchBar> = {
    query: '',
    onQueryChange: vi.fn(),
    currentMatch: 0,
    totalMatches: 0,
    onNext: vi.fn(),
    onPrev: vi.fn(),
    onClose: vi.fn(),
    inputRef: createRef<HTMLInputElement>(),
    ...overrides,
  };
  return { ...render(<TranscriptSearchBar {...defaultProps} />), props: defaultProps };
}

describe('TranscriptSearchBar', () => {
  it('renders input with placeholder', () => {
    renderSearchBar();
    expect(screen.getByPlaceholderText('Search transcript...')).toBeInTheDocument();
  });

  it('displays match count when query is non-empty', () => {
    renderSearchBar({ query: 'test', currentMatch: 2, totalMatches: 5 });
    expect(screen.getByText('2 of 5')).toBeInTheDocument();
  });

  it('displays "0 of 0" when query has no matches', () => {
    renderSearchBar({ query: 'nope', currentMatch: 0, totalMatches: 0 });
    expect(screen.getByText('0 of 0')).toBeInTheDocument();
  });

  it('does not display match count when query is empty', () => {
    renderSearchBar({ query: '', totalMatches: 0 });
    expect(screen.queryByText(/of/)).not.toBeInTheDocument();
  });

  it('calls onQueryChange when typing', async () => {
    const user = userEvent.setup();
    const { props } = renderSearchBar();

    await user.type(screen.getByPlaceholderText('Search transcript...'), 'hello');
    expect(props.onQueryChange).toHaveBeenCalled();
  });

  it('calls onNext on Enter', async () => {
    const user = userEvent.setup();
    const { props } = renderSearchBar({ query: 'test', totalMatches: 3, currentMatch: 1 });

    const input = screen.getByPlaceholderText('Search transcript...');
    await user.click(input);
    await user.keyboard('{Enter}');

    expect(props.onNext).toHaveBeenCalledTimes(1);
  });

  it('calls onPrev on Shift+Enter', async () => {
    const user = userEvent.setup();
    const { props } = renderSearchBar({ query: 'test', totalMatches: 3, currentMatch: 1 });

    const input = screen.getByPlaceholderText('Search transcript...');
    await user.click(input);
    await user.keyboard('{Shift>}{Enter}{/Shift}');

    expect(props.onPrev).toHaveBeenCalledTimes(1);
  });

  it('calls onClose on Escape', async () => {
    const user = userEvent.setup();
    const { props } = renderSearchBar();

    const input = screen.getByPlaceholderText('Search transcript...');
    await user.click(input);
    await user.keyboard('{Escape}');

    expect(props.onClose).toHaveBeenCalledTimes(1);
  });

  it('calls onNext when clicking next button', async () => {
    const user = userEvent.setup();
    const { props } = renderSearchBar({ query: 'test', totalMatches: 2, currentMatch: 1 });

    await user.click(screen.getByTitle('Next match (Enter)'));
    expect(props.onNext).toHaveBeenCalledTimes(1);
  });

  it('calls onPrev when clicking previous button', async () => {
    const user = userEvent.setup();
    const { props } = renderSearchBar({ query: 'test', totalMatches: 2, currentMatch: 1 });

    await user.click(screen.getByTitle('Previous match (Shift+Enter)'));
    expect(props.onPrev).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when clicking close button', async () => {
    const user = userEvent.setup();
    const { props } = renderSearchBar();

    await user.click(screen.getByTitle('Close search (Escape)'));
    expect(props.onClose).toHaveBeenCalledTimes(1);
  });

  it('disables nav buttons when no matches', () => {
    renderSearchBar({ query: 'test', totalMatches: 0 });

    expect(screen.getByTitle('Previous match (Shift+Enter)')).toBeDisabled();
    expect(screen.getByTitle('Next match (Enter)')).toBeDisabled();
  });

  it('enables nav buttons when matches exist', () => {
    renderSearchBar({ query: 'test', totalMatches: 3, currentMatch: 1 });

    expect(screen.getByTitle('Previous match (Shift+Enter)')).not.toBeDisabled();
    expect(screen.getByTitle('Next match (Enter)')).not.toBeDisabled();
  });
});
