import { useState, useCallback, useEffect } from 'react';
import { useDropdown } from '@/hooks';
import { useAnalyticsPolling } from '@/hooks/useAnalyticsPolling';
import { analyticsAPI, APIError } from '@/services/api';
import { RelativeTime } from '@/components/RelativeTime';
import Alert from '@/components/Alert';
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
  const { analytics: polledAnalytics, loading, error, forceRefetch } = useAnalyticsPolling(
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
  const [regenerateError, setRegenerateError] = useState<string | null>(null);

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
    const title = analytics?.suggested_session_title;
    if (title && onSuggestedTitleChange) {
      onSuggestedTitleChange(title);
    }
  }, [analytics?.suggested_session_title, onSuggestedTitleChange]);

  // Handle Smart Recap regeneration (owner only)
  // Generation is synchronous - this call blocks until the LLM completes (~60-90s)
  const handleRegenerateSmartRecap = useCallback(async () => {
    if (isRegenerating || initialAnalytics !== undefined) return; // Disabled in Storybook mode
    setIsRegenerating(true);
    setRegenerateError(null); // Clear any previous error
    try {
      // Regeneration is synchronous - waits for LLM to complete
      await analyticsAPI.regenerateSmartRecap(sessionId);
      // Force a fresh fetch to get the newly generated card
      await forceRefetch();
    } catch (err) {
      if (err instanceof APIError) {
        if (err.status === 409) {
          // Generation already in progress - not an error to show user
          console.log('Smart recap generation already in progress');
        } else if (err.status === 403) {
          // Quota exceeded
          setRegenerateError('Monthly recap limit reached. Try again next month.');
        } else {
          console.error('Failed to regenerate smart recap:', err);
          setRegenerateError('Failed to regenerate recap. Please try again.');
        }
      } else {
        console.error('Failed to regenerate smart recap:', err);
        setRegenerateError('Failed to regenerate recap. Please try again.');
      }
    } finally {
      setIsRegenerating(false);
    }
  }, [sessionId, isRegenerating, initialAnalytics, forceRefetch]);

  // Get cards data from the new cards-based format
  const cards: Partial<AnalyticsCards> = analytics?.cards ?? {};

  // Get per-card errors for graceful degradation
  const cardErrors: Record<string, string> = analytics?.card_errors ?? {};

  // Get ordered cards from registry
  const orderedCards = getOrderedCards();

  // Render analytics cards using the registry
  const renderAnalyticsCards = () => {
    if (loading && !analytics) {
      return (
        <div className={styles.card}>
          <div className={styles.cardContent}>
            <div className={styles.loading}>
              <div className={styles.loadingSpinner} />
              <div className={styles.loadingTitle}>Loading analytics...</div>
              <div className={styles.loadingSubtitle}>
                First load includes generating Smart Recap with AI
              </div>
            </div>
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
          const cardError = cardErrors[cardDef.key];

          // Skip rendering wrapper if card wouldn't render (avoids empty grid cells)
          // But always render if there's an error (to show the error state)
          if (!cardError && cardDef.shouldRender && !loading && !cardDef.shouldRender(cardData)) {
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
                error={cardError}
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

      {regenerateError && (
        <Alert variant="error" onClose={() => setRegenerateError(null)}>
          {regenerateError}
        </Alert>
      )}

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
