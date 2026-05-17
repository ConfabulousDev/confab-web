// Shared TIL fetch hook (CF-417).
//
// Skips the fetch when the active provider doesn't support TILs. Today only
// the Claude adapter sets supportsTILs=true — Codex messages don't carry the
// message_uuid that TILs reference.

import { useEffect, useState } from 'react';
import { tilsAPI, type TIL } from '@/services/api';

const EMPTY = new Map<string, TIL[]>();

export function useSessionTILs(
  sessionId: string,
  enabled: boolean,
): Map<string, TIL[]> {
  const [tils, setTils] = useState<Map<string, TIL[]>>(EMPTY);

  useEffect(() => {
    // Synchronize with sessionId/enabled: clear stale badges from the previous
    // session before the new fetch lands. The rule's "you might not need an
    // effect" advice doesn't apply here — this *is* the synchronization point.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTils(EMPTY);
    if (!enabled) return;
    let cancelled = false;

    tilsAPI
      .listForSession(sessionId)
      .then((response) => {
        if (cancelled) return;
        const map = new Map<string, TIL[]>();
        for (const til of response.tils) {
          if (!til.message_uuid) continue;
          const existing = map.get(til.message_uuid) ?? [];
          existing.push(til);
          map.set(til.message_uuid, existing);
        }
        setTils(map);
      })
      .catch(() => {
        // Non-critical — TIL markers simply won't appear.
      });

    return () => {
      cancelled = true;
    };
  }, [sessionId, enabled]);

  return tils;
}
