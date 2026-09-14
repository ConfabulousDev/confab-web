// we3k: pure display helpers shared by the thread strip's chips, their tooltip,
// and the All-subagents dropdown.

import type { TranscriptThreadRef, TranscriptThreadStatus } from '@/providers/types';

/** Status in words, for accessible names and details (never color alone). */
export const THREAD_STATUS_LABEL: Record<TranscriptThreadStatus, string> = {
  running: 'running',
  completed: 'completed',
  failed: 'failed',
  stopped: 'stopped',
  unknown: 'status unknown',
};

export function threadStatus(thread: TranscriptThreadRef): TranscriptThreadStatus {
  return thread.status ?? 'unknown';
}

/** Launched by another subagent (not Main, not an unknown deep link). */
export function isNestedThread(thread: TranscriptThreadRef): boolean {
  return typeof thread.parentThreadId === 'string';
}

/**
 * Label of the thread that launched `thread`: "Main", the parent's label, the
 * parent's raw id when it isn't in `threads`, or undefined when unknown.
 */
export function parentLabel(thread: TranscriptThreadRef, threads: readonly TranscriptThreadRef[]): string | undefined {
  const { parentThreadId } = thread;
  if (parentThreadId === undefined) return undefined;
  if (parentThreadId === null) return 'Main';
  return threads.find((t) => t.id === parentThreadId)?.label ?? parentThreadId;
}

/** Case-insensitive match on label, subtitle (agent type) or model. */
export function threadMatchesQuery(thread: TranscriptThreadRef, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return [thread.label, thread.subtitle, thread.model].some((value) => value?.toLowerCase().includes(q));
}
