import { useState } from 'react';
import { Link } from 'react-router-dom';
import type { GitInfo, SessionDetail } from '@/types';
import { useCopyToClipboard } from '@/hooks';
import { formatDuration, formatDateTime, formatModelName } from '@/utils/formatting';
import { sessionsAPI } from '@/services/api';
import type {
  MessageCategory,
  UserSubcategory,
  AssistantSubcategory,
  HierarchicalCounts,
  FilterState,
} from './messageCategories';
import MetaItem from './MetaItem';
import GitInfoMeta from './GitInfoMeta';
import FilterDropdown from './FilterDropdown';
import styles from './SessionHeader.module.css';

const MAX_CUSTOM_TITLE_LENGTH = 255;

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
    {/* Antenna */}
    <line x1="12" y1="2" x2="12" y2="6" />
    <circle cx="12" cy="2" r="1" fill="currentColor" />
    {/* Head */}
    <rect x="4" y="6" width="16" height="14" rx="2" />
    {/* Eyes */}
    <circle cx="9" cy="13" r="2" />
    <circle cx="15" cy="13" r="2" />
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

const EditIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
    <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
  </svg>
);

const CloseIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="18" y1="6" x2="6" y2="18" />
    <line x1="6" y1="6" x2="18" y2="18" />
  </svg>
);

// NOTE: This component has a large prop list (18 props). This is acceptable for now
// as it serves as the main session header orchestrating multiple concerns. If this
// grows further or filter-related props are needed elsewhere, consider:
// - Extracting filter state to a React context (SessionFilterContext)
// - Using a compound component pattern for the header sections
// - Grouping related props into objects (e.g., filterProps: { counts, visible, onToggle })
interface SessionHeaderProps {
  sessionId: string;
  title?: string;
  hasCustomTitle?: boolean;
  autoTitle?: string; // The auto-derived title (summary || first_user_message)
  externalId: string;
  model?: string;
  durationMs?: number;
  sessionDate?: Date;
  gitInfo?: GitInfo | null;
  onShare?: () => void;
  onDelete?: () => void;
  onSessionUpdate?: (session: SessionDetail) => void;
  isOwner?: boolean;
  isShared?: boolean;
  sharedByEmail?: string | null; // Email of session owner (for non-owner access)
  // Filter props - optional, only shown on transcript tab
  // Currently drilled from SessionViewer -> SessionHeader -> FilterDropdown
  categoryCounts?: HierarchicalCounts;
  filterState?: FilterState;
  onToggleCategory?: (category: MessageCategory) => void;
  onToggleUserSubcategory?: (subcategory: UserSubcategory) => void;
  onToggleAssistantSubcategory?: (subcategory: AssistantSubcategory) => void;
}

function SessionHeader({
  sessionId,
  title,
  hasCustomTitle = false,
  autoTitle,
  externalId,
  model,
  durationMs,
  sessionDate,
  gitInfo,
  onShare,
  onDelete,
  onSessionUpdate,
  isOwner = true,
  isShared = false,
  sharedByEmail,
  categoryCounts,
  filterState,
  onToggleCategory,
  onToggleUserSubcategory,
  onToggleAssistantSubcategory,
}: SessionHeaderProps) {
  const { copy, copied } = useCopyToClipboard();
  const displayTitle = title || `Session ${externalId.substring(0, 8)}`;

  // Edit mode state
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(title || '');
  const [saving, setSaving] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const canEdit = isOwner && !isShared && onSessionUpdate;

  async function handleSave() {
    if (!onSessionUpdate) return;

    const trimmedValue = editValue.trim();
    if (trimmedValue.length > MAX_CUSTOM_TITLE_LENGTH) {
      setEditError(`Title must be ${MAX_CUSTOM_TITLE_LENGTH} characters or less`);
      return;
    }

    setSaving(true);
    setEditError(null);

    try {
      // If empty, clear the custom title (revert to auto)
      const newTitle = trimmedValue || null;
      const updatedSession = await sessionsAPI.updateTitle(sessionId, newTitle);
      onSessionUpdate(updatedSession);
      setIsEditing(false);
    } catch (err) {
      setEditError(err instanceof Error ? err.message : 'Failed to update title');
    } finally {
      setSaving(false);
    }
  }

  async function handleClearCustomTitle() {
    if (!onSessionUpdate) return;

    setSaving(true);
    setEditError(null);

    try {
      const updatedSession = await sessionsAPI.updateTitle(sessionId, null);
      onSessionUpdate(updatedSession);
      setIsEditing(false);
      setEditValue(autoTitle || '');
    } catch (err) {
      setEditError(err instanceof Error ? err.message : 'Failed to clear title');
    } finally {
      setSaving(false);
    }
  }

  function handleStartEdit() {
    setEditValue(title || '');
    setEditError(null);
    setIsEditing(true);
  }

  function handleCancelEdit() {
    setEditValue(title || '');
    setEditError(null);
    setIsEditing(false);
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSave();
    } else if (e.key === 'Escape') {
      handleCancelEdit();
    }
  }

  return (
    <header className={styles.header}>
      <div className={styles.titleSection}>
        <div className={styles.titleRow}>
          {isEditing ? (
            <div className={styles.editContainer}>
              <input
                type="text"
                className={styles.titleInput}
                value={editValue}
                onChange={(e) => setEditValue(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={autoTitle || 'Enter session title...'}
                maxLength={MAX_CUSTOM_TITLE_LENGTH}
                autoFocus
                disabled={saving}
              />
              <div className={styles.editActions}>
                <button
                  className={styles.saveBtn}
                  onClick={handleSave}
                  disabled={saving}
                  title="Save title"
                >
                  {saving ? '...' : 'Save'}
                </button>
                {hasCustomTitle && (
                  <button
                    className={styles.clearBtn}
                    onClick={handleClearCustomTitle}
                    disabled={saving}
                    title="Clear custom title and use auto-generated"
                  >
                    Reset
                  </button>
                )}
                <button
                  className={styles.cancelBtn}
                  onClick={handleCancelEdit}
                  disabled={saving}
                  title="Cancel editing"
                >
                  {CloseIcon}
                </button>
              </div>
              {editError && <div className={styles.editError}>{editError}</div>}
            </div>
          ) : (
            <>
              <h1 className={styles.title}>{displayTitle}</h1>
              {canEdit && (
                <button
                  className={styles.editBtn}
                  onClick={handleStartEdit}
                  title="Edit session title"
                  aria-label="Edit session title"
                >
                  {EditIcon}
                </button>
              )}
            </>
          )}
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
        {categoryCounts && filterState && onToggleCategory && onToggleUserSubcategory && onToggleAssistantSubcategory && (
          <FilterDropdown
            counts={categoryCounts}
            filterState={filterState}
            onToggleCategory={onToggleCategory}
            onToggleUserSubcategory={onToggleUserSubcategory}
            onToggleAssistantSubcategory={onToggleAssistantSubcategory}
          />
        )}
        {isShared ? (
          isOwner ? (
            // Owner viewing their own share link - clickable to switch to owner view
            <Link to={`/sessions/${sessionId}`} className={styles.sharedIndicatorLink} title="Switch to owner view">
              {ShareIcon}
              <span>Shared Session</span>
            </Link>
          ) : (
            <div className={styles.sharedIndicator}>
              {ShareIcon}
              <span>{sharedByEmail ? `Shared by ${sharedByEmail}` : 'Shared Session'}</span>
            </div>
          )
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
