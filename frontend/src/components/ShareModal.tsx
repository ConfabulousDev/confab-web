import Modal from './Modal';
import ThemedImage from './ThemedImage';
import styles from './ShareModal.module.css';

interface ShareModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function ShareModal({ isOpen, onClose }: ShareModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Share sessions with your team">
      <h2 className={styles.title}>Share sessions with your team</h2>
      <p className={styles.subtitle}>
        Generate shareable links for collaboration
      </p>
      <ThemedImage
        src="/share.png"
        alt="Confabulous share links interface"
        className={styles.image}
      />
    </Modal>
  );
}

export default ShareModal;
