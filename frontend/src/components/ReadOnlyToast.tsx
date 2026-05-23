// CF-483: ReadOnlyToast listens for the `confab:read-only` CustomEvent
// dispatched by api.ts on read_only_user error responses and shows a
// transient toast. Mounted by AppLayout. Single toast at a time;
// re-firing while visible resets the dismiss timer (debounced replace).

import { useEffect, useRef, useState } from 'react';
import { READ_ONLY_EVENT } from '@/utils/demoIdentity';
import styles from './ReadOnlyToast.module.css';

export const TOAST_TEXT = 'This is a read-only demo.';
export const TOAST_DURATION_MS = 3000;

function ReadOnlyToast() {
  const [visible, setVisible] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    function onReadOnly() {
      if (timerRef.current !== null) clearTimeout(timerRef.current);
      setVisible(true);
      timerRef.current = setTimeout(() => {
        setVisible(false);
        timerRef.current = null;
      }, TOAST_DURATION_MS);
    }
    window.addEventListener(READ_ONLY_EVENT, onReadOnly);
    return () => {
      window.removeEventListener(READ_ONLY_EVENT, onReadOnly);
      if (timerRef.current !== null) clearTimeout(timerRef.current);
    };
  }, []);

  if (!visible) return null;
  return (
    <div className={styles.toast} role="status" aria-live="polite">
      {TOAST_TEXT}
    </div>
  );
}

export default ReadOnlyToast;
