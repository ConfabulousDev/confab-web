import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import {
  DEFAULT_OPENCODE_FILTER_STATE,
  type OpenCodeFilterState,
} from '@/components/session/opencodeCategories';
import {
  DEFAULT_HIDDEN,
  pathsFromState,
  stateFromPaths,
  useOpenCodeTranscriptFilters,
} from './useOpenCodeTranscriptFilters';

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
    expect(pathsFromState({ user: true, assistant: false, tool: true, unknown: false })).toEqual([
      'assistant',
      'unknown',
    ]);
  });

  it('treats foreign tokens as no-ops (cross-provider URLs)', () => {
    // 'user.prompt' / 'assistant.commentary' belong to other providers; OpenCode
    // categories are flat, so they leave every OpenCode category visible.
    expect(stateFromPaths(['user.prompt', 'assistant.commentary'])).toEqual(
      DEFAULT_OPENCODE_FILTER_STATE,
    );
  });

  it('round-trips: stateFromPaths(pathsFromState(s)) === s', () => {
    const samples: OpenCodeFilterState[] = [
      DEFAULT_OPENCODE_FILTER_STATE,
      { user: false, assistant: true, tool: false, unknown: true },
      { user: false, assistant: false, tool: false, unknown: false },
    ];
    for (const s of samples) {
      expect(stateFromPaths(pathsFromState(s))).toEqual(s);
    }
  });
});

describe('useOpenCodeTranscriptFilters', () => {
  it('reads default (all visible) when no ?hide= param', () => {
    const { result } = renderHook(() => useOpenCodeTranscriptFilters());
    expect(result.current.filterState).toEqual(DEFAULT_OPENCODE_FILTER_STATE);
  });

  it('toggleCategory hides a category and writes it to the URL', () => {
    const { result } = renderHook(() => useOpenCodeTranscriptFilters());

    act(() => result.current.toggleCategory('tool'));

    expect(currentParams.get('hide')).toBe('tool');
  });

  it('toggling a hidden category back to visible clears it from the URL', () => {
    currentParams = new URLSearchParams({ hide: 'tool' });
    const { result } = renderHook(() => useOpenCodeTranscriptFilters());
    expect(result.current.filterState.tool).toBe(false);

    act(() => result.current.toggleCategory('tool'));

    // Back to the all-visible default → no hide param.
    expect(currentParams.get('hide')).toBeNull();
  });
});
