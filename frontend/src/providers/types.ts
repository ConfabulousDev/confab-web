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
import type { TIL } from '@/schemas/api';
import type { TranscriptLine } from '@/types';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import type {
  FilterState,
  HierarchicalCounts,
  MessageCategory,
  UserSubcategory,
  AssistantSubcategory,
  AttachmentSubcategory,
} from '@/components/session/messageCategories';
import type {
  CodexFilterState,
  CodexHierarchicalCounts,
  CodexCategory,
  CodexAssistantSubcategory,
  CodexToolCallSubcategory,
} from '@/components/session/codexCategories';

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
  tilsByMessageUuid: Map<string, TIL[]>;
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
  readonly supportsTILs: boolean;

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
   * Per-session Tokens summary card tooltip for the "Estimated cost" row.
   * Each provider supplies its own copy (5-minute prompt caching note for
   * Claude, OpenAI-pricing note for Codex). CF-436.
   */
  readonly tokensCostTooltip: string;

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
  toggleCategory: (category: MessageCategory) => void;
  toggleUserSubcategory: (sub: UserSubcategory) => void;
  toggleAssistantSubcategory: (sub: AssistantSubcategory) => void;
  toggleAttachmentSubcategory: (sub: AttachmentSubcategory) => void;
}

export interface CodexToggles {
  toggleCategory: (category: CodexCategory) => void;
  toggleAssistantSubcategory: (sub: CodexAssistantSubcategory) => void;
  toggleToolCallSubcategory: (sub: CodexToolCallSubcategory) => void;
}

export type ClaudeAdapter = ProviderAdapter<
  TranscriptLine,
  TranscriptLine,
  FilterState,
  ClaudeToggles,
  HierarchicalCounts
>;

export type CodexAdapter = ProviderAdapter<
  RawCodexLine,
  CodexRenderItem,
  CodexFilterState,
  CodexToggles,
  CodexHierarchicalCounts
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
