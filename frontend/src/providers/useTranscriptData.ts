// Shared transcript data hook (CF-417).
//
// Encapsulates initial load + visibility-gated polling for every provider.
// SessionViewer calls this for the main transcript and (et0r) once more for the
// active subagent thread; the adapter supplies the per-provider fetch and
// normalize logic. Storybook stories bypass the fetch by passing a `seed` of
// prefetched raw lines.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useVisibility } from '@/hooks/useVisibility';
import { APIError } from '@/services/api';
import type { OpaqueAdapter } from './types';

const TRANSCRIPT_POLL_INTERVAL_MS = 15000;

interface TranscriptSeed {
  raw: unknown[];
}

interface TranscriptDataOptions {
  /**
   * Reuse the adapter's in-memory cache for the initial load (et0r D9: a
   * revisited subagent tab renders instantly, then polls for new lines).
   */
  preferCache?: boolean;
}

interface TranscriptData {
  items: unknown[];
  raw: unknown[];
  loading: boolean;
  error: string | null;
  /**
   * et0r D7: the initial load 404'd (e.g. a subagent file not uploaded yet).
   * While set, polling retries the initial load and clears it on success.
   */
  notFound: boolean;
}

const NO_OPTIONS: TranscriptDataOptions = {};

function isNotFoundError(e: unknown): boolean {
  return e instanceof APIError && e.status === 404;
}

/**
 * Initial-load + polling for one transcript file.
 *
 * When `seed` is provided, both the initial fetch and polling are skipped and
 * the seed's lines are returned as-is; Storybook stories use this to render
 * against prefetched fixtures. No fetch happens while `fileName` is undefined.
 */
export function useTranscriptData(
  adapter: OpaqueAdapter,
  sessionId: string,
  fileName: string | undefined,
  seed: TranscriptSeed | undefined,
  options: TranscriptDataOptions = NO_OPTIONS,
): TranscriptData {
  const willFetch = seed === undefined;
  const preferCache = options.preferCache ?? false;

  const [raw, setRaw] = useState<unknown[]>([]);
  const [loading, setLoading] = useState(willFetch);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);
  const lineCountRef = useRef(0);
  const isVisible = useVisibility();

  // Drop the previous file's data in the same render the source changes, so a
  // session or thread switch never paints stale rows while the next fetch is
  // in flight (React "adjust state on prop change" pattern).
  const sourceKey = `${sessionId}\u0000${fileName ?? ''}`;
  const [prevSourceKey, setPrevSourceKey] = useState(sourceKey);
  if (sourceKey !== prevSourceKey) {
    setPrevSourceKey(sourceKey);
    setRaw([]);
    setLoading(willFetch);
    setError(null);
    setNotFound(false);
  }

  // Initial load.
  useEffect(() => {
    if (!willFetch || !fileName) return;
    let cancelled = false;
    lineCountRef.current = 0;

    adapter
      .fetchInitial(sessionId, fileName, !preferCache)
      .then((parsed) => {
        if (cancelled) return;
        setRaw(parsed.raw);
        lineCountRef.current = parsed.totalLines;
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        if (isNotFoundError(e)) setNotFound(true);
        setError(e instanceof Error ? e.message : 'Failed to load transcript');
        console.error('Failed to load transcript:', e);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [adapter, sessionId, fileName, willFetch, preferCache]);

  // Visibility-gated polling. While `notFound`, retry the initial load instead
  // of an incremental fetch so a late-uploaded file loads from line 0.
  useEffect(() => {
    if (!willFetch || !isVisible || loading || !fileName) return;
    let cancelled = false;

    const intervalId = setInterval(async () => {
      try {
        if (notFound) {
          const parsed = await adapter.fetchInitial(sessionId, fileName, true);
          if (cancelled) return;
          lineCountRef.current = parsed.totalLines;
          setRaw(parsed.raw);
          setNotFound(false);
          setError(null);
          return;
        }
        const { newRaw, newTotalLineCount } = await adapter.fetchIncremental(
          sessionId,
          fileName,
          lineCountRef.current,
        );
        if (cancelled) return;
        if (newRaw.length > 0) {
          setRaw((prev) => [...prev, ...newRaw]);
          lineCountRef.current = newTotalLineCount;
        }
      } catch (e: unknown) {
        if (!cancelled) console.warn('Failed to poll for new transcript lines:', e);
      }
    }, TRANSCRIPT_POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
  }, [adapter, sessionId, fileName, willFetch, isVisible, loading, notFound]);

  const effectiveRaw = seed ? seed.raw : raw;
  // Stabilize items via the adapter's normalize. Claude's is identity; Codex normalizes.
  const items = useMemo(() => adapter.normalize(effectiveRaw), [adapter, effectiveRaw]);

  return { items, raw: effectiveRaw, loading, error, notFound };
}
