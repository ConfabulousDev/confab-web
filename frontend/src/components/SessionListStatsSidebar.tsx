import { useMemo } from 'react';
import type { Session } from '@/types';
import { useSpinner } from '@/hooks';
import { formatDuration } from '@/utils/formatting';
import PageSidebar from './PageSidebar';
import styles from './session/SessionStatsSidebar.module.css';

interface SessionListStatsSidebarProps {
  sessions: Session[];
  loading?: boolean;
}

/**
 * Count sessions created within the last N days
 */
function countSessionsInDays(sessions: Session[], days: number): number {
  const now = new Date();
  const cutoffDate = new Date(now.getTime() - days * 24 * 60 * 60 * 1000);

  return sessions.filter((s) => {
    const sessionDate = new Date(s.first_seen);
    return sessionDate >= cutoffDate;
  }).length;
}

/**
 * Calculate duration statistics from sessions that have both first_seen and last_sync_time
 * Returns min, median, max in milliseconds
 */
function calculateDurationStats(sessions: Session[]): {
  min: number | null;
  median: number | null;
  max: number | null;
  count: number;
} {
  const durations: number[] = [];

  for (const session of sessions) {
    if (session.first_seen && session.last_sync_time) {
      const start = new Date(session.first_seen).getTime();
      const end = new Date(session.last_sync_time).getTime();
      const duration = end - start;
      if (duration > 0) {
        durations.push(duration);
      }
    }
  }

  if (durations.length === 0) {
    return { min: null, median: null, max: null, count: 0 };
  }

  durations.sort((a, b) => a - b);

  const min = durations[0] ?? null;
  const max = durations[durations.length - 1] ?? null;
  const mid = Math.floor(durations.length / 2);
  const median = durations.length % 2 === 0
    ? ((durations[mid - 1] ?? 0) + (durations[mid] ?? 0)) / 2
    : durations[mid] ?? null;

  return { min, median, max, count: durations.length };
}

/**
 * Format a number with locale-appropriate formatting
 */
function formatNumber(n: number, decimals: number = 0): string {
  return n.toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

/**
 * Format duration, handling null values
 */
function formatDurationOrDash(ms: number | null): string {
  return ms === null ? '-' : formatDuration(ms);
}

// Tooltips for stats
const TOOLTIPS = {
  totalSessions: 'Total number of sessions matching current filters',
  last7Days: 'Sessions created in the last 7 days',
  last30Days: 'Sessions created in the last 30 days',
  durationMin: 'Shortest session duration (first message to last sync)',
  durationMedian: 'Median session duration',
  durationMax: 'Longest session duration',
};

function SessionListStatsSidebar({ sessions, loading = false }: SessionListStatsSidebarProps) {
  const spinner = useSpinner(loading);

  // Calculate stats
  const stats = useMemo(() => {
    const last7Days = countSessionsInDays(sessions, 7);
    const last30Days = countSessionsInDays(sessions, 30);
    const durationStats = calculateDurationStats(sessions);

    return {
      totalSessions: sessions.length,
      last7Days,
      last30Days,
      duration: durationStats,
    };
  }, [sessions]);

  const renderValue = (value: string) => (loading ? spinner : value);

  return (
    <PageSidebar collapsible={false}>
      <div>
        {/* Overview Section */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>Overview</div>
          <div className={styles.statRow} title={TOOLTIPS.totalSessions}>
            <span className={styles.statLabel}>
              <span className={styles.statIcon}>#</span>
              Total
            </span>
            <span className={styles.statValue}>
              {renderValue(formatNumber(stats.totalSessions))}
            </span>
          </div>
          <div className={styles.statRow} title={TOOLTIPS.last7Days}>
            <span className={styles.statLabel}>Last 7 days</span>
            <span className={styles.statValue}>
              {renderValue(formatNumber(stats.last7Days))}
            </span>
          </div>
          <div className={styles.statRow} title={TOOLTIPS.last30Days}>
            <span className={styles.statLabel}>Last 30 days</span>
            <span className={styles.statValue}>
              {renderValue(formatNumber(stats.last30Days))}
            </span>
          </div>
        </div>

        {/* Duration Section */}
        <div className={styles.section}>
          <div className={styles.sectionHeader}>Duration</div>
          <div className={styles.statRow} title={TOOLTIPS.durationMin}>
            <span className={styles.statLabel}>Min</span>
            <span className={styles.statValue}>
              {renderValue(formatDurationOrDash(stats.duration.min))}
            </span>
          </div>
          <div className={styles.statRow} title={TOOLTIPS.durationMedian}>
            <span className={styles.statLabel}>Median</span>
            <span className={styles.statValue}>
              {renderValue(formatDurationOrDash(stats.duration.median))}
            </span>
          </div>
          <div className={styles.statRow} title={TOOLTIPS.durationMax}>
            <span className={styles.statLabel}>Max</span>
            <span className={styles.statValue}>
              {renderValue(formatDurationOrDash(stats.duration.max))}
            </span>
          </div>
        </div>
      </div>
    </PageSidebar>
  );
}

export default SessionListStatsSidebar;
