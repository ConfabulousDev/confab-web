// Spec tests for the Cursor row-nav / copy-payload helpers (a9gr).

import { describe, it, expect } from 'vitest';
import type { CursorRenderItem } from './cursorCategories';
import {
  buildCursorRowNav,
  cursorRowKindLabel,
  buildCursorRowCopyText,
} from './cursorRowNav';

const u = (id: string, text = 'hi'): CursorRenderItem => ({ kind: 'user', id, text });
const a = (id: string, text = 'hello'): CursorRenderItem => ({ kind: 'assistant', id, text });
const t = (id: string, input?: string): CursorRenderItem => ({
  kind: 'tool',
  id,
  toolName: 'Read',
  input,
});

describe('buildCursorRowNav', () => {
  it('links consecutive rows of the same kind (next + prev)', () => {
    // user, assistant, user → the two users are same-kind neighbors.
    const items = [u('0'), a('1'), u('2')];
    const { nextOfSameKind, prevOfSameKind } = buildCursorRowNav(items);
    expect(nextOfSameKind.get(0)).toBe(2);
    expect(prevOfSameKind.get(2)).toBe(0);
  });

  it('chains three same-kind rows pairwise', () => {
    const items = [t('0-0'), t('1-0'), t('2-0')];
    const { nextOfSameKind, prevOfSameKind } = buildCursorRowNav(items);
    expect(nextOfSameKind.get(0)).toBe(1);
    expect(nextOfSameKind.get(1)).toBe(2);
    expect(prevOfSameKind.get(2)).toBe(1);
    expect(prevOfSameKind.get(1)).toBe(0);
  });

  it('keeps user / assistant / tool chains independent', () => {
    // 0 user, 1 assistant, 2 tool, 3 assistant, 4 user
    const items = [u('0'), a('1'), t('1-0'), a('2'), u('3')];
    const { nextOfSameKind, prevOfSameKind } = buildCursorRowNav(items);
    // users: 0 → 4
    expect(nextOfSameKind.get(0)).toBe(4);
    expect(prevOfSameKind.get(4)).toBe(0);
    // assistants: 1 → 3
    expect(nextOfSameKind.get(1)).toBe(3);
    expect(prevOfSameKind.get(3)).toBe(1);
    // lone tool at idx 2 has no same-kind neighbor in either direction
    expect(nextOfSameKind.has(2)).toBe(false);
    expect(prevOfSameKind.has(2)).toBe(false);
  });

  it('leaves a single row of a kind with no neighbors (both buttons hidden)', () => {
    const items = [u('0')];
    const { nextOfSameKind, prevOfSameKind } = buildCursorRowNav(items);
    expect(nextOfSameKind.size).toBe(0);
    expect(prevOfSameKind.size).toBe(0);
  });

  it('first-of-kind has next but no prev; last-of-kind has prev but no next', () => {
    const items = [u('0'), u('1')];
    const { nextOfSameKind, prevOfSameKind } = buildCursorRowNav(items);
    expect(nextOfSameKind.get(0)).toBe(1);
    expect(prevOfSameKind.has(0)).toBe(false);
    expect(prevOfSameKind.get(1)).toBe(0);
    expect(nextOfSameKind.has(1)).toBe(false);
  });
});

describe('cursorRowKindLabel', () => {
  it('labels each kind', () => {
    expect(cursorRowKindLabel(u('0'))).toBe('user prompt');
    expect(cursorRowKindLabel(a('1'))).toBe('assistant message');
    expect(cursorRowKindLabel(t('1-0'))).toBe('tool call');
  });
});

describe('buildCursorRowCopyText', () => {
  it('copies the user prompt text', () => {
    expect(buildCursorRowCopyText(u('0', 'add validation'))).toBe('add validation');
  });

  it('copies the assistant narrative source', () => {
    expect(buildCursorRowCopyText(a('1', '**bold** answer'))).toBe('**bold** answer');
  });

  it('copies the tool input summary', () => {
    expect(buildCursorRowCopyText(t('1-0', 'gh api dependabot/alerts'))).toBe(
      'gh api dependabot/alerts',
    );
  });

  it('returns undefined for an empty assistant payload', () => {
    expect(buildCursorRowCopyText(a('1', ''))).toBeUndefined();
  });

  it('returns undefined for a tool row with no input', () => {
    expect(buildCursorRowCopyText(t('1-0', undefined))).toBeUndefined();
  });
});
