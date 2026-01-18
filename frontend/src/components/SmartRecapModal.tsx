import Modal from './Modal';
import ThemedImage from './ThemedImage';
import styles from './SmartRecapModal.module.css';

interface SmartRecapModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function SmartRecapModal({ isOpen, onClose }: SmartRecapModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Smart Recap">
      <h2 className={styles.title}>Smart Recap</h2>
      <p className={styles.subtitle}>
        AI-powered session insights with actionable feedback
      </p>
      <ThemedImage
        src="/smart-recap.png"
        alt="Confabulous Smart Recap showing session analysis"
        className={styles.image}
      />
      <div className={styles.linkWrapper}>
        <a
          href="https://confabulous.dev/sessions/3b46a065-6d73-4035-9564-089b0c372806"
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

export default SmartRecapModal;
