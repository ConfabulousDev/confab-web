import Quickstart from './Quickstart';
import styles from './QuickstartModal.module.css';

interface QuickstartModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function QuickstartModal({ isOpen, onClose }: QuickstartModalProps) {
  if (!isOpen) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
          ×
        </button>
        <Quickstart />
      </div>
    </div>
  );
}

export default QuickstartModal;
