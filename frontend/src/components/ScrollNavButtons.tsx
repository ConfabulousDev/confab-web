import { useEffect, useState, useCallback, type RefObject } from 'react';
import styles from './ScrollNavButtons.module.css';

interface ScrollNavButtonsProps {
  scrollRef: RefObject<HTMLElement | null>;
  /** Threshold in pixels before showing the top button (default: 100) */
  threshold?: number;
  /** Custom handler for scrolling to top (useful for virtualized lists) */
  onScrollToTop?: () => void;
  /** Custom handler for scrolling to bottom (useful for virtualized lists) */
  onScrollToBottom?: () => void;
  /** Called when at-bottom state changes (useful for auto-scroll on new content) */
  onAtBottomChange?: (atBottom: boolean) => void;
  /** Dependency value that triggers button visibility re-evaluation when changed */
  contentDependency?: number;
}

/**
 * Floating navigation buttons for scrolling to top/bottom of a container.
 * Attaches to a scrollable element via ref and shows/hides based on scroll position.
 */
function ScrollNavButtons({
  scrollRef,
  threshold = 100,
  onScrollToTop,
  onScrollToBottom,
  onAtBottomChange,
  contentDependency,
}: ScrollNavButtonsProps) {
  const [showTopButton, setShowTopButton] = useState(false);
  const [showBottomButton, setShowBottomButton] = useState(false);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) return;

    let lastAtBottom: boolean | null = null;

    const updateButtonVisibility = () => {
      const { scrollTop, scrollHeight, clientHeight } = scrollElement;
      const atTop = scrollTop < threshold;
      const atBottom = scrollTop + clientHeight >= scrollHeight - threshold;

      setShowTopButton(!atTop);
      setShowBottomButton(!atBottom);

      // Notify parent of atBottom state changes
      if (onAtBottomChange && atBottom !== lastAtBottom) {
        lastAtBottom = atBottom;
        onAtBottomChange(atBottom);
      }
    };

    scrollElement.addEventListener('scroll', updateButtonVisibility);

    // Use ResizeObserver to detect when content size changes
    const resizeObserver = new ResizeObserver(updateButtonVisibility);
    resizeObserver.observe(scrollElement);

    updateButtonVisibility(); // Initial check

    return () => {
      scrollElement.removeEventListener('scroll', updateButtonVisibility);
      resizeObserver.disconnect();
    };
  }, [scrollRef, threshold, onAtBottomChange, contentDependency]);

  const scrollToTop = useCallback(() => {
    if (onScrollToTop) {
      onScrollToTop();
    } else {
      scrollRef.current?.scrollTo({ top: 0 });
    }
  }, [scrollRef, onScrollToTop]);

  const scrollToBottom = useCallback(() => {
    if (onScrollToBottom) {
      onScrollToBottom();
    } else if (scrollRef.current) {
      scrollRef.current.scrollTo({ top: scrollRef.current.scrollHeight });
    }
  }, [scrollRef, onScrollToBottom]);

  if (!showTopButton && !showBottomButton) {
    return null;
  }

  return (
    <div className={styles.navButtons}>
      {showTopButton && (
        <button
          className={styles.navButton}
          onClick={scrollToTop}
          title="Go to top"
          aria-label="Go to top"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="17 11 12 6 7 11" />
            <polyline points="17 18 12 13 7 18" />
          </svg>
        </button>
      )}
      {showBottomButton && (
        <button
          className={styles.navButton}
          onClick={scrollToBottom}
          title="Go to bottom"
          aria-label="Go to bottom"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="7 13 12 18 17 13" />
            <polyline points="7 6 12 11 17 6" />
          </svg>
        </button>
      )}
    </div>
  );
}

export default ScrollNavButtons;
