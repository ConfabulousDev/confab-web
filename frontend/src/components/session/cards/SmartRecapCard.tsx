import { CardWrapper, SectionHeader, CardError } from './Card';
import {
  SparklesIcon,
  CheckCircleIcon,
  AlertCircleIcon,
  LightbulbIcon,
  RefreshIcon,
} from '@/components/icons';
import type {
  SmartRecapCardData,
  SmartRecapQuotaInfo,
} from '@/schemas/api';
import type { CardProps } from './types';
import { formatRelativeTime } from '@/utils/formatting';
import styles from './SmartRecapCard.module.css';
import panelStyles from '../SessionSummaryPanel.module.css';

interface SmartRecapCardProps extends CardProps<SmartRecapCardData> {
  quota?: SmartRecapQuotaInfo | null;
  /** Why smart recap data is missing: "quota_exceeded" (owner) or "unavailable" (non-owner) */
  missingReason?: 'quota_exceeded' | 'unavailable' | null;
  /** Callback to force regeneration (only available to owners) */
  onRefresh?: () => void;
  /** Whether a refresh is in progress */
  isRefreshing?: boolean;
}

/**
 * Displays AI-generated session insights including:
 * - Recap of what happened
 * - Things that went well
 * - Things that didn't go well
 * - Suggestions for improvement
 */
export function SmartRecapCard({
  data,
  loading,
  error,
  quota,
  missingReason,
  onRefresh,
  isRefreshing,
}: SmartRecapCardProps) {
  // Error state (graceful degradation)
  if (error && !data) {
    return <CardError title="Smart Recap" error={error} icon={SparklesIcon} />;
  }

  // Loading state (initial load or during refresh)
  if ((loading && !data) || isRefreshing) {
    return (
      <CardWrapper title="Smart Recap" icon={SparklesIcon}>
        <div className={styles.generating}>
          <div className={styles.spinner} />
          <span>{isRefreshing ? 'Generating AI recap...' : 'Loading...'}</span>
        </div>
      </CardWrapper>
    );
  }

  // No data - show placeholder based on missing reason
  if (!data) {
    if (missingReason === 'quota_exceeded') {
      return (
        <CardWrapper
          title="Smart Recap"
          icon={SparklesIcon}
          subtitle={quota ? `${quota.used}/${quota.limit} used` : undefined}
        >
          <div className={styles.quotaPlaceholder}>
            <p className={styles.quotaPlaceholderTitle}>Monthly limit reached</p>
            <p className={styles.quotaPlaceholderText}>
              Recaps will be available next month.
            </p>
          </div>
        </CardWrapper>
      );
    }
    if (missingReason === 'unavailable') {
      return (
        <CardWrapper title="Smart Recap" icon={SparklesIcon}>
          <div className={styles.quotaPlaceholder}>
            <p className={styles.quotaPlaceholderText}>
              No smart recap available for this session.
            </p>
          </div>
        </CardWrapper>
      );
    }
    return null;
  }

  // Build subtitle showing when generated, model, and quota
  const modelShort = data.model_used.replace(/-\d{8}$/, '');
  const subtitleParts = [
    formatRelativeTime(data.computed_at),
    modelShort,
    ...(quota ? [`${quota.used}/${quota.limit} used`] : []),
  ];
  const subtitle = subtitleParts.join(' · ');

  // Refresh button for owners (disabled if quota exceeded)
  // Note: isRefreshing check not needed here since we return early with generating UI
  const refreshButton = onRefresh ? (
    <button
      className={panelStyles.cardActionButton}
      onClick={onRefresh}
      disabled={quota?.exceeded}
      title={quota?.exceeded ? 'Monthly limit reached' : 'Regenerate recap'}
      aria-label="Regenerate recap"
    >
      {RefreshIcon}
    </button>
  ) : null;

  return (
    <CardWrapper title="Smart Recap" icon={SparklesIcon} subtitle={subtitle} action={refreshButton}>
      {/* Recap */}
      <div className={styles.recap}>{data.recap}</div>

      {/* What went well */}
      {data.went_well.length > 0 && (
        <>
          <SectionHeader label="Went Well" icon={CheckCircleIcon} />
          <ul className={styles.list}>
            {data.went_well.map((item, i) => (
              <li key={i} className={styles.listItemSuccess}>
                <span className={styles.listIcon}>{CheckCircleIcon}</span>
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </>
      )}

      {/* What didn't go well */}
      {data.went_bad.length > 0 && (
        <>
          <SectionHeader label="Needs Improvement" icon={AlertCircleIcon} />
          <ul className={styles.list}>
            {data.went_bad.map((item, i) => (
              <li key={i} className={styles.listItemWarning}>
                <span className={styles.listIcon}>{AlertCircleIcon}</span>
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </>
      )}

      {/* Suggestions */}
      {(data.human_suggestions.length > 0 ||
        data.environment_suggestions.length > 0 ||
        data.default_context_suggestions.length > 0) && (
        <>
          <SectionHeader label="Suggestions" icon={LightbulbIcon} />
          <ul className={styles.list}>
            {data.human_suggestions.map((item, i) => (
              <li key={`human-${i}`} className={styles.listItem}>
                <span className={styles.listIcon}>{LightbulbIcon}</span>
                <span>{item}</span>
              </li>
            ))}
            {data.environment_suggestions.map((item, i) => (
              <li key={`env-${i}`} className={styles.listItem}>
                <span className={styles.listIcon}>{LightbulbIcon}</span>
                <span>{item}</span>
              </li>
            ))}
            {data.default_context_suggestions.map((item, i) => (
              <li key={`ctx-${i}`} className={styles.listItem}>
                <span className={styles.listIcon}>{LightbulbIcon}</span>
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </>
      )}

      {/* Footer - only show quota warning if exceeded */}
      {quota?.exceeded && (
        <div className={styles.footer}>
          <span className={styles.quotaWarning}>Monthly limit reached</span>
        </div>
      )}
    </CardWrapper>
  );
}
