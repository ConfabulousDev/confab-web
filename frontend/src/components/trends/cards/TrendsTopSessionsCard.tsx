import { TrendsCard } from './TrendsCard';
import { ChatIcon, TrendingUpIcon } from '@/components/icons';
import { getProviderMetadataOrFallback } from '@/utils/providers';
import { formatDuration } from '@/utils';
import { formatCost } from '@/utils/tokenStats';
import type { TrendsTopSessionsCard as TrendsTopSessionsCardData } from '@/schemas/api';
import styles from './TrendsTopSessionsCard.module.css';

// N options for the Costliest Sessions limit; mirrors the backend allowlist.
const TOP_N_OPTIONS = [10, 25, 50] as const;

interface TrendsTopSessionsCardProps {
  data: TrendsTopSessionsCardData | null;
  /** Currently selected N (defaults to the backend default of 10). */
  topN?: number;
  /** When provided, renders the 10/25/50 selector wired to this handler. */
  onTopNChange?: (n: number) => void;
  /** Dims the list and disables the selector while a refetch is in flight. */
  loading?: boolean;
  // 2hh1: when a model filter is active, flag that it's session-level — these
  // rows rank by full-session cost, not just the selected model's portion.
  modelFilterActive?: boolean;
}

const MODEL_FILTER_CAVEAT =
  'A model filter is active. It narrows to sessions that used the selected model(s); these rows rank by full-session cost, not just that model.';

/** Strip org prefix from "org/repo" → "repo" */
function formatRepoName(repo: string): string {
  const parts = repo.split('/');
  return parts.length > 1 ? parts[parts.length - 1]! : repo;
}

// Diverges from the app-wide getProviderIcon (which defaults to Claude for
// unknown values). The Costliest Sessions card must not assert Claude
// identity for empty/unknown providers — surface a neutral ChatIcon instead.
function getRowProviderIcon(provider: string) {
  return getProviderMetadataOrFallback(provider, 'neutral')?.icon ?? ChatIcon;
}

function TopNSelector({
  topN,
  onTopNChange,
  disabled,
}: {
  topN: number;
  onTopNChange: (n: number) => void;
  disabled: boolean;
}) {
  return (
    <span className={styles.topNSelector} role="group" aria-label="Number of sessions">
      {TOP_N_OPTIONS.map((n) => (
        <button
          key={n}
          type="button"
          className={styles.topNOption}
          aria-pressed={n === topN}
          disabled={disabled}
          onClick={() => onTopNChange(n)}
        >
          {n}
        </button>
      ))}
    </span>
  );
}

export function TrendsTopSessionsCard({
  data,
  topN = 10,
  onTopNChange,
  loading = false,
  modelFilterActive = false,
}: TrendsTopSessionsCardProps) {
  if (!data || data.sessions.length === 0) return null;

  const subtitle = `Top ${data.sessions.length} by cost`;
  const headerAction = onTopNChange ? (
    <TopNSelector topN={topN} onTopNChange={onTopNChange} disabled={loading} />
  ) : undefined;

  return (
    <div className={styles.wrapper}>
      <TrendsCard
        title="Costliest Sessions"
        icon={TrendingUpIcon}
        subtitle={subtitle}
        headerAction={headerAction}
        caveat={modelFilterActive ? MODEL_FILTER_CAVEAT : undefined}
      >
        <div
          className={styles.sessionList}
          data-loading={loading || undefined}
          aria-busy={loading || undefined}
        >
          {data.sessions.map((session, index) => (
            <a
              key={session.id}
              href={`/sessions/${session.id}`}
              target="_blank"
              rel="noopener noreferrer"
              className={styles.sessionRow}
            >
              <span className={styles.rank}>{index + 1}</span>
              <div className={styles.sessionInfo}>
                <span className={styles.sessionTitle} title={session.title}>
                  <span className={styles.providerIcon}>{getRowProviderIcon(session.provider)}</span>
                  {session.title}
                </span>
                <div className={styles.sessionMeta}>
                  {session.git_repo && (
                    <span className={styles.repo}>{formatRepoName(session.git_repo)}</span>
                  )}
                  {session.duration_ms != null && (
                    <span>{formatDuration(session.duration_ms)}</span>
                  )}
                </div>
              </div>
              <span className={styles.cost}>{formatCost(parseFloat(session.estimated_cost_usd))}</span>
            </a>
          ))}
        </div>
      </TrendsCard>
    </div>
  );
}
