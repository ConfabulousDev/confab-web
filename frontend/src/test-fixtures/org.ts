// Shared org-view fixtures for stories and tests. Centralizes per-provider
// default values so a hypothetical third provider can be added by extending
// `DEFAULTS_BY_PROVIDER` rather than touching every story by hand. Mirrors
// the CF-420 pattern used by `session.ts`.

import type { OrgUserAnalytics } from '@/schemas/api';
import type { ProviderId } from '@/utils/providers';

// Per-provider defaults are story decoration only — do not rely on the
// numeric values for unit-test assertions.
interface ProviderDefaults {
  emailDomain: string;
  sessionCount: number;
  totalCostUSD: string;
  totalDurationMs: number;
  totalAssistantTimeMs: number;
  totalUserTimeMs: number;
  avgCostUSD: string;
  avgDurationMs: number;
  avgAssistantTimeMs: number;
  avgUserTimeMs: number;
}

const DEFAULTS_BY_PROVIDER: Record<ProviderId, ProviderDefaults> = {
  'claude-code': {
    emailDomain: 'example.com',
    sessionCount: 45,
    totalCostUSD: '128.50',
    totalDurationMs: 432_000_000,
    totalAssistantTimeMs: 216_000_000,
    totalUserTimeMs: 216_000_000,
    avgCostUSD: '2.86',
    avgDurationMs: 9_600_000,
    avgAssistantTimeMs: 4_800_000,
    avgUserTimeMs: 4_800_000,
  },
  codex: {
    emailDomain: 'example.com',
    sessionCount: 22,
    totalCostUSD: '64.40',
    totalDurationMs: 198_000_000,
    totalAssistantTimeMs: 132_000_000,
    totalUserTimeMs: 66_000_000,
    avgCostUSD: '2.93',
    avgDurationMs: 9_000_000,
    avgAssistantTimeMs: 6_000_000,
    avgUserTimeMs: 3_000_000,
  },
  opencode: {
    emailDomain: 'example.com',
    sessionCount: 10,
    totalCostUSD: '32.00',
    totalDurationMs: 90_000_000,
    totalAssistantTimeMs: 60_000_000,
    totalUserTimeMs: 30_000_000,
    avgCostUSD: '3.20',
    avgDurationMs: 9_000_000,
    avgAssistantTimeMs: 6_000_000,
    avgUserTimeMs: 3_000_000,
  },
};

let nextID = 1;
function nextFixtureUserID(): number {
  nextID += 1;
  return nextID;
}

/**
 * Build an `OrgUserAnalytics` fixture for the given canonical provider.
 * Overrides win — any field caller-set is preserved verbatim (including
 * `user`, where the caller can supply both `email` and `name`).
 *
 * The helper does not encode anything about "user X used provider Y" — it
 * just stamps reasonable defaults so stories don't need to spell every
 * field out for the table to render meaningfully.
 */
export function makeOrgUserFixture(
  provider: ProviderId,
  overrides: Partial<OrgUserAnalytics> = {},
): OrgUserAnalytics {
  const defaults = DEFAULTS_BY_PROVIDER[provider];
  const id = overrides.user?.id ?? nextFixtureUserID();
  const name = overrides.user?.name ?? `User ${id}`;
  const email = overrides.user?.email ?? `user-${id}@${defaults.emailDomain}`;

  return {
    user: { id, email, name },
    session_count: defaults.sessionCount,
    total_cost_usd: defaults.totalCostUSD,
    total_duration_ms: defaults.totalDurationMs,
    total_assistant_time_ms: defaults.totalAssistantTimeMs,
    total_user_time_ms: defaults.totalUserTimeMs,
    avg_cost_usd: defaults.avgCostUSD,
    avg_duration_ms: defaults.avgDurationMs,
    avg_assistant_time_ms: defaults.avgAssistantTimeMs,
    avg_user_time_ms: defaults.avgUserTimeMs,
    ...overrides,
  };
}
