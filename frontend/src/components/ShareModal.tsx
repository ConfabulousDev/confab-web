import styles from './ShareModal.module.css';

interface ShareModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function ShareModal({ isOpen, onClose }: ShareModalProps) {
  if (!isOpen) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
          ×
        </button>
        <h2 className={styles.title}>Share sessions with your team</h2>
        <p className={styles.subtitle}>
          Generate shareable links for collaboration
        </p>
        <img
          src="/share.png"
          alt="Confabulous share links interface"
          className={styles.image}
        />
      </div>
    </div>
  );
}

export default ShareModal;
