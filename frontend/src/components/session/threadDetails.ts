// we3k / jgk8: pure display helpers shared by the thread strip's chips, their
// tooltip, the thread list dropdowns and the subagent path breadcrumb.

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

/**
 * jgk8: the open thread's ancestry, depth-1 first and `thread` last, walked
 * through `known` up to Main. Undefined when any link is unknown (a cold deep
 * link, or an ancestor not discovered this visit).
 */
export function threadPath(
  thread: TranscriptThreadRef,
  known: ReadonlyMap<string, TranscriptThreadRef>,
): TranscriptThreadRef[] | undefined {
  const path = [thread];
  const seen = new Set([thread.id]);
  let current = thread;
  while (typeof current.parentThreadId === 'string') {
    const parent = known.get(current.parentThreadId);
    if (!parent || seen.has(parent.id)) return undefined;
    seen.add(parent.id);
    path.unshift(parent);
    current = parent;
  }
  return current.parentThreadId === null ? path : undefined;
}

/** The subagent deep link that Copy link writes. */
export function threadDeepLink(sessionId: string, threadId: string): string {
  return `${window.location.origin}/sessions/${sessionId}?tab=transcript&agent=${encodeURIComponent(threadId)}`;
}

/** jgk8 D7: All-subagents footer for agents launched by other subagents; undefined when there are none. */
export function nestedThreadsNote(count: number): string | undefined {
  if (count <= 0) return undefined;
  const who = count === 1 ? 'a subagent' : 'subagents';
  return `${count} more launched by ${who}. Open one to see the ones it launched.`;
}
