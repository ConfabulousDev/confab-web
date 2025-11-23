import { useState, useEffect, useMemo } from 'react';
import type { RunDetail, TranscriptLine, AgentNode } from '@/types';
import { fetchParsedTranscript } from '@/services/transcriptService';
import { buildAgentTree } from '@/services/agentTreeBuilder';
import MessageList from './MessageList';
import styles from './TranscriptViewer.module.css';

interface TranscriptViewerProps {
  run: RunDetail;
  shareToken?: string;
  sessionId?: string;
}

function TranscriptViewer({ run, shareToken, sessionId }: TranscriptViewerProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>([]);
  const [agents, setAgents] = useState<AgentNode[]>([]);
  const [expanded, setExpanded] = useState(true);
  const [showThinking, setShowThinking] = useState(true);

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

  // Expand/collapse all controls
  const [expandAllAgents, setExpandAllAgents] = useState(true);
  const [expandAllTools, setExpandAllTools] = useState(false);
  const [expandAllResults, setExpandAllResults] = useState(true);

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
      const transcriptFile = run.files.find((f) => f.file_type === 'transcript');
      if (!transcriptFile) {
        throw new Error('No transcript file found');
      }

      // Fetch and parse transcript
      // Use provided sessionId or fall back to transcript_path (for non-shared views)
      const effectiveSessionId = sessionId || run.transcript_path;

      const t1 = performance.now();
      const parsed = await fetchParsedTranscript(run.id, transcriptFile.id, effectiveSessionId, shareToken);
      const t2 = performance.now();
      console.log(`⏱️ fetchParsedTranscript took ${Math.round(t2 - t1)}ms`);

      // Build agent tree
      const shareOptions = shareToken && sessionId ? { sessionId, shareToken } : undefined;
      const t3 = performance.now();
      const agentTree = await buildAgentTree(run.id, parsed.messages, run.files, shareOptions);
      const t4 = performance.now();
      console.log(`⏱️ buildAgentTree took ${Math.round(t4 - t3)}ms`);

      setAgents(agentTree);

      const total = performance.now() - t0;
      console.log(`⏱️ Data load complete: ${Math.round(total)}ms`, {
        messageCount: parsed.messages.length,
        agentCount: agentTree.length,
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

  function toggleExpanded() {
    setExpanded(!expanded);
  }

  function toggleExpandAllAgents() {
    setExpandAllAgents(!expandAllAgents);
  }

  function toggleExpandAllTools() {
    setExpandAllTools(!expandAllTools);
  }

  function toggleExpandAllResults() {
    setExpandAllResults(!expandAllResults);
  }

  return (
    <div className={styles.transcriptViewer}>
      <div className={styles.transcriptHeader}>
        <h3>Transcript</h3>
        <div className={styles.headerControls}>
          <button
            className={`${styles.toggleBtn} ${styles.thinkingToggle} ${showThinking ? styles.active : ''}`}
            onClick={() => setShowThinking(!showThinking)}
            title={showThinking ? 'Hide thinking blocks' : 'Show thinking blocks'}
          >
            💭 {showThinking ? 'Hide' : 'Show'} Thinking
          </button>
          <button
            className={styles.toggleBtn}
            onClick={toggleExpandAllAgents}
            title={expandAllAgents ? 'Collapse all agents' : 'Expand all agents'}
          >
            🤖 {expandAllAgents ? 'Collapse' : 'Expand'} Agents
          </button>
          <button
            className={styles.toggleBtn}
            onClick={toggleExpandAllTools}
            title={expandAllTools ? 'Collapse all tool blocks' : 'Expand all tool blocks'}
          >
            🛠️ {expandAllTools ? 'Collapse' : 'Expand'} Tools
          </button>
          <button
            className={styles.toggleBtn}
            onClick={toggleExpandAllResults}
            title={expandAllResults ? 'Collapse all results' : 'Expand all results'}
          >
            ✅ {expandAllResults ? 'Collapse' : 'Expand'} Results
          </button>
          <button className={styles.toggleBtn} onClick={toggleExpanded}>
            {expanded ? 'Collapse' : 'Expand'} All
          </button>
        </div>
      </div>

      {loading ? (
        <div className={styles.loading}>Loading transcript...</div>
      ) : error ? (
        <div className={styles.error}>
          <strong>Error:</strong> {error}
        </div>
      ) : expanded ? (
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
            <div className={styles.transcriptMeta}>
              <span>{messages.length} messages</span>
              {agents.length > 0 && (
                <span>
                  {agents.length} agent{agents.length === 1 ? '' : 's'}
                </span>
              )}
            </div>
          )}

          <MessageList
            messages={messages}
            agents={agents}
            run={run}
            showThinking={showThinking}
            expandAllAgents={expandAllAgents}
            expandAllTools={expandAllTools}
            expandAllResults={expandAllResults}
          />
        </div>
      ) : null}
    </div>
  );
}

export default TranscriptViewer;
