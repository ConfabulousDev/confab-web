import { useState, useMemo } from 'react';
import { formatDuration } from '@/utils';
import { formatCost } from '@/utils/tokenStats';
import type { OrgUserAnalytics } from '@/schemas/api';
import type { SortDirection } from '@/utils/sorting';
import styles from './OrgTable.module.css';

type SortField =
  | 'name'
  | 'session_count'
  | 'total_cost'
  | 'total_duration'
  | 'claude_time'
  | 'user_time'
  | 'avg_cost'
  | 'avg_duration'
  | 'avg_claude'
  | 'avg_user';

interface Column {
  key: SortField;
  label: string;
  tooltip?: string;
}

const COST_TOOLTIP = 'Estimated based on token usage and model pricing';

const COLUMNS: Column[] = [
  { key: 'name', label: 'User', tooltip: 'Organization member' },
  { key: 'session_count', label: 'Sessions', tooltip: 'Total number of sessions' },
  { key: 'total_cost', label: 'Est. Cost', tooltip: COST_TOOLTIP },
  { key: 'total_duration', label: 'Total Duration', tooltip: 'Sum of all session durations' },
  { key: 'claude_time', label: 'Claude Time', tooltip: 'Total time Claude was actively responding' },
  { key: 'user_time', label: 'User Time', tooltip: 'Total time the user was active' },
  { key: 'avg_cost', label: 'Avg Cost', tooltip: 'Average estimated cost per session' },
  { key: 'avg_duration', label: 'Avg Duration', tooltip: 'Average session duration' },
  { key: 'avg_claude', label: 'Avg Claude', tooltip: 'Average Claude response time per session' },
  { key: 'avg_user', label: 'Avg User', tooltip: 'Average user active time per session' },
];

function getUserDisplayName(user: OrgUserAnalytics['user']): string {
  return user.name ?? user.email;
}

function getSortValue(row: OrgUserAnalytics, field: SortField): number | string {
  switch (field) {
    case 'name':
      return getUserDisplayName(row.user).toLowerCase();
    case 'session_count':
      return row.session_count;
    case 'total_cost':
      return parseFloat(row.total_cost_usd);
    case 'total_duration':
      return row.total_duration_ms;
    case 'claude_time':
      return row.total_claude_time_ms;
    case 'user_time':
      return row.total_user_time_ms;
    case 'avg_cost':
      return parseFloat(row.avg_cost_usd);
    case 'avg_duration':
      return row.avg_duration_ms ?? 0;
    case 'avg_claude':
      return row.avg_claude_time_ms ?? 0;
    case 'avg_user':
      return row.avg_user_time_ms ?? 0;
  }
}

function formatCostCompact(value: number | string): string {
  const num = typeof value === 'number' ? value : parseFloat(value);
  if (num === 0) return '$0';
  return formatCost(num);
}

function formatDurationOrDash(ms: number | null | undefined): string {
  if (ms == null || ms === 0) return '-';
  return formatDuration(ms);
}

interface OrgTableProps {
  users: OrgUserAnalytics[];
}

function OrgTable({ users }: OrgTableProps) {
  const [sortField, setSortField] = useState<SortField>('name');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');

  const handleSort = (field: SortField) => {
    if (field === sortField) {
      setSortDirection(prev => prev === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection(field === 'name' ? 'asc' : 'desc');
    }
  };

  const sortedUsers = useMemo(() => {
    return [...users].sort((a, b) => {
      const aVal = getSortValue(a, sortField);
      const bVal = getSortValue(b, sortField);
      if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1;
      if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1;
      return 0;
    });
  }, [users, sortField, sortDirection]);

  const summary = useMemo(() => {
    const totals = users.reduce(
      (acc, u) => ({
        sessions: acc.sessions + u.session_count,
        cost: acc.cost + parseFloat(u.total_cost_usd),
        duration: acc.duration + u.total_duration_ms,
        claude: acc.claude + u.total_claude_time_ms,
        user: acc.user + u.total_user_time_ms,
      }),
      { sessions: 0, cost: 0, duration: 0, claude: 0, user: 0 },
    );

    const avg = (value: number) =>
      totals.sessions > 0 ? Math.round(value / totals.sessions) : null;

    return {
      sessionCount: totals.sessions,
      totalCost: totals.cost,
      totalDuration: totals.duration,
      totalClaude: totals.claude,
      totalUser: totals.user,
      avgCost: totals.sessions > 0 ? totals.cost / totals.sessions : 0,
      avgDuration: avg(totals.duration),
      avgClaude: avg(totals.claude),
      avgUser: avg(totals.user),
    };
  }, [users]);

  const sortArrow = (field: SortField) => {
    if (sortField !== field) return null;
    return (
      <span className={styles.sortIndicator}>
        {sortDirection === 'asc' ? '▲' : '▼'}
      </span>
    );
  };

  return (
    <div className={styles.tableWrapper}>
      <table className={styles.table}>
        <thead>
          <tr>
            {COLUMNS.map((col) => (
              <th
                key={col.key}
                className={`${col.key === 'name' ? styles.alignLeft : styles.alignRight} ${styles.sortableHeader}`}
                onClick={() => handleSort(col.key)}
                title={col.tooltip}
              >
                <span className={styles.headerContent}>
                  {col.label}
                  {sortArrow(col.key)}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          <tr className={styles.summaryRow}>
            <td className={styles.alignLeft}>Everyone</td>
            <td className={styles.alignRight}>{summary.sessionCount}</td>
            <td className={styles.alignRight}>{formatCostCompact(summary.totalCost)}</td>
            <td className={styles.alignRight}>{formatDurationOrDash(summary.totalDuration)}</td>
            <td className={styles.alignRight}>{formatDurationOrDash(summary.totalClaude)}</td>
            <td className={styles.alignRight}>{formatDurationOrDash(summary.totalUser)}</td>
            <td className={styles.alignRight}>{formatCostCompact(summary.avgCost)}</td>
            <td className={styles.alignRight}>{formatDurationOrDash(summary.avgDuration)}</td>
            <td className={styles.alignRight}>{formatDurationOrDash(summary.avgClaude)}</td>
            <td className={styles.alignRight}>{formatDurationOrDash(summary.avgUser)}</td>
          </tr>

          {sortedUsers.map((row) => {
            const hasData = row.session_count > 0;
            const cellClass = hasData ? styles.numericCell : styles.zeroCell;

            return (
              <tr key={row.user.id}>
                <td className={styles.alignLeft}>
                  <div className={styles.userCell}>
                    <span className={styles.userName}>{getUserDisplayName(row.user)}</span>
                    {row.user.name && <span className={styles.userEmail}>{row.user.email}</span>}
                  </div>
                </td>
                <td className={`${styles.alignRight} ${cellClass}`}>{row.session_count}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatCostCompact(row.total_cost_usd)}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatDurationOrDash(row.total_duration_ms)}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatDurationOrDash(row.total_claude_time_ms)}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatDurationOrDash(row.total_user_time_ms)}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatCostCompact(row.avg_cost_usd)}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatDurationOrDash(row.avg_duration_ms)}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatDurationOrDash(row.avg_claude_time_ms)}</td>
                <td className={`${styles.alignRight} ${cellClass}`}>{formatDurationOrDash(row.avg_user_time_ms)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export default OrgTable;
