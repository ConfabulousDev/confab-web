import type { GitInfo } from '@/types';
import { useCopyToClipboard } from '@/hooks';
import { formatDuration, formatDateTime, formatModelName } from '@/utils/formatting';
import type { MessageCategory, MessageCategoryCounts } from './messageCategories';
import MetaItem from './MetaItem';
import GitInfoMeta from './GitInfoMeta';
import FilterDropdown from './FilterDropdown';
import styles from './SessionHeader.module.css';

// SVG Icons
const CopyIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
  </svg>
);

const CheckIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <polyline points="20 6 9 17 4 12" />
  </svg>
);

const ModelIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect x="2" y="6" width="20" height="12" rx="2" />
    <path d="M12 12h.01" />
    <path d="M17 12h.01" />
    <path d="M7 12h.01" />
  </svg>
);

const DurationIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10" />
    <polyline points="12 6 12 12 16 14" />
  </svg>
);

const CalendarIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
    <line x1="16" y1="2" x2="16" y2="6" />
    <line x1="8" y1="2" x2="8" y2="6" />
    <line x1="3" y1="10" x2="21" y2="10" />
  </svg>
);

const ShareIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
    <polyline points="16 6 12 2 8 6" />
    <line x1="12" y1="2" x2="12" y2="15" />
  </svg>
);

interface SessionHeaderProps {
  title?: string;
  externalId: string;
  model?: string;
  durationMs?: number;
  sessionDate?: Date;
  gitInfo?: GitInfo | null;
  onShare?: () => void;
  onDelete?: () => void;
  isOwner?: boolean;
  isShared?: boolean;
  // Filter props
  categoryCounts: MessageCategoryCounts;
  visibleCategories: Set<MessageCategory>;
  onToggleCategory: (category: MessageCategory) => void;
}

function SessionHeader({
  title,
  externalId,
  model,
  durationMs,
  sessionDate,
  gitInfo,
  onShare,
  onDelete,
  isOwner = true,
  isShared = false,
  categoryCounts,
  visibleCategories,
  onToggleCategory,
}: SessionHeaderProps) {
  const { copy, copied } = useCopyToClipboard();
  const displayTitle = title || `Session ${externalId.substring(0, 8)}`;

  return (
    <header className={styles.header}>
      <div className={styles.titleSection}>
        <div className={styles.titleRow}>
          <h1 className={styles.title}>{displayTitle}</h1>
          <button
            className={styles.copyIdBtn}
            onClick={() => copy(externalId)}
            title={copied ? 'Copied!' : 'Copy Claude Code session id'}
            aria-label="Copy Claude Code session id"
          >
            {copied ? CheckIcon : CopyIcon}
          </button>
        </div>
        <div className={styles.metadata}>
          <GitInfoMeta gitInfo={gitInfo} />
          {model && (
            <MetaItem icon={ModelIcon} value={formatModelName(model)} />
          )}
          {durationMs !== undefined && durationMs > 0 && (
            <MetaItem icon={DurationIcon} value={formatDuration(durationMs)} />
          )}
          {sessionDate && (
            <MetaItem icon={CalendarIcon} value={formatDateTime(sessionDate)} />
          )}
        </div>
      </div>

      <div className={styles.actions}>
        <FilterDropdown
          counts={categoryCounts}
          visibleCategories={visibleCategories}
          onToggleCategory={onToggleCategory}
        />
        {isShared ? (
          <div className={styles.sharedIndicator}>
            {ShareIcon}
            <span>Shared Session</span>
          </div>
        ) : isOwner && (
          <>
            {onShare && (
              <button className={styles.btnShare} onClick={onShare}>
                Share
              </button>
            )}
            {onDelete && (
              <button className={styles.btnDelete} onClick={onDelete}>
                Delete
              </button>
            )}
          </>
        )}
      </div>
    </header>
  );
}

export default SessionHeader;
