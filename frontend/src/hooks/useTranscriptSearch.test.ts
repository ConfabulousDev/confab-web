import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import type { UserMessage, AssistantMessage } from '@/types';
import { useTranscriptSearch } from './useTranscriptSearch';

// Helper to build minimal test messages
function makeUserMessage(text: string, uuid = 'u1'): UserMessage {
  return {
    type: 'user',
    uuid,
    timestamp: '2025-01-15T10:00:00Z',
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/dev',
    sessionId: 's1',
    version: '1.0.0',
    message: { role: 'user', content: text },
  };
}

function makeAssistantMessage(text: string, uuid = 'a1'): AssistantMessage {
  return {
    type: 'assistant',
    uuid,
    timestamp: '2025-01-15T10:00:05Z',
    parentUuid: 'u1',
    isSidechain: false,
    userType: 'external',
    cwd: '/dev',
    sessionId: 's1',
    version: '1.0.0',
    requestId: 'req-1',
    message: {
      model: 'claude-sonnet-4-20250514',
      id: 'msg-1',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: { input_tokens: 100, output_tokens: 50 },
    },
  };
}

describe('useTranscriptSearch', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('initializes with closed state and empty matches', () => {
    const messages = [makeUserMessage('hello world')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    expect(result.current.isOpen).toBe(false);
    expect(result.current.query).toBe('');
    expect(result.current.matches).toEqual([]);
    expect(result.current.currentMatchIndex).toBe(0);
    expect(result.current.currentMatchFilteredIndex).toBeNull();
  });

  it('opens search', () => {
    const messages = [makeUserMessage('hello')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.open());
    expect(result.current.isOpen).toBe(true);
  });

  it('closes search and resets state', () => {
    const messages = [makeUserMessage('hello')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.open());
    act(() => result.current.setQuery('hello'));
    act(() => { vi.advanceTimersByTime(200); });

    act(() => result.current.close());
    expect(result.current.isOpen).toBe(false);
    expect(result.current.query).toBe('');
    expect(result.current.matches).toEqual([]);
  });

  it('finds matches after debounce', () => {
    const messages = [
      makeUserMessage('hello world', 'u1'),
      makeAssistantMessage('goodbye world', 'a1'),
      makeUserMessage('hello again', 'u2'),
    ];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('hello'));

    // Before debounce — no matches yet
    expect(result.current.matches).toEqual([]);

    act(() => { vi.advanceTimersByTime(200); });

    // After debounce — matches found
    expect(result.current.matches).toEqual([0, 2]);
    expect(result.current.currentMatchIndex).toBe(0);
    expect(result.current.currentMatchFilteredIndex).toBe(0);
  });

  it('is case insensitive', () => {
    const messages = [
      makeUserMessage('Hello World'),
      makeAssistantMessage('HELLO again'),
    ];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('hello'));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.matches).toEqual([0, 1]);
  });

  it('returns empty matches for empty query', () => {
    const messages = [makeUserMessage('hello')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery(''));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.matches).toEqual([]);
  });

  it('returns empty matches for whitespace-only query', () => {
    const messages = [makeUserMessage('hello')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('   '));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.matches).toEqual([]);
  });

  it('returns empty matches when no messages match', () => {
    const messages = [makeUserMessage('hello'), makeAssistantMessage('world')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('zzzzz'));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.matches).toEqual([]);
    expect(result.current.currentMatchFilteredIndex).toBeNull();
  });

  it('navigates to next match with wraparound', () => {
    const messages = [
      makeUserMessage('foo', 'u1'),
      makeAssistantMessage('bar', 'a1'),
      makeUserMessage('foo again', 'u2'),
    ];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('foo'));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.currentMatchIndex).toBe(0);
    expect(result.current.currentMatchFilteredIndex).toBe(0);

    act(() => result.current.goToNextMatch());
    expect(result.current.currentMatchIndex).toBe(1);
    expect(result.current.currentMatchFilteredIndex).toBe(2);

    // Wraparound
    act(() => result.current.goToNextMatch());
    expect(result.current.currentMatchIndex).toBe(0);
    expect(result.current.currentMatchFilteredIndex).toBe(0);
  });

  it('navigates to previous match with wraparound', () => {
    const messages = [
      makeUserMessage('test', 'u1'),
      makeAssistantMessage('other', 'a1'),
      makeUserMessage('test again', 'u2'),
    ];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('test'));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.currentMatchIndex).toBe(0);

    // Previous from first should wrap to last
    act(() => result.current.goToPreviousMatch());
    expect(result.current.currentMatchIndex).toBe(1);
    expect(result.current.currentMatchFilteredIndex).toBe(2);

    act(() => result.current.goToPreviousMatch());
    expect(result.current.currentMatchIndex).toBe(0);
  });

  it('does not crash when navigating with no matches', () => {
    const messages = [makeUserMessage('hello')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('zzz'));
    act(() => { vi.advanceTimersByTime(200); });

    // Should be no-ops
    act(() => result.current.goToNextMatch());
    act(() => result.current.goToPreviousMatch());

    expect(result.current.matches).toEqual([]);
    expect(result.current.currentMatchIndex).toBe(0);
  });

  it('resets to match 0 when messages change (filter change)', () => {
    const allMessages = [
      makeUserMessage('hello', 'u1'),
      makeAssistantMessage('hello there', 'a1'),
      makeUserMessage('hello world', 'u2'),
    ];
    const { result, rerender } = renderHook(
      ({ msgs }) => useTranscriptSearch(msgs),
      { initialProps: { msgs: allMessages } },
    );

    act(() => result.current.open());
    act(() => result.current.setQuery('hello'));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.matches).toEqual([0, 1, 2]);

    // Navigate to match 2
    act(() => result.current.goToNextMatch());
    act(() => result.current.goToNextMatch());
    expect(result.current.currentMatchIndex).toBe(2);

    // Simulate filter change — fewer messages
    const filtered = [allMessages[0]!, allMessages[2]!];
    rerender({ msgs: filtered });
    act(() => { vi.advanceTimersByTime(200); });

    // Should reset to match 0
    expect(result.current.currentMatchIndex).toBe(0);
    expect(result.current.matches).toEqual([0, 1]);
  });

  it('debounces highlightQuery at 300ms separately from match-finding at 150ms', () => {
    const messages = [makeUserMessage('hello world')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('hello'));

    // At 150ms: debouncedQuery fires (matches found), but highlightQuery not yet
    act(() => { vi.advanceTimersByTime(150); });
    expect(result.current.matches).toEqual([0]);
    expect(result.current.highlightQuery).toBe('');

    // At 300ms: highlightQuery fires
    act(() => { vi.advanceTimersByTime(150); });
    expect(result.current.highlightQuery).toBe('hello');
  });

  it('clears highlightQuery on close', () => {
    const messages = [makeUserMessage('hello')];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.open());
    act(() => result.current.setQuery('hello'));
    act(() => { vi.advanceTimersByTime(300); });
    expect(result.current.highlightQuery).toBe('hello');

    act(() => result.current.close());
    expect(result.current.highlightQuery).toBe('');
  });

  it('searches tool_use content', () => {
    const msg: AssistantMessage = {
      type: 'assistant',
      uuid: 'a1',
      timestamp: '2025-01-15T10:00:00Z',
      parentUuid: 'u1',
      isSidechain: false,
      userType: 'external',
      cwd: '/dev',
      sessionId: 's1',
      version: '1.0.0',
      requestId: 'req-1',
      message: {
        model: 'claude-sonnet-4-20250514',
        id: 'msg-1',
        type: 'message',
        role: 'assistant',
        content: [
          {
            type: 'tool_use',
            id: 'tool-1',
            name: 'Read',
            input: { file_path: '/Users/dev/special_file.ts' },
          },
        ],
        stop_reason: 'end_turn',
        stop_sequence: null,
        usage: { input_tokens: 100, output_tokens: 50 },
      },
    };
    const messages = [msg];
    const { result } = renderHook(() => useTranscriptSearch(messages));

    act(() => result.current.setQuery('special_file'));
    act(() => { vi.advanceTimersByTime(200); });

    expect(result.current.matches).toEqual([0]);
  });
});
