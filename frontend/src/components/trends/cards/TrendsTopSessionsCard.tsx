import { TrendsCard } from './TrendsCard';
import { ChatIcon, TrendingUpIcon } from '@/components/icons';
import { getProviderMetadataOrFallback } from '@/utils/providers';
import { formatDuration } from '@/utils';
import { formatCost } from '@/utils/tokenStats';
import type { TrendsTopSessionsCard as TrendsTopSessionsCardData } from '@/schemas/api';
import styles from './TrendsTopSessionsCard.module.css';

interface TrendsTopSessionsCardProps {
  data: TrendsTopSessionsCardData | null;
}

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

export function TrendsTopSessionsCard({ data }: TrendsTopSessionsCardProps) {
  if (!data || data.sessions.length === 0) return null;

  const subtitle = `Top ${data.sessions.length} by cost`;

  return (
    <div className={styles.wrapper}>
      <TrendsCard title="Costliest Sessions" icon={TrendingUpIcon} subtitle={subtitle}>
        <div className={styles.sessionList}>
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
