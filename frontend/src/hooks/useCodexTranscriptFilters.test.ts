import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import {
  DEFAULT_CODEX_FILTER_STATE,
  type CodexFilterState,
} from '@/components/session/codexCategories';
import {
  DEFAULT_HIDDEN,
  pathsFromState,
  stateFromPaths,
  useCodexTranscriptFilters,
} from './useCodexTranscriptFilters';

// Mock react-router-dom's useSearchParams the same way useSessionFilters tests do.
const mockSetSearchParams = vi.fn();
let currentParams = new URLSearchParams();

vi.mock('react-router-dom', () => ({
  useSearchParams: () => [currentParams, mockSetSearchParams],
}));

function setParams(params: Record<string, string>): void {
  currentParams = new URLSearchParams(params);
}

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

// ---------------------------------------------------------------------------
// DEFAULT_HIDDEN
// ---------------------------------------------------------------------------

describe('DEFAULT_HIDDEN', () => {
  it('contains exactly the tokens for default-hidden categories', () => {
    expect(DEFAULT_HIDDEN).toEqual(['reasoning_hidden']);
  });
});

// ---------------------------------------------------------------------------
// pathsFromState / stateFromPaths (pure helpers)
// ---------------------------------------------------------------------------

describe('pathsFromState', () => {
  it('returns ["reasoning_hidden"] for the default state', () => {
    expect(pathsFromState(DEFAULT_CODEX_FILTER_STATE)).toEqual(['reasoning_hidden']);
  });

  it('returns an empty list when everything is visible', () => {
    const allVisible: CodexFilterState = {
      ...DEFAULT_CODEX_FILTER_STATE,
      reasoning_hidden: true,
    };
    expect(pathsFromState(allVisible)).toEqual([]);
  });

  it('emits sub-paths for hidden assistant subs', () => {
    const state: CodexFilterState = {
      ...DEFAULT_CODEX_FILTER_STATE,
      reasoning_hidden: true,
      assistant: { commentary: false, final: true },
    };
    expect(pathsFromState(state)).toContain('assistant.commentary');
    expect(pathsFromState(state)).not.toContain('assistant.final');
  });

  it('emits sub-paths for hidden tool_call subs', () => {
    const state: CodexFilterState = {
      ...DEFAULT_CODEX_FILTER_STATE,
      reasoning_hidden: true,
      tool_call: {
        exec_command: false,
        apply_patch: true,
        web_search: true,
        generic: false,
      },
    };
    const paths = pathsFromState(state);
    expect(paths).toContain('tool_call.exec_command');
    expect(paths).toContain('tool_call.generic');
    expect(paths).not.toContain('tool_call.apply_patch');
    expect(paths).not.toContain('tool_call.web_search');
  });

  it('emits flat tokens for hidden top-level categories', () => {
    const state: CodexFilterState = {
      ...DEFAULT_CODEX_FILTER_STATE,
      user: false,
      turn_separator: false,
      compacted: false,
      unknown: false,
    };
    const paths = pathsFromState(state);
    expect(paths).toContain('user');
    expect(paths).toContain('turn_separator');
    expect(paths).toContain('compacted');
    expect(paths).toContain('unknown');
    expect(paths).toContain('reasoning_hidden');
  });
});

describe('stateFromPaths', () => {
  it('returns DEFAULT_CODEX_FILTER_STATE when given DEFAULT_HIDDEN', () => {
    expect(stateFromPaths(DEFAULT_HIDDEN)).toEqual(DEFAULT_CODEX_FILTER_STATE);
  });

  it('returns the all-visible state when paths is empty', () => {
    const allVisible: CodexFilterState = {
      ...DEFAULT_CODEX_FILTER_STATE,
      reasoning_hidden: true,
    };
    expect(stateFromPaths([])).toEqual(allVisible);
  });

  it('treats unknown tokens as no-ops (forward-compat for cross-provider URLs)', () => {
    // user.prompt is a Claude-side token; on Codex it should not change any
    // Codex slot. This guarantees the same `?hide=` slot is safe to share.
    const result = stateFromPaths(['user.prompt', 'reasoning_hidden']);
    expect(result).toEqual(DEFAULT_CODEX_FILTER_STATE);
  });

  it('round-trips: stateFromPaths(pathsFromState(s)) === s', () => {
    const samples: CodexFilterState[] = [
      DEFAULT_CODEX_FILTER_STATE,
      {
        ...DEFAULT_CODEX_FILTER_STATE,
        reasoning_hidden: true,
      },
      {
        user: false,
        assistant: { commentary: false, final: true },
        tool_call: {
          exec_command: false,
          apply_patch: true,
          web_search: false,
          generic: true,
        },
        reasoning_hidden: false,
        compacted: false,
        turn_separator: false,
        turn_aborted: false,
        unknown: true,
      },
      {
        user: true,
        assistant: { commentary: true, final: true },
        tool_call: {
          exec_command: true,
          apply_patch: true,
          web_search: true,
          generic: true,
        },
        reasoning_hidden: true,
        compacted: true,
        turn_separator: true,
        turn_aborted: true,
        unknown: true,
      },
    ];
    for (const s of samples) {
      expect(stateFromPaths(pathsFromState(s))).toEqual(s);
    }
  });
});

// ---------------------------------------------------------------------------
// useCodexTranscriptFilters (hook behavior)
// ---------------------------------------------------------------------------

describe('useCodexTranscriptFilters', () => {
  it('returns DEFAULT_CODEX_FILTER_STATE when the URL has no ?hide= param', () => {
    const { result } = renderHook(() => useCodexTranscriptFilters());
    expect(result.current.filterState).toEqual(DEFAULT_CODEX_FILTER_STATE);
  });

  it('parses ?hide= and reflects the state', () => {
    setParams({ hide: 'reasoning_hidden,tool_call.exec_command' });
    const { result } = renderHook(() => useCodexTranscriptFilters());
    expect(result.current.filterState.reasoning_hidden).toBe(false);
    expect(result.current.filterState.tool_call.exec_command).toBe(false);
    expect(result.current.filterState.tool_call.apply_patch).toBe(true);
  });

  describe('toggleCategory', () => {
    it('flips a flat category (user)', () => {
      const { result } = renderHook(() => useCodexTranscriptFilters());
      act(() => result.current.toggleCategory('user'));
      expect(currentParams.get('hide')).toContain('user');
    });

    it('flips reasoning_hidden from default-hidden to visible (URL slot empties)', () => {
      const { result } = renderHook(() => useCodexTranscriptFilters());
      act(() => result.current.toggleCategory('reasoning_hidden'));
      // After toggling, reasoning_hidden is visible — `hide` no longer needs it.
      // Param should either be absent or not contain reasoning_hidden.
      const hide = currentParams.get('hide');
      expect(hide === null || !hide.includes('reasoning_hidden')).toBe(true);
    });

    it('tri-state collapse for assistant parent: if any sub is visible, all go hidden', () => {
      const { result } = renderHook(() => useCodexTranscriptFilters());
      // Default has both subs visible — first toggle should hide both.
      act(() => result.current.toggleCategory('assistant'));
      const hide = currentParams.get('hide') ?? '';
      expect(hide.split(',')).toContain('assistant.commentary');
      expect(hide.split(',')).toContain('assistant.final');
    });

    it('tri-state collapse for assistant parent: if all subs hidden, all go visible', () => {
      setParams({ hide: 'reasoning_hidden,assistant.commentary,assistant.final' });
      const { result } = renderHook(() => useCodexTranscriptFilters());
      act(() => result.current.toggleCategory('assistant'));
      const hide = currentParams.get('hide') ?? '';
      const tokens = hide.split(',').filter(Boolean);
      expect(tokens).not.toContain('assistant.commentary');
      expect(tokens).not.toContain('assistant.final');
    });

    it('tri-state collapse for tool_call parent applies to all four subs', () => {
      const { result } = renderHook(() => useCodexTranscriptFilters());
      act(() => result.current.toggleCategory('tool_call'));
      const tokens = (currentParams.get('hide') ?? '').split(',');
      expect(tokens).toContain('tool_call.exec_command');
      expect(tokens).toContain('tool_call.apply_patch');
      expect(tokens).toContain('tool_call.web_search');
      expect(tokens).toContain('tool_call.generic');
    });
  });

  describe('toggleAssistantSubcategory', () => {
    it('flips one sub without touching the other', () => {
      const { result } = renderHook(() => useCodexTranscriptFilters());
      act(() => result.current.toggleAssistantSubcategory('commentary'));
      const tokens = (currentParams.get('hide') ?? '').split(',');
      expect(tokens).toContain('assistant.commentary');
      expect(tokens).not.toContain('assistant.final');
    });
  });

  describe('toggleToolCallSubcategory', () => {
    it('flips one sub without touching the other three', () => {
      const { result } = renderHook(() => useCodexTranscriptFilters());
      act(() => result.current.toggleToolCallSubcategory('web_search'));
      const tokens = (currentParams.get('hide') ?? '').split(',');
      expect(tokens).toContain('tool_call.web_search');
      expect(tokens).not.toContain('tool_call.exec_command');
      expect(tokens).not.toContain('tool_call.apply_patch');
      expect(tokens).not.toContain('tool_call.generic');
    });
  });

  describe('setFilterState', () => {
    it('writes the full state to ?hide= via pathsFromState', () => {
      const { result } = renderHook(() => useCodexTranscriptFilters());
      const target: CodexFilterState = {
        ...DEFAULT_CODEX_FILTER_STATE,
        reasoning_hidden: true,
        compacted: false,
      };
      act(() => result.current.setFilterState(target));
      const tokens = (currentParams.get('hide') ?? '').split(',').filter(Boolean);
      expect(tokens).toContain('compacted');
      expect(tokens).not.toContain('reasoning_hidden');
    });
  });
});
