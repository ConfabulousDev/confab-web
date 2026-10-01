// jgk8: pure helpers behind the per-level subagent breadcrumb.

import { describe, expect, it } from 'vitest';
import type { TranscriptThreadRef } from '@/providers/types';
import { nestedThreadsNote, threadDeepLink, threadPath } from './threadDetails';

function thread(id: string, parentThreadId: string | null | undefined): TranscriptThreadRef {
  return { id, fileName: `agent-${id}.jsonl`, label: id, parentThreadId };
}

const d1 = thread('d1', null);
const d2 = thread('d2', 'd1');
const d3 = thread('d3', 'd2');
const known = new Map([d1, d2, d3].map((t) => [t.id, t]));

describe('threadPath', () => {
  it('returns just the thread for an agent launched from Main', () => {
    expect(threadPath(d1, known)).toEqual([d1]);
  });

  it('walks known parents up to Main, depth-1 first', () => {
    expect(threadPath(d3, known)).toEqual([d1, d2, d3]);
  });

  it('is undefined when the parent is unknown (cold deep link)', () => {
    expect(threadPath(thread('zz', undefined), known)).toBeUndefined();
  });

  it('is undefined when an ancestor is missing from the known threads', () => {
    expect(threadPath(thread('orphan', 'missing'), known)).toBeUndefined();
    expect(threadPath(d3, new Map([[d2.id, d2], [d3.id, d3]]))).toBeUndefined();
  });

  it('is undefined on a parent cycle instead of looping', () => {
    const x = thread('x', 'y');
    const y = thread('y', 'x');
    expect(threadPath(x, new Map([[x.id, x], [y.id, y]]))).toBeUndefined();
  });
});

describe('threadDeepLink', () => {
  it('builds the transcript-tab subagent URL with an encoded id', () => {
    expect(threadDeepLink('s1', 'a/b')).toBe(`${window.location.origin}/sessions/s1?tab=transcript&agent=a%2Fb`);
  });
});

describe('nestedThreadsNote', () => {
  it('says how many more subagents other subagents launched', () => {
    expect(nestedThreadsNote(347)).toBe('347 more launched by subagents. Open one to see the ones it launched.');
  });

  it('uses the singular for one', () => {
    expect(nestedThreadsNote(1)).toBe('1 more launched by a subagent. Open one to see the ones it launched.');
  });

  it('is undefined for zero or a negative count', () => {
    expect(nestedThreadsNote(0)).toBeUndefined();
    expect(nestedThreadsNote(-3)).toBeUndefined();
  });
});
