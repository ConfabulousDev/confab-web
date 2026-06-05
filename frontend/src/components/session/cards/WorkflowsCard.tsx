import { CardWrapper, CardLoading, CardError } from './Card';
import { BranchIcon, RobotIcon, TokenIcon, DurationIcon, CheckCircleIcon } from '@/components/icons';
import type { WorkflowsCardData } from '@/schemas/api';
import type { CardProps } from './types';
import { formatCost, formatTokenCount } from '@/utils/tokenStats';
import { formatDuration } from '@/utils/formatting';
import styles from './WorkflowsCard.module.css';

/**
 * Workflows card (CF-534): one row per Claude Code workflow run, grouped
 * server-side by runId. Shows per-run agent count, a token subtotal + cost, an
 * activity-span duration, and — when a run journal was uploaded — a
 * succeeded/total completion count. Runs arrive already ordered by start time;
 * they are labelled "Run 1…N" with the opaque runId in a hover title.
 */
export function WorkflowsCard({ data, loading, error }: CardProps<WorkflowsCardData>) {
  if (error && !data) {
    return <CardError title="Workflows" error={error} icon={BranchIcon} />;
  }

  if (loading && !data) {
    return (
      <CardWrapper title="Workflows" icon={BranchIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data || data.runs.length === 0) return null;

  return (
    <CardWrapper title="Workflows" icon={BranchIcon}>
      {data.runs.map((run, i) => {
        const totalTokens =
          run.input_tokens + run.output_tokens + run.cache_creation + run.cache_read;
        const agentLabel = run.agent_count === 1 ? 'agent' : 'agents';
        return (
          <div key={run.run_id} className={styles.run} title={run.run_id}>
            <div className={styles.runHeader}>
              <span className={styles.runLabel}>Run {i + 1}</span>
              {run.has_journal && (
                <span
                  className={styles.runStatus}
                  title="Agents with a journal result line (incomplete agents may have errored or still be running)"
                >
                  <span className={styles.statusIcon}>{CheckCircleIcon}</span>
                  {run.succeeded_agents}/{run.agent_count} completed
                </span>
              )}
            </div>
            <div className={styles.runMeta}>
              <span className={styles.metaItem} title="Subagents in this run">
                <span className={styles.metaIcon}>{RobotIcon}</span>
                {run.agent_count} {agentLabel}
              </span>
              <span className={styles.metaItem} title="Total tokens (input + output + cache)">
                <span className={styles.metaIcon}>{TokenIcon}</span>
                {formatTokenCount(totalTokens)}
              </span>
              <span className={styles.metaItem} title="Estimated cost for this run">
                {formatCost(parseFloat(run.estimated_usd))}
              </span>
              {run.duration_ms > 0 && (
                <span className={styles.metaItem} title="Run duration (first to last agent activity)">
                  <span className={styles.metaIcon}>{DurationIcon}</span>
                  {formatDuration(run.duration_ms)}
                </span>
              )}
            </div>
          </div>
        );
      })}
    </CardWrapper>
  );
}
