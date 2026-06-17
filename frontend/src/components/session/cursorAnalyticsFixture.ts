// Shared Cursor analytics fixture for Storybook + schema regression (cd3z).
//
// Mirrors what ComputeFromCursorRollout produces for a representative Cursor
// session and what the analytics HTTP endpoint then serves on the wire
// (`TestGetSessionAnalytics_Cursor_HTTP_Integration`, gevp). The Cursor shape
// differs from Claude/Codex in ways that have repeatedly tripped the Summary
// tab, so the fixture pins them deliberately:
//   - Cursor synced JSONL carries NO usage data, so tokens are all zero and the
//     flat cost is "0" (shopspring/decimal marshals with quotes -> string,
//     which the Zod string schema accepts);
//   - the backend always writes a tokens_v2 card to storage but OMITS it from
//     the Cards map when by_provider is empty, so `cards.tokens_v2` is absent
//     here (matching the live wire) and the flat Tokens card renders instead;
//   - Cursor lines have no timestamps, so session.duration_ms and the
//     conversation turn-timing fields are null;
//   - Cursor's tool names are its own (Read / Grep / StrReplace / Write), not
//     Claude's Edit/MultiEdit;
//   - smart_recap is absent (degrades to the missing-state placeholder); the
//     hard "Failed to load analytics" error must NOT appear for this payload.
//
// Imported by SessionSummaryPanel.stories.tsx and the schema regression test so
// the visual story and the parse assertion stay in sync (this file imports only
// types, so there is no circular import).

import type { SessionAnalytics } from '@/schemas/api';

export function buildCursorAnalyticsFixture(): SessionAnalytics {
  return {
    computed_at: new Date(Date.now() - 60000).toISOString(),
    computed_lines: 40,
    // Cursor JSONL has no usage data -> zero tokens, "0" cost (string).
    tokens: { input: 0, output: 0, cache_creation: 0, cache_read: 0 },
    cost: { estimated_usd: '0' },
    compaction: { auto: 0, manual: 0, avg_time_ms: null },
    cards: {
      // Flat Tokens card (no tokens_v2 card present -> this one renders).
      tokens: { input: 0, output: 0, cache_creation: 0, cache_read: 0, estimated_usd: '0' },
      session: {
        total_messages: 4,
        user_messages: 2,
        assistant_messages: 2,
        human_prompts: 2,
        tool_results: 0,
        text_responses: 2,
        tool_calls: 4,
        thinking_blocks: 0,
        // No timestamps in Cursor JSONL -> duration unknowable.
        duration_ms: null,
        models_used: [],
        compaction_auto: 0,
        compaction_manual: 0,
        compaction_avg_time_ms: null,
      },
      conversation: {
        user_turns: 2,
        assistant_turns: 2,
        // No timestamps -> turn timings null.
        avg_assistant_turn_ms: null,
        avg_user_thinking_ms: null,
      },
      code_activity: {
        files_read: 1,
        files_modified: 2,
        lines_added: 0,
        lines_removed: 0,
        search_count: 1,
        language_breakdown: { go: 2 },
      },
      tools: {
        total_calls: 4,
        tool_stats: {
          Read: { success: 1, errors: 0 },
          Grep: { success: 1, errors: 0 },
          StrReplace: { success: 1, errors: 0 },
          Write: { success: 1, errors: 0 },
        },
        error_count: 0,
      },
    },
  };
}
