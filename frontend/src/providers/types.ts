// Provider-keyed adapter dispatch (CF-417).
//
// Two views of the same adapter:
//   - `ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts>` is the
//     fully-typed implementer view. Each adapter file (claudeAdapter,
//     codexAdapter) writes its literal against the concrete-typed alias
//     (`ClaudeAdapter` / `CodexAdapter`) so its closures stay self-checked.
//   - `OpaqueAdapter` is the consumer view (SessionViewer, registry). Every
//     method signature is widened to `unknown[]` / `unknown` so the call site
//     never needs `as` casts; items flow opaquely from `fetchInitial` through
//     `itemMatchesFilter` and out to `TranscriptPane`.
//
// The cast from a concrete adapter to `OpaqueAdapter` happens exactly once
// per adapter, at its module's `export const ... = ... as OpaqueAdapter`
// line. See `claudeAdapter.tsx` / `codexAdapter.tsx`.

import type { FC } from 'react';
import type { ProviderId } from '@/utils/providers';
import type { TokenUsage } from '@/utils/tokenStats';
import type { TranscriptLine } from '@/types';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import type {
  ClaudeFilterState,
  ClaudeHierarchicalCounts,
  ClaudeCategory,
  ClaudeUserSubcategory,
  ClaudeAssistantSubcategory,
  ClaudeAttachmentSubcategory,
} from '@/components/session/claudeCategories';
import type {
  CodexFilterState,
  CodexHierarchicalCounts,
  CodexCategory,
  CodexAssistantSubcategory,
  CodexToolCallSubcategory,
} from '@/components/session/codexCategories';
import type {
  OpenCodeFilterState,
  OpenCodeHierarchicalCounts,
  OpenCodeCategory,
  OpenCodeRenderItem,
} from '@/components/session/opencodeCategories';
import type { OpenCodeRawEntry } from '@/services/opencodeTranscriptService';
import type {
  CursorFilterState,
  CursorHierarchicalCounts,
  CursorCategory,
  CursorRenderItem,
} from '@/components/session/cursorCategories';
import type { CursorRawEntry } from '@/services/cursorTranscriptService';

export interface FilterAPI<TFilterState, TToggles> {
  state: TFilterState;
  setState: (state: TFilterState, opts?: { replace?: boolean }) => void;
  toggles: TToggles;
}

export interface TranscriptPaneProps<TItem> {
  sessionId: string;
  items: TItem[];
  filteredItems: TItem[];
  /** Always provided. Claude pane ignores; Codex pane reads for the timeline bar. */
  visibleIndices: Set<number>;
  loading: boolean;
  error: string | null;
  /** Provider-specific opaque id. Claude: message UUID. Codex: lineId. */
  targetId?: string;
  isCostMode: boolean;
  /** Session time bounds (ce79). Always provided by SessionViewer. Only the
   *  Cursor pane reads them — its JSONL has no per-message time, so it estimates
   *  per-row timestamps by interpolating over `[firstSeen, lastSyncAt]`. Other
   *  panes ignore them (they carry real per-message times). */
  firstSeen?: string | null;
  lastSyncAt?: string | null;
}

export interface SessionMetaFallback {
  firstSeen?: string | null;
  lastSyncAt?: string | null;
}

export interface SessionMetaResult {
  durationMs?: number;
  sessionDate?: Date;
}

export interface ProviderAdapter<TRaw, TItem, TFilterState, TToggles, TCounts> {
  readonly id: ProviderId;
  fetchInitial(
    sessionId: string,
    fileName: string,
    skipCache?: boolean,
  ): Promise<{ items: TItem[]; totalLines: number; raw: TRaw[] }>;

  fetchIncremental(
    sessionId: string,
    fileName: string,
    currentLineCount: number,
  ): Promise<{ newItems: TItem[]; newRaw: TRaw[]; newTotalLineCount: number }>;

  normalize(raw: TRaw[]): TItem[];

  extractModel(raw: TRaw[], items: TItem[]): string | undefined;

  computeMeta(
    items: TItem[],
    raw: TRaw[],
    fallback: SessionMetaFallback,
  ): SessionMetaResult;

  useFilters(): FilterAPI<TFilterState, TToggles>;

  countCategories(items: TItem[]): TCounts;

  itemMatchesFilter(item: TItem, state: TFilterState): boolean;

  useDeepLinkFilterReset(
    items: TItem[],
    targetId: string | undefined,
    filters: FilterAPI<TFilterState, TToggles>,
  ): void;

  /**
   * Per-message cost in USD. The base implementation is just
   * `calculateCost(id, model, usage)`. Claude overrides to apply the fast
   * multiplier (6x) and add per-request web-search dollars on top.
   */
  calculateMessageCost(model: string, usage: TokenUsage, message: TItem): number;

  /**
   * Optional. Append provider-specific lines to a cost-tooltip's base lines
   * (`$cost`, blank, input, output). Claude appends Cache/Speed/Tier/Web search
   * lines; Codex appends Cached (hit) and Reasoning sub-lines.
   *
   * Receives the `message` so subclasses can reach Claude wire-shape extras
   * (speed, service_tier, server_tool_use) or Codex per-item reasoning count
   * that don't live on the canonical `TokenUsage`.
   */
  extendCostTooltip?(base: string[], usage: TokenUsage, message: TItem): string[];

  /**
   * When false, synced transcripts lack token/cost fields (st5f). Omitted = true.
   */
  readonly tokensMeasurable?: boolean;

  /** Tokens card "Estimated cost" row tooltip (CF-436). Also used for unmeasured UX. */
  readonly tokensCostTooltip: string;

  /** Conversation card "Token speed" tooltip when timing or output tokens are missing. */
  readonly tokenSpeedUnavailableTooltip?: string;

  /**
   * Conversation card footer note (zsk4). Set when a provider's synced
   * transcript has no per-message timestamps, so per-turn timing and
   * utilization can't be computed. The card renders this as a single muted
   * footer instead of five "Not available" rows. Omitted for providers that
   * carry real per-turn timing (Claude, Codex, OpenCode).
   */
  readonly conversationTimingUnavailableNote?: string;

  /**
   * Per-session Tokens summary card tooltip for the "Fast mode" row.
   * Only the provider that surfaces the row (Claude's Anthropic priority
   * tier) defines this. CF-436.
   */
  readonly tokensFastTooltip?: string;

  FilterDropdown: FC<{
    counts: TCounts;
    filters: FilterAPI<TFilterState, TToggles>;
  }>;

  TranscriptPane: FC<TranscriptPaneProps<TItem>>;
}

export interface ClaudeToggles {
  toggleCategory: (category: ClaudeCategory) => void;
  toggleUserSubcategory: (sub: ClaudeUserSubcategory) => void;
  toggleAssistantSubcategory: (sub: ClaudeAssistantSubcategory) => void;
  toggleAttachmentSubcategory: (sub: ClaudeAttachmentSubcategory) => void;
}

export interface CodexToggles {
  toggleCategory: (category: CodexCategory) => void;
  toggleAssistantSubcategory: (sub: CodexAssistantSubcategory) => void;
  toggleToolCallSubcategory: (sub: CodexToolCallSubcategory) => void;
}

export type ClaudeAdapter = ProviderAdapter<
  TranscriptLine,
  TranscriptLine,
  ClaudeFilterState,
  ClaudeToggles,
  ClaudeHierarchicalCounts
>;

export type CodexAdapter = ProviderAdapter<
  RawCodexLine,
  CodexRenderItem,
  CodexFilterState,
  CodexToggles,
  CodexHierarchicalCounts
>;

export interface OpenCodeToggles {
  toggleCategory: (category: OpenCodeCategory) => void;
}

export type OpenCodeAdapter = ProviderAdapter<
  OpenCodeRawEntry,
  OpenCodeRenderItem,
  OpenCodeFilterState,
  OpenCodeToggles,
  OpenCodeHierarchicalCounts
>;

export interface CursorToggles {
  toggleCategory: (category: CursorCategory) => void;
}

export type CursorAdapter = ProviderAdapter<
  CursorRawEntry,
  CursorRenderItem,
  CursorFilterState,
  CursorToggles,
  CursorHierarchicalCounts
>;

/**
 * Consumer-facing adapter shape. All payload types are widened to `unknown`
 * so SessionViewer treats items, raw lines, filter state, and counts
 * opaquely — no per-provider narrowing at the call site, no `as` casts.
 *
 * Each concrete adapter is structurally a `ProviderAdapter<...>` instance;
 * the widening happens once at the adapter's `export const` boundary.
 */
export type OpaqueAdapter = ProviderAdapter<unknown, unknown, unknown, Record<string, unknown>, unknown>;
