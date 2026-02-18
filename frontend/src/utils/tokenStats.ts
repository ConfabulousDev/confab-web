import type { TranscriptLine } from '@/types';

interface TokenStats {
  input: number;
  output: number;
  cacheCreated: number;
  cacheRead: number;
}

// Pricing per million tokens (5-minute cache pricing)
// Source: https://www.anthropic.com/pricing
interface ModelPricing {
  input: number;
  output: number;
  cacheWrite: number;  // 5-minute cache: 1.25x input
  cacheRead: number;   // 0.1x input
}

const MODEL_PRICING: Record<string, ModelPricing> = {
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
};

// Zero pricing for unknown models — cost will be underreported rather than silently wrong.
const ZERO_PRICING: ModelPricing = { input: 0, output: 0, cacheWrite: 0, cacheRead: 0 };

/**
 * Extract model family from full model name
 * e.g., "claude-opus-4-5-20251101" -> "opus-4-5"
 */
function getModelFamily(modelName: string): string {
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
 * Calculate token stats by summing usage from all assistant messages
 */
export function calculateTokenStats(messages: TranscriptLine[]): TokenStats {
  const stats: TokenStats = {
    input: 0,
    output: 0,
    cacheCreated: 0,
    cacheRead: 0,
  };

  for (const message of messages) {
    if (message.type === 'assistant') {
      const usage = message.message.usage;
      stats.input += usage.input_tokens;
      stats.output += usage.output_tokens;
      stats.cacheCreated += usage.cache_creation_input_tokens ?? 0;
      stats.cacheRead += usage.cache_read_input_tokens ?? 0;
    }
  }

  return stats;
}

/**
 * Calculate estimated cost from messages
 * Returns cost in dollars
 */
export function calculateEstimatedCost(messages: TranscriptLine[]): number {
  let totalCost = 0;

  for (const message of messages) {
    if (message.type === 'assistant') {
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

      totalCost += inputCost + outputCost + cacheWriteCost + cacheReadCost;
    }
  }

  return totalCost;
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
