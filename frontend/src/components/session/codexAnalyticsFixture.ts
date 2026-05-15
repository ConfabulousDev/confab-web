// Shared Codex analytics fixture for Storybook (CF-364).
//
// Mirrors what ComputeFromCodexRollout produces for a representative Codex
// session, with the provider-specific shape that distinguishes it from Claude:
//   - models_used contains GPT model strings (gpt-5);
//   - tokens.cache_creation = 0 (OpenAI doesn't charge for cache writes);
//   - code_activity.files_read = 0 (Codex has no Read tool);
//   - smart_recap items have empty message_id (Codex messages have no
//     deep-linkable UUID — SmartRecapCard renders them as plain text);
//   - agents_and_skills and redactions are absent so their card-level
//     shouldRender hide-gates fire cleanly.
//
// Imported by SessionViewer.stories.tsx and SessionSummaryPanel.stories.tsx
// so both `CodexSession` stories stay in sync without circular imports
// (this file imports only types).

import type { SessionAnalytics } from '@/schemas/api';

export function buildCodexAnalyticsFixture(): SessionAnalytics {
  return {
    computed_at: new Date(Date.now() - 60000).toISOString(),
    computed_lines: 50,
    tokens: { input: 800, output: 200, cache_creation: 0, cache_read: 200 },
    cost: { estimated_usd: '0.0125' },
    compaction: { auto: 1, manual: 0, avg_time_ms: null },
    cards: {
      tokens: {
        input: 800,
        output: 200,
        cache_creation: 0,
        cache_read: 200,
        estimated_usd: '0.0125',
      },
      session: {
        total_messages: 8,
        user_messages: 2,
        assistant_messages: 4,
        human_prompts: 2,
        tool_results: 2,
        text_responses: 2,
        tool_calls: 2,
        thinking_blocks: 1,
        duration_ms: 11000,
        models_used: ['gpt-5'],
        compaction_auto: 1,
        compaction_manual: 0,
        compaction_avg_time_ms: null,
      },
      conversation: {
        user_turns: 2,
        assistant_turns: 2,
        avg_assistant_turn_ms: 5500,
        avg_user_thinking_ms: 9000,
      },
      code_activity: {
        files_read: 0,
        files_modified: 1,
        lines_added: 3,
        lines_removed: 0,
        search_count: 0,
        language_breakdown: { md: 1 },
      },
      tools: {
        total_calls: 3,
        tool_stats: {
          apply_patch: { success: 1, errors: 0 },
          exec_command: { success: 1, errors: 0 },
          web_search_call: { success: 1, errors: 0 },
        },
        error_count: 0,
      },
      smart_recap: {
        recap: 'Added the Linear MCP entry to the Codex config; verified via web search that the JSONL format matches the documented schema.',
        went_well: [
          { text: 'Found the right config path via exec_command pwd.', message_id: '' },
          { text: 'apply_patch landed cleanly with no rebase needed.', message_id: '' },
        ],
        went_bad: [
          { text: 'Initial web_search returned a generic page; needed a second query.', message_id: '' },
        ],
        human_suggestions: [],
        environment_suggestions: [],
        default_context_suggestions: [],
        computed_at: new Date(Date.now() - 30000).toISOString(),
        model_used: 'claude-sonnet-4-5',
      },
    },
  };
}
