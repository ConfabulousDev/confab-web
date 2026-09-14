// et0r: Claude subagent wire-shape builders for tests and stories.
//
// Shapes mirror real Claude Code JSONL (verified against local transcripts):
//   - Agent tool_use: assistant `tool_use` block, `name: "Agent"` (legacy
//     "Task"), `input: {description, prompt, subagent_type?}`.
//   - Async (background) result: user line with a tool_result block + sibling
//     `toolUseResult = {isAsync, status:"async_launched", agentId, description,
//     resolvedModel, prompt, outputFile, canReadOutputFile}`.
//   - Sync (foreground) result: `toolUseResult = {status:"completed", prompt,
//     agentId, content, totalDurationMs, totalTokens, totalToolUseCount}` — no
//     description / model (those come from the paired tool_use input).
//   - Completion: `<task-notification>` XML as a user string message, or as a
//     `queued_command` attachment prompt with `commandMode:"task-notification"`.
// All ids/text are fictional.

import type { AssistantMessage, AttachmentMessage, UserMessage } from '@/types';

const BASE = {
  parentUuid: null,
  isSidechain: false,
  userType: 'external',
  cwd: '/Users/dev/project',
  sessionId: 'fixture-session',
  version: '2.1.270',
} as const;

interface AgentToolUseOptions {
  uuid: string;
  toolUseId: string;
  description?: string;
  subagentType?: string;
  model?: string;
  name?: string;
  timestamp?: string;
  /** Set on lines that live inside a subagent's own JSONL. */
  agentId?: string;
}

export function agentToolUse({
  uuid,
  toolUseId,
  description,
  subagentType,
  model,
  name = 'Agent',
  timestamp = '2026-09-13T10:00:00Z',
  agentId,
}: AgentToolUseOptions): AssistantMessage {
  const input: Record<string, unknown> = { prompt: `Do the work for ${description ?? toolUseId}` };
  if (description !== undefined) input.description = description;
  if (subagentType !== undefined) input.subagent_type = subagentType;
  if (model !== undefined) input.model = model;
  return {
    ...BASE,
    type: 'assistant',
    uuid,
    timestamp,
    requestId: `req-${uuid}`,
    ...(agentId ? { agentId, isSidechain: true } : {}),
    message: {
      model: 'claude-opus-5',
      id: `msg-${uuid}`,
      type: 'message',
      role: 'assistant',
      content: [{ type: 'tool_use', id: toolUseId, name, input }],
      stop_reason: 'tool_use',
      stop_sequence: null,
      usage: { input_tokens: 10, output_tokens: 5 },
    },
  };
}

interface AsyncResultOptions {
  uuid: string;
  toolUseId: string;
  agentId: string;
  description?: string;
  resolvedModel?: string;
  timestamp?: string;
}

export function asyncAgentResult({
  uuid,
  toolUseId,
  agentId,
  description = 'Explore the codebase',
  resolvedModel = 'claude-opus-5',
  timestamp = '2026-09-13T10:00:01Z',
}: AsyncResultOptions): UserMessage {
  return {
    ...BASE,
    type: 'user',
    uuid,
    timestamp,
    message: {
      role: 'user',
      content: [
        {
          type: 'tool_result',
          tool_use_id: toolUseId,
          content: [
            {
              type: 'text',
              text: `Async agent launched successfully.\nagentId: ${agentId}\nThe agent is working in the background.`,
            },
          ],
        },
      ],
    },
    toolUseResult: {
      isAsync: true,
      status: 'async_launched',
      agentId,
      description,
      resolvedModel,
      prompt: 'Do the work',
      outputFile: `/tmp/tasks/${agentId}.output`,
      canReadOutputFile: true,
    },
  };
}

interface SyncResultOptions {
  uuid: string;
  toolUseId: string;
  agentId: string;
  isError?: boolean;
  text?: string;
  totalDurationMs?: number;
  totalTokens?: number;
  totalToolUseCount?: number;
  timestamp?: string;
}

export function syncAgentResult({
  uuid,
  toolUseId,
  agentId,
  isError = false,
  text = 'Found 3 call sites.',
  totalDurationMs = 42_000,
  totalTokens = 12_345,
  totalToolUseCount = 7,
  timestamp = '2026-09-13T10:01:00Z',
}: SyncResultOptions): UserMessage {
  return {
    ...BASE,
    type: 'user',
    uuid,
    timestamp,
    message: {
      role: 'user',
      content: [
        {
          type: 'tool_result',
          tool_use_id: toolUseId,
          content: [{ type: 'text', text }],
          ...(isError ? { is_error: true } : {}),
        },
      ],
    },
    toolUseResult: {
      status: 'completed',
      prompt: 'Do the work',
      agentId,
      content: [{ type: 'text', text }],
      totalDurationMs,
      totalTokens,
      totalToolUseCount,
    },
  };
}

interface NotificationOptions {
  taskId: string;
  toolUseId?: string;
  status?: string;
  summary?: string;
  result?: string;
}

export function taskNotificationXml({
  taskId,
  toolUseId = 'toolu_fixture',
  status = 'completed',
  summary = 'Agent "Explore the codebase" finished',
  result = 'All done.',
}: NotificationOptions): string {
  return (
    '<task-notification>\n' +
    `<task-id>${taskId}</task-id>\n` +
    `<tool-use-id>${toolUseId}</tool-use-id>\n` +
    `<output-file>/tmp/tasks/${taskId}.output</output-file>\n` +
    `<status>${status}</status>\n` +
    `<summary>${summary}</summary>\n` +
    '<note>A task-notification fires each time this agent stops.</note>\n' +
    `<result>${result}</result>\n` +
    '</task-notification>'
  );
}

export function taskNotificationMessage(
  uuid: string,
  options: NotificationOptions,
  timestamp = '2026-09-13T10:05:00Z',
): UserMessage {
  return {
    ...BASE,
    type: 'user',
    uuid,
    timestamp,
    message: { role: 'user', content: taskNotificationXml(options) },
  };
}

export function queuedTaskNotification(
  uuid: string,
  options: NotificationOptions,
  timestamp = '2026-09-13T10:05:00Z',
): AttachmentMessage {
  return {
    ...BASE,
    type: 'attachment',
    uuid,
    timestamp,
    attachment: {
      type: 'queued_command',
      prompt: taskNotificationXml(options),
      commandMode: 'task-notification',
    },
  };
}

/** A plain line inside a subagent's own JSONL (`isSidechain` + `agentId`). */
export function subagentAssistantText(
  uuid: string,
  agentId: string,
  text: string,
  timestamp = '2026-09-13T10:00:10Z',
): AssistantMessage {
  return {
    ...BASE,
    type: 'assistant',
    uuid,
    timestamp,
    isSidechain: true,
    agentId,
    requestId: `req-${uuid}`,
    message: {
      model: 'claude-opus-5',
      id: `msg-${uuid}`,
      type: 'message',
      role: 'assistant',
      content: [{ type: 'text', text }],
      stop_reason: 'end_turn',
      stop_sequence: null,
      usage: { input_tokens: 10, output_tokens: 5 },
    },
  };
}
