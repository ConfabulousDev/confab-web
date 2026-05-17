// Shared transcript data hook (CF-417).
//
// Encapsulates initial load + visibility-gated polling for every provider.
// SessionViewer calls this once; the adapter supplies the per-provider fetch
// and normalize logic. Storybook stories bypass the fetch by passing a `seed`
// of prefetched raw lines.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useVisibility } from '@/hooks/useVisibility';
import type { OpaqueAdapter } from './types';

const TRANSCRIPT_POLL_INTERVAL_MS = 15000;

export interface TranscriptSeed {
  raw: unknown[];
}

export interface TranscriptData {
  items: unknown[];
  raw: unknown[];
  loading: boolean;
  error: string | null;
}

/**
 * Initial-load + polling for the active provider's transcript.
 *
 * When `seed` is provided, both the initial fetch and polling are skipped;
 * Storybook stories use this to render against prefetched fixtures.
 */
export function useTranscriptData(
  adapter: OpaqueAdapter,
  sessionId: string,
  fileName: string | undefined,
  seed: TranscriptSeed | undefined,
): TranscriptData {
  const willFetch = seed === undefined;

  const [raw, setRaw] = useState<unknown[]>(seed?.raw ?? []);
  const [loading, setLoading] = useState(willFetch);
  const [error, setError] = useState<string | null>(null);
  const lineCountRef = useRef(0);
  const isVisible = useVisibility();

  // Initial load. Also clears raw + resets lineCountRef when switching
  // sessions, so we don't render stale data while the next fetch is in flight.
  useEffect(() => {
    if (!willFetch || !fileName) return;
    let cancelled = false;
    // Synchronize with sessionId/adapter: drop stale data before the new fetch
    // resolves. The rule's "you might not need an effect" advice doesn't apply
    // — these resets *are* the synchronization point.
    /* eslint-disable react-hooks/set-state-in-effect */
    setRaw([]);
    setLoading(true);
    setError(null);
    /* eslint-enable react-hooks/set-state-in-effect */
    lineCountRef.current = 0;

    adapter
      .fetchInitial(sessionId, fileName, true)
      .then((parsed) => {
        if (cancelled) return;
        setRaw(parsed.raw);
        lineCountRef.current = parsed.totalLines;
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : 'Failed to load transcript');
        console.error('Failed to load transcript:', e);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [adapter, sessionId, fileName, willFetch]);

  // Visibility-gated polling.
  useEffect(() => {
    if (!willFetch || !isVisible || loading || !fileName) return;

    const intervalId = setInterval(async () => {
      try {
        const { newRaw, newTotalLineCount } = await adapter.fetchIncremental(
          sessionId,
          fileName,
          lineCountRef.current,
        );
        if (newRaw.length > 0) {
          setRaw((prev) => [...prev, ...newRaw]);
          lineCountRef.current = newTotalLineCount;
        }
      } catch (e: unknown) {
        console.warn('Failed to poll for new transcript lines:', e);
      }
    }, TRANSCRIPT_POLL_INTERVAL_MS);

    return () => clearInterval(intervalId);
  }, [adapter, sessionId, fileName, willFetch, isVisible, loading]);

  // Stabilize items via the adapter's normalize. Claude's is identity; Codex normalizes.
  const items = useMemo(() => adapter.normalize(raw), [adapter, raw]);

  return { items, raw, loading, error };
}
