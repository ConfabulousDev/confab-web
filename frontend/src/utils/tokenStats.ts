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
 *   - `cacheWrite`: 5-minute cache-creation tokens. Anthropic charges 1.25x
 *     input; OpenAI charges 0 (set to 0 by the Codex normalizer).
 *   - `cacheWrite1h`: 1-hour cache-creation tokens (Anthropic charges 2x input).
 *     Split out from `cacheWrite` by the Claude normalizer from the wire
 *     `cache_creation` object; 0 for legacy lines and non-Claude providers (rd9v).
 *   - `cacheRead`: cache-hit tokens (Codex's `cached_input_tokens`,
 *     Anthropic's `cache_read_input_tokens`).
 */
export interface TokenUsage {
  input: number;
  output: number;
  cacheWrite: number;
  cacheWrite1h: number;
  cacheRead: number;
}

export interface ModelPricing {
  input: number;
  output: number;
  cacheWrite: number;
  cacheWrite1h: number;
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
// `cursor` is keyed but always empty: Cursor transcripts carry no token/cost
// data and the backend serves no cursor pricing, so cost UI stays hidden. The
// key is required because PricingTable is Record<ProviderId, …>.
let activePricing: PricingTable = { 'claude-code': {}, codex: {}, opencode: {}, cursor: {} };

/** Install the effective price table fetched from the backend (CF-515). */
export function setPricingTable(table: PricingTable): void {
  activePricing = table;
}

const ZERO_PRICING: ModelPricing = { input: 0, output: 0, cacheWrite: 0, cacheWrite1h: 0, cacheRead: 0 };

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
 *  - `claude-code` / `claude-fable-5`            → `'fable-5'`
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
  const match = name.match(/^(opus|sonnet|haiku|fable)-(\d(?:-\d)?)(?!\d)/);
  return match ? `${match[1]}-${match[2]}` : name;
}

// sonnet5Sep1 is the boundary between Sonnet 5 introductory and standard pricing.
// Sessions whose first_seen is before this instant use the "sonnet-5-intro" rates
// ($2 input, $10 output); sessions on or after use the "sonnet-5" standard rates
// ($3 input, $15 output). The introductory period runs through Aug 31, 2026.
const SONNET5_SEP1 = new Date('2026-09-01T00:00:00Z');

function getPricing(provider: ProviderId, modelName: string, sessionAt?: Date): ModelPricing {
  // `getModelFamily` performs the unknown-provider check.
  const family = getModelFamily(provider, modelName);

  // Sonnet 5 date-aware routing: sessions starting before 2026-09-01 use the
  // introductory rates stored under "sonnet-5-intro"; on or after that date they
  // use the standard "sonnet-5" rates. When sessionAt is omitted, new Date() is
  // used — correct for newly-computed sessions displayed at their session time.
  const effectiveFamily =
    family === 'sonnet-5' && (sessionAt ?? new Date()) < SONNET5_SEP1
      ? 'sonnet-5-intro'
      : family;

  const pricing = activePricing[provider]?.[effectiveFamily];
  if (!pricing) {
    console.warn(`Unknown model for pricing: ${modelName} (provider: ${provider}, family: ${effectiveFamily})`);
    return ZERO_PRICING;
  }
  return pricing;
}

/**
 * Single arithmetic surface. No fast multiplier, no server-tool add-on —
 * those are Claude-specific and live on the provider adapter.
 *
 * Unknown provider throws; unknown model warns and returns 0.
 *
 * `sessionAt` is the session's `first_seen` date. When supplied, it is
 * forwarded to `getPricing` for date-aware routing (e.g. Sonnet 5 intro
 * rates through Aug 31, 2026). When omitted, `new Date()` is used — correct
 * for newly-computed sessions where `sessionAt ≈ now`.
 */
export function calculateCost(
  provider: ProviderId,
  model: string,
  usage: TokenUsage,
  sessionAt?: Date,
): number {
  const pricing = getPricing(provider, model, sessionAt);
  // 1h cache writes fall back to the 5m rate when cacheWrite1h is missing/0
  // (e.g. a stale remote pricing doc) so they never bill $0 (rd9v).
  const effective1hRate = pricing.cacheWrite1h || pricing.cacheWrite;
  return (
    usage.input * pricing.input +
    usage.output * pricing.output +
    usage.cacheWrite * pricing.cacheWrite +
    usage.cacheWrite1h * effective1hRate +
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
  cache_creation?: {
    ephemeral_5m_input_tokens: number;
    ephemeral_1h_input_tokens: number;
  };
}

export function normalizeClaudeUsage(wire: ClaudeWireUsage): TokenUsage {
  // Split cache-creation by ephemeral tier when the nested object is present;
  // legacy lines (no object) are treated as all-5m (rd9v).
  const cacheWrite = wire.cache_creation
    ? wire.cache_creation.ephemeral_5m_input_tokens
    : wire.cache_creation_input_tokens ?? 0;
  const cacheWrite1h = wire.cache_creation?.ephemeral_1h_input_tokens ?? 0;
  return {
    input: wire.input_tokens,
    output: wire.output_tokens,
    cacheWrite,
    cacheWrite1h,
    cacheRead: wire.cache_read_input_tokens ?? 0,
  };
}

/**
 * Total cache-creation tokens for display (5m + 1h). The canonical TokenUsage
 * splits cache writes by ephemeral tier for billing (rd9v); display surfaces
 * that show a single "cache write" count must use this combined total so the
 * number stays the full cache-creation count (decision: no separate 1h line).
 */
export function cacheWriteTotal(usage: TokenUsage): number {
  return usage.cacheWrite + usage.cacheWrite1h;
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
 * Compact cost for tight spots (e.g. histogram bar labels that can reach into the
 * millions). Below $1,000 it matches formatCost exactly ('$12.50', '<$0.01');
 * above it abbreviates: '$1.2K', '$3.4M', '$1.0B'.
 */
export function formatCostCompact(cost: number): string {
  if (cost >= 1_000_000_000) return `$${(cost / 1_000_000_000).toFixed(1)}B`;
  if (cost >= 1_000_000) return `$${(cost / 1_000_000).toFixed(1)}M`;
  if (cost >= 1_000) return `$${(cost / 1_000).toFixed(1)}K`;
  return formatCost(cost);
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

// CF-525: token speed = output tokens generated per second.
//
// One shared arithmetic surface so the three display surfaces (Conversation
// card, Trends, transcript) never diverge. The numerator is output tokens
// only; the denominator is a duration in ms — assistant generation time for
// the exact surfaces, predecessor-timestamp delta for the approximate
// per-message badge.

/**
 * Output tokens per second, or `null` when the duration can't yield a rate
 * (zero, negative, or NaN — e.g. missing timing, clock skew, or shared
 * streamed timestamps). Zero output with a real duration is a genuine `0`,
 * not `null`. Rounding/abbreviation is the formatter's job.
 */
export function computeTokenSpeed(outputTokens: number, durationMs: number): number | null {
  if (!(durationMs > 0)) return null; // 0, negative, NaN
  return outputTokens / (durationMs / 1000);
}

/**
 * Format a token speed for display: `"85 tok/s"`, `"1.5k tok/s"` (k-suffix via
 * `formatTokenCount`), or `"—"` for `null`. The approximation `~` marker for
 * the per-message badge is applied at the call site, not here, so the exact
 * session/Trends numbers stay unqualified.
 */
export function formatTokenSpeed(speed: number | null): string {
  if (speed == null) return '—';
  return `${formatTokenCount(Math.round(speed))} tok/s`;
}

/**
 * Approximate per-message token speed for the transcript badge. Duration is
 * estimated as the gap between this assistant message's timestamp and the
 * immediately preceding transcript entry's timestamp.
 *
 * Returns `null` (badge omitted) when there is no predecessor, when output is
 * zero, when either timestamp is unparseable, or when the gap is non-positive
 * (streamed blocks sharing a stamp, or clock skew). This is an estimate — the
 * caller marks it as such.
 */
export function computeMessageTokenSpeed(
  outputTokens: number,
  prevTimestamp: string | undefined,
  timestamp: string,
): number | null {
  if (outputTokens <= 0 || prevTimestamp == null) return null;
  const prevMs = Date.parse(prevTimestamp);
  const thisMs = Date.parse(timestamp);
  if (Number.isNaN(prevMs) || Number.isNaN(thisMs)) return null;
  return computeTokenSpeed(outputTokens, thisMs - prevMs);
}
