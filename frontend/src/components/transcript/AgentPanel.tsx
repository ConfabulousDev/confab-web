import type { AgentNode, RunDetail } from '@/types';
import MessageList from './MessageList';
import styles from './AgentPanel.module.css';

interface AgentPanelProps {
  agent: AgentNode;
  run: RunDetail;
  depth?: number;
}

function AgentPanel({ agent, run, depth = 0 }: AgentPanelProps) {
  // Get color based on depth
  const colors = ['#007bff', '#6f42c1', '#28a745', '#fd7e14', '#dc3545', '#17a2b8'];
  const borderColor = colors[depth % colors.length];

  // Calculate indentation (20px per level, max 100px)
  const indentation = Math.min(depth * 20, 100);

  // Determine if this is a deeply nested agent (depth > 3)
  const isDeeplyNested = depth > 3;

  return (
    <div
      className={`${styles.agentPanel} ${isDeeplyNested ? styles.deeplyNested : ''}`}
      style={{ marginLeft: `${indentation}px`, borderLeftColor: borderColor }}
    >
      <div className={styles.agentHeader}>
        <div className={styles.agentInfo}>
          <span className={styles.agentIcon}>🤖</span>
          {depth > 0 && (
            <span className={styles.depthIndicator} title={`Nesting level ${depth}`}>
              L{depth}
            </span>
          )}
          <span className={styles.agentLabel}>Agent: {agent.agentId}</span>
          <span className={styles.messageCount}>{agent.transcript.length} messages</span>
          {agent.children.length > 0 && (
            <span className={styles.childCount} title={`${agent.children.length} sub-agent(s)`}>
              {agent.children.length} sub-agent{agent.children.length === 1 ? '' : 's'}
            </span>
          )}
          {agent.metadata.status && (
            <span className={`${styles.agentStatus} ${styles[`status-${agent.metadata.status}`]}`}>
              {agent.metadata.status}
            </span>
          )}
        </div>
      </div>

      <div className={styles.agentContent}>
        {agent.metadata.totalDurationMs && (
          <div className={styles.agentMeta}>
            <span>Duration: {(agent.metadata.totalDurationMs / 1000).toFixed(1)}s</span>
            {agent.metadata.totalTokens && <span>Tokens: {agent.metadata.totalTokens.toLocaleString()}</span>}
            {agent.metadata.totalToolUseCount && <span>Tools: {agent.metadata.totalToolUseCount}</span>}
          </div>
        )}

        <MessageList
          messages={agent.transcript}
          agents={agent.children}
          run={run}
        />

        {/* Recursively render child agents */}
        {agent.children.length > 0 && (
          <div className={styles.childAgents}>
            {agent.children.map((child, i) => (
              <AgentPanel
                key={i}
                agent={child}
                run={run}
                depth={depth + 1}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export default AgentPanel;
