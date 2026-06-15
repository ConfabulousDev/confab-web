import { describe, it, expect } from 'vitest';
import {
  DEFAULT_CLAUDE_FILTER_STATE,
  type ClaudeFilterState,
} from '@/components/session/claudeCategories';
import {
  DEFAULT_HIDDEN,
  pathsFromState,
  stateFromPaths,
} from './useClaudeTranscriptFilters';

describe('Claude transcript filter paths', () => {
  it('DEFAULT_HIDDEN derives from DEFAULT_CLAUDE_FILTER_STATE', () => {
    expect(stateFromPaths(DEFAULT_HIDDEN)).toEqual(DEFAULT_CLAUDE_FILTER_STATE);
  });

  it('emits a dot-path for each hidden subcategory and flat category', () => {
    const state: ClaudeFilterState = {
      ...DEFAULT_CLAUDE_FILTER_STATE,
      user: { ...DEFAULT_CLAUDE_FILTER_STATE.user, prompt: false },
      system: false,
    };
    const paths = pathsFromState(state);
    expect(paths).toContain('user.prompt');
    expect(paths).toContain('system');
    expect(paths).not.toContain('user.skill');
  });

  it('treats foreign tokens as no-ops (cross-provider URLs)', () => {
    // 'tool_call.exec_command' belongs to Codex; ignored on the Claude side.
    expect(stateFromPaths(['tool_call.exec_command'])).toEqual(
      stateFromPaths([]),
    );
  });

  it('round-trips: stateFromPaths(pathsFromState(s)) === s', () => {
    const samples: ClaudeFilterState[] = [
      DEFAULT_CLAUDE_FILTER_STATE,
      {
        user: { prompt: false, 'tool-result': true, skill: false },
        assistant: { text: true, 'tool-use': false, thinking: true },
        attachment: {
          hook: false,
          'file-edit': true,
          'queued-command': false,
          'deferred-tools': true,
          'mcp-instructions': false,
        },
        system: false,
        'file-history-snapshot': true,
        summary: false,
        'queue-operation': true,
        'pr-link': false,
        'away-summary': true,
        unknown: false,
      },
    ];
    for (const s of samples) {
      expect(stateFromPaths(pathsFromState(s))).toEqual(s);
    }
  });
});
