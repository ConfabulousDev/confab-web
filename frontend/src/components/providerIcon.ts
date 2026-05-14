// Provider-specific identity icon helper. Lives in its own file to keep
// `icons.tsx` exporting only React elements (HMR fast-refresh rule).
//
// Anything outside the canonical providers falls back to Claude — historically
// all sessions were Claude-only and unknown values most often originate from
// older rows.

import { ClaudeCodeIcon, CodexIcon } from './icons';

export function getProviderIcon(provider: string) {
  return provider === 'codex' ? CodexIcon : ClaudeCodeIcon;
}
