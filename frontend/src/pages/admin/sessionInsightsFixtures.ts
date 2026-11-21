// Shared fixtures for the Session Insights page's tests and stories (c3ts).
import type { ProviderShareResponse, SessionPoint, SessionPointsResponse } from '@/schemas/api';

export const fixtureShare: ProviderShareResponse = {
  from: '2026-03-01T00:00:00Z',
  to: '2026-03-08T00:00:00Z',
  bucket_size: 'day',
  buckets: [
    { start: '2026-03-02T00:00:00Z', counts: { 'claude-code': 6, codex: 2 } },
    { start: '2026-03-04T00:00:00Z', counts: { 'claude-code': 3, codex: 3, cursor: 1 } },
    { start: '2026-03-06T00:00:00Z', counts: { opencode: 2, 'claude-code': 4 } },
  ],
  totals: { 'claude-code': 13, codex: 5, cursor: 1, opencode: 2 },
};

export function fixturePoint(overrides: Partial<SessionPoint> = {}): SessionPoint {
  return {
    id: 'sess-1',
    first_seen: '2026-03-02T10:00:00Z',
    provider: 'claude-code',
    title: 'Refactor the sync worker',
    owner_email: 'ada@example.com',
    repo: 'acme/widgets',
    branch: 'main',
    lines: 420,
    tokens: 120_000,
    cost_usd: 1.85,
    user_turns: 12,
    duration_ms: 1_800_000,
    ...overrides,
  };
}

const PROVIDERS = ['claude-code', 'codex', 'opencode', 'cursor'] as const;

/** A deterministic spread of points across providers, sizes, and days. */
export function fixturePoints(count: number): SessionPoint[] {
  return Array.from({ length: count }, (_, i) => {
    const provider = PROVIDERS[i % 4]!;
    const scale = 1 + ((i * 37) % 97);
    return fixturePoint({
      id: `sess-${i}`,
      provider,
      first_seen: new Date(Date.UTC(2026, 2, 1 + (i % 7), (i * 5) % 24)).toISOString(),
      title: `Session ${i}`,
      owner_email: i % 3 === 0 ? 'ada@example.com' : 'grace@example.com',
      lines: scale * 25,
      tokens: provider === 'cursor' ? null : scale * 4_000,
      cost_usd: provider === 'cursor' ? null : scale * 0.07,
      user_turns: 1 + (scale % 30),
      duration_ms: scale * 60_000,
    });
  });
}

export function fixturePointsResponse(points: SessionPoint[], total = points.length): SessionPointsResponse {
  return { total, returned: points.length, limit: 5000, points };
}
