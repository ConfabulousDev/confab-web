import { useState, useEffect, useMemo } from 'react';
import type { SessionDetail, TranscriptLine, AgentNode } from '@/types';
import { fetchParsedTranscript, type TranscriptValidationError } from '@/services/transcriptService';
import { buildAgentTree } from '@/services/agentTreeBuilder';
import MessageList from './MessageList';
import ValidationErrorsPanel from './ValidationErrorsPanel';
import styles from './TranscriptViewer.module.css';

interface TranscriptViewerProps {
  session: SessionDetail;
  shareToken?: string;
}

function TranscriptViewer({ session, shareToken }: TranscriptViewerProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>([]);
  const [agents, setAgents] = useState<AgentNode[]>([]);
  const [validationErrors, setValidationErrors] = useState<TranscriptValidationError[]>([]);

  // Batched rendering state
  const [renderingBatch, setRenderingBatch] = useState(false);
  const [renderProgress, setRenderProgress] = useState({ current: 0, total: 0 });

  // Computed progress
  const progressPercent = useMemo(
    () => (renderProgress.total > 0 ? Math.round((renderProgress.current / renderProgress.total) * 100) : 0),
    [renderProgress]
  );

  const progressWidth = useMemo(
    () => (renderProgress.total > 0 ? (renderProgress.current / renderProgress.total) * 100 : 0),
    [renderProgress]
  );

  useEffect(() => {
    loadTranscript();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function loadTranscript() {
    setLoading(true);
    setError(null);

    const t0 = performance.now();
    try {
      // Find transcript file
      const transcriptFile = session.files.find((f) => f.file_type === 'transcript');
      if (!transcriptFile) {
        throw new Error('No transcript file found');
      }

      // Fetch and parse transcript using session ID and file name
      const t1 = performance.now();
      const parsed = await fetchParsedTranscript(session.id, transcriptFile.file_name, shareToken);
      const t2 = performance.now();
      console.log(`⏱️ fetchParsedTranscript took ${Math.round(t2 - t1)}ms`);

      // Build agent tree
      const t3 = performance.now();
      const agentTree = await buildAgentTree(session.id, parsed.messages, session.files, shareToken);
      const t4 = performance.now();
      console.log(`⏱️ buildAgentTree took ${Math.round(t4 - t3)}ms`);

      setAgents(agentTree);
      setValidationErrors(parsed.validationErrors);

      const total = performance.now() - t0;
      console.log(`⏱️ Data load complete: ${Math.round(total)}ms`, {
        messageCount: parsed.messages.length,
        agentCount: agentTree.length,
        validationErrorCount: parsed.validationErrors.length,
      });

      // Start batched rendering
      setLoading(false);
      if (parsed.messages.length > 0) {
        setRenderingBatch(true);
        setRenderProgress({ current: 0, total: parsed.messages.length });
        // Start rendering after state update
        setTimeout(() => renderNextBatch([], parsed.messages), 0);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load transcript');
      console.error('Failed to load transcript:', e);
      setLoading(false);
    }
  }

  function renderNextBatch(currentMessages: TranscriptLine[], allMsgs: TranscriptLine[]) {
    const BATCH_SIZE = 100;
    const start = currentMessages.length;
    const end = Math.min(start + BATCH_SIZE, allMsgs.length);

    // Add next batch of messages
    const newMessages = [...currentMessages, ...allMsgs.slice(start, end)];
    setMessages(newMessages);
    setRenderProgress({ current: end, total: allMsgs.length });

    // Continue if there are more messages
    if (end < allMsgs.length) {
      // Slightly longer delay to allow UI updates
      requestAnimationFrame(() => {
        setTimeout(() => renderNextBatch(newMessages, allMsgs), 10);
      });
    } else {
      // Rendering complete - delay slightly to show 100% completion
      setTimeout(() => {
        setRenderingBatch(false);
        console.log(`⏱️ Rendering complete: ${newMessages.length} messages`);
      }, 300);
    }
  }

  return (
    <div className={styles.transcriptViewer}>
      {loading ? (
        <div className={styles.loading}>Loading transcript...</div>
      ) : error ? (
        <div className={styles.error}>
          <strong>Error:</strong> {error}
        </div>
      ) : (
        <div className={styles.transcriptContent}>
          {renderingBatch ? (
            <div className={styles.renderingProgress}>
              <div className={styles.progressText}>
                Rendering messages: {renderProgress.current.toLocaleString()} / {renderProgress.total.toLocaleString()} (
                {progressPercent}%)
              </div>
              <div className={styles.progressBar}>
                <div className={styles.progressFill} style={{ width: `${progressWidth}%` }}></div>
              </div>
            </div>
          ) : (
            <>
              <div className={styles.transcriptMeta}>
                <span>{messages.length} messages</span>
                {agents.length > 0 && (
                  <span>
                    {agents.length} agent{agents.length === 1 ? '' : 's'}
                  </span>
                )}
                {validationErrors.length > 0 && (
                  <span className={styles.errorCount}>
                    {validationErrors.length} parse error{validationErrors.length === 1 ? '' : 's'}
                  </span>
                )}
              </div>

              {validationErrors.length > 0 && (
                <ValidationErrorsPanel errors={validationErrors} />
              )}
            </>
          )}

          <MessageList
            messages={messages}
            agents={agents}
            session={session}
          />
        </div>
      )}
    </div>
  );
}

export default TranscriptViewer;
