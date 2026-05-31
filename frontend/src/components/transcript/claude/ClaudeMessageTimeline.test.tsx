// CF-369: cost-mode scroll-nav offset contract on the Claude timeline.
//
// Mirrors the equivalent test in CodexMessageTimeline.test.tsx so the
// shared SCROLL_NAV_COST_MODE_RIGHT constant (timelineUtils.ts) is locked
// from both ends — a future refactor that touches the constant or its
// import on either provider will fail loudly here.

import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import type { AssistantMessage, TranscriptLine } from '@/types';
import ClaudeMessageTimeline from './ClaudeMessageTimeline';

function assistantWithUsage(overrides: Partial<AssistantMessage> = {}): AssistantMessage {
  return {
    type: 'assistant',
    uuid: 'assistant-uuid-1',
    timestamp: '2026-05-13T18:00:05Z',
    parentUuid: null,
    isSidechain: false,
    userType: 'external',
    cwd: '/test',
    sessionId: 'session-1',
    version: '1.0.0',
    requestId: 'req-1',
    message: {
      model: 'claude-sonnet-4-20250514',
      id: 'msg-1',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'hello' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: 1_000_000,
        output_tokens: 100_000,
        cache_creation_input_tokens: 0,
        cache_read_input_tokens: 0,
      },
    },
    ...overrides,
  };
}

describe('ClaudeMessageTimeline cost-mode scroll-nav offset (CF-369)', () => {
  it('sets inline right: 56px on ScrollNavButtons when isCostMode is on', () => {
    const msg = assistantWithUsage();
    const messages: TranscriptLine[] = [msg];
    render(
      <ClaudeMessageTimeline
        messages={messages}
        allMessages={messages}
        sessionId="test-session"
        isCostMode
      />,
    );
    const nav = document.querySelector<HTMLElement>('[class*="navButtons"]');
    expect(nav).not.toBeNull();
    expect(nav?.style.right).toBe('56px');
  });

  it('does not set inline right on ScrollNavButtons when isCostMode is off', () => {
    const msg = assistantWithUsage();
    const messages: TranscriptLine[] = [msg];
    render(
      <ClaudeMessageTimeline
        messages={messages}
        allMessages={messages}
        sessionId="test-session"
        isCostMode={false}
      />,
    );
    const nav = document.querySelector<HTMLElement>('[class*="navButtons"]');
    expect(nav).not.toBeNull();
    expect(nav?.style.right).toBe('');
  });
});
