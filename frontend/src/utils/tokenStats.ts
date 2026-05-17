// CF-418: canonical TokenUsage + provider-keyed pricing.
//
// `TokenUsage` is the provider-agnostic shape every cost path operates on.
// Both transcript services normalize their wire shape into this struct at
// parse time, then components and the cost arithmetic all read one shape.
//
// Provider-specific adjustments (Claude fast multiplier, web-search dollars,
// Codex reasoning-token display) live on the `ProviderAdapter` — not here.
// `calculateCost` is pure base arithmetic.

import type { ProviderId } from './providers';
import { PROVIDER_VALUES } from './providers';

/**
 * Canonical per-message token usage. Cost-billable, provider-agnostic.
 *
 *   - `input`: uncached input tokens (Codex's `max(0, input - cached)`,
 *     Anthropic's `input_tokens` already-uncached).
 *   - `output`: total output tokens, including reasoning where applicable
 *     (Codex folds `reasoning_output_tokens` here; reasoning bills at the
 *     output rate).
 *   - `cacheWrite`: cache-creation tokens. Anthropic charges 1.25x input;
 *     OpenAI charges 0 (set to 0 by the Codex normalizer).
 *   - `cacheRead`: cache-hit tokens (Codex's `cached_input_tokens`,
 *     Anthropic's `cache_read_input_tokens`).
 */
export interface TokenUsage {
  input: number;
  output: number;
  cacheWrite: number;
  cacheRead: number;
}

interface ModelPricing {
  input: number;
  output: number;
  cacheWrite: number;
  cacheRead: number;
}

// Provider-keyed pricing tables. Adding a third provider is one outer key
// plus N inner rows — no code branches.
// Sources:
//   - https://www.anthropic.com/pricing
//   - https://developers.openai.com/api/docs/pricing
const MODEL_PRICING: Record<ProviderId, Record<string, ModelPricing>> = {
  'claude-code': {
    'opus-4-7': { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.50 },
    'opus-4-6': { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.50 },
    'opus-4-5': { input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.50 },
    'opus-4-1': { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.50 },
    'opus-4': { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.50 },
    'sonnet-4-6': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
    'sonnet-4-5': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
    'sonnet-4': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
    'sonnet-3-7': { input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.30 },
    'haiku-4-5': { input: 1, output: 5, cacheWrite: 1.25, cacheRead: 0.10 },
    'haiku-3-5': { input: 0.80, output: 4, cacheWrite: 1.00, cacheRead: 0.08 },
    'opus-3': { input: 15, output: 75, cacheWrite: 18.75, cacheRead: 1.50 },
    'haiku-3': { input: 0.25, output: 1.25, cacheWrite: 0.30, cacheRead: 0.03 },
  },
  codex: {
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
  },
};

const ZERO_PRICING: ModelPricing = { input: 0, output: 0, cacheWrite: 0, cacheRead: 0 };

// Server tool pricing (per request, not per token).
// Source: https://docs.anthropic.com/en/about-claude/pricing
export const WEB_SEARCH_COST_PER_REQUEST = 0.01;

// Fast mode multiplier applied by the Claude adapter when usage.speed === 'fast'.
export const FAST_MODE_MULTIPLIER = 6;

// OpenAI appends pinned-snapshot suffixes like "-2026-05-01" to model names;
// the Codex branch of getModelFamily strips them.
const OPENAI_DATE_SUFFIX = /-\d{4}-\d{2}-\d{2}$/;

function assertKnownProvider(provider: string): asserts provider is ProviderId {
  if (!PROVIDER_VALUES.some((id) => id === provider)) {
    throw new Error(`Unknown provider: ${provider}`);
  }
}

/**
 * Extract pricing-table key from a full model name. Provider-aware.
 *  - `claude-code` / `claude-opus-4-5-20251101` → `'opus-4-5'`
 *  - `codex` / `gpt-5-2026-05-01`               → `'gpt-5'`
 *  - `codex` / `gpt-5.5`                         → `'gpt-5.5'` (pass-through)
 *
 * Throws on unknown provider — matches `getAdapter()` (CF-417).
 */
export function getModelFamily(provider: ProviderId, modelName: string): string {
  assertKnownProvider(provider);
  if (provider === 'codex') {
    return modelName.replace(OPENAI_DATE_SUFFIX, '');
  }
  // Claude: strip the `claude-` prefix, then match the family pattern.
  const name = modelName.replace(/^claude-/, '');
  const match = name.match(/^(opus|sonnet|haiku)-(\d(?:-\d)?)(?!\d)/);
  return match ? `${match[1]}-${match[2]}` : name;
}

function getPricing(provider: ProviderId, modelName: string): ModelPricing {
  // `getModelFamily` performs the unknown-provider check.
  const family = getModelFamily(provider, modelName);
  const pricing = MODEL_PRICING[provider][family];
  if (!pricing) {
    console.warn(`Unknown model for pricing: ${modelName} (provider: ${provider}, family: ${family})`);
    return ZERO_PRICING;
  }
  return pricing;
}

/**
 * Single arithmetic surface. No fast multiplier, no server-tool add-on —
 * those are Claude-specific and live on the provider adapter.
 *
 * Unknown provider throws; unknown model warns and returns 0.
 */
export function calculateCost(
  provider: ProviderId,
  model: string,
  usage: TokenUsage,
): number {
  const pricing = getPricing(provider, model);
  return (
    usage.input * pricing.input +
    usage.output * pricing.output +
    usage.cacheWrite * pricing.cacheWrite +
    usage.cacheRead * pricing.cacheRead
  ) / 1_000_000;
}

/**
 * Build a cost-badge tooltip. Base lines (`$cost`, blank, input, output)
 * come from this function; per-provider extras (Speed/Tier/Web searches
 * for Claude, Cached (hit) / Reasoning for Codex) come from the adapter's
 * `extendCostTooltip` hook.
 *
 * `adapter` is a structural duck-type so this module doesn't have to
 * import from providers/ (would be circular: tokenStats → providers → tokenStats).
 */
export function buildCostTooltip(
  adapter: {
    extendCostTooltip?(base: string[], usage: TokenUsage, message: unknown): string[];
  },
  usage: TokenUsage,
  cost: number,
  message: unknown,
): string {
  const base = [
    formatCost(cost),
    '',
    `Input tokens (in): ${usage.input.toLocaleString()}`,
    `Output tokens (out): ${usage.output.toLocaleString()}`,
  ];
  const extended = adapter.extendCostTooltip?.(base, usage, message) ?? base;
  return extended.join('\n');
}

/**
 * Per-message Claude wire-shape → canonical TokenUsage. Exported for the
 * Claude transcript service.
 */
interface ClaudeWireUsage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
}

export function normalizeClaudeUsage(wire: ClaudeWireUsage): TokenUsage {
  return {
    input: wire.input_tokens,
    output: wire.output_tokens,
    cacheWrite: wire.cache_creation_input_tokens ?? 0,
    cacheRead: wire.cache_read_input_tokens ?? 0,
  };
}

/**
 * Format cost for display. `<$0.01` is the floor for tiny non-zero amounts.
 */
export function formatCost(cost: number): string {
  if (cost === 0) return '$0.00';
  if (cost < 0.01) return '<$0.01';
  return `$${cost.toFixed(2)}`;
}

/**
 * Format token count for display. 500 → '500', 1500 → '1.5k', 1_500_000 → '1.5M'.
 */
export function formatTokenCount(count: number): string {
  if (count >= 1_000_000_000) return `${(count / 1_000_000_000).toFixed(1)}B`;
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}k`;
  return count.toString();
}
