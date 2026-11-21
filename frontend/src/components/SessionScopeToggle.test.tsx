import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import SessionScopeToggle from './SessionScopeToggle';

describe('SessionScopeToggle (xtrf)', () => {
  it('marks the default scope as pressed', () => {
    render(<SessionScopeToggle scope="" onScopeChange={() => {}} />);
    expect(screen.getByRole('button', { name: 'Mine & shared' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'All sessions' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('marks All sessions as pressed when scope is all', () => {
    render(<SessionScopeToggle scope="all" onScopeChange={() => {}} />);
    expect(screen.getByRole('button', { name: 'All sessions' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('reports the chosen scope', () => {
    const onScopeChange = vi.fn();
    render(<SessionScopeToggle scope="" onScopeChange={onScopeChange} />);
    fireEvent.click(screen.getByRole('button', { name: 'All sessions' }));
    expect(onScopeChange).toHaveBeenCalledWith('all');
    fireEvent.click(screen.getByRole('button', { name: 'Mine & shared' }));
    expect(onScopeChange).toHaveBeenCalledWith('');
  });

  it('disables both options when disabled', () => {
    render(<SessionScopeToggle scope="" onScopeChange={() => {}} disabled />);
    for (const btn of screen.getAllByRole('button')) {
      expect(btn).toBeDisabled();
    }
  });
});
