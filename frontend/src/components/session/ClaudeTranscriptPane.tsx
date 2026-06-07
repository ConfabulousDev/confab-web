// Renders the transcript-tab content for Claude Code sessions.
//
// Thin wrapper around ClaudeMessageTimeline: receives messages, filter results, and
// cost-mode state from SessionViewer (which holds the state so the header can
// render the filter chips and cost toggle alongside the timeline). Encapsulates
// the loading / error / empty / timeline branching so the parent shell stays
// focused on routing.

import type { TranscriptLine } from '@/types';
import ClaudeMessageTimeline from '@/components/transcript/claude/ClaudeMessageTimeline';
import TranscriptPaneStatus from './TranscriptPaneStatus';

export interface ClaudeTranscriptPaneProps {
  loading: boolean;
  error: string | null;
  filteredMessages: TranscriptLine[];
  allMessages: TranscriptLine[];
  sessionId: string;
  targetMessageUuid?: string;
  isCostMode: boolean;
}

export default function ClaudeTranscriptPane({
  loading,
  error,
  filteredMessages,
  allMessages,
  sessionId,
  targetMessageUuid,
  isCostMode,
}: ClaudeTranscriptPaneProps) {
  if (loading || error) {
    return <TranscriptPaneStatus loading={loading} error={error} />;
  }

  return (
    <ClaudeMessageTimeline
      messages={filteredMessages}
      allMessages={allMessages}
      targetMessageUuid={targetMessageUuid}
      sessionId={sessionId}
      isCostMode={isCostMode}
    />
  );
}
