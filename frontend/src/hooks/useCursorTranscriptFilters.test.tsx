import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import {
  DEFAULT_CURSOR_FILTER_STATE,
  type CursorFilterState,
} from '@/components/session/cursorCategories';
import {
  DEFAULT_HIDDEN,
  pathsFromState,
  stateFromPaths,
  useCursorTranscriptFilters,
} from './useCursorTranscriptFilters';

const mockSetSearchParams = vi.fn();
let currentParams = new URLSearchParams();

vi.mock('react-router-dom', () => ({
  useSearchParams: () => [currentParams, mockSetSearchParams],
}));

beforeEach(() => {
  currentParams = new URLSearchParams();
  mockSetSearchParams.mockClear();
  mockSetSearchParams.mockImplementation(
    (updater: (prev: URLSearchParams) => URLSearchParams) => {
      if (typeof updater === 'function') {
        currentParams = updater(currentParams);
      }
    },
  );
});

describe('pathsFromState / stateFromPaths', () => {
  it('DEFAULT_HIDDEN is empty (all categories visible by default)', () => {
    expect(DEFAULT_HIDDEN).toEqual([]);
  });

  it('emits a token only for hidden categories', () => {
    expect(pathsFromState({ user: true, assistant: false, tool: false })).toEqual([
      'assistant',
      'tool',
    ]);
  });

  it('treats foreign tokens as no-ops (cross-provider URLs)', () => {
    // 'user.prompt' / 'unknown' belong to other providers; Cursor categories are
    // flat, so foreign tokens leave every Cursor category visible.
    expect(stateFromPaths(['user.prompt', 'unknown'])).toEqual(DEFAULT_CURSOR_FILTER_STATE);
  });

  it('round-trips: stateFromPaths(pathsFromState(s)) === s', () => {
    const samples: CursorFilterState[] = [
      DEFAULT_CURSOR_FILTER_STATE,
      { user: false, assistant: true, tool: false },
      { user: false, assistant: false, tool: false },
    ];
    for (const s of samples) {
      expect(stateFromPaths(pathsFromState(s))).toEqual(s);
    }
  });
});

describe('useCursorTranscriptFilters', () => {
  it('reads default (all visible) when no ?hide= param', () => {
    const { result } = renderHook(() => useCursorTranscriptFilters());
    expect(result.current.filterState).toEqual(DEFAULT_CURSOR_FILTER_STATE);
  });

  it('toggleCategory hides a category and writes it to the URL', () => {
    const { result } = renderHook(() => useCursorTranscriptFilters());

    act(() => result.current.toggleCategory('tool'));

    expect(currentParams.get('hide')).toBe('tool');
  });

  it('toggling a hidden category back to visible clears it from the URL', () => {
    currentParams = new URLSearchParams({ hide: 'tool' });
    const { result } = renderHook(() => useCursorTranscriptFilters());
    expect(result.current.filterState.tool).toBe(false);

    act(() => result.current.toggleCategory('tool'));

    expect(currentParams.get('hide')).toBeNull();
  });
});
