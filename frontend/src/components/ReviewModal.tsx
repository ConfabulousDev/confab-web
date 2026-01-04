import Modal from './Modal';
import ThemedImage from './ThemedImage';
import styles from './ReviewModal.module.css';

interface ReviewModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function ReviewModal({ isOpen, onClose }: ReviewModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Review your sessions">
      <h2 className={styles.title}>Review your sessions</h2>
      <p className={styles.subtitle}>
        Browse conversations with full context and history
      </p>
      <ThemedImage
        src="/review.png"
        alt="Confabulous session transcript view"
        className={styles.image}
      />
      <div className={styles.linkWrapper}>
        <a
          href="https://confabulous.dev/sessions/3b46a065-6d73-4035-9564-089b0c372806?tab=transcript"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.link}
        >
          View full session
        </a>
      </div>
    </Modal>
  );
}

export default ReviewModal;
