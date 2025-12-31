import { CardWrapper, CardLoading, StatRow, SectionHeader } from './Card';
import { CodeIcon, SearchIcon, FileIcon } from '@/components/icons';
import type { CodeActivityCardData } from '@/schemas/api';
import type { CardProps } from './types';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import styles from './CodeActivityCard.module.css';

interface LanguageChartData {
  extension: string;
  count: number;
}

function prepareChartData(languageBreakdown: Record<string, number>): LanguageChartData[] {
  return Object.entries(languageBreakdown)
    .map(([extension, count]) => ({ extension, count }))
    .sort((a, b) => b.count - a.count); // Most used first
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    dataKey: string;
    payload: LanguageChartData;
  }>;
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const firstPayload = payload[0];
  if (!firstPayload) return null;
  const { extension, count } = firstPayload.payload;

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>.{extension}</div>
      <div className={styles.tooltipRow}>{count} file{count !== 1 ? 's' : ''}</div>
    </div>
  );
}

// File read icon
const FileReadIcon = (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <line x1="16" y1="13" x2="8" y2="13" />
    <line x1="16" y1="17" x2="8" y2="17" />
  </svg>
);

// File edit icon
const FileEditIcon = (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M12 20h9" />
    <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
  </svg>
);

// Plus icon for lines added
const PlusIcon = (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="12" y1="5" x2="12" y2="19" />
    <line x1="5" y1="12" x2="19" y2="12" />
  </svg>
);

// Minus icon for lines removed
const MinusIcon = (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="5" y1="12" x2="19" y2="12" />
  </svg>
);

export function CodeActivityCard({ data, loading }: CardProps<CodeActivityCardData>) {
  if (loading && !data) {
    return (
      <CardWrapper title="Code Activity" icon={CodeIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  // Don't render if no file activity
  const totalFiles = data.files_read + data.files_modified;
  if (totalFiles === 0 && data.search_count === 0) return null;

  const chartData = prepareChartData(data.language_breakdown);
  const hasLanguages = chartData.length > 0;

  // Calculate dynamic height based on number of languages (min 80px, 24px per lang)
  const chartHeight = Math.max(80, chartData.length * 24);

  // Calculate dynamic YAxis width based on longest extension (~8px per char at 11px font)
  const maxLabelLength = hasLanguages ? Math.max(...chartData.map((d) => d.extension.length)) : 0;
  const yAxisWidth = Math.max(30, maxLabelLength * 8 + 8);

  return (
    <CardWrapper title="Code Activity" icon={CodeIcon}>
      <div className={styles.statsGrid}>
        <StatRow label="Files read" value={data.files_read} icon={FileReadIcon} />
        <StatRow label="Files modified" value={data.files_modified} icon={FileEditIcon} />
        <StatRow
          label="Lines added"
          value={data.lines_added}
          icon={PlusIcon}
          valueClassName={styles.linesAdded}
        />
        <StatRow
          label="Lines removed"
          value={data.lines_removed}
          icon={MinusIcon}
          valueClassName={styles.linesRemoved}
        />
        <StatRow label="Searches" value={data.search_count} icon={SearchIcon} />
      </div>

      {hasLanguages && (
        <>
          <SectionHeader label="File extensions" icon={FileIcon} />
          <div className={styles.chartContainer} style={{ height: chartHeight }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={chartData}
                layout="vertical"
                margin={{ top: 0, right: 24, left: 0, bottom: 0 }}
                barSize={14}
              >
                <XAxis
                  type="number"
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 10, fill: 'var(--color-text-tertiary)' }}
                  tickFormatter={(value) => (value === 0 ? '' : String(value))}
                />
                <YAxis
                  type="category"
                  dataKey="extension"
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 11, fill: 'var(--color-text-secondary)' }}
                  width={yAxisWidth}
                  interval={0}
                />
                <Tooltip
                  content={<CustomTooltip />}
                  cursor={{ fill: 'var(--color-bg-hover)', opacity: 0.5 }}
                />
                <Bar dataKey="count" fill="var(--color-accent)" radius={[2, 2, 2, 2]} isAnimationActive={false} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </>
      )}
    </CardWrapper>
  );
}
