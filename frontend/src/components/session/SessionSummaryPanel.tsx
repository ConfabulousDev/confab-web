import { useState, useCallback, useEffect } from 'react';
import { useDropdown } from '@/hooks';
import { useAnalyticsPolling } from '@/hooks/useAnalyticsPolling';
import { analyticsAPI } from '@/services/api';
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
  /** Callback when a suggested title arrives from Smart Recap */
  onSuggestedTitleChange?: (title: string) => void;
}

function SessionSummaryPanel({ sessionId, isOwner, initialAnalytics, initialGithubLinks, onSuggestedTitleChange }: SessionSummaryPanelProps) {
  // Use polling hook for live updates (disabled in Storybook mode)
  const { analytics: polledAnalytics, loading, error, refetch } = useAnalyticsPolling(
    sessionId,
    initialAnalytics === undefined // Disable polling in Storybook mode
  );

  // Use initial analytics for Storybook, polled analytics for real usage
  const analytics = initialAnalytics ?? polledAnalytics;

  // State for revealing GitHub card - default to true if there are initial links
  const hasInitialLinks = (initialGithubLinks?.length ?? 0) > 0;
  const [showGitHubCard, setShowGitHubCard] = useState(hasInitialLinks);

  // State for Smart Recap regeneration
  const [isRegenerating, setIsRegenerating] = useState(false);

  // Dropdown for actions menu
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  // Toggle GitHub card visibility
  const handleToggleGitHubCard = () => {
    setShowGitHubCard(!showGitHubCard);
    toggle();
  };

  // Auto-show card when links are fetched from API
  const handleHasLinksChange = useCallback((hasLinks: boolean) => {
    if (hasLinks) {
      setShowGitHubCard(true);
    }
  }, []);

  // Notify parent when suggested title arrives from analytics
  useEffect(() => {
    if (analytics?.suggested_session_title && onSuggestedTitleChange) {
      onSuggestedTitleChange(analytics.suggested_session_title);
    }
  }, [analytics?.suggested_session_title, onSuggestedTitleChange]);

  // Handle Smart Recap regeneration (owner only)
  const handleRegenerateSmartRecap = useCallback(async () => {
    if (isRegenerating || initialAnalytics !== undefined) return; // Disabled in Storybook mode
    setIsRegenerating(true);
    try {
      await analyticsAPI.regenerateSmartRecap(sessionId);
      // Trigger a refetch to get the "generating" state and start polling
      await refetch();
    } catch (err) {
      console.error('Failed to regenerate smart recap:', err);
    } finally {
      setIsRegenerating(false);
    }
  }, [sessionId, isRegenerating, initialAnalytics, refetch]);

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

          // Skip rendering wrapper if card wouldn't render (avoids empty grid cells)
          if (cardDef.shouldRender && !loading && !cardDef.shouldRender(cardData)) {
            return null;
          }

          const spanClass = cardDef.span === 'full' ? styles.spanFull
            : cardDef.span === 2 ? styles.span2
            : cardDef.span === 3 ? styles.span3
            : '';
          const sizeClass = cardDef.size === 'compact' ? styles.sizeCompact
            : cardDef.size === 'tall' ? styles.sizeTall
            : styles.sizeStandard;

          // Build additional props for specific cards
          const extraProps: Record<string, unknown> = {};
          if (cardDef.key === 'smart_recap' && isOwner) {
            // Only show quota to session owner (private info)
            extraProps.quota = analytics?.smart_recap_quota;
            // Provide refresh capability to owners
            extraProps.onRefresh = handleRegenerateSmartRecap;
            extraProps.isRefreshing = isRegenerating;
          }

          return (
            <div key={cardDef.key} className={`${spanClass} ${sizeClass}`.trim()}>
              <CardComponent
                data={cardData}
                loading={loading}
                {...extraProps}
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
                    onClick={handleToggleGitHubCard}
                  >
                    <span className={styles.menuItemIcon}>{GitHubIcon}</span>
                    <span className={styles.menuItemLabel}>Show GitHub card</span>
                    <span className={`${styles.toggle} ${showGitHubCard ? styles.on : ''}`}>
                      <span className={styles.toggleKnob} />
                    </span>
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <div className={styles.grid}>
        {renderAnalyticsCards()}

        {/* GitHub Links - visibility controlled by toggle for owners */}
        <GitHubLinksCard
          sessionId={sessionId}
          isOwner={isOwner}
          initialLinks={initialGithubLinks}
          forceShow={showGitHubCard}
          onHasLinksChange={handleHasLinksChange}
        />
      </div>
    </div>
  );
}

export default SessionSummaryPanel;
