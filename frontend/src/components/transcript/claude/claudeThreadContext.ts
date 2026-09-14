// et0r: subagent-thread context for the Claude timeline.
//
// Provided once by ClaudeMessageTimeline so row renderers (ClaudeTimelineMessage)
// and nested attachment renderers (QueuedCommand) can render subagent cards
// without prop-drilling through AttachmentContent. The default value has an
// empty index, so rows rendered outside a timeline never show cards.

import { createContext, useContext } from 'react';
import { buildClaudeAgentIndex, type ClaudeAgentIndex } from './claudeAgentIndex';

export interface ClaudeThreadContextValue {
  agentIndex: ClaudeAgentIndex;
  /** Present only when the host can switch threads (adapter `threads` capability). */
  onOpenThread?: (threadId: string) => void;
  /** The subagent thread being rendered; null = Main. */
  activeThreadId: string | null;
}

export const ClaudeThreadContext = createContext<ClaudeThreadContextValue>({
  agentIndex: buildClaudeAgentIndex([]),
  activeThreadId: null,
});

export function useClaudeThread(): ClaudeThreadContextValue {
  return useContext(ClaudeThreadContext);
}
