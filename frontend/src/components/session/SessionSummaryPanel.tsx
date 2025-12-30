import { useState } from 'react';
import { useDropdown } from '@/hooks';
import { useAnalyticsPolling } from '@/hooks/useAnalyticsPolling';
import { RelativeTime } from '@/components/RelativeTime';
import { MoreVerticalIcon, GitHubIcon } from '@/components/icons';
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

  // State for revealing GitHub card when empty
  const [showGitHubCard, setShowGitHubCard] = useState(false);
  const [hasGitHubLinks, setHasGitHubLinks] = useState(
    (initialGithubLinks?.length ?? 0) > 0
  );

  // Dropdown for actions menu
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  // Handle "Link to GitHub" menu action
  const handleLinkToGitHub = () => {
    setShowGitHubCard(true);
    toggle();
  };

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
          const spanClass = cardDef.span === 2 ? styles.span2 : cardDef.span === 3 ? styles.span3 : undefined;
          return (
            <div key={cardDef.key} className={spanClass}>
              <CardComponent
                data={cardData}
                loading={loading}
              />
            </div>
          );
        })}
      </>
    );
  };

  return (
    <div className={styles.panel}>
      <div className={styles.header}>
        <h2 className={styles.title}>Session Summary</h2>
        <div className={styles.headerRight}>
          {analytics && (
            <div className={styles.lastUpdated} title="When analytics were last computed">
              Updated <RelativeTime date={analytics.computed_at} />
            </div>
          )}
          {isOwner && (
            <div className={styles.menuContainer} ref={containerRef}>
              <button
                className={styles.menuButton}
                onClick={toggle}
                title="Actions"
                aria-label="Actions menu"
                aria-expanded={isOpen}
              >
                {MoreVerticalIcon}
              </button>
              {isOpen && (
                <div className={styles.menuDropdown}>
                  <button
                    className={styles.menuItem}
                    onClick={handleLinkToGitHub}
                    disabled={hasGitHubLinks || showGitHubCard}
                  >
                    <span className={styles.menuItemIcon}>{GitHubIcon}</span>
                    Link to GitHub
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <div className={styles.grid}>
        {/* GitHub Links - hidden by default when empty for owners */}
        <GitHubLinksCard
          sessionId={sessionId}
          isOwner={isOwner}
          initialLinks={initialGithubLinks}
          forceShow={showGitHubCard}
          onLinksChange={setHasGitHubLinks}
        />

        {renderAnalyticsCards()}
      </div>
    </div>
  );
}

export default SessionSummaryPanel;
