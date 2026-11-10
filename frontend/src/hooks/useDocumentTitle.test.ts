import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useDocumentTitle } from './useDocumentTitle';

describe('useDocumentTitle', () => {
  beforeEach(() => {
    document.title = '';
  });

  it('uses the bare product name for the home page title', () => {
    renderHook(() => useDocumentTitle('Confabulous Code'));
    expect(document.title).toBe('Confabulous Code');
  });

  it('formats page titles as "<Page> | Confabulous Code"', () => {
    renderHook(() => useDocumentTitle('Sessions'));
    expect(document.title).toBe('Sessions | Confabulous Code');
  });

  it('does not treat the old short name as the home title', () => {
    renderHook(() => useDocumentTitle('Confabulous'));
    expect(document.title).toBe('Confabulous | Confabulous Code');
  });

  it('updates the title when the page title changes', () => {
    const { rerender } = renderHook(({ title }) => useDocumentTitle(title), {
      initialProps: { title: 'Sessions' },
    });
    rerender({ title: 'Trends' });
    expect(document.title).toBe('Trends | Confabulous Code');
  });

  it('resets to the bare product name on unmount', () => {
    const { unmount } = renderHook(() => useDocumentTitle('API Keys'));
    unmount();
    expect(document.title).toBe('Confabulous Code');
  });
});
