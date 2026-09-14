// et0r: pure Claude subagent index.
//
// Links each Agent/Task tool call in a transcript stream to the subagent that
// ran it (`toolUseResult.agentId`) and to that subagent's own JSONL file, and
// folds in `<task-notification>` completions. Works on any stream — the main
// transcript or a subagent's file (nested agents use the same wire shape).
//
// The CLI uploads classic subagent transcripts flat as `agent-<id>.jsonl`;
// `agentFileName` is the single frontend owner of that naming contract. Names
// are constructed from ids, never parsed. Workflow-run subagents
// (`subagents/workflows/<runId>/…`) have no per-agent tool_result in the parent
// stream and are intentionally not indexed.

import { z } from 'zod';
import type { TranscriptLine } from '@/types';
import {
  isAssistantMessage,
  isAttachmentMessage,
  isQueuedCommandAttachment,
  isToolResultBlock,
  isToolUseBlock,
  isUserMessage,
} from '@/types';
import type { TranscriptThreadStatus } from '@/providers/types';

/**
 * we3k D3: raw index / notification status → the shared display vocabulary used
 * by the thread strip, its dropdowns and `SubagentCard`. Real notifications
 * carry `completed`, `failed` and `killed`; the index adds `running` / `error`.
 */
export function normalizeAgentStatus(status: string | undefined): TranscriptThreadStatus {
  switch (status) {
    case 'running':
    case 'completed':
      return status;
    case 'error':
    case 'failed':
      return 'failed';
    case 'killed':
      return 'stopped';
    default:
      return 'unknown';
  }
}

/** `Agent` is current; `Task` is the legacy name (backend `isAgentToolName` accepts both). */
const AGENT_TOOL_NAMES = new Set(['Agent', 'Task']);

const AgentToolInputSchema = z.object({
  description: z.string().optional(),
  subagent_type: z.string().optional(),
  model: z.string().optional(),
});

// Narrow view of the Agent tool's `toolUseResult`. The transcript schema's
// union parses it as passthrough Bash/record data, so read it here instead of
// widening that union.
const AgentToolUseResultSchema = z.object({
  agentId: z.string().min(1),
  status: z.string().optional(),
  description: z.string().optional(),
  resolvedModel: z.string().optional(),
  totalDurationMs: z.number().optional(),
  totalTokens: z.number().optional(),
  totalToolUseCount: z.number().optional(),
});

export interface ClaudeAgentInfo {
  agentId: string;
  /** The launching Agent/Task `tool_use` id. */
  toolUseId: string;
  /** uuid of the user line carrying the tool_result (the launch row). */
  resultMessageUuid: string;
  description?: string;
  subagentType?: string;
  model?: string;
  /**
   * `running` (background launch), `completed`, `error` (tool_result
   * `is_error`), or the `<status>` of the latest task notification.
   */
  status: string;
  totalDurationMs?: number;
  totalTokens?: number;
  totalToolUseCount?: number;
  fileName: string;
  lastNotificationUuid?: string;
}

export interface ClaudeAgentIndex {
  /** Keyed by agentId, in launch order. */
  agents: ReadonlyMap<string, ClaudeAgentInfo>;
  /** Keyed by the launching tool_use id. */
  byToolUseId: ReadonlyMap<string, ClaudeAgentInfo>;
}

interface TaskNotification {
  taskId: string;
  toolUseId?: string;
  status?: string;
  summary?: string;
  result?: string;
}

export function agentFileName(agentId: string): string {
  return `agent-${agentId}.jsonl`;
}

/** Display name for an agent: description → subagent_type → agentId. */
export function agentDisplayName(agent: ClaudeAgentInfo): string {
  return agent.description || agent.subagentType || agent.agentId;
}

const NOTIFICATION_OPEN = '<task-notification>';

function tagPattern(tag: string): RegExp {
  return new RegExp(`<${tag}>([\\s\\S]*?)</${tag}>`);
}

const TASK_ID_RE = tagPattern('task-id');
const TOOL_USE_ID_RE = tagPattern('tool-use-id');
const STATUS_RE = tagPattern('status');
const SUMMARY_RE = tagPattern('summary');
// Greedy: the agent's final text can itself contain XML-looking tags.
const RESULT_RE = /<result>([\s\S]*)<\/result>/;

function readTag(text: string, re: RegExp): string | undefined {
  const value = re.exec(text)?.[1]?.trim();
  return value ? value : undefined;
}

/** Parse `<task-notification>` XML. Returns null for anything else or when there is no task id. */
export function parseTaskNotification(text: string): TaskNotification | null {
  if (!text.trimStart().startsWith(NOTIFICATION_OPEN)) return null;
  // Header tags are read from before <result> so agent output can't spoof them.
  const resultStart = text.indexOf('<result>');
  const head = resultStart >= 0 ? text.slice(0, resultStart) : text;
  const taskId = readTag(head, TASK_ID_RE);
  if (!taskId) return null;
  return {
    taskId,
    toolUseId: readTag(head, TOOL_USE_ID_RE),
    status: readTag(head, STATUS_RE),
    summary: readTag(head, SUMMARY_RE),
    result: readTag(text, RESULT_RE),
  };
}

/**
 * The raw `<task-notification>` text carried by a line: plain user string
 * content, or a `queued_command` attachment in `task-notification` mode.
 */
export function getTaskNotificationText(message: TranscriptLine): string | null {
  if (isUserMessage(message)) {
    const content = message.message.content;
    return typeof content === 'string' && content.trimStart().startsWith(NOTIFICATION_OPEN) ? content : null;
  }
  if (
    isAttachmentMessage(message) &&
    isQueuedCommandAttachment(message) &&
    message.attachment.commandMode === 'task-notification'
  ) {
    return message.attachment.prompt;
  }
  return null;
}

/**
 * The indexed agent a notification refers to, if any. Notifications also fire
 * for background Bash commands; those ids never match an agent.
 */
export function findNotifiedAgent(
  index: ClaudeAgentIndex,
  notification: TaskNotification,
): ClaudeAgentInfo | undefined {
  return (
    index.agents.get(notification.taskId) ??
    (notification.toolUseId ? index.byToolUseId.get(notification.toolUseId) : undefined)
  );
}

function launchStatus(isError: boolean | undefined, resultStatus: string | undefined): string {
  if (isError) return 'error';
  if (resultStatus === 'async_launched') return 'running';
  return resultStatus ?? 'completed';
}

export function buildClaudeAgentIndex(messages: readonly TranscriptLine[]): ClaudeAgentIndex {
  const agents = new Map<string, ClaudeAgentInfo>();
  const byToolUseId = new Map<string, ClaudeAgentInfo>();
  const index: ClaudeAgentIndex = { agents, byToolUseId };

  const launches = new Map<string, z.infer<typeof AgentToolInputSchema>>();
  for (const message of messages) {
    if (!isAssistantMessage(message)) continue;
    for (const block of message.message.content) {
      if (isToolUseBlock(block) && AGENT_TOOL_NAMES.has(block.name)) {
        const input = AgentToolInputSchema.safeParse(block.input);
        launches.set(block.id, input.success ? input.data : {});
      }
    }
  }
  if (launches.size === 0) return index;

  for (const message of messages) {
    if (isUserMessage(message) && Array.isArray(message.message.content)) {
      const result = AgentToolUseResultSchema.safeParse(message.toolUseResult);
      if (result.success && !agents.has(result.data.agentId)) {
        const block = message.message.content
          .filter(isToolResultBlock)
          .find((b) => launches.has(b.tool_use_id));
        if (block) {
          const tur = result.data;
          const input = launches.get(block.tool_use_id) ?? {};
          const info: ClaudeAgentInfo = {
            agentId: tur.agentId,
            toolUseId: block.tool_use_id,
            resultMessageUuid: message.uuid,
            description: tur.description || input.description,
            subagentType: input.subagent_type,
            model: tur.resolvedModel || input.model,
            status: launchStatus(block.is_error, tur.status),
            totalDurationMs: tur.totalDurationMs,
            totalTokens: tur.totalTokens,
            totalToolUseCount: tur.totalToolUseCount,
            fileName: agentFileName(tur.agentId),
          };
          agents.set(info.agentId, info);
          byToolUseId.set(info.toolUseId, info);
        }
      }
    }

    const text = getTaskNotificationText(message);
    const notification = text ? parseTaskNotification(text) : null;
    const notified = notification ? findNotifiedAgent(index, notification) : undefined;
    if (notification && notified && 'uuid' in message && typeof message.uuid === 'string') {
      if (notification.status) notified.status = notification.status;
      notified.lastNotificationUuid = message.uuid;
    }
  }

  return index;
}
