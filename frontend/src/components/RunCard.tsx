import { useState } from 'react';
import type { RunDetail } from '@/types';
import { formatBytes, getRepoWebURL, getCommitURL } from '@/utils';
import { useTodos } from '@/hooks';
import TranscriptViewer from './transcript/TranscriptViewer';
import styles from './RunCard.module.css';

interface RunCardProps {
  run: RunDetail;
  showGitInfo?: boolean;
  shareToken?: string;
  sessionId?: string;
}

function RunCard({ run, showGitInfo = true, shareToken, sessionId }: RunCardProps) {
  const { todos } = useTodos({ run, shareToken, sessionId });
  const [expandedFiles, setExpandedFiles] = useState<Set<number>>(new Set());

  function toggleFileExpanded(fileId: number) {
    setExpandedFiles((prev) => {
      const next = new Set(prev);
      if (next.has(fileId)) {
        next.delete(fileId);
      } else {
        next.add(fileId);
      }
      return next;
    });
  }

  return (
    <div className={styles.runCard}>
      <div className={styles.runHeader}>
        <div className={styles.headerLeft}>
          <h3>Session Details</h3>
        </div>
      </div>

      <div className={styles.runInfo}>
        <div className={styles.infoRow}>
          <span className={styles.label}>Working Directory:</span>
          <code className={styles.value}>{run.cwd}</code>
        </div>
        <div className={styles.infoRow}>
          <span className={styles.label}>End Reason:</span>
          <span className={styles.value}>{run.reason}</span>
        </div>
        <div className={styles.infoRow}>
          <span className={styles.label}>Transcript:</span>
          <code className={styles.value}>{run.transcript_path}</code>
        </div>
      </div>

      {showGitInfo && run.git_info && (
        <div className={styles.gitInfoSection}>
          <h4>Git Information</h4>
          <div className={styles.gitInfo}>
            {run.git_info.repo_url && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Repository:</span>
                {getRepoWebURL(run.git_info.repo_url) ? (
                  <a
                    href={getRepoWebURL(run.git_info.repo_url)!}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`${styles.value} ${styles.link}`}
                  >
                    {run.git_info.repo_url}
                  </a>
                ) : (
                  <code className={styles.value}>{run.git_info.repo_url}</code>
                )}
              </div>
            )}

            {run.git_info.branch && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Branch:</span>
                <code className={styles.value}>{run.git_info.branch}</code>
                {run.git_info.is_dirty && <span className={styles.dirtyBadge}>⚠ Uncommitted changes</span>}
              </div>
            )}

            {run.git_info.commit_sha && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Commit:</span>
                {getCommitURL(run.git_info) ? (
                  <a
                    href={getCommitURL(run.git_info)!}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`${styles.value} ${styles.link}`}
                  >
                    <code>{run.git_info.commit_sha.substring(0, 7)}</code>
                  </a>
                ) : (
                  <code className={styles.value}>{run.git_info.commit_sha.substring(0, 7)}</code>
                )}
              </div>
            )}

            {run.git_info.commit_message && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Message:</span>
                <span className={styles.value}>{run.git_info.commit_message}</span>
              </div>
            )}

            {run.git_info.author && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Author:</span>
                <span className={styles.value}>{run.git_info.author}</span>
              </div>
            )}
          </div>
        </div>
      )}

      {run.files && run.files.length > 0 && (
        <div className={styles.filesSection}>
          <h4>Files ({run.files.length})</h4>
          <div className={styles.filesList}>
            {run.files.map((file) => {
              const isExpandable = file.file_type === 'transcript';
              const isExpanded = expandedFiles.has(file.id);

              return (
                <div key={file.id} className={styles.fileItemWrapper}>
                  <div
                    className={`${styles.fileItem} ${isExpandable ? styles.expandable : ''} ${isExpanded ? styles.expanded : ''}`}
                    onClick={isExpandable ? () => toggleFileExpanded(file.id) : undefined}
                  >
                    <div className={styles.fileInfo}>
                      {isExpandable && (
                        <span className={styles.expandIcon}>{isExpanded ? '▼' : '▶'}</span>
                      )}
                      <span className={`${styles.fileType} ${styles[file.file_type]}`}>{file.file_type}</span>
                      <code className={styles.filePath}>{file.file_path}</code>
                    </div>
                    <span className={styles.fileSize}>{formatBytes(file.size_bytes)}</span>
                  </div>
                  {isExpanded && (
                    <div className={styles.fileContent}>
                      <TranscriptViewer run={run} shareToken={shareToken} sessionId={sessionId} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {todos.length > 0 && (
        <div className={styles.todosSection}>
          <h4>Todo Lists ({todos.length})</h4>
          {todos.map((todoGroup, i) => (
            <div key={i} className={styles.todoGroup}>
              <h5>Agent: {todoGroup.agent_id}</h5>
              <div className={styles.todoList}>
                {todoGroup.items.map((item, j) => (
                  <div key={j} className={`${styles.todoItem} ${styles[`status-${item.status}`]}`}>
                    <span className={styles.todoStatusIcon}>
                      {item.status === 'completed' ? '✓' : item.status === 'in_progress' ? '⟳' : '○'}
                    </span>
                    <span className={styles.todoContent}>{item.content}</span>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

    </div>
  );
}

export default RunCard;
