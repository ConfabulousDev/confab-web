// CF-420: shared session fixtures for stories and tests.
//
// Centralizes the per-provider default values (model name, external_id
// format, transcript file name) so a hypothetical third provider can be
// added by extending `DEFAULTS_BY_PROVIDER` rather than touching every
// story and test that builds a session by hand.
//
// Call sites that need to exercise provider-specific UI branches still pass
// an explicit provider literal; this helper only removes *defaulted* provider
// strings — the ones that exist because every fixture has to specify something.

import type { Session, SessionDetail } from '@/schemas/api';
import type { ProviderId } from '@/utils/providers';

interface ProviderDefaults {
  externalIdPrefix: string;
  transcriptFileName: string;
}

const DEFAULTS_BY_PROVIDER: Record<ProviderId, ProviderDefaults> = {
  'claude-code': {
    externalIdPrefix: 'claude-fixture',
    transcriptFileName: 'transcript.jsonl',
  },
  codex: {
    externalIdPrefix: 'codex-fixture',
    transcriptFileName: 'rollout.jsonl',
  },
  opencode: {
    externalIdPrefix: 'opencode-fixture',
    transcriptFileName: 'messages.jsonl',
  },
};

const FIXTURE_TIMESTAMP = '2026-05-13T01:00:00Z';
const FIXTURE_EMAIL = 'fixture@example.com';

/**
 * Build a `SessionDetail` fixture. Overrides win — any field caller-set is
 * preserved verbatim.
 */
export function makeSessionDetailFixture(
  provider: ProviderId,
  overrides: Partial<SessionDetail> = {},
): SessionDetail {
  const defaults = DEFAULTS_BY_PROVIDER[provider];
  return {
    id: 'fixture-session-id',
    external_id: `${defaults.externalIdPrefix}-external-id`,
    provider,
    custom_title: null,
    summary: null,
    first_user_message: null,
    first_seen: FIXTURE_TIMESTAMP,
    files: [
      {
        file_name: defaults.transcriptFileName,
        file_type: 'transcript',
        last_synced_line: 10,
        updated_at: FIXTURE_TIMESTAMP,
      },
    ],
    owner_email: FIXTURE_EMAIL,
    ...overrides,
  };
}

/**
 * Per-day per-provider cost fixture for trends-card stories. Routes the
 * full daily cost to a single provider so the stacked bar chart renders in
 * that provider's brand color instead of the generic fallback. Used by both
 * `TrendsTokensCard.stories.tsx` and `TrendsPage.stories.tsx`.
 */
export function singleProviderDailyCosts(
  providerId: string,
  daily: Array<{ date: string; cost_usd: string }>,
): Array<{ date: string; cost_usd: string; per_provider: Record<string, string> }> {
  return daily.map((d) => ({ ...d, per_provider: { [providerId]: d.cost_usd } }));
}

/**
 * Build a list-item `Session` fixture (for session-list pages and trends
 * cards that index a list of recent sessions).
 */
export function makeSessionFixture(
  provider: ProviderId,
  overrides: Partial<Session> = {},
): Session {
  const defaults = DEFAULTS_BY_PROVIDER[provider];
  return {
    id: 'fixture-session-id',
    external_id: `${defaults.externalIdPrefix}-external-id`,
    first_seen: FIXTURE_TIMESTAMP,
    file_count: 1,
    provider,
    total_lines: 10,
    is_owner: true,
    access_type: 'owner',
    owner_email: FIXTURE_EMAIL,
    ...overrides,
  };
}
