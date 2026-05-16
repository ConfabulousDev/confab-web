import type { TranscriptLine } from '@/types';
import { isAssistantMessage } from '@/types';
import type { CodexAssistantUsage } from '@/types/codexRenderItem';

// Pricing per million tokens (5-minute cache pricing)
// Source: https://www.anthropic.com/pricing
interface ModelPricing {
  input: number;
  output: number;
  cacheWrite: number;  // 5-minute cache: 1.25x input
  cacheRead: number;   // 0.1x input
}

const MODEL_PRICING: Record<string, ModelPricing> = {
  // Opus 4.7
  'opus-4-7': { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.50 },
  // Opus 4.6
  'opus-4-6': { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.50 },
  // Opus 4.5
  'opus-4-5': { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.50 },
  // Opus 4.1 and 4
  'opus-4-1': { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.50 },
  'opus-4': { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.50 },
  // Sonnet 4.6, 4.5, 4, 3.7
  'sonnet-4-6': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
  'sonnet-4-5': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
  'sonnet-4': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
  'sonnet-3-7': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
  // Haiku 4.5
  'haiku-4-5': { input: 1, output: 5, cacheWrite: 1.25, cacheRead: 0.10 },
  // Haiku 3.5
  'haiku-3-5': { input: 0.80, output: 4, cacheWrite: 1.00, cacheRead: 0.08 },
  // Opus 3 (deprecated)
  'opus-3': { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.50 },
  // Haiku 3
  'haiku-3': { input: 0.25, output: 1.25, cacheWrite: 0.30, cacheRead: 0.03 },

  // OpenAI / Codex models — CacheWrite=0 (caching is free/automatic).
  // Source: https://developers.openai.com/api/docs/pricing
  'gpt-5': { input: 1.25, output: 10.00, cacheWrite: 0, cacheRead: 0.125 },
  'gpt-5-mini': { input: 0.25, output: 2.00, cacheWrite: 0, cacheRead: 0.025 },
  'gpt-5-nano': { input: 0.05, output: 0.40, cacheWrite: 0, cacheRead: 0.005 },
  'gpt-5.4-mini': { input: 0.75, output: 4.50, cacheWrite: 0, cacheRead: 0.075 },
  'gpt-5.5': { input: 5.00, output: 30.00, cacheWrite: 0, cacheRead: 0.50 },
  'gpt-4o': { input: 2.50, output: 10.00, cacheWrite: 0, cacheRead: 1.25 },
  'gpt-4o-mini': { input: 0.15, output: 0.60, cacheWrite: 0, cacheRead: 0.075 },
  'gpt-4-turbo': { input: 10.00, output: 30.00, cacheWrite: 0, cacheRead: 0 },
  'o1': { input: 15.00, output: 60.00, cacheWrite: 0, cacheRead: 7.50 },
  'o1-mini': { input: 1.10, output: 4.40, cacheWrite: 0, cacheRead: 0.55 },
  'o3': { input: 2.00, output: 8.00, cacheWrite: 0, cacheRead: 0.50 },
  'o3-mini': { input: 1.10, output: 4.40, cacheWrite: 0, cacheRead: 0.55 },
  'o4-mini': { input: 1.10, output: 4.40, cacheWrite: 0, cacheRead: 0.275 },
};

// Zero pricing for unknown models — cost will be underreported rather than silently wrong.
const ZERO_PRICING: ModelPricing = { input: 0, output: 0, cacheWrite: 0, cacheRead: 0 };

// Server tool pricing (per request, not per token)
// Source: https://docs.anthropic.com/en/about-claude/pricing
export const WEB_SEARCH_COST_PER_REQUEST = 0.01; // $10 per 1,000 searches

// Fast mode multiplier applied to all token costs
const FAST_MODE_MULTIPLIER = 6;

// Strip the YYYY-MM-DD suffix OpenAI appends to pinned snapshots,
// e.g. "gpt-5-2026-05-01" -> "gpt-5". Pure names like "gpt-5.5" pass through.
const OPENAI_DATE_SUFFIX = /-\d{4}-\d{2}-\d{2}$/;

function isOpenAIModel(name: string): boolean {
  return /^gpt-/.test(name) || /^o[134]/.test(name);
}

/**
 * Extract pricing-table key from a full model name.
 *  - Claude:  "claude-opus-4-5-20251101" -> "opus-4-5"
 *  - OpenAI:  "gpt-5-2026-05-01"          -> "gpt-5"
 *  - OpenAI:  "gpt-5.5"                   -> "gpt-5.5" (pass-through)
 */
function getModelFamily(modelName: string): string {
  if (isOpenAIModel(modelName)) {
    return modelName.replace(OPENAI_DATE_SUFFIX, '');
  }

  // Remove "claude-" prefix if present
  const name = modelName.replace(/^claude-/, '');

  // Match patterns like "opus-4-5", "sonnet-4", "haiku-3-5"
  // Minor version is a single digit; date suffixes (e.g., 20250514) are excluded via lookahead
  const match = name.match(/^(opus|sonnet|haiku)-(\d(?:-\d)?)(?!\d)/);
  if (match) {
    return `${match[1]}-${match[2]}`;
  }

  return name;
}

/**
 * Get pricing for a model
 */
function getPricing(modelName: string): ModelPricing {
  const family = getModelFamily(modelName);
  const pricing = MODEL_PRICING[family];
  if (!pricing) {
    console.warn(`Unknown model for pricing: ${modelName} (family: ${family})`);
    return ZERO_PRICING;
  }
  return pricing;
}

/**
 * Calculate cost for a single message.
 * Returns cost in dollars (0 for non-assistant messages).
 */
export function calculateMessageCost(message: TranscriptLine): number {
  if (!isAssistantMessage(message)) return 0;

  const usage = message.message.usage;
  const pricing = getPricing(message.message.model);

  const inputTokens = usage.input_tokens;
  const outputTokens = usage.output_tokens;
  const cacheWriteTokens = usage.cache_creation_input_tokens ?? 0;
  const cacheReadTokens = usage.cache_read_input_tokens ?? 0;

  // Cost per token (pricing is per million tokens)
  const inputCost = (inputTokens * pricing.input) / 1_000_000;
  const outputCost = (outputTokens * pricing.output) / 1_000_000;
  const cacheWriteCost = (cacheWriteTokens * pricing.cacheWrite) / 1_000_000;
  const cacheReadCost = (cacheReadTokens * pricing.cacheRead) / 1_000_000;

  let cost = inputCost + outputCost + cacheWriteCost + cacheReadCost;

  // Fast mode: 6x all token costs
  if (usage.speed === 'fast') {
    cost *= FAST_MODE_MULTIPLIER;
  }

  // Server tool costs (per-request pricing, not affected by fast mode)
  cost += (usage.server_tool_use?.web_search_requests ?? 0) * WEB_SEARCH_COST_PER_REQUEST;

  return cost;
}

/**
 * CF-362: per-API-call cost for a Codex assistant message.
 *
 * Mirrors `applyCodexTokens` in backend/internal/analytics/codex_adapter.go:
 *   uncached = max(0, input_tokens - cached_input_tokens)
 *   output   = output_tokens + reasoning_output_tokens
 *   cost     = uncached * input_rate
 *            + cached   * cache_read_rate
 *            + output   * output_rate
 *
 * OpenAI cache writes are free, so `cacheWrite` is unused. Unknown models
 * fall through `getPricing`'s zero-pricing path, returning $0 with a warning.
 */
export function calculateCodexAssistantCost(
  model: string,
  usage: CodexAssistantUsage,
): number {
  const pricing = getPricing(model);
  const cached = usage.cached_input_tokens ?? 0;
  const uncached = Math.max(0, usage.input_tokens - cached);
  const output = usage.output_tokens + (usage.reasoning_output_tokens ?? 0);
  return (
    (uncached * pricing.input + cached * pricing.cacheRead + output * pricing.output) /
    1_000_000
  );
}

/**
 * CF-362: verbose multi-line tooltip for a Codex cost badge.
 *
 * Mirrors `buildCostTooltip` in `TimelineMessage.tsx` for visual parity, but
 * omits Claude-only lines (speed, service_tier, server_tool_use) and adds
 * Codex-specific sub-lines for cached input and reasoning output. Same first
 * three lines (`$cost`, blank, `Input tokens (in): N`) so the formatting
 * feels consistent across providers when toggling between sessions.
 */
export function buildCodexCostTooltip(
  usage: CodexAssistantUsage,
  cost: number,
): string {
  const lines: string[] = [formatCost(cost), ''];
  lines.push(`Input tokens (in): ${usage.input_tokens.toLocaleString()}`);
  if (usage.cached_input_tokens && usage.cached_input_tokens > 0) {
    lines.push(`  Cached (hit): ${usage.cached_input_tokens.toLocaleString()}`);
  }
  lines.push(`Output tokens (out): ${usage.output_tokens.toLocaleString()}`);
  if (usage.reasoning_output_tokens && usage.reasoning_output_tokens > 0) {
    lines.push(`  Reasoning: ${usage.reasoning_output_tokens.toLocaleString()}`);
  }
  return lines.join('\n');
}

/**
 * Format cost for display
 * Examples: 0.50 -> "$0.50", 4.23 -> "$4.23", 0.05 -> "$0.05"
 */
export function formatCost(cost: number): string {
  if (cost === 0) {
    return '$0.00';
  }
  if (cost < 0.01) {
    return '<$0.01';
  }
  return `$${cost.toFixed(2)}`;
}

/**
 * Format token count for display using natural units (k, M, B)
 * Examples: 500 -> "500", 1500 -> "1.5k", 1500000 -> "1.5M"
 */
export function formatTokenCount(count: number): string {
  if (count >= 1_000_000_000) {
    return `${(count / 1_000_000_000).toFixed(1)}B`;
  }
  if (count >= 1_000_000) {
    return `${(count / 1_000_000).toFixed(1)}M`;
  }
  if (count >= 1_000) {
    return `${(count / 1_000).toFixed(1)}k`;
  }
  return count.toString();
}
