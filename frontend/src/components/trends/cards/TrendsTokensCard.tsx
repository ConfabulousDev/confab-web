import { useMemo } from 'react';
import { TrendsCard, StatRow } from './TrendsCard';
import { TokenIcon } from '@/components/icons';
import { CostAmount } from '@/components/CostAmount';
import { formatTokenCount } from '@/utils/tokenStats';
import {
  providerLabel,
  getProviderMetadataOrFallback,
} from '@/utils/providers';
import type {
  TrendsTokensCard as TrendsTokensCardData,
  TrendsTokensPerProvider,
} from '@/schemas/api';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './TrendsTokensCard.module.css';

const UNKNOWN_PROVIDER_COLOR = '#9ca3af';
// Synthetic stack key used when no per-provider breakdown is available
// (older wire payloads). Cannot collide with a canonical provider id.
const FALLBACK_STACK_KEY = '__total__';

function providerColor(providerId: string): string {
  // The synthetic single-stack bar uses the shared money green (fdp3).
  if (providerId === FALLBACK_STACK_KEY) return 'var(--color-cost)';
  const meta = getProviderMetadataOrFallback(providerId, 'neutral');
  return meta?.brandColor ?? UNKNOWN_PROVIDER_COLOR;
}

interface TrendsTokensCardProps {
  data: TrendsTokensCardData | null;
  // 2hh1: when a model filter is active, flag that it's session-level — these
  // totals still reflect full-session cost, not just the selected model.
  modelFilterActive?: boolean;
}

const MODEL_FILTER_CAVEAT =
  'A model filter is active. It narrows to sessions that used the selected model(s); these totals still reflect full-session cost, not just that model.';

function formatChartDate(dateStr: string): string {
  const date = new Date(dateStr + 'T00:00:00');
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

interface ChartRow {
  date: string;
  total: number;
  [providerId: string]: string | number;
}

interface TooltipPayloadEntry {
  name: string;
  value: number;
  color: string;
  payload: ChartRow;
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadEntry[];
  showBreakdown: boolean;
}

function CustomTooltip({ active, payload, showBreakdown }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const firstPayload = payload[0];
  if (!firstPayload) return null;

  const row = firstPayload.payload;
  const date = new Date(row.date + 'T00:00:00');
  const formattedDate = date.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });
  const nonZero = payload.filter((p) => p.value > 0);

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipDate}>{formattedDate}</div>
      <div className={styles.tooltipValue}><CostAmount usd={row.total} /></div>
      {showBreakdown && nonZero.length > 0 && (
        <div className={styles.tooltipBreakdown}>
          {nonZero.map((p) => (
            <div key={p.name} className={styles.tooltipRow}>
              <span className={styles.tooltipDot} style={{ background: p.color }} />
              <span className={styles.tooltipProviderLabel}>{providerLabel(p.name)}</span>
              <CostAmount usd={p.value} className={styles.tooltipProviderValue} />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// Elevated grand-total headline, rendered identically in single- and
// multi-provider modes so the total cost reads the same regardless of layout.
function TotalCostRow({ usd }: { usd: string }) {
  return (
    <div className={styles.totalCostRow} data-testid="trends-total-cost">
      <span className={styles.totalCostLabel}>Total Cost</span>
      <CostAmount usd={parseFloat(usd)} className={styles.totalCostValue} />
    </div>
  );
}

// Tri-state cache row, shared by single-provider and per-provider sections.
// The "Create" half is shown purely when the provider actually has
// cache_creation > 0 in the data (vcpa — no provider-id allowlist). Returns
// null when both numbers are 0.
function CacheRow({
  cacheCreation,
  cacheRead,
}: {
  cacheCreation: number;
  cacheRead: number;
}) {
  const hasCreate = cacheCreation > 0;
  if (hasCreate) {
    return (
      <StatRow
        label="Cache (Create / Read)"
        value={`${formatTokenCount(cacheCreation)} / ${formatTokenCount(cacheRead)}`}
      />
    );
  }
  if (cacheRead > 0) {
    return <StatRow label="Cache Read" value={formatTokenCount(cacheRead)} />;
  }
  return null;
}

// Inner rows shared by single-provider mode and per-provider sections.
function TokensStatRows({ data }: { data: TrendsTokensPerProvider }) {
  const totalTokens = data.total_input_tokens + data.total_output_tokens;
  return (
    <>
      <StatRow label="Total Tokens" value={formatTokenCount(totalTokens)} />
      <StatRow
        label="Input / Output"
        value={`${formatTokenCount(data.total_input_tokens)} / ${formatTokenCount(data.total_output_tokens)}`}
      />
      <CacheRow
        cacheCreation={data.total_cache_creation_tokens}
        cacheRead={data.total_cache_read_tokens}
      />
    </>
  );
}

interface TrendsTokensPerProviderListProps {
  entries: Array<[string, TrendsTokensPerProvider]>;
}

function TrendsTokensPerProviderList({ entries }: TrendsTokensPerProviderListProps) {
  return (
    <div className={styles.providerSections}>
      {entries.map(([providerId, e]) => (
        <section key={providerId} className={styles.providerSection}>
          <header className={styles.providerHeader}>{providerLabel(providerId)}</header>
          <div className={styles.providerRows}>
            <StatRow label="Cost" value={<CostAmount usd={parseFloat(e.total_cost_usd)} />} />
            <TokensStatRows data={e} />
          </div>
        </section>
      ))}
    </div>
  );
}

export function TrendsTokensCard({ data, modelFilterActive = false }: TrendsTokensCardProps) {
  const perProviderEntries = useMemo(
    () =>
      data
        ? Object.entries(data.per_provider).sort(([a], [b]) => a.localeCompare(b))
        : [],
    [data],
  );

  // Stacked series order matches the per-provider sections above so the bar
  // segment colors line up with the section labels. Falls back to a single
  // synthetic stack when no per-provider breakdown is available.
  const stackProviderIds: string[] = useMemo(() => {
    if (perProviderEntries.length > 0) return perProviderEntries.map(([id]) => id);
    if (data && data.daily_costs.length > 0) return [FALLBACK_STACK_KEY];
    return [];
  }, [perProviderEntries, data]);

  const chartData: ChartRow[] = useMemo(() => {
    if (!data) return [];
    return data.daily_costs.map((d) => {
      const total = parseFloat(d.cost_usd);
      const row: ChartRow = { date: d.date, total };
      for (const providerId of stackProviderIds) {
        row[providerId] =
          providerId === FALLBACK_STACK_KEY
            ? total
            : parseFloat(d.per_provider[providerId] ?? '0');
      }
      return row;
    });
  }, [data, stackProviderIds]);

  if (!data) return null;

  const multiProvider = perProviderEntries.length >= 2;
  const hasChartData = chartData.length > 1;
  // The fallback path always emits exactly one stack key, so length > 1
  // implies real per-provider stacking and the breakdown is meaningful.
  const tooltipShowBreakdown = stackProviderIds.length > 1;

  return (
    <TrendsCard
      title="Tokens & Cost"
      icon={TokenIcon}
      caveat={modelFilterActive ? MODEL_FILTER_CAVEAT : undefined}
    >
      {/* h7xe: the grand-total headline renders identically in both modes, so
          it lives above the layout branch rather than inside each one. */}
      <TotalCostRow usd={data.total_cost_usd} />
      {multiProvider ? (
        <TrendsTokensPerProviderList entries={perProviderEntries} />
      ) : (
        <TokensStatRows data={data} />
      )}

      {hasChartData && (
        <div className={styles.chartContainer}>
          <div className={styles.chartLabel}>Daily Cost</div>
          <ResponsiveContainer width="100%" height={160}>
            <BarChart data={chartData} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
              <XAxis
                dataKey="date"
                tickFormatter={formatChartDate}
                tick={{ fontSize: 10, fill: 'var(--color-text-muted)' }}
                axisLine={false}
                tickLine={false}
                angle={-45}
                textAnchor="end"
                tickMargin={10}
                height={56}
              />
              <YAxis hide domain={[0, 'dataMax']} />
              <Tooltip
                content={<CustomTooltip showBreakdown={tooltipShowBreakdown} />}
                cursor={{ fill: 'var(--color-bg-primary)' }}
              />
              {stackProviderIds.map((providerId) => (
                <Bar
                  key={providerId}
                  dataKey={providerId}
                  stackId="cost"
                  fill={providerColor(providerId)}
                  isAnimationActive={false}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </TrendsCard>
  );
}
