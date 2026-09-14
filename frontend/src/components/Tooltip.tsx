// we3k D5: shared themed tooltip for a single focusable trigger, replacing the
// native `title` attribute where the browser-chrome look is unwanted.
//
// Shows after a short hover delay, or immediately on keyboard focus (a focus
// that follows a pointer press is a click and shows nothing). Hides on pointer
// leave / press, blur, Escape and any scroll. Rendered through a fixed-position
// portal so scroll containers and their overflow masks never clip it. Events
// are handled on a `display: contents` wrapper, so the trigger's own props are
// untouched apart from `aria-describedby`.

import {
  cloneElement,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactElement,
  type ReactNode,
} from 'react';
import { createPortal } from 'react-dom';
import styles from './Tooltip.module.css';

const HOVER_DELAY_MS = 300;
const GAP_PX = 6;
const VIEWPORT_MARGIN_PX = 8;

interface TooltipProps {
  content: ReactNode;
  /** A single focusable element. */
  children: ReactElement<{ 'aria-describedby'?: string }>;
  /** Suppress the tooltip, e.g. while a menu anchored to the trigger is open. */
  disabled?: boolean;
}

function triggerRect(wrapper: Element): DOMRect | null {
  return wrapper.firstElementChild?.getBoundingClientRect() ?? null;
}

export default function Tooltip({ content, children, disabled = false }: TooltipProps) {
  const id = useId();
  const [anchor, setAnchor] = useState<DOMRect | null>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const hoverTimerRef = useRef<number | undefined>(undefined);
  const pointerPressedRef = useRef(false);

  const hide = useCallback(() => {
    window.clearTimeout(hoverTimerRef.current);
    setAnchor(null);
  }, []);

  const visible = anchor !== null && !disabled;

  useEffect(() => () => window.clearTimeout(hoverTimerRef.current), []);

  useEffect(() => {
    if (!visible) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') hide();
    }
    document.addEventListener('keydown', handleKeyDown);
    window.addEventListener('scroll', hide, true);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('scroll', hide, true);
    };
  }, [visible, hide]);

  // Center below the trigger, clamped to the viewport; flip above when there is no room below.
  useLayoutEffect(() => {
    const el = tooltipRef.current;
    if (!visible || !el || !anchor) return;
    const { width, height } = el.getBoundingClientRect();
    const centered = anchor.left + anchor.width / 2 - width / 2;
    const left = Math.max(VIEWPORT_MARGIN_PX, Math.min(centered, window.innerWidth - width - VIEWPORT_MARGIN_PX));
    const below = anchor.bottom + GAP_PX;
    const fitsBelow = below + height <= window.innerHeight - VIEWPORT_MARGIN_PX;
    el.style.left = `${left}px`;
    el.style.top = `${fitsBelow ? below : Math.max(VIEWPORT_MARGIN_PX, anchor.top - GAP_PX - height)}px`;
  }, [visible, anchor, content]);

  return (
    <>
      <span
        className={styles.trigger}
        onPointerEnter={(event) => {
          if (event.pointerType === 'touch') return;
          const wrapper = event.currentTarget;
          window.clearTimeout(hoverTimerRef.current);
          hoverTimerRef.current = window.setTimeout(() => setAnchor(triggerRect(wrapper)), HOVER_DELAY_MS);
        }}
        onPointerLeave={hide}
        onPointerDown={() => {
          pointerPressedRef.current = true;
          hide();
        }}
        onFocus={(event) => {
          if (pointerPressedRef.current) {
            pointerPressedRef.current = false;
            return;
          }
          window.clearTimeout(hoverTimerRef.current);
          setAnchor(triggerRect(event.currentTarget));
        }}
        onBlur={() => {
          pointerPressedRef.current = false;
          hide();
        }}
      >
        {cloneElement(children, { 'aria-describedby': visible ? id : children.props['aria-describedby'] })}
      </span>
      {visible &&
        createPortal(
          <div
            ref={tooltipRef}
            id={id}
            role="tooltip"
            className={styles.tooltip}
            style={{ top: anchor.bottom + GAP_PX, left: anchor.left }}
          >
            {content}
          </div>,
          document.body,
        )}
    </>
  );
}
