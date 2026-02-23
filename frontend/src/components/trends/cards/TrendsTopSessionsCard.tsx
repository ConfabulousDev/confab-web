import { TrendsCard } from './TrendsCard';
import { TrendingUpIcon } from '@/components/icons';
import { formatDuration } from '@/utils';
import type { TrendsTopSessionsCard as TrendsTopSessionsCardData } from '@/schemas/api';
import styles from './TrendsTopSessionsCard.module.css';

interface TrendsTopSessionsCardProps {
  data: TrendsTopSessionsCardData | null;
}

function formatCost(usd: string): string {
  const value = parseFloat(usd);
  if (value >= 1) {
    return `$${value.toFixed(2)}`;
  }
  return `$${value.toFixed(4)}`;
}

/** Strip org prefix from "org/repo" → "repo" */
function formatRepoName(repo: string): string {
  const parts = repo.split('/');
  return parts.length > 1 ? parts[parts.length - 1]! : repo;
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
              <span className={styles.cost}>{formatCost(session.estimated_cost_usd)}</span>
            </a>
          ))}
        </div>
      </TrendsCard>
    </div>
  );
}
