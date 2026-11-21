import { describe, it, expect } from 'vitest';
import { SessionDetailSchema } from './api';

const baseDetail = {
  id: '123e4567-e89b-12d3-a456-426614174000',
  external_id: 'ext-1',
  provider: 'claude-code',
  first_seen: '2026-01-01T00:00:00Z',
  files: [],
  owner_email: 'owner@example.com',
};

// xtrf: GET /sessions/{id} carries how the viewer reached the session.
describe('SessionDetailSchema access_type', () => {
  it.each(['owner', 'recipient', 'system', 'public', 'admin'])('accepts %s', (accessType) => {
    const parsed = SessionDetailSchema.parse({ ...baseDetail, access_type: accessType });
    expect(parsed.access_type).toBe(accessType);
  });

  it('accepts a response without access_type (older backends)', () => {
    expect(SessionDetailSchema.parse(baseDetail).access_type).toBeUndefined();
  });

  it('rejects an unknown access_type', () => {
    expect(SessionDetailSchema.safeParse({ ...baseDetail, access_type: 'superuser' }).success).toBe(false);
  });
});
