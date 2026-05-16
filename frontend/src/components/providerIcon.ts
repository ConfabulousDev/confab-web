// Provider-specific identity icon helper. Lives in its own file to keep
// `icons.tsx` exporting only React elements (HMR fast-refresh rule).
//
// Delegates to the PROVIDER_METADATA registry; unknown values fall back to
// the Claude icon (historically all sessions were Claude-only and unknown
// values most often originate from older rows).

import { getProviderMetadataOrFallback } from '@/utils/providers';

export function getProviderIcon(provider: string) {
  return getProviderMetadataOrFallback(provider, 'claude').icon;
}
