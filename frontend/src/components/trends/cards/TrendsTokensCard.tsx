import { useMemo } from 'react';
import { TrendsCard, StatRow } from './TrendsCard';
import { TokenIcon } from '@/components/icons';
import { formatTokenCount, formatCost } from '@/utils/tokenStats';
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

const COST_GREEN = '#22c55e';
const UNKNOWN_PROVIDER_COLOR = '#9ca3af';
// Synthetic stack key used when no per-provider breakdown is available
// (older wire payloads). Cannot collide with a canonical provider id.
const FALLBACK_STACK_KEY = '__total__';

function providerColor(providerId: string): string {
  if (providerId === FALLBACK_STACK_KEY) return COST_GREEN;
  const meta = getProviderMetadataOrFallback(providerId, 'neutral');
  return meta?.brandColor ?? UNKNOWN_PROVIDER_COLOR;
}

interface TrendsTokensCardProps {
  data: TrendsTokensCardData | null;
}

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
      <div className={styles.tooltipValue}>{formatCost(row.total)}</div>
      {showBreakdown && nonZero.length > 0 && (
        <div className={styles.tooltipBreakdown}>
          {nonZero.map((p) => (
            <div key={p.name} className={styles.tooltipRow}>
              <span className={styles.tooltipDot} style={{ background: p.color }} />
              <span className={styles.tooltipProviderLabel}>{providerLabel(p.name)}</span>
              <span className={styles.tooltipProviderValue}>{formatCost(p.value)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// Providers with no cache-write concept (Codex/OpenAI today) hide the
// Cache row's "Create / " prefix rather than dashing it.
// TODO(opencode): this hardcodes "only Codex lacks cache-write", but OpenCode
// aggregates many providers (OpenAI, Gemini, DeepSeek, …) that mostly don't bill
// cache writes either. Once OpenCode token data lands, drive this off whether the
// provider actually has cache_write > 0 in the data rather than a provider-id
// allowlist (which also tidies the Codex special-case).
function providerHasCacheWrite(providerId: string): boolean {
  return providerId !== 'codex';
}

function CostValue({ usd, className }: { usd: string; className?: string }) {
  const n = parseFloat(usd);
  return (
    <span
      className={className}
      style={{
        color: n === 0 ? 'var(--color-warning-text)' : COST_GREEN,
        fontWeight: 600,
      }}
    >
      {formatCost(n)}
    </span>
  );
}

// Tri-state cache row, parameterized so single-provider and per-provider
// sections share the same rules. Returns null when both numbers are 0.
function CacheRow({
  providerId,
  cacheCreation,
  cacheRead,
}: {
  providerId: string;
  cacheCreation: number;
  cacheRead: number;
}) {
  const hasCreate = providerHasCacheWrite(providerId) && cacheCreation > 0;
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
function TokensStatRows({
  providerId,
  data,
}: {
  providerId: string;
  data: TrendsTokensPerProvider;
}) {
  const totalTokens = data.total_input_tokens + data.total_output_tokens;
  return (
    <>
      <StatRow label="Total Tokens" value={formatTokenCount(totalTokens)} />
      <StatRow
        label="Input / Output"
        value={`${formatTokenCount(data.total_input_tokens)} / ${formatTokenCount(data.total_output_tokens)}`}
      />
      <CacheRow
        providerId={providerId}
        cacheCreation={data.total_cache_creation_tokens}
        cacheRead={data.total_cache_read_tokens}
      />
    </>
  );
}

interface TrendsTokensPerProviderListProps {
  entries: Array<[string, TrendsTokensPerProvider]>;
  totalCostUSD: string;
}

function TrendsTokensPerProviderList({
  entries,
  totalCostUSD,
}: TrendsTokensPerProviderListProps) {
  return (
    <>
      <div className={styles.totalCostRow}>
        <span className={styles.totalCostLabel}>Total Cost</span>
        <CostValue usd={totalCostUSD} className={styles.totalCostValue} />
      </div>
      <div className={styles.providerSections}>
        {entries.map(([providerId, e]) => (
          <section key={providerId} className={styles.providerSection}>
            <header className={styles.providerHeader}>{providerLabel(providerId)}</header>
            <div className={styles.providerRows}>
              <StatRow label="Cost" value={<CostValue usd={e.total_cost_usd} />} />
              <TokensStatRows providerId={providerId} data={e} />
            </div>
          </section>
        ))}
      </div>
    </>
  );
}

export function TrendsTokensCard({ data }: TrendsTokensCardProps) {
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
  const singleProviderId =
    perProviderEntries.length === 1 ? perProviderEntries[0]![0] : '';
  // The fallback path always emits exactly one stack key, so length > 1
  // implies real per-provider stacking and the breakdown is meaningful.
  const tooltipShowBreakdown = stackProviderIds.length > 1;

  return (
    <TrendsCard title="Tokens & Cost" icon={TokenIcon}>
      {multiProvider ? (
        <TrendsTokensPerProviderList
          entries={perProviderEntries}
          totalCostUSD={data.total_cost_usd}
        />
      ) : (
        <>
          <StatRow label="Total Cost" value={<CostValue usd={data.total_cost_usd} />} />
          <TokensStatRows providerId={singleProviderId} data={data} />
        </>
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
