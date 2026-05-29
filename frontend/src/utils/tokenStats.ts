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
 *   - `output`: total output tokens — the wire `output_tokens` passed
 *     through unchanged. On the OpenAI wire, `reasoning_output_tokens`
 *     is a SUBSET of `output_tokens` (CF-471), so reasoning is already
 *     included here and bills at the output rate implicitly. The raw
 *     reasoning count is preserved separately on the assistant render
 *     item (`reasoningTokens`) for the cost-tooltip sub-line.
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

export interface ModelPricing {
  input: number;
  output: number;
  cacheWrite: number;
  cacheRead: number;
}

/** Provider-keyed price table: provider → model family → per-million rates. */
export type PricingTable = Record<ProviderId, Record<string, ModelPricing>>;

// The frontend bundles NO price data. The active table is fetched from this
// app's own backend (GET /api/v1/pricing) once at bootstrap via
// `setPricingTable`. The single source of truth is the backend's embedded
// pricing.json (refreshable from confabulous.dev). Until the fetch lands the
// table is empty — getPricing then warns and bills $0, but cost UI renders
// only after auth + session-data load, by which point the table is populated.
let activePricing: PricingTable = { 'claude-code': {}, codex: {} };

/** Install the effective price table fetched from the backend (CF-515). */
export function setPricingTable(table: PricingTable): void {
  activePricing = table;
}

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
  const pricing = activePricing[provider]?.[family];
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
