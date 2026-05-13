// Renders the transcript-tab content for Codex sessions.
//
// Self-contained: handles its own fetch + 15s poll, normalizes raw lines into
// render items via useMemo, and hands them to CodexMessageTimeline. Mirrors
// the Claude transcript flow (line-offset incremental polling, append on
// new lines) so the two providers behave consistently.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useVisibility } from '@/hooks/useVisibility';
import {
  fetchParsedCodexTranscript,
  fetchNewCodexLines,
  normalizeCodexLines,
} from '@/services/codexTranscriptService';
import type { RawCodexLine } from '@/schemas/codexTranscript';
import CodexMessageTimeline from '@/components/transcript/codex/CodexMessageTimeline';
import styles from './CodexTranscriptPane.module.css';

// Match the Claude poll cadence for parity.
const TRANSCRIPT_POLL_INTERVAL_MS = 15000;

export interface CodexTranscriptPaneProps {
  sessionId: string;
  transcriptFileName: string | undefined;
  /** For Storybook: pass raw lines directly instead of fetching from API */
  initialRawLines?: RawCodexLine[];
}

export default function CodexTranscriptPane({
  sessionId,
  transcriptFileName,
  initialRawLines,
}: CodexTranscriptPaneProps) {
  // Storybook mode: skip fetching entirely and render whatever was passed in.
  const isStorybookMode = initialRawLines !== undefined;
  const missingFile = !transcriptFileName && !isStorybookMode;

  const [loading, setLoading] = useState(!isStorybookMode && !missingFile);
  const [error, setError] = useState<string | null>(
    missingFile ? 'No transcript file found' : null,
  );
  const [rawLines, setRawLines] = useState<RawCodexLine[]>(initialRawLines ?? []);

  // Track raw file position for incremental polling.
  const lineCountRef = useRef(0);
  const isVisible = useVisibility();

  // Initial load.
  useEffect(() => {
    if (isStorybookMode) return;
    if (!transcriptFileName) return;

    let cancelled = false;
    lineCountRef.current = 0;

    fetchParsedCodexTranscript(sessionId, transcriptFileName, true)
      .then((parsed) => {
        if (cancelled) return;
        setRawLines(parsed.rawLines);
        lineCountRef.current = parsed.totalLines;
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : 'Failed to load transcript');
        console.error('Failed to load Codex transcript:', e);
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [sessionId, transcriptFileName, isStorybookMode]);

  // Incremental polling.
  useEffect(() => {
    if (isStorybookMode || !isVisible || loading || !transcriptFileName) {
      return;
    }

    const poll = async () => {
      try {
        const { newRawLines, newTotalLineCount } = await fetchNewCodexLines(
          sessionId,
          transcriptFileName,
          lineCountRef.current,
        );
        if (newRawLines.length > 0) {
          setRawLines((prev) => [...prev, ...newRawLines]);
          lineCountRef.current = newTotalLineCount;
        }
      } catch (e) {
        console.warn('Failed to poll for new Codex lines:', e);
      }
    };

    const id = setInterval(poll, TRANSCRIPT_POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [isStorybookMode, isVisible, loading, sessionId, transcriptFileName]);

  // Re-derive render items whenever raw lines change. Pure, cheap inside useMemo.
  const items = useMemo(() => normalizeCodexLines(rawLines), [rawLines]);

  if (loading) {
    return <div className={styles.loading}>Loading transcript...</div>;
  }
  if (error) {
    return (
      <div className={styles.error}>
        <strong>Error:</strong> {error}
      </div>
    );
  }

  return <CodexMessageTimeline items={items} />;
}
