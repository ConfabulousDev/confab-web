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
}: ScrollNavButtonsProps) {
  const [showTopButton, setShowTopButton] = useState(false);
  const [showBottomButton, setShowBottomButton] = useState(false);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) return;

    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = scrollElement;
      const atTop = scrollTop < threshold;
      const atBottom = scrollTop + clientHeight >= scrollHeight - threshold;

      setShowTopButton(!atTop);
      setShowBottomButton(!atBottom);
    };

    scrollElement.addEventListener('scroll', handleScroll);
    handleScroll(); // Initial check

    return () => scrollElement.removeEventListener('scroll', handleScroll);
  }, [scrollRef, threshold]);

  const scrollToTop = useCallback(() => {
    if (onScrollToTop) {
      onScrollToTop();
    } else {
      scrollRef.current?.scrollTo({ top: 0, behavior: 'smooth' });
    }
  }, [scrollRef, onScrollToTop]);

  const scrollToBottom = useCallback(() => {
    if (onScrollToBottom) {
      onScrollToBottom();
    } else if (scrollRef.current) {
      scrollRef.current.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
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
