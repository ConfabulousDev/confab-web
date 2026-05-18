// Provider-specific identity icon helper. Lives in its own file to keep
// `icons.tsx` exporting only React elements (HMR fast-refresh rule).
//
// Delegates to the PROVIDER_METADATA registry. CF-366 split the
// unknown-value policy:
//
//   - Canonical and legacy values (e.g. `'claude-code'`, `'codex'`,
//     `'Claude Code'`, `'CLAUDE-CODE'`) resolve to their brand icon —
//     `getProviderMetadataOrFallback` normalizes the input before lookup
//     so the legacy DB display form still hits the Claude entry.
//   - Truly unknown values (a future third-party rollout the frontend
//     hasn't caught up to yet, or an empty/missing field) render the
//     neutral RobotIcon instead of silently impersonating Claude.

import { RobotIcon } from '@/components/icons';
import { getProviderMetadataOrFallback } from '@/utils/providers';

export function getProviderIcon(provider: string) {
  return getProviderMetadataOrFallback(provider, 'neutral')?.icon ?? RobotIcon;
}
