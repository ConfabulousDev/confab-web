// CF-418 regression goldens.
//
// Hardcoded session-total costs computed BEFORE the refactor (using the
// pre-refactor calculateMessageCost / calculateCodexAssistantCost). The
// new code path — parse layer normalizes to canonical TokenUsage, adapter
// applies provider-specific adjustments — must reproduce these totals to
// within $0.0001.
//
// Cost numbers are money-related and user-visible. These tests are the
// last line of defense against shape-mismatch bugs in normalization.

import { describe, expect, it } from 'vitest';
import type { AssistantMessage } from '@/types';
import type { CodexAssistantItem } from '@/types/codexRenderItem';
import type { TokenUsage } from '@/utils/tokenStats';
import { claudeAdapter } from '@/providers/claudeAdapter';
import { codexAdapter } from '@/providers/codexAdapter';

// ---------------------------------------------------------------------------
// Claude session golden — two assistant messages.
//
// Pre-refactor calculation (using existing MODEL_PRICING values):
//
//   Message 1: sonnet-4 family
//     usage = { input_tokens: 50_000, output_tokens: 5_000,
//               cache_creation_input_tokens: 20_000,
//               cache_read_input_tokens: 10_000 }
//     pricing = sonnet-4 { input:3, output:15, cacheWrite:3.75, cacheRead:0.30 }
//     cost = (50_000*3 + 5_000*15 + 20_000*3.75 + 10_000*0.30) / 1e6
//          = (150_000 + 75_000 + 75_000 + 3_000) / 1e6 = 0.303
//
//   Message 2: opus-4-7, speed=fast, 2 web search requests
//     usage = { input_tokens: 100_000, output_tokens: 10_000 }
//     pricing = opus-4-7 { input:5, output:25 }
//     base = (100_000*5 + 10_000*25) / 1e6 = 0.75
//     fast 6x = 4.50
//     web_search 2 * $0.01 = 0.02   (NOT multiplied by fast)
//     cost = 4.52
//
//   session_total = 0.303 + 4.52 = 4.823
// ---------------------------------------------------------------------------

const CLAUDE_SESSION_TOTAL_USD = 4.823;

function claudeAssistant(
  model: string,
  tokenUsage: TokenUsage,
  wireExtras: {
    speed?: string;
    service_tier?: string | null;
    server_tool_use?: { web_search_requests?: number };
  } = {},
): AssistantMessage & { tokenUsage: TokenUsage } {
  return {
    type: 'assistant',
    uuid: 'uuid',
    timestamp: '2026-05-13T01:00:00Z',
    parentUuid: null,
    isSidechain: false,
    userType: 'human',
    cwd: '/test',
    sessionId: 's',
    version: '1.0',
    requestId: 'req',
    message: {
      model,
      id: 'msg',
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text: 'x' }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: {
        input_tokens: tokenUsage.input,
        output_tokens: tokenUsage.output,
        cache_creation_input_tokens: tokenUsage.cacheWrite,
        cache_read_input_tokens: tokenUsage.cacheRead,
        ...wireExtras,
      },
    },
    tokenUsage,
  };
}

describe('golden: Claude session total cost', () => {
  it('reproduces pre-refactor cost within $0.0001', () => {
    const m1 = claudeAssistant('claude-sonnet-4-20250514', {
      input: 50_000,
      output: 5_000,
      cacheWrite: 20_000,
      cacheRead: 10_000,
    });
    const m2 = claudeAssistant(
      'claude-opus-4-7-20260301',
      { input: 100_000, output: 10_000, cacheWrite: 0, cacheRead: 0 },
      { speed: 'fast', server_tool_use: { web_search_requests: 2 } },
    );

    const total =
      claudeAdapter.calculateMessageCost(m1.message.model, m1.tokenUsage, m1) +
      claudeAdapter.calculateMessageCost(m2.message.model, m2.tokenUsage, m2);

    expect(Math.abs(total - CLAUDE_SESSION_TOTAL_USD)).toBeLessThan(0.0001);
  });
});

// ---------------------------------------------------------------------------
// Codex session golden — two assistant items.
//
// Pre-refactor calculation (using existing calculateCodexAssistantCost):
//
//   Item 1: gpt-5
//     usage = { input_tokens: 500_000, cached_input_tokens: 100_000,
//               output_tokens: 50_000, reasoning_output_tokens: 20_000 }
//     uncached = max(0, 500_000 - 100_000) = 400_000
//     output total = 50_000 + 20_000 = 70_000
//     pricing = gpt-5 { input:1.25, output:10, cacheRead:0.125 }
//     cost = (400_000*1.25 + 100_000*0.125 + 70_000*10) / 1e6
//          = (500_000 + 12_500 + 700_000) / 1e6 = 1.2125
//
//   Item 2: gpt-5-mini
//     usage = { input_tokens: 200_000, cached_input_tokens: 50_000,
//               output_tokens: 10_000 }
//     uncached = 150_000
//     output = 10_000
//     pricing = gpt-5-mini { input:0.25, output:2.00, cacheRead:0.025 }
//     cost = (150_000*0.25 + 50_000*0.025 + 10_000*2.00) / 1e6
//          = (37_500 + 1_250 + 20_000) / 1e6 = 0.05875
//
//   session_total = 1.2125 + 0.05875 = 1.27125
//
// In the post-refactor canonical TokenUsage, the parse layer has already done
// `uncached = max(0, input_tokens - cached_input_tokens)` and folded reasoning
// into output, so the items carry:
//   Item 1: { input:400_000, output:70_000, cacheWrite:0, cacheRead:100_000 }
//   Item 2: { input:150_000, output:10_000, cacheWrite:0, cacheRead:50_000 }
// ---------------------------------------------------------------------------

const CODEX_SESSION_TOTAL_USD = 1.27125;

function codexAssistant(model: string, usage: TokenUsage): CodexAssistantItem {
  return {
    kind: 'assistant',
    lineId: '0',
    timestamp: '2026-05-13T01:00:00Z',
    text: 'x',
    phase: 'final',
    model,
    usage,
  };
}

describe('golden: Codex session total cost', () => {
  it('reproduces pre-refactor cost within $0.0001', () => {
    const i1 = codexAssistant('gpt-5', {
      input: 400_000,
      output: 70_000,
      cacheWrite: 0,
      cacheRead: 100_000,
    });
    const i2 = codexAssistant('gpt-5-mini', {
      input: 150_000,
      output: 10_000,
      cacheWrite: 0,
      cacheRead: 50_000,
    });

    const total =
      codexAdapter.calculateMessageCost(i1.model, i1.usage!, i1) +
      codexAdapter.calculateMessageCost(i2.model, i2.usage!, i2);

    expect(Math.abs(total - CODEX_SESSION_TOTAL_USD)).toBeLessThan(0.0001);
  });
});
