import { CardWrapper, SectionHeader } from './Card';
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
  quota,
  onRefresh,
  isRefreshing,
}: SmartRecapCardProps) {
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

  // No data
  if (!data) return null;

  const recapData = data;

  // Build subtitle showing when generated, model, staleness, and quota
  const subtitleParts: string[] = [];
  // Show when the recap was generated
  subtitleParts.push(formatRelativeTime(recapData.computed_at));
  // Show model name (extract just the model part, e.g., "claude-haiku-4-5")
  const modelShort = recapData.model_used.replace(/-\d{8}$/, '');
  subtitleParts.push(modelShort);
  if (recapData.is_stale) {
    subtitleParts.push('Outdated');
  }
  if (quota) {
    subtitleParts.push(`${quota.used}/${quota.limit} used`);
  }
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
      <div className={styles.recap}>{recapData.recap}</div>

      {/* What went well */}
      {recapData.went_well.length > 0 && (
        <>
          <SectionHeader label="Went Well" icon={CheckCircleIcon} />
          <ul className={styles.list}>
            {recapData.went_well.map((item, i) => (
              <li key={i} className={styles.listItemSuccess}>
                <span className={styles.listIcon}>{CheckCircleIcon}</span>
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </>
      )}

      {/* What didn't go well */}
      {recapData.went_bad.length > 0 && (
        <>
          <SectionHeader label="Needs Improvement" icon={AlertCircleIcon} />
          <ul className={styles.list}>
            {recapData.went_bad.map((item, i) => (
              <li key={i} className={styles.listItemWarning}>
                <span className={styles.listIcon}>{AlertCircleIcon}</span>
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </>
      )}

      {/* Suggestions */}
      {(recapData.human_suggestions.length > 0 ||
        recapData.environment_suggestions.length > 0 ||
        recapData.default_context_suggestions.length > 0) && (
        <>
          <SectionHeader label="Suggestions" icon={LightbulbIcon} />
          <ul className={styles.list}>
            {recapData.human_suggestions.map((item, i) => (
              <li key={`human-${i}`} className={styles.listItem}>
                <span className={styles.listIcon}>{LightbulbIcon}</span>
                <span>{item}</span>
              </li>
            ))}
            {recapData.environment_suggestions.map((item, i) => (
              <li key={`env-${i}`} className={styles.listItem}>
                <span className={styles.listIcon}>{LightbulbIcon}</span>
                <span>{item}</span>
              </li>
            ))}
            {recapData.default_context_suggestions.map((item, i) => (
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
