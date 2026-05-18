import { TrendsCard, StatRow } from './TrendsCard';
import { TokenIcon } from '@/components/icons';
import { formatTokenCount, formatCost } from '@/utils/tokenStats';
import { providerLabel } from '@/utils/providers';
import type {
  TrendsTokensCard as TrendsTokensCardData,
  TrendsTokensPerProvider,
} from '@/schemas/api';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './TrendsTokensCard.module.css';

interface TrendsTokensCardProps {
  data: TrendsTokensCardData | null;
}

// Format date for chart axis
function formatChartDate(dateStr: string): string {
  const date = new Date(dateStr + 'T00:00:00');
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    value: number;
    payload: { date: string; cost_usd: string };
  }>;
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const firstPayload = payload[0];
  if (!firstPayload) return null;
  const item = firstPayload.payload;
  const date = new Date(item.date + 'T00:00:00');
  const formattedDate = date.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });

  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipDate}>{formattedDate}</div>
      <div className={styles.tooltipValue}>{formatCost(parseFloat(item.cost_usd))}</div>
    </div>
  );
}

// CF-435: providers that have no cache-write concept render `—` in the per-row
// Cache Create cell (structural N/A, not a measurement of zero). Today only
// Codex/OpenAI fits; new providers default to literal numbers until added here.
function providerHasCacheWrite(providerId: string): boolean {
  return providerId !== 'codex';
}

interface TrendsTokensProviderTableProps {
  entries: Array<[string, TrendsTokensPerProvider]>;
  totalCostUSD: string;
}

function TrendsTokensProviderTable({ entries, totalCostUSD }: TrendsTokensProviderTableProps) {
  return (
    <div className={styles.tableWrap}>
      <table className={styles.providerTable}>
        <thead>
          <tr>
            <th scope="col" className={styles.providerCol}>Provider</th>
            <th scope="col">Input</th>
            <th scope="col">Output</th>
            <th scope="col">Cache Read</th>
            <th scope="col">Cache Create</th>
            <th scope="col">Cost</th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([providerId, e]) => (
            <tr key={providerId}>
              <td className={styles.providerCol}>{providerLabel(providerId)}</td>
              <td>{formatTokenCount(e.total_input_tokens)}</td>
              <td>{formatTokenCount(e.total_output_tokens)}</td>
              <td>{formatTokenCount(e.total_cache_read_tokens)}</td>
              <td>
                {providerHasCacheWrite(providerId)
                  ? formatTokenCount(e.total_cache_creation_tokens)
                  : '—'}
              </td>
              <td>{formatCost(parseFloat(e.total_cost_usd))}</td>
            </tr>
          ))}
          <tr className={styles.totalRow}>
            <td className={styles.providerCol}>Total</td>
            <td>—</td>
            <td>—</td>
            <td>—</td>
            <td>—</td>
            <td>{formatCost(parseFloat(totalCostUSD))}</td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}

export function TrendsTokensCard({ data }: TrendsTokensCardProps) {
  if (!data) return null;

  const perProviderEntries = Object.entries(data.per_provider).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  const multiProvider = perProviderEntries.length >= 2;

  const totalTokens = data.total_input_tokens + data.total_output_tokens;

  // Prepare chart data
  const chartData = data.daily_costs.map((d) => ({
    ...d,
    costValue: parseFloat(d.cost_usd),
  }));

  const hasChartData = chartData.length > 1;

  return (
    <TrendsCard title="Tokens & Cost" icon={TokenIcon}>
      {multiProvider ? (
        <TrendsTokensProviderTable
          entries={perProviderEntries}
          totalCostUSD={data.total_cost_usd}
        />
      ) : (
        <>
          <StatRow
            label="Total Cost"
            value={
              <span style={{
                color: parseFloat(data.total_cost_usd) === 0 ? 'var(--color-warning-text)' : '#22c55e',
                fontWeight: 600,
              }}>
                {formatCost(parseFloat(data.total_cost_usd))}
              </span>
            }
          />
          <StatRow
            label="Total Tokens"
            value={formatTokenCount(totalTokens)}
          />
          <StatRow
            label="Input / Output"
            value={`${formatTokenCount(data.total_input_tokens)} / ${formatTokenCount(data.total_output_tokens)}`}
          />
          {/* CF-436: Tri-state cache row. OpenAI doesn't bill cache writes, so a
              Codex-only filtered window has total_cache_creation_tokens === 0.
              Collapse to "Cache Read" when creation is 0 and read > 0; hide
              entirely when both are 0. */}
          {data.total_cache_creation_tokens > 0 && (
            <StatRow
              label="Cache (Create / Read)"
              value={`${formatTokenCount(data.total_cache_creation_tokens)} / ${formatTokenCount(data.total_cache_read_tokens)}`}
            />
          )}
          {data.total_cache_creation_tokens === 0 && data.total_cache_read_tokens > 0 && (
            <StatRow
              label="Cache Read"
              value={formatTokenCount(data.total_cache_read_tokens)}
            />
          )}
        </>
      )}

      {hasChartData && (
        <div className={styles.chartContainer}>
          <div className={styles.chartLabel}>Daily Cost</div>
          <ResponsiveContainer width="100%" height={140}>
            <AreaChart data={chartData} margin={{ top: 8, right: 0, left: 0, bottom: 24 }}>
              <defs>
                <linearGradient id="costGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="date"
                tickFormatter={formatChartDate}
                tick={{ fontSize: 10, fill: 'var(--color-text-muted)' }}
                axisLine={false}
                tickLine={false}
                angle={-45}
                textAnchor="end"
                height={40}
              />
              <YAxis hide domain={[0, 'dataMax']} />
              <Tooltip
                content={<CustomTooltip />}
                cursor={{ stroke: 'var(--color-border)', strokeDasharray: '3 3' }}
              />
              <Area
                type="monotone"
                dataKey="costValue"
                stroke="#22c55e"
                strokeWidth={2}
                fill="url(#costGradient)"
                dot={{ r: 3, fill: '#22c55e', strokeWidth: 0 }}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </TrendsCard>
  );
}
