import type { SessionDetail, SyncFileDetail } from '@/types';
import { getRepoWebURL, getCommitURL } from '@/utils';
import { useTodos, useToggleSet } from '@/hooks';
import TranscriptViewer from './transcript/TranscriptViewer';
import styles from './SessionCard.module.css';

interface SessionCardProps {
  session: SessionDetail;
  showGitInfo?: boolean;
  shareToken?: string;
}

function SessionCard({ session, showGitInfo = true, shareToken }: SessionCardProps) {
  const { todos } = useTodos({ session, shareToken });
  const expandedFiles = useToggleSet<string>();

  return (
    <div className={styles.sessionCard}>
      <div className={styles.sessionHeader}>
        <div className={styles.headerLeft}>
          <h3>Session Details</h3>
        </div>
      </div>

      <div className={styles.sessionInfo}>
        {session.cwd && (
          <div className={styles.infoRow}>
            <span className={styles.label}>Working Directory:</span>
            <code className={styles.value}>{session.cwd}</code>
          </div>
        )}
        {session.transcript_path && (
          <div className={styles.infoRow}>
            <span className={styles.label}>Transcript:</span>
            <code className={styles.value}>{session.transcript_path}</code>
          </div>
        )}
      </div>

      {showGitInfo && session.git_info && (
        <div className={styles.gitInfoSection}>
          <h4>Git Information</h4>
          <div className={styles.gitInfo}>
            {session.git_info.repo_url && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Repository:</span>
                {getRepoWebURL(session.git_info.repo_url) ? (
                  <a
                    href={getRepoWebURL(session.git_info.repo_url)!}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`${styles.value} ${styles.link}`}
                  >
                    {session.git_info.repo_url}
                  </a>
                ) : (
                  <code className={styles.value}>{session.git_info.repo_url}</code>
                )}
              </div>
            )}

            {session.git_info.branch && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Branch:</span>
                <code className={styles.value}>{session.git_info.branch}</code>
                {session.git_info.is_dirty && <span className={styles.dirtyBadge}>Uncommitted changes</span>}
              </div>
            )}

            {session.git_info.commit_sha && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Commit:</span>
                {getCommitURL(session.git_info) ? (
                  <a
                    href={getCommitURL(session.git_info)!}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`${styles.value} ${styles.link}`}
                  >
                    <code>{session.git_info.commit_sha.substring(0, 7)}</code>
                  </a>
                ) : (
                  <code className={styles.value}>{session.git_info.commit_sha.substring(0, 7)}</code>
                )}
              </div>
            )}

            {session.git_info.commit_message && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Message:</span>
                <span className={styles.value}>{session.git_info.commit_message}</span>
              </div>
            )}

            {session.git_info.author && (
              <div className={styles.infoRow}>
                <span className={styles.label}>Author:</span>
                <span className={styles.value}>{session.git_info.author}</span>
              </div>
            )}
          </div>
        </div>
      )}

      {session.files && session.files.length > 0 && (
        <div className={styles.filesSection}>
          <h4>Files ({session.files.length})</h4>
          <div className={styles.filesList}>
            {session.files.map((file: SyncFileDetail) => {
              const isExpandable = file.file_type === 'transcript';
              const isExpanded = expandedFiles.has(file.file_name);

              return (
                <div key={file.file_name} className={styles.fileItemWrapper}>
                  <div
                    className={`${styles.fileItem} ${isExpandable ? styles.expandable : ''} ${isExpanded ? styles.expanded : ''}`}
                    onClick={isExpandable ? () => expandedFiles.toggle(file.file_name) : undefined}
                  >
                    <div className={styles.fileInfo}>
                      {isExpandable && (
                        <span className={styles.expandIcon}>{isExpanded ? '▼' : '▶'}</span>
                      )}
                      <span className={`${styles.fileType} ${styles[file.file_type]}`}>{file.file_type}</span>
                      <code className={styles.filePath}>{file.file_name}</code>
                    </div>
                    <span className={styles.fileLines}>{file.last_synced_line} lines</span>
                  </div>
                  {isExpanded && (
                    <div className={styles.fileContent}>
                      <TranscriptViewer session={session} shareToken={shareToken} />
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

export default SessionCard;
