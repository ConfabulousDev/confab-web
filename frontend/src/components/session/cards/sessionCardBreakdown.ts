import type { SessionCardData } from '@/schemas/api';

// Colors for the SessionCard bar chart.
export const BREAKDOWN_COLORS = {
  humanPrompts: '#3b82f6', // blue
  toolResults: '#8b5cf6', // purple
  textResponses: '#22c55e', // green
  toolCalls: '#f59e0b', // amber
  thinkingBlocks: '#ec4899', // pink
};

export interface BreakdownEntry {
  name: string;
  fullName: string;
  value: number;
  color: string;
  [key: string]: string | number; // Index signature for Recharts compatibility
}

/**
 * Build provider-aware breakdown entries for the SessionCard bar chart
 * (CF-437):
 *   - Reasoning bar: "Thinking" / "Thinking blocks" (Claude) vs.
 *                    "Reasoning" / "Reasoning steps"  (Codex).
 *   - Tool results bar: hidden for Codex when tool_calls == tool_results
 *     (every successful Codex tool call has output, so the two bars are
 *     visually redundant). Still shown when they diverge (failed/pending tools).
 *   - Claude always shows both bars regardless of equality.
 *
 * Extracted from SessionCard.tsx to keep the component file focused and
 * to satisfy the `react-refresh/only-export-components` lint rule, which
 * forbids mixing component and non-component exports in the same file.
 * The bar-label contract is verified against this function's return value
 * directly — Recharts' rendered SVG output isn't reliable in jsdom.
 */
export function prepareBreakdownData(data: SessionCardData, provider: string): BreakdownEntry[] {
  const isCodex = provider === 'codex';

  const reasoningName = isCodex ? 'Reasoning' : 'Thinking';
  const reasoningFullName = isCodex ? 'Reasoning steps' : 'Thinking blocks';

  const hideToolResults = isCodex && data.tool_calls === data.tool_results;

  const entries: BreakdownEntry[] = [
    { name: 'Prompts', fullName: 'Human prompts', value: data.human_prompts, color: BREAKDOWN_COLORS.humanPrompts },
    { name: 'Tool res', fullName: 'Tool results', value: hideToolResults ? 0 : data.tool_results, color: BREAKDOWN_COLORS.toolResults },
    { name: 'Txt resp', fullName: 'Text responses', value: data.text_responses, color: BREAKDOWN_COLORS.textResponses },
    { name: 'Tool calls', fullName: 'Tool calls', value: data.tool_calls, color: BREAKDOWN_COLORS.toolCalls },
    { name: reasoningName, fullName: reasoningFullName, value: data.thinking_blocks, color: BREAKDOWN_COLORS.thinkingBlocks },
  ];
  // Filter out zero values and sort by value descending
  return entries.filter((e) => e.value > 0).sort((a, b) => b.value - a.value);
}
