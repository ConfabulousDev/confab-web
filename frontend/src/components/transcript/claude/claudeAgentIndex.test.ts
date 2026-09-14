// et0r: pure Claude subagent index — links Agent/Task tool calls to their
// subagent transcript files and tracks completion notifications.

import { describe, it, expect } from 'vitest';
import type { TranscriptLine } from '@/types';
import {
  agentToolUse,
  asyncAgentResult,
  syncAgentResult,
  taskNotificationMessage,
  queuedTaskNotification,
  taskNotificationXml,
  subagentAssistantText,
} from '@/test-fixtures/claudeSubagent';
import {
  agentFileName,
  buildClaudeAgentIndex,
  getTaskNotificationText,
  normalizeAgentStatus,
  parseTaskNotification,
} from './claudeAgentIndex';

// we3k D3: one status vocabulary for the thread strip, dropdowns and SubagentCard.
describe('normalizeAgentStatus', () => {
  it.each([
    ['running', 'running'],
    ['completed', 'completed'],
    ['error', 'failed'],
    ['failed', 'failed'],
    ['killed', 'stopped'],
  ])('maps %s to %s', (raw, expected) => {
    expect(normalizeAgentStatus(raw)).toBe(expected);
  });

  it('maps unrecognized, empty or missing statuses to unknown', () => {
    expect(normalizeAgentStatus('paused')).toBe('unknown');
    expect(normalizeAgentStatus('')).toBe('unknown');
    expect(normalizeAgentStatus(undefined)).toBe('unknown');
  });
});

describe('agentFileName', () => {
  it('constructs the CLI naming contract agent-<id>.jsonl', () => {
    expect(agentFileName('a9f2b8d95dea9eced')).toBe('agent-a9f2b8d95dea9eced.jsonl');
  });
});

describe('parseTaskNotification', () => {
  it('extracts task id, tool use id, status, summary and result', () => {
    const parsed = parseTaskNotification(
      taskNotificationXml({
        taskId: 'a1',
        toolUseId: 'toolu_1',
        status: 'completed',
        summary: 'Agent "Explore" finished',
        result: 'Line one\n<b>kept</b>',
      }),
    );
    expect(parsed).toEqual({
      taskId: 'a1',
      toolUseId: 'toolu_1',
      status: 'completed',
      summary: 'Agent "Explore" finished',
      result: 'Line one\n<b>kept</b>',
    });
  });

  it('returns null for text that is not a task notification', () => {
    expect(parseTaskNotification('hello <task-id>x</task-id>')).toBeNull();
    expect(parseTaskNotification('')).toBeNull();
  });

  it('returns null when the notification carries no task id', () => {
    expect(parseTaskNotification('<task-notification>\n<status>completed</status>\n</task-notification>')).toBeNull();
  });
});

describe('getTaskNotificationText', () => {
  it('reads plain user string content', () => {
    const msg = taskNotificationMessage('n1', { taskId: 'a1' });
    expect(getTaskNotificationText(msg)).toContain('<task-id>a1</task-id>');
  });

  it('reads a queued_command attachment in task-notification mode', () => {
    const msg = queuedTaskNotification('n1', { taskId: 'a1' });
    expect(getTaskNotificationText(msg)).toContain('<task-id>a1</task-id>');
  });

  it('ignores ordinary user prompts', () => {
    const msg = subagentAssistantText('x', 'a1', 'hi');
    expect(getTaskNotificationText(msg)).toBeNull();
  });
});

describe('buildClaudeAgentIndex', () => {
  it('indexes an async (background) Agent launch with description, type, model and running status', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1', description: 'Explore', subagentType: 'Explore' }),
      asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1', description: 'Explore', resolvedModel: 'claude-opus-5' }),
    ]);
    const info = index.agents.get('a1');
    expect(info).toMatchObject({
      agentId: 'a1',
      toolUseId: 't1',
      resultMessageUuid: 'r1',
      description: 'Explore',
      subagentType: 'Explore',
      model: 'claude-opus-5',
      status: 'running',
      fileName: 'agent-a1.jsonl',
    });
    expect(index.byToolUseId.get('t1')).toBe(info);
  });

  it('indexes a sync (foreground) result, taking description/model from the tool_use input and keeping stats', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1', description: 'Find callers', subagentType: 'general-purpose', model: 'sonnet' }),
      syncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1', totalDurationMs: 1000, totalTokens: 200, totalToolUseCount: 3 }),
    ]);
    expect(index.agents.get('a1')).toMatchObject({
      description: 'Find callers',
      subagentType: 'general-purpose',
      model: 'sonnet',
      status: 'completed',
      totalDurationMs: 1000,
      totalTokens: 200,
      totalToolUseCount: 3,
    });
  });

  it('marks an is_error tool_result as error', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1', description: 'Broken' }),
      syncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1', isError: true }),
    ]);
    expect(index.agents.get('a1')?.status).toBe('error');
  });

  it('accepts the legacy Task tool name', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1', name: 'Task', description: 'Legacy' }),
      syncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' }),
    ]);
    expect(index.agents.get('a1')?.description).toBe('Legacy');
  });

  it('ignores an Agent tool_result without an agentId', () => {
    const result = syncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' });
    const withoutId = { ...result, toolUseResult: { status: 'completed' } };
    const index = buildClaudeAgentIndex([agentToolUse({ uuid: 'u1', toolUseId: 't1' }), withoutId]);
    expect(index.agents.size).toBe(0);
  });

  it('ignores a result whose tool_use is not an Agent/Task call (or is missing)', () => {
    const bash = agentToolUse({ uuid: 'u1', toolUseId: 't1', name: 'Bash' });
    const index = buildClaudeAgentIndex([
      bash,
      syncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' }),
      syncAgentResult({ uuid: 'r2', toolUseId: 'missing', agentId: 'a2' }),
    ]);
    expect(index.agents.size).toBe(0);
  });

  it('preserves launch order', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1', description: 'First' }),
      agentToolUse({ uuid: 'u2', toolUseId: 't2', description: 'Second' }),
      asyncAgentResult({ uuid: 'r2', toolUseId: 't2', agentId: 'a2' }),
      asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' }),
    ]);
    // Launch order = order results arrive (agent ids only exist once the result lands).
    expect([...index.agents.keys()]).toEqual(['a2', 'a1']);
  });

  it('updates status from a <task-notification> user string and records its uuid', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1' }),
      asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' }),
      taskNotificationMessage('n1', { taskId: 'a1', status: 'completed' }),
    ]);
    expect(index.agents.get('a1')).toMatchObject({ status: 'completed', lastNotificationUuid: 'n1' });
  });

  it('updates status from a queued_command task-notification attachment', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1' }),
      asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' }),
      queuedTaskNotification('n1', { taskId: 'a1', status: 'failed' }),
    ]);
    expect(index.agents.get('a1')).toMatchObject({ status: 'failed', lastNotificationUuid: 'n1' });
  });

  it('keeps the latest of repeated notifications (resumed agent)', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1' }),
      asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' }),
      taskNotificationMessage('n1', { taskId: 'a1', status: 'completed' }),
      taskNotificationMessage('n2', { taskId: 'a1', status: 'killed' }),
    ]);
    expect(index.agents.get('a1')).toMatchObject({ status: 'killed', lastNotificationUuid: 'n2' });
  });

  it('ignores notifications for unknown ids (e.g. background Bash commands)', () => {
    const index = buildClaudeAgentIndex([
      agentToolUse({ uuid: 'u1', toolUseId: 't1' }),
      asyncAgentResult({ uuid: 'r1', toolUseId: 't1', agentId: 'a1' }),
      taskNotificationMessage('n1', { taskId: 'bash-task', toolUseId: 'toolu_other', status: 'completed' }),
    ]);
    expect(index.agents.size).toBe(1);
    expect(index.agents.get('a1')).toMatchObject({ status: 'running' });
    expect(index.agents.get('a1')?.lastNotificationUuid).toBeUndefined();
  });

  it('indexes nested agents launched inside a subagent stream', () => {
    const stream: TranscriptLine[] = [
      subagentAssistantText('s1', 'parent', 'Spawning a helper'),
      agentToolUse({ uuid: 's2', toolUseId: 'tn', description: 'Nested helper', agentId: 'parent' }),
      asyncAgentResult({ uuid: 's3', toolUseId: 'tn', agentId: 'child' }),
    ];
    const index = buildClaudeAgentIndex(stream);
    expect(index.agents.get('child')).toMatchObject({ resultMessageUuid: 's3', fileName: 'agent-child.jsonl' });
  });

  it('returns an empty index for an empty stream', () => {
    const index = buildClaudeAgentIndex([]);
    expect(index.agents.size).toBe(0);
    expect(index.byToolUseId.size).toBe(0);
  });
});
