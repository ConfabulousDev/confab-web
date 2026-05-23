// CF-475 spec tests for buildTILDeepLink.
//
// Contract:
//   - Claude TIL (`provider: 'claude-code'`) with non-null message_uuid:
//     `/sessions/{id}?tab=transcript&msg=<uuid>` — unchanged from pre-CF-475.
//   - Claude TIL with null message_uuid: do NOT fall back to created_at;
//     the resolver can't interpret a timestamp for Claude, so omit `&msg=`
//     and just navigate to the transcript top.
//   - Codex TIL with non-null message_uuid: prefer the explicit UUID
//     (future-proofs against the CLI eventually populating one).
//   - Codex TIL with null message_uuid: use `created_at` as the timestamp.
//   - Legacy alias `provider: 'Claude Code'` is treated as Claude.
//   - Unknown provider value: defer to message_uuid-only behavior (no
//     timestamp fallback); never crash.

import { describe, it, expect } from 'vitest';
import { buildTILDeepLink } from './buildTILDeepLink';
import type { TILWithSession } from '@/schemas/api';

function makeTIL(overrides: Partial<TILWithSession>): TILWithSession {
  return {
    id: 1,
    title: 't',
    summary: 's',
    session_id: 'sess-abc',
    message_uuid: null,
    created_at: '2026-05-13T18:00:00Z',
    session_title: null,
    git_repo: null,
    git_branch: null,
    owner_email: 'a@b.c',
    is_owner: true,
    access_type: 'owner',
    provider: 'claude-code',
    ...overrides,
  };
}

describe('buildTILDeepLink', () => {
  describe('Claude provider', () => {
    it('uses message_uuid when present', () => {
      const til = makeTIL({
        provider: 'claude-code',
        message_uuid: 'uuid-123',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript&msg=uuid-123',
      );
    });

    it('omits &msg= when message_uuid is null (no timestamp fallback)', () => {
      const til = makeTIL({
        provider: 'claude-code',
        message_uuid: null,
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript',
      );
    });

    it('omits &msg= when message_uuid is the empty string', () => {
      const til = makeTIL({
        provider: 'claude-code',
        message_uuid: '',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript',
      );
    });

    it('treats legacy "Claude Code" provider value as Claude', () => {
      const til = makeTIL({
        provider: 'Claude Code',
        message_uuid: 'uuid-legacy',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript&msg=uuid-legacy',
      );
    });
  });

  describe('Codex provider', () => {
    it('falls back to created_at when message_uuid is null', () => {
      const til = makeTIL({
        provider: 'codex',
        message_uuid: null,
        created_at: '2026-05-13T18:00:00Z',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript&msg=' +
          encodeURIComponent('2026-05-13T18:00:00Z'),
      );
    });

    it('falls back to created_at when message_uuid is the empty string', () => {
      const til = makeTIL({
        provider: 'codex',
        message_uuid: '',
        created_at: '2026-05-13T18:00:00Z',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript&msg=' +
          encodeURIComponent('2026-05-13T18:00:00Z'),
      );
    });

    it('prefers message_uuid over created_at when both are present', () => {
      // Future-proofing: if a future CLI populates Codex message_uuid with
      // a real identifier, we honor it.
      const til = makeTIL({
        provider: 'codex',
        message_uuid: 'codex-uuid-future',
        created_at: '2026-05-13T18:00:00Z',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript&msg=codex-uuid-future',
      );
    });
  });

  describe('unknown provider', () => {
    // `provider` is a permissive `z.string()` on the wire schema — these
    // unknown values are plain strings at the type level, so no
    // `@ts-expect-error` is needed. The runtime check still hits the
    // unknown-provider branch in `pickDeepLinkTarget`.
    it('uses message_uuid when present, no timestamp fallback', () => {
      const til = makeTIL({
        provider: 'future-provider',
        message_uuid: 'something',
        created_at: '2026-05-13T18:00:00Z',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript&msg=something',
      );
    });

    it('omits &msg= when message_uuid is null', () => {
      const til = makeTIL({
        provider: 'future-provider',
        message_uuid: null,
        created_at: '2026-05-13T18:00:00Z',
      });
      expect(buildTILDeepLink(til)).toBe(
        '/sessions/sess-abc?tab=transcript',
      );
    });
  });
});
