import { describe, it, expect } from 'vitest';
import type {
  CodexRenderItem,
  CodexUserItem,
  CodexAssistantItem,
  CodexToolCallItem,
  CodexReasoningHiddenItem,
  CodexCompactedItem,
  CodexTurnSeparatorItem,
  CodexUnknownItem,
} from '@/types/codexRenderItem';
import {
  categorizeCodexToolCall,
  countCodexCategories,
  codexItemMatchesFilter,
  DEFAULT_CODEX_FILTER_STATE,
} from './codexCategories';
import type { CodexFilterState } from './codexCategories';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const TS = '2026-05-15T00:00:00.000Z';

const userItem: CodexUserItem = {
  kind: 'user',
  lineId: '0',
  timestamp: TS,
  text: 'hello',
};

const assistantCommentary: CodexAssistantItem = {
  kind: 'assistant',
  lineId: '1',
  timestamp: TS,
  text: 'thinking aloud',
  phase: 'commentary',
  model: 'gpt-5',
};

const assistantFinal: CodexAssistantItem = {
  kind: 'assistant',
  lineId: '2',
  timestamp: TS,
  text: 'final answer',
  phase: 'final',
  model: 'gpt-5',
};

function toolCall(toolName: string, lineId = '3'): CodexToolCallItem {
  return {
    kind: 'tool_call',
    lineId,
    timestamp: TS,
    toolName,
    callId: `call-${toolName}-${lineId}`,
    rawInput: {},
    status: 'completed',
  };
}

const reasoningHidden: CodexReasoningHiddenItem = {
  kind: 'reasoning_hidden',
  lineId: '10',
  timestamp: TS,
};

const compacted: CodexCompactedItem = {
  kind: 'compacted',
  lineId: '11',
  timestamp: TS,
  replacementCount: 3,
};

const turnSeparator: CodexTurnSeparatorItem = {
  kind: 'turn_separator',
  lineId: '12',
  timestamp: TS,
  turnIndex: 1,
  durationMs: 1000,
};

const unknown: CodexUnknownItem = {
  kind: 'unknown',
  lineId: '13',
  timestamp: TS,
  rawLine: {},
};

// ---------------------------------------------------------------------------
// categorizeCodexToolCall
// ---------------------------------------------------------------------------

describe('categorizeCodexToolCall', () => {
  it('buckets exec_command toolName as exec_command', () => {
    expect(categorizeCodexToolCall(toolCall('exec_command'))).toBe('exec_command');
  });

  it('buckets apply_patch toolName as apply_patch', () => {
    expect(categorizeCodexToolCall(toolCall('apply_patch'))).toBe('apply_patch');
  });

  it('buckets web_search_call toolName as web_search', () => {
    expect(categorizeCodexToolCall(toolCall('web_search_call'))).toBe('web_search');
  });

  it('buckets MCP tool names as generic', () => {
    expect(categorizeCodexToolCall(toolCall('mcp__github__create_issue'))).toBe('generic');
  });

  it('buckets shell as generic (no special-case alias)', () => {
    expect(categorizeCodexToolCall(toolCall('shell'))).toBe('generic');
  });

  it('buckets local_shell as generic', () => {
    expect(categorizeCodexToolCall(toolCall('local_shell'))).toBe('generic');
  });

  it('buckets arbitrary unknown tool names as generic', () => {
    expect(categorizeCodexToolCall(toolCall('something_new_v2'))).toBe('generic');
  });
});

// ---------------------------------------------------------------------------
// countCodexCategories
// ---------------------------------------------------------------------------

describe('countCodexCategories', () => {
  it('returns all-zero counts for an empty array', () => {
    const counts = countCodexCategories([]);
    expect(counts.user).toBe(0);
    expect(counts.assistant.total).toBe(0);
    expect(counts.assistant.commentary).toBe(0);
    expect(counts.assistant.final).toBe(0);
    expect(counts.tool_call.total).toBe(0);
    expect(counts.tool_call.exec_command).toBe(0);
    expect(counts.tool_call.apply_patch).toBe(0);
    expect(counts.tool_call.web_search).toBe(0);
    expect(counts.tool_call.generic).toBe(0);
    expect(counts.reasoning_hidden).toBe(0);
    expect(counts.compacted).toBe(0);
    expect(counts.turn_separator).toBe(0);
    expect(counts.unknown).toBe(0);
  });

  it('counts one of each kind across a mixed fixture', () => {
    const items: CodexRenderItem[] = [
      userItem,
      assistantCommentary,
      assistantFinal,
      toolCall('exec_command', '3'),
      toolCall('apply_patch', '4'),
      toolCall('web_search_call', '5'),
      toolCall('mcp__github__create_issue', '6'),
      reasoningHidden,
      compacted,
      turnSeparator,
      unknown,
    ];
    const counts = countCodexCategories(items);
    expect(counts.user).toBe(1);
    expect(counts.assistant.total).toBe(2);
    expect(counts.assistant.commentary).toBe(1);
    expect(counts.assistant.final).toBe(1);
    expect(counts.tool_call.total).toBe(4);
    expect(counts.tool_call.exec_command).toBe(1);
    expect(counts.tool_call.apply_patch).toBe(1);
    expect(counts.tool_call.web_search).toBe(1);
    expect(counts.tool_call.generic).toBe(1);
    expect(counts.reasoning_hidden).toBe(1);
    expect(counts.compacted).toBe(1);
    expect(counts.turn_separator).toBe(1);
    expect(counts.unknown).toBe(1);
  });

  it('distinguishes assistant commentary from final', () => {
    const items: CodexRenderItem[] = [assistantCommentary, assistantCommentary, assistantFinal];
    const counts = countCodexCategories(items);
    expect(counts.assistant.commentary).toBe(2);
    expect(counts.assistant.final).toBe(1);
    expect(counts.assistant.total).toBe(3);
  });

  it('sums multiple users into the flat user count', () => {
    const items: CodexRenderItem[] = [userItem, { ...userItem, lineId: '1' }, { ...userItem, lineId: '2' }];
    expect(countCodexCategories(items).user).toBe(3);
  });

  it('sums multiple tool_call subs independently', () => {
    const items: CodexRenderItem[] = [
      toolCall('exec_command', '3'),
      toolCall('exec_command', '4'),
      toolCall('apply_patch', '5'),
    ];
    const counts = countCodexCategories(items);
    expect(counts.tool_call.exec_command).toBe(2);
    expect(counts.tool_call.apply_patch).toBe(1);
    expect(counts.tool_call.total).toBe(3);
  });
});

// ---------------------------------------------------------------------------
// codexItemMatchesFilter
// ---------------------------------------------------------------------------

describe('codexItemMatchesFilter', () => {
  describe('with DEFAULT_CODEX_FILTER_STATE', () => {
    it('shows user items', () => {
      expect(codexItemMatchesFilter(userItem, DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows assistant commentary', () => {
      expect(codexItemMatchesFilter(assistantCommentary, DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows assistant final', () => {
      expect(codexItemMatchesFilter(assistantFinal, DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows exec_command tool calls', () => {
      expect(codexItemMatchesFilter(toolCall('exec_command'), DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows apply_patch tool calls', () => {
      expect(codexItemMatchesFilter(toolCall('apply_patch'), DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows web_search tool calls', () => {
      expect(codexItemMatchesFilter(toolCall('web_search_call'), DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows generic tool calls', () => {
      expect(codexItemMatchesFilter(toolCall('mcp__github__create_issue'), DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows compacted dividers', () => {
      expect(codexItemMatchesFilter(compacted, DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows turn_separator dividers', () => {
      expect(codexItemMatchesFilter(turnSeparator, DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('shows unknown items', () => {
      expect(codexItemMatchesFilter(unknown, DEFAULT_CODEX_FILTER_STATE)).toBe(true);
    });
    it('HIDES reasoning_hidden (the only default-hidden category)', () => {
      expect(codexItemMatchesFilter(reasoningHidden, DEFAULT_CODEX_FILTER_STATE)).toBe(false);
    });
  });

  describe('with all-false filter state', () => {
    const allFalse: CodexFilterState = {
      user: false,
      assistant: { commentary: false, final: false },
      tool_call: { exec_command: false, apply_patch: false, web_search: false, generic: false },
      reasoning_hidden: false,
      compacted: false,
      turn_separator: false,
      turn_aborted: false,
      unknown: false,
    };
    it.each<[string, CodexRenderItem]>([
      ['user', userItem],
      ['assistant commentary', assistantCommentary],
      ['assistant final', assistantFinal],
      ['exec_command', toolCall('exec_command')],
      ['apply_patch', toolCall('apply_patch')],
      ['web_search', toolCall('web_search_call')],
      ['generic', toolCall('mcp__github__create_issue')],
      ['reasoning_hidden', reasoningHidden],
      ['compacted', compacted],
      ['turn_separator', turnSeparator],
      ['unknown', unknown],
    ])('hides every variant (%s)', (_label, item) => {
      expect(codexItemMatchesFilter(item, allFalse)).toBe(false);
    });
  });

  describe('subcategory independence', () => {
    it('hiding assistant.commentary keeps assistant.final visible', () => {
      const state: CodexFilterState = {
        ...DEFAULT_CODEX_FILTER_STATE,
        assistant: { commentary: false, final: true },
      };
      expect(codexItemMatchesFilter(assistantCommentary, state)).toBe(false);
      expect(codexItemMatchesFilter(assistantFinal, state)).toBe(true);
    });

    it('hiding tool_call.exec_command keeps tool_call.apply_patch visible', () => {
      const state: CodexFilterState = {
        ...DEFAULT_CODEX_FILTER_STATE,
        tool_call: { exec_command: false, apply_patch: true, web_search: true, generic: true },
      };
      expect(codexItemMatchesFilter(toolCall('exec_command'), state)).toBe(false);
      expect(codexItemMatchesFilter(toolCall('apply_patch'), state)).toBe(true);
      expect(codexItemMatchesFilter(toolCall('web_search_call'), state)).toBe(true);
      expect(codexItemMatchesFilter(toolCall('mcp__github__create_issue'), state)).toBe(true);
    });
  });

  describe('opting into hidden defaults', () => {
    it('shows reasoning_hidden when the chip is enabled', () => {
      const state: CodexFilterState = {
        ...DEFAULT_CODEX_FILTER_STATE,
        reasoning_hidden: true,
      };
      expect(codexItemMatchesFilter(reasoningHidden, state)).toBe(true);
    });
  });

  describe('flat categories', () => {
    it('respects the unknown flag', () => {
      const state: CodexFilterState = { ...DEFAULT_CODEX_FILTER_STATE, unknown: false };
      expect(codexItemMatchesFilter(unknown, state)).toBe(false);
    });
    it('respects the turn_separator flag', () => {
      const state: CodexFilterState = { ...DEFAULT_CODEX_FILTER_STATE, turn_separator: false };
      expect(codexItemMatchesFilter(turnSeparator, state)).toBe(false);
    });
    it('respects the compacted flag', () => {
      const state: CodexFilterState = { ...DEFAULT_CODEX_FILTER_STATE, compacted: false };
      expect(codexItemMatchesFilter(compacted, state)).toBe(false);
    });
    it('respects the user flag', () => {
      const state: CodexFilterState = { ...DEFAULT_CODEX_FILTER_STATE, user: false };
      expect(codexItemMatchesFilter(userItem, state)).toBe(false);
    });
  });
});
