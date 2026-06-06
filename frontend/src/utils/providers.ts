// AI Provider registry — single source of truth for cosmetic per-provider
// strings (label, brand prose, icon, brand color, copy-id menu strings).
//
// Migration notes (CF-416):
//   - Cosmetic call sites (providerIcon, SessionHeader, CopyIdDropdown,
//     TrendsTopSessionsCard, FilterChipsBar) all read from PROVIDER_METADATA.
//   - SessionViewer dispatch (Phase 2 / CF-417) and pricing (Phase 3) are
//     intentionally NOT yet keyed on this registry.
//
// Backend authority: backend/internal/models/provider.go owns the canonical
// list and validation. PROVIDER_VALUES is the closed-enum cosmetic projection
// the frontend uses for filter chips; the backend may emit additional values
// during a rollout (handled by providerLabel passthrough + the 'claude' /
// 'neutral' fallback policies below).
//
// Marketing prose hardcoded outside this file: Quickstart.tsx, HeroCards.tsx,
// QuickstartCTA.tsx all spell out "Claude Code and Codex" in conjunctive
// sentences that don't generate cleanly at N != 2 providers. Hand-update
// those when a third provider lands.

import type { ReactNode } from 'react';
import { ClaudeCodeIcon, CodexIcon, OpenCodeIcon } from '@/components/icons';

export const PROVIDER_VALUES = ['claude-code', 'codex', 'opencode'] as const;
export type ProviderId = (typeof PROVIDER_VALUES)[number];

/**
 * Per-card provider-specific tooltip strings. Keyed by card name, then by row.
 * Undefined entries are interpreted as "no provider-specific tooltip on this
 * row" — the card falls back to whatever default tooltip (if any) the row has.
 *
 * Currently only Codex declares any entries; Claude leaves `cardTooltips`
 * undefined since its row labels are self-explanatory for that provider.
 * Added in CF-439 for the Code Activity card's Searches row.
 */
export interface ProviderCardTooltips {
  codeActivity?: {
    searches?: string;
  };
}

export interface ProviderMetadata {
  id: ProviderId;
  /** Filter chip / dropdown label (e.g. "Claude Code"). */
  label: string;
  /** SessionHeader meta-strip brand prose when no model is resolved. */
  brandDisplayName: string;
  /** Pre-instantiated JSX element — matches the icons.tsx export style. */
  icon: ReactNode;
  /** Hex literal mirroring the SVG fill; for future row-border consumers. */
  brandColor: string;
  /** CopyIdDropdown menu copy: the "Copy <agent> ID" entry. */
  resumeCommand: {
    idLabel: string;
    commandHint: string;
  };
  /** Optional per-card tooltip overrides. See `ProviderCardTooltips`. */
  cardTooltips?: ProviderCardTooltips;
}

export const PROVIDER_METADATA: Record<ProviderId, ProviderMetadata> = {
  'claude-code': {
    id: 'claude-code',
    label: 'Claude Code',
    brandDisplayName: 'Claude Code',
    icon: ClaudeCodeIcon,
    brandColor: '#d97757',
    resumeCommand: { idLabel: 'Copy Claude Code ID', commandHint: 'for /resume' },
  },
  codex: {
    id: 'codex',
    label: 'Codex',
    brandDisplayName: 'Codex',
    icon: CodexIcon,
    brandColor: '#10a37f',
    resumeCommand: { idLabel: 'Copy Codex ID', commandHint: 'for codex resume' },
    cardTooltips: {
      codeActivity: {
        searches: "Codex's web_search_call is not counted as file search",
      },
    },
  },
  opencode: {
    id: 'opencode',
    label: 'OpenCode',
    brandDisplayName: 'OpenCode',
    icon: OpenCodeIcon,
    brandColor: '#6366f1',
    resumeCommand: { idLabel: 'Copy OpenCode ID', commandHint: 'for opencode resume' },
  },
};

/** Canonical lookup. Use when the caller has a validated ProviderId. */
export function getProviderMetadata(provider: ProviderId): ProviderMetadata {
  return PROVIDER_METADATA[provider];
}

function isProviderId(value: string): value is ProviderId {
  return PROVIDER_VALUES.some((id) => id === value);
}

// Mirrors backend NormalizeProvider — handles the legacy "Claude Code"
// display form that may slip through unnormalised on the wire.
function normalizeProvider(value: string): string {
  return value.toLowerCase().replace(/\s+/g, '-');
}

/**
 * Mirrors the spirit of backend `models.LegacyAliases`: returns true when
 * `value` is a non-canonical spelling whose normalized form is the
 * canonical `claude-code` id. The classic case is the legacy DB display
 * form `'Claude Code'`.
 *
 * Functionally redundant with `normalizeProvider(value) === 'claude-code' &&
 * value !== 'claude-code'` — promoted to its own export (CF-366) so the
 * legacy-detection rule has one documented home, in case future callers
 * need to branch on "did we just rescue a legacy row?" without re-deriving
 * the rule. `getProviderMetadataOrFallback` handles the rescue
 * transparently via `normalizeProvider`, so most call sites do not need
 * this helper directly.
 */
export function isLegacyClaudeCode(value: string): boolean {
  return normalizeProvider(value) === 'claude-code' && value !== 'claude-code';
}

/**
 * Tolerant lookup with explicit fallback. The SINGLE place that codifies
 * the unknown-provider policy.
 *
 *   - `'claude'`: unknown values resolve to the claude-code entry. Use when
 *     the caller wants Claude as a sensible default (legacy/empty rows).
 *   - `'neutral'`: unknown values return null; the caller substitutes a
 *     neutral element (e.g. ChatIcon in TrendsTopSessionsCard).
 */
export function getProviderMetadataOrFallback(provider: string, fallback: 'claude'): ProviderMetadata;
export function getProviderMetadataOrFallback(provider: string, fallback: 'neutral'): ProviderMetadata | null;
export function getProviderMetadataOrFallback(
  provider: string,
  fallback: 'claude' | 'neutral',
): ProviderMetadata | null {
  const normalized = normalizeProvider(provider);
  if (isProviderId(normalized)) return PROVIDER_METADATA[normalized];
  return fallback === 'claude' ? PROVIDER_METADATA['claude-code'] : null;
}

/**
 * Display label for any provider string (including unknown future values).
 * Passes unknown values through as-is so a backend-first provider rollout
 * still renders a readable chip while the frontend catches up.
 */
export function providerLabel(value: string): string {
  return isProviderId(value) ? PROVIDER_METADATA[value].label : value;
}
