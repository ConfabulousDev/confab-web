// CF-475: Build a transcript-deep-link URL from a TIL list row.
//
// The `?msg=` query-param semantics differ by provider:
//   - Claude (`claude-code`, including the legacy `"Claude Code"` alias):
//     the value is the message UUID (`til.message_uuid`); the transcript
//     resolves it by exact match. Falls through to no `&msg=` when null
//     (the resolver can't interpret a timestamp for Claude).
//   - Codex (`codex`): the value is an ISO 8601 timestamp. Codex messages
//     have no per-message UUID, so the resolver maps the timestamp to the
//     latest render item with `timestamp <= target`. `til.created_at`
//     (server clock when the CLI POSTed the TIL) approximates `/til`
//     invocation time within ms. If the CLI ever populates Codex
//     `message_uuid` with a real identifier we prefer it.
//   - Unknown provider value: behaves like Claude (UUID-only, no timestamp
//     fallback) — fail safe.
import { getProviderMetadataOrFallback } from '@/utils/providers';
import type { TILWithSession } from '@/schemas/api';

export function buildTILDeepLink(til: TILWithSession): string {
  const base = `/sessions/${til.session_id}?tab=transcript`;
  const target = pickDeepLinkTarget(til);
  if (target === null) return base;
  return `${base}&msg=${encodeURIComponent(target)}`;
}

function pickDeepLinkTarget(til: TILWithSession): string | null {
  if (til.message_uuid) return til.message_uuid;
  const meta = getProviderMetadataOrFallback(til.provider, 'neutral');
  if (meta?.id === 'codex') return til.created_at;
  return null;
}
