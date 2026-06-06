// Provider adapter registry (CF-417).
//
// Maps a canonical provider id to its adapter. Adding a third provider:
//   1. Add the new id to `PROVIDER_VALUES` in `utils/providers.ts` (Phase 1).
//   2. Write its adapter file under `providers/`.
//   3. Register it here.
//
// `getAdapter` throws on unknown providers — backend already normalizes
// `session.provider` on read (see `backend/internal/models/provider.go`),
// so this only fires when a new provider rolls out backend-first.

import { PROVIDER_VALUES, type ProviderId } from '@/utils/providers';
import { claudeAdapter } from './claudeAdapter';
import { codexAdapter } from './codexAdapter';
import { opencodeAdapter } from './opencodeAdapter';
import type { OpaqueAdapter } from './types';

// Each concrete adapter is fully typed against `ClaudeAdapter` / `CodexAdapter`
// inside its module; widening to `OpaqueAdapter` happens once, here. See
// `types.ts` for the rationale on the opaque boundary.
/* eslint-disable @typescript-eslint/consistent-type-assertions */
const REGISTRY: Record<ProviderId, OpaqueAdapter> = {
  'claude-code': claudeAdapter as unknown as OpaqueAdapter,
  codex: codexAdapter as unknown as OpaqueAdapter,
  opencode: opencodeAdapter as unknown as OpaqueAdapter,
};
/* eslint-enable @typescript-eslint/consistent-type-assertions */

function isProviderId(value: string): value is ProviderId {
  return PROVIDER_VALUES.some((id) => id === value);
}

export function getAdapter(provider: string): OpaqueAdapter {
  const normalized = provider.toLowerCase().replace(/\s+/g, '-');
  if (!isProviderId(normalized)) {
    throw new Error(
      `[providers] no adapter registered for provider '${provider}' (normalized: '${normalized}')`,
    );
  }
  return REGISTRY[normalized];
}
