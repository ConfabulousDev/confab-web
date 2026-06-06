import { describe, expect, it } from 'vitest';
import {
  countOpenCodeCategories,
  opencodeItemMatchesFilter,
  DEFAULT_OPENCODE_FILTER_STATE,
  type OpenCodeRenderItem,
  type OpenCodeFilterState,
} from './opencodeCategories';

const TS = 1717689600000;

const userItem: OpenCodeRenderItem = {
  kind: 'user',
  id: 'msg_01',
  text: 'Find all Go files',
  timeCreated: TS,
};

const assistantItem: OpenCodeRenderItem = {
  kind: 'assistant',
  id: 'msg_02',
  text: 'I found 2 files.',
  reasoning: 'Let me check...',
  timeCreated: TS + 10000,
};

const toolItem: OpenCodeRenderItem = {
  kind: 'tool',
  id: 'prt_01',
  toolName: 'Bash',
  status: 'completed',
  timeCreated: TS + 5000,
};

describe('countOpenCodeCategories', () => {
  it('counts user messages', () => {
    const counts = countOpenCodeCategories([userItem]);
    expect(counts.user).toBe(1);
    expect(counts.assistant).toBe(0);
    expect(counts.tool).toBe(0);
  });

  it('counts assistant messages', () => {
    const counts = countOpenCodeCategories([assistantItem]);
    expect(counts.user).toBe(0);
    expect(counts.assistant).toBe(1);
    expect(counts.tool).toBe(0);
  });

  it('counts tool calls', () => {
    const counts = countOpenCodeCategories([toolItem]);
    expect(counts.user).toBe(0);
    expect(counts.assistant).toBe(0);
    expect(counts.tool).toBe(1);
  });

  it('counts mixed items correctly', () => {
    const counts = countOpenCodeCategories([userItem, assistantItem, toolItem, toolItem]);
    expect(counts.user).toBe(1);
    expect(counts.assistant).toBe(1);
    expect(counts.tool).toBe(2);
  });

  it('returns all zeros for empty array', () => {
    const counts = countOpenCodeCategories([]);
    expect(counts.user).toBe(0);
    expect(counts.assistant).toBe(0);
    expect(counts.tool).toBe(0);
  });
});

describe('opencodeItemMatchesFilter', () => {
  const allVisible: OpenCodeFilterState = { user: true, assistant: true, tool: true };
  const allHidden: OpenCodeFilterState = { user: false, assistant: false, tool: false };

  it('shows all items when all filters are on', () => {
    expect(opencodeItemMatchesFilter(userItem, allVisible)).toBe(true);
    expect(opencodeItemMatchesFilter(assistantItem, allVisible)).toBe(true);
    expect(opencodeItemMatchesFilter(toolItem, allVisible)).toBe(true);
  });

  it('hides all items when all filters are off', () => {
    expect(opencodeItemMatchesFilter(userItem, allHidden)).toBe(false);
    expect(opencodeItemMatchesFilter(assistantItem, allHidden)).toBe(false);
    expect(opencodeItemMatchesFilter(toolItem, allHidden)).toBe(false);
  });

  it('hides user messages when user filter is off', () => {
    const state: OpenCodeFilterState = { ...DEFAULT_OPENCODE_FILTER_STATE, user: false };
    expect(opencodeItemMatchesFilter(userItem, state)).toBe(false);
    expect(opencodeItemMatchesFilter(assistantItem, state)).toBe(true);
  });

  it('hides assistant messages when assistant filter is off', () => {
    const state: OpenCodeFilterState = { ...DEFAULT_OPENCODE_FILTER_STATE, assistant: false };
    expect(opencodeItemMatchesFilter(assistantItem, state)).toBe(false);
    expect(opencodeItemMatchesFilter(userItem, state)).toBe(true);
  });

  it('hides tool calls when tool filter is off', () => {
    const state: OpenCodeFilterState = { ...DEFAULT_OPENCODE_FILTER_STATE, tool: false };
    expect(opencodeItemMatchesFilter(toolItem, state)).toBe(false);
    expect(opencodeItemMatchesFilter(userItem, state)).toBe(true);
  });
});
