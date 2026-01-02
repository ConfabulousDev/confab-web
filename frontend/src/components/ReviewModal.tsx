import styles from './ReviewModal.module.css';

interface ReviewModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function ReviewModal({ isOpen, onClose }: ReviewModalProps) {
  if (!isOpen) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
          ×
        </button>
        <h2 className={styles.title}>Review your sessions</h2>
        <p className={styles.subtitle}>
          Browse conversations with full context and history
        </p>
        <img
          src="/review.png"
          alt="Confabulous session transcript view"
          className={styles.image}
        />
        <a
          href="https://confabulous.dev/sessions/3b46a065-6d73-4035-9564-089b0c372806"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.link}
        >
          View this session live
        </a>
      </div>
    </div>
  );
}

export default ReviewModal;
