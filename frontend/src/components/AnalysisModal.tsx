import styles from './AnalysisModal.module.css';
import ThemedImage from './ThemedImage';

interface AnalysisModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function AnalysisModal({ isOpen, onClose }: AnalysisModalProps) {
  if (!isOpen) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
          ×
        </button>
        <h2 className={styles.title}>Session Analytics</h2>
        <p className={styles.subtitle}>
          Track metrics, timing, and code activity for every session
        </p>
        <ThemedImage
          src="/analysis.png"
          alt="Session analytics showing duration, messages, Claude utilization, and code activity metrics"
          className={styles.image}
        />
      </div>
    </div>
  );
}

export default AnalysisModal;
