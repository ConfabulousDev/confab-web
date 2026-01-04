import Modal from './Modal';
import ThemedImage from './ThemedImage';
import styles from './AnalysisModal.module.css';

interface AnalysisModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function AnalysisModal({ isOpen, onClose }: AnalysisModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Session Analytics">
      <h2 className={styles.title}>Session Analytics</h2>
      <p className={styles.subtitle}>
        Track metrics, timing, and code activity for every session
      </p>
      <ThemedImage
        src="/analysis.png"
        alt="Session analytics showing duration, messages, Claude utilization, and code activity metrics"
        className={styles.image}
      />
    </Modal>
  );
}

export default AnalysisModal;
