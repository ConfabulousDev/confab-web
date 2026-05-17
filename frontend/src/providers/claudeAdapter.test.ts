// CF-417 spec: claudeAdapter satisfies the ProviderAdapter contract and
// delegates to the existing transcriptService / messageCategories APIs.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  fetchParsedTranscript,
  fetchNewTranscriptMessages,
} from '@/services/transcriptService';
import {
  DEFAULT_FILTER_STATE,
  messageMatchesFilter,
  countHierarchicalCategories,
} from '@/components/session/messageCategories';
import { computeSessionMeta } from '@/utils/sessionMeta';
import type { TranscriptLine, UserMessage, AssistantMessage } from '@/types';
import { claudeAdapter } from './claudeAdapter';

vi.mock('@/services/transcriptService', () => ({
  fetchParsedTranscript: vi.fn(),
  fetchNewTranscriptMessages: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

function userMessage(uuid: string, timestamp: string, text = 'hi'): UserMessage {
  return {
    type: 'user',
    uuid,
    timestamp,
    parentUuid: null,
    isSidechain: false,
    userType: 'human',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0',
    message: { role: 'user', content: text },
  };
}

function assistantMessage(
  uuid: string,
  timestamp: string,
  model = 'claude-sonnet-4-20250514',
): AssistantMessage {
  return {
    type: 'assistant',
    uuid,
    timestamp,
    parentUuid: null,
    isSidechain: false,
    userType: 'human',
    cwd: '/test',
    sessionId: 'test-session',
    version: '1.0',
    requestId: 'req-123',
    message: {
      model,
      id: 'msg-123',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'hello' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: 10,
        output_tokens: 5,
        cache_creation_input_tokens: 0,
        cache_read_input_tokens: 0,
      },
    },
  };
}

describe('claudeAdapter', () => {
  it('has id="claude-code" and supportsTILs=true', () => {
    expect(claudeAdapter.id).toBe('claude-code');
    expect(claudeAdapter.supportsTILs).toBe(true);
  });

  it('fetchInitial delegates to fetchParsedTranscript and reshapes the result', async () => {
    const messages: TranscriptLine[] = [userMessage('u1', '2026-05-13T01:00:00Z')];
    vi.mocked(fetchParsedTranscript).mockResolvedValue({
      sessionId: 's',
      messages,
      agents: [],
      validationErrors: [],
      totalLines: 1,
      metadata: {
        version: '1.0',
        messageCount: 1,
        agentCount: 0,
        parseErrorCount: 0,
      },
    });

    const result = await claudeAdapter.fetchInitial('s', 'transcript.jsonl', true);

    expect(fetchParsedTranscript).toHaveBeenCalledWith('s', 'transcript.jsonl', true);
    expect(result.items).toBe(messages);
    expect(result.raw).toBe(messages);
    expect(result.totalLines).toBe(1);
  });

  it('fetchIncremental delegates to fetchNewTranscriptMessages', async () => {
    const newMessages: TranscriptLine[] = [userMessage('u2', '2026-05-13T01:01:00Z')];
    vi.mocked(fetchNewTranscriptMessages).mockResolvedValue({
      newMessages,
      newTotalLineCount: 5,
    });

    const result = await claudeAdapter.fetchIncremental('s', 'transcript.jsonl', 3);

    expect(fetchNewTranscriptMessages).toHaveBeenCalledWith('s', 'transcript.jsonl', 3);
    expect(result.newItems).toBe(newMessages);
    expect(result.newRaw).toBe(newMessages);
    expect(result.newTotalLineCount).toBe(5);
  });

  it('normalize is the identity function', () => {
    const messages: TranscriptLine[] = [userMessage('u1', '2026-05-13T01:00:00Z')];
    expect(claudeAdapter.normalize(messages)).toBe(messages);
  });

  it('extractModel returns first assistant message model', () => {
    const messages: TranscriptLine[] = [
      userMessage('u1', '2026-05-13T01:00:00Z'),
      assistantMessage('a1', '2026-05-13T01:00:01Z', 'claude-opus-4-6'),
      assistantMessage('a2', '2026-05-13T01:00:02Z', 'claude-sonnet-4-6'),
    ];
    expect(claudeAdapter.extractModel(messages, messages)).toBe('claude-opus-4-6');
  });

  it('extractModel returns undefined when no assistant message present', () => {
    const messages: TranscriptLine[] = [userMessage('u1', '2026-05-13T01:00:00Z')];
    expect(claudeAdapter.extractModel(messages, messages)).toBeUndefined();
  });

  it('computeMeta delegates to computeSessionMeta over the items', () => {
    const messages: TranscriptLine[] = [
      userMessage('u1', '2026-05-13T01:00:00Z'),
      assistantMessage('a1', '2026-05-13T01:05:00Z'),
    ];
    const meta = claudeAdapter.computeMeta(messages, messages, {});
    const expected = computeSessionMeta(messages, {});
    expect(meta.durationMs).toBe(expected.durationMs);
    expect(meta.sessionDate?.toISOString()).toBe(expected.sessionDate?.toISOString());
  });

  it('countCategories delegates to countHierarchicalCategories', () => {
    const messages: TranscriptLine[] = [
      userMessage('u1', '2026-05-13T01:00:00Z'),
      assistantMessage('a1', '2026-05-13T01:05:00Z'),
    ];
    expect(claudeAdapter.countCategories(messages)).toEqual(
      countHierarchicalCategories(messages),
    );
  });

  it('itemMatchesFilter delegates to messageMatchesFilter', () => {
    const msg = userMessage('u1', '2026-05-13T01:00:00Z');
    expect(claudeAdapter.itemMatchesFilter(msg, DEFAULT_FILTER_STATE)).toBe(
      messageMatchesFilter(msg, DEFAULT_FILTER_STATE),
    );
  });

  it('exposes FilterDropdown and TranscriptPane as renderable components', () => {
    expect(typeof claudeAdapter.FilterDropdown).toBe('function');
    expect(typeof claudeAdapter.TranscriptPane).toBe('function');
  });

  it('exposes useFilters and useDeepLinkFilterReset as functions', () => {
    expect(typeof claudeAdapter.useFilters).toBe('function');
    expect(typeof claudeAdapter.useDeepLinkFilterReset).toBe('function');
  });
});
