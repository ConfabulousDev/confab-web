import { useState } from 'react';
import Modal from './Modal';
import ThemedImage from './ThemedImage';
import { useTheme } from '@/hooks/useTheme';
import styles from './PRLinkingModal.module.css';

interface PRLinkingModalProps {
  isOpen: boolean;
  onClose: () => void;
}

interface ZoomableImageProps {
  src: string;
  alt: string;
  className?: string;
}

function ZoomableImage({ src, alt, className }: ZoomableImageProps) {
  const [showZoom, setShowZoom] = useState(false);
  const [darkFailed, setDarkFailed] = useState(false);
  const { resolvedTheme } = useTheme();

  const darkSrc = src.replace(/(\.[^.]+)$/, '-dark$1');
  const shouldUseDark = resolvedTheme === 'dark' && !darkFailed;
  const themedSrc = shouldUseDark ? darkSrc : src;

  const handleError = () => {
    if (shouldUseDark) {
      setDarkFailed(true);
    }
  };

  return (
    <>
      <img
        src={themedSrc}
        alt={alt}
        className={`${className} ${styles.zoomable}`}
        onClick={() => setShowZoom(true)}
        onError={handleError}
      />
      {showZoom && (
        <div className={styles.zoomOverlay} onClick={() => setShowZoom(false)}>
          <div className={styles.zoomPopup}>
            <img src={themedSrc} alt={alt} />
          </div>
        </div>
      )}
    </>
  );
}

function GitHubIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
    </svg>
  );
}

function ConfabIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className}>
      <rect width="32" height="32" rx="6" fill="#1a1a1a"/>
      <text x="16" y="24" fontFamily="Lobster, cursive" fontSize="22" fill="#ffffff" textAnchor="middle">C</text>
    </svg>
  );
}

function PRLinkingModal({ isOpen, onClose }: PRLinkingModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Two-way linking between Confabulous and GitHub">
      <h2 className={styles.title}>Two-way linking between Confabulous and GitHub</h2>

      <div className={styles.sections}>
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>
            <ConfabIcon className={styles.icon} />
            <span className={styles.confabName}>Confabulous</span>
          </h3>
          <p className={styles.sectionDesc}>
            Session links PRs and commits
          </p>
          <ThemedImage
            src="/confab-to-github.png"
            alt="Confabulous session showing linked GitHub PRs and commits"
            className={`${styles.image} ${styles.imageSmall}`}
          />
        </div>

        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>
            <GitHubIcon className={styles.icon} />
            GitHub
          </h3>

          <div className={styles.subsection}>
            <p className={styles.sectionDesc}>PR links to session in Confabulous</p>
            <ThemedImage
              src="/github-to-confab.png"
              alt="GitHub PR with Confabulous link"
              className={`${styles.image} ${styles.imageSmall}`}
            />
          </div>

          <div className={styles.subsection}>
            <p className={styles.sectionDesc}>Commit too</p>
            <ZoomableImage
              src="/github-to-confab-commit.png"
              alt="GitHub commit with Confabulous link in commit message"
              className={styles.image}
            />
          </div>
        </div>
      </div>
    </Modal>
  );
}

export default PRLinkingModal;
