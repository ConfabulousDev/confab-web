import { useAnalyticsPolling } from '@/hooks/useAnalyticsPolling';
import { RelativeTime } from '@/components/RelativeTime';
import type { SessionAnalytics, GitHubLink, AnalyticsCards } from '@/schemas/api';
import { getOrderedCards } from './cards';
import GitHubLinksCard from './GitHubLinksCard';
import styles from './SessionSummaryPanel.module.css';

interface SessionSummaryPanelProps {
  sessionId: string;
  isOwner: boolean;
  /** For Storybook: pass analytics directly instead of fetching from API */
  initialAnalytics?: SessionAnalytics;
  /** For Storybook: pass GitHub links directly instead of fetching from API */
  initialGithubLinks?: GitHubLink[];
}

function SessionSummaryPanel({ sessionId, isOwner, initialAnalytics, initialGithubLinks }: SessionSummaryPanelProps) {
  // Use polling hook for live updates (disabled in Storybook mode)
  const { analytics: polledAnalytics, loading, error } = useAnalyticsPolling(
    sessionId,
    initialAnalytics === undefined // Disable polling in Storybook mode
  );

  // Use initial analytics for Storybook, polled analytics for real usage
  const analytics = initialAnalytics ?? polledAnalytics;

  // Get cards data from the new cards-based format
  const cards: Partial<AnalyticsCards> = analytics?.cards ?? {};

  // Get ordered cards from registry
  const orderedCards = getOrderedCards();

  // Render analytics cards using the registry
  const renderAnalyticsCards = () => {
    if (loading && !analytics) {
      return (
        <div className={styles.card}>
          <div className={styles.cardContent}>
            <div className={styles.loading}>Loading analytics...</div>
          </div>
        </div>
      );
    }

    if (error && !analytics) {
      return (
        <div className={styles.card}>
          <div className={styles.cardContent}>
            <div className={styles.analyticsError}>Failed to load analytics</div>
          </div>
        </div>
      );
    }

    if (!analytics) {
      return (
        <div className={styles.card}>
          <div className={styles.cardContent}>
            <div className={styles.analyticsEmpty}>No analytics available</div>
          </div>
        </div>
      );
    }

    return (
      <>
        {orderedCards.map((cardDef) => {
          const CardComponent = cardDef.component;
          const cardData = cards[cardDef.key] ?? null;
          return (
            <CardComponent
              key={cardDef.key}
              data={cardData}
              loading={loading}
            />
          );
        })}
      </>
    );
  };

  return (
    <div className={styles.panel}>
      <div className={styles.header}>
        <h2 className={styles.title}>Session Summary</h2>
        {analytics && (
          <div className={styles.lastUpdated} title="When analytics were last computed">
            Updated <RelativeTime date={analytics.computed_at} />
          </div>
        )}
      </div>

      <div className={styles.grid}>
        {/* GitHub Links - always rendered, independent of analytics */}
        <GitHubLinksCard
          sessionId={sessionId}
          isOwner={isOwner}
          initialLinks={initialGithubLinks}
        />

        {renderAnalyticsCards()}
      </div>
    </div>
  );
}

export default SessionSummaryPanel;
