import { useState } from 'react';
import QuickstartModal from './QuickstartModal';
import styles from './QuickstartCTA.module.css';

const DISMISS_KEY = 'quickstart-cta-dismissed';

interface QuickstartCTAProps {
  show: boolean;
}

function QuickstartCTA({ show }: QuickstartCTAProps) {
  const [dismissed, setDismissed] = useState(() => {
    return localStorage.getItem(DISMISS_KEY) === 'true';
  });
  const [modalOpen, setModalOpen] = useState(false);

  if (!show || dismissed) {
    return null;
  }

  const handleDismiss = () => {
    localStorage.setItem(DISMISS_KEY, 'true');
    setDismissed(true);
  };

  return (
    <>
      <div className={styles.banner}>
        <div className={styles.content}>
          <span className={styles.message}>
            Set up session syncing to track your own Claude Code sessions.
          </span>
          <button
            className={styles.setupBtn}
            onClick={() => setModalOpen(true)}
          >
            Get started
          </button>
        </div>
        <button
          className={styles.dismissBtn}
          onClick={handleDismiss}
          aria-label="Dismiss"
        >
          &times;
        </button>
      </div>
      <QuickstartModal isOpen={modalOpen} onClose={() => setModalOpen(false)} />
    </>
  );
}

export default QuickstartCTA;
