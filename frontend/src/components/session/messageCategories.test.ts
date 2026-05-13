import { describe, it, expect } from 'vitest';
import { validateParsedTranscriptLine } from '@/schemas/transcript';
import { countHierarchicalCategories, messageMatchesFilter, DEFAULT_FILTER_STATE, getRoleLabel } from './messageCategories';
import type { FilterState } from './messageCategories';

function parseLine(obj: unknown) {
  const result = validateParsedTranscriptLine(obj, JSON.stringify(obj), 0);
  if (!result.success) throw new Error(`Validation failed: ${JSON.stringify(result.error)}`);
  return result.data;
}

const prLinkMessage = parseLine({
  type: 'pr-link',
  prNumber: 22,
  prRepository: 'ConfabulousDev/confab-web',
  prUrl: 'https://github.com/ConfabulousDev/confab-web/pull/22',
  sessionId: 'session-123',
  timestamp: '2026-02-22T08:00:41.865Z',
});

describe('messageCategories', () => {
  describe('countHierarchicalCategories', () => {
    it('counts pr-link messages', () => {
      const counts = countHierarchicalCategories([prLinkMessage]);
      expect(counts['pr-link']).toBe(1);
    });

    it('counts pr-link alongside other types', () => {
      const queueOp = parseLine({
        type: 'queue-operation',
        operation: 'enqueue',
        timestamp: '2026-02-22T08:00:00Z',
        sessionId: 'session-123',
      });

      const counts = countHierarchicalCategories([prLinkMessage, queueOp]);
      expect(counts['pr-link']).toBe(1);
      expect(counts['queue-operation']).toBe(1);
    });
  });

  describe('messageMatchesFilter', () => {
    it('hides pr-link by default', () => {
      expect(messageMatchesFilter(prLinkMessage, DEFAULT_FILTER_STATE)).toBe(false);
    });

    it('shows pr-link when filter is enabled', () => {
      const filterState: FilterState = {
        ...DEFAULT_FILTER_STATE,
        'pr-link': true,
      };
      expect(messageMatchesFilter(prLinkMessage, filterState)).toBe(true);
    });

    it('hides system messages by default (deep-link filter reset scenario)', () => {
      const systemMessage = parseLine({
        type: 'system',
        uuid: 'sys-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        subtype: 'info',
        content: 'System info message',
      });
      expect(messageMatchesFilter(systemMessage, DEFAULT_FILTER_STATE)).toBe(false);
    });

    it('shows system messages after filter reset with system enabled', () => {
      const systemMessage = parseLine({
        type: 'system',
        uuid: 'sys-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        subtype: 'info',
        content: 'System info message',
      });
      const resetState: FilterState = { ...DEFAULT_FILTER_STATE, system: true };
      expect(messageMatchesFilter(systemMessage, resetState)).toBe(true);
    });

    it('shows user messages with DEFAULT_FILTER_STATE (deep-link targets visible after reset)', () => {
      const userMessage = parseLine({
        type: 'user',
        uuid: 'user-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        message: { role: 'user', content: 'Hello' },
      });
      expect(messageMatchesFilter(userMessage, DEFAULT_FILTER_STATE)).toBe(true);
    });

    it('hides assistant messages with only empty thinking blocks', () => {
      const emptyThinkingMessage = parseLine({
        type: 'assistant',
        uuid: 'asst-uuid-empty',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        requestId: 'req-1',
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-empty',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'thinking', thinking: '', signature: 'abc123' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: { input_tokens: 100, output_tokens: 50 },
        },
      });
      expect(messageMatchesFilter(emptyThinkingMessage, DEFAULT_FILTER_STATE)).toBe(false);
    });

    it('shows assistant messages with non-empty thinking blocks', () => {
      const thinkingMessage = parseLine({
        type: 'assistant',
        uuid: 'asst-uuid-thinking',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        requestId: 'req-1',
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-thinking',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'thinking', thinking: 'Let me analyze this...', signature: 'abc123' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: { input_tokens: 100, output_tokens: 50 },
        },
      });
      expect(messageMatchesFilter(thinkingMessage, DEFAULT_FILTER_STATE)).toBe(true);
    });

    it('routes away_summary system rows to away-summary, not system', () => {
      const awaySummary = parseLine({
        type: 'system',
        uuid: 'sys-uuid-aw',
        timestamp: '2026-04-20T22:35:57.594Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/home/user/project',
        sessionId: 'session-1',
        version: '2.1.116',
        subtype: 'away_summary',
        content: 'You stepped away. Here is what changed.',
      });
      const counts = countHierarchicalCategories([awaySummary]);
      expect(counts['away-summary']).toBe(1);
      expect(counts.system).toBe(0);
    });

    it('routes attachment subtypes into their sub-buckets and parent total', () => {
      function attachmentLine(attachment: Record<string, unknown>, uuidSuffix: string) {
        return parseLine({
          type: 'attachment',
          uuid: `att-${uuidSuffix}`,
          timestamp: '2026-04-20T22:31:25.657Z',
          parentUuid: null,
          isSidechain: false,
          userType: 'external',
          cwd: '/home/user/project',
          sessionId: 'session-1',
          version: '2.1.140',
          attachment,
        });
      }

      const messages = [
        attachmentLine({ type: 'hook_success', hookName: 'h', hookEvent: 'SessionStart', toolUseID: 't', stdout: '', stderr: '', exitCode: 0, durationMs: 1 }, '1'),
        attachmentLine({ type: 'hook_blocking_error', hookName: 'h', hookEvent: 'PreToolUse', toolUseID: 't', blockingError: { blockingError: 'no', command: 'x' } }, '2'),
        attachmentLine({ type: 'edited_text_file', filename: '/a.md', snippet: '     1\tx' }, '3'),
        attachmentLine({ type: 'queued_command', prompt: 'hi', commandMode: 'prompt' }, '4'),
        attachmentLine({ type: 'deferred_tools_delta', addedNames: ['X'], removedNames: [] }, '5'),
        attachmentLine({ type: 'mcp_instructions_delta', addedNames: ['Y'], removedNames: [] }, '6'),
      ];
      const counts = countHierarchicalCategories(messages);
      expect(counts.attachment.hook).toBe(2);
      expect(counts.attachment['file-edit']).toBe(1);
      expect(counts.attachment['queued-command']).toBe(1);
      expect(counts.attachment['deferred-tools']).toBe(1);
      expect(counts.attachment['mcp-instructions']).toBe(1);
      expect(counts.attachment.total).toBe(6);
    });

    it('does not increment attachment.total for noisy/unknown subtypes', () => {
      function attachmentLine(attachment: Record<string, unknown>, uuidSuffix: string) {
        return parseLine({
          type: 'attachment',
          uuid: `att-${uuidSuffix}`,
          timestamp: '2026-04-20T22:31:25.657Z',
          parentUuid: null,
          isSidechain: false,
          userType: 'external',
          cwd: '/home/user/project',
          sessionId: 'session-1',
          version: '2.1.140',
          attachment,
        });
      }
      const messages = [
        attachmentLine({ type: 'task_reminder', content: 'r', itemCount: 1 }, '1'),
        attachmentLine({ type: 'skill_listing', content: 's', skillCount: 1, isInitial: true }, '2'),
        attachmentLine({ type: 'command_permissions', allowedTools: [] }, '3'),
        attachmentLine({ type: 'future_unknown', whatever: true }, '4'),
      ];
      const counts = countHierarchicalCategories(messages);
      expect(counts.attachment.total).toBe(0);
    });

    it('DEFAULT_FILTER_STATE hides all attachment subs and away-summary', () => {
      expect(DEFAULT_FILTER_STATE.attachment.hook).toBe(false);
      expect(DEFAULT_FILTER_STATE.attachment['file-edit']).toBe(false);
      expect(DEFAULT_FILTER_STATE.attachment['queued-command']).toBe(false);
      expect(DEFAULT_FILTER_STATE.attachment['deferred-tools']).toBe(false);
      expect(DEFAULT_FILTER_STATE.attachment['mcp-instructions']).toBe(false);
      expect(DEFAULT_FILTER_STATE['away-summary']).toBe(false);
    });

    it('messageMatchesFilter respects each attachment sub-chip independently', () => {
      const hookRow = parseLine({
        type: 'attachment',
        uuid: 'att-h',
        timestamp: '2026-04-20T22:31:25.657Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/home/user/project',
        sessionId: 'session-1',
        version: '2.1.140',
        attachment: { type: 'hook_success', hookName: 'h', hookEvent: 'SessionStart', toolUseID: 't', stdout: 'x', stderr: '', exitCode: 0, durationMs: 1 },
      });
      expect(messageMatchesFilter(hookRow, DEFAULT_FILTER_STATE)).toBe(false);
      const hookOn: FilterState = { ...DEFAULT_FILTER_STATE, attachment: { ...DEFAULT_FILTER_STATE.attachment, hook: true } };
      expect(messageMatchesFilter(hookRow, hookOn)).toBe(true);
      // file-edit on should NOT show a hook row
      const fileOn: FilterState = { ...DEFAULT_FILTER_STATE, attachment: { ...DEFAULT_FILTER_STATE.attachment, 'file-edit': true } };
      expect(messageMatchesFilter(hookRow, fileOn)).toBe(false);
    });

    it('messageMatchesFilter hides noisy attachment subtypes regardless of chip state', () => {
      const reminder = parseLine({
        type: 'attachment',
        uuid: 'att-r',
        timestamp: '2026-04-20T22:31:25.657Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/home/user/project',
        sessionId: 'session-1',
        version: '2.1.140',
        attachment: { type: 'task_reminder', content: 'r', itemCount: 1 },
      });
      const allOn: FilterState = {
        ...DEFAULT_FILTER_STATE,
        attachment: { hook: true, 'file-edit': true, 'queued-command': true, 'deferred-tools': true, 'mcp-instructions': true },
        'away-summary': true,
      };
      expect(messageMatchesFilter(reminder, allOn)).toBe(false);
    });

    it('messageMatchesFilter respects the away-summary chip', () => {
      const awaySummary = parseLine({
        type: 'system',
        uuid: 'sys-uuid-aw',
        timestamp: '2026-04-20T22:35:57.594Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/home/user/project',
        sessionId: 'session-1',
        version: '2.1.116',
        subtype: 'away_summary',
        content: 'Summary content',
      });
      expect(messageMatchesFilter(awaySummary, DEFAULT_FILTER_STATE)).toBe(false);
      // Enabling system alone should NOT show it (it's not a system match)
      const sysOn: FilterState = { ...DEFAULT_FILTER_STATE, system: true };
      expect(messageMatchesFilter(awaySummary, sysOn)).toBe(false);
      const awayOn: FilterState = { ...DEFAULT_FILTER_STATE, 'away-summary': true };
      expect(messageMatchesFilter(awaySummary, awayOn)).toBe(true);
    });

    it('getRoleLabel returns Attachment for attachment rows', () => {
      const attachmentRow = parseLine({
        type: 'attachment',
        uuid: 'att-h',
        timestamp: '2026-04-20T22:31:25.657Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/home/user/project',
        sessionId: 'session-1',
        version: '2.1.140',
        attachment: { type: 'hook_success', hookName: 'h', hookEvent: 'SessionStart', toolUseID: 't', stdout: '', stderr: '', exitCode: 0, durationMs: 1 },
      });
      expect(getRoleLabel(attachmentRow)).toBe('Attachment');
    });

    it('getRoleLabel returns Resume Summary for away_summary system rows', () => {
      const awaySummary = parseLine({
        type: 'system',
        uuid: 'sys-uuid-aw',
        timestamp: '2026-04-20T22:35:57.594Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/home/user/project',
        sessionId: 'session-1',
        version: '2.1.116',
        subtype: 'away_summary',
        content: 'Summary',
      });
      expect(getRoleLabel(awaySummary)).toBe('Resume Summary');
    });

    it('shows assistant messages with DEFAULT_FILTER_STATE (deep-link targets visible after reset)', () => {
      const assistantMessage = parseLine({
        type: 'assistant',
        uuid: 'asst-uuid-1',
        timestamp: '2026-02-22T08:00:00Z',
        parentUuid: null,
        isSidechain: false,
        userType: 'external',
        cwd: '/test',
        sessionId: 'session-123',
        version: '1.0.0',
        requestId: 'req-1',
        message: {
          model: 'claude-sonnet-4-20250514',
          id: 'msg-1',
          type: 'message',
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello!' }],
          stop_reason: 'end_turn',
          stop_sequence: null,
          usage: { input_tokens: 100, output_tokens: 50 },
        },
      });
      expect(messageMatchesFilter(assistantMessage, DEFAULT_FILTER_STATE)).toBe(true);
    });
  });
});
