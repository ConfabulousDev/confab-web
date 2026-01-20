import Modal from './Modal';
import styles from './PrivacyModal.module.css';

interface PrivacyModalProps {
  isOpen: boolean;
  onClose: () => void;
}

interface FeatureItemProps {
  icon: string;
  title: string;
  description: string;
}

function FeatureItem({ icon, title, description }: FeatureItemProps) {
  return (
    <div className={styles.featureItem}>
      <span className={styles.featureIcon}>{icon}</span>
      <div className={styles.featureContent}>
        <span className={styles.featureTitle}>{title}</span>
        <span className={styles.featureDescription}>{description}</span>
      </div>
    </div>
  );
}

function PrivacyModal({ isOpen, onClose }: PrivacyModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Privacy and security">
      <h2 className={styles.title}>Privacy & Security</h2>
      <p className={styles.subtitle}>
        Your session data is protected at every step
      </p>
      <div className={styles.features}>
        <FeatureItem
          icon="🔐"
          title="Encrypted at Rest"
          description="All stored data is encrypted using industry-standard encryption."
        />
        <FeatureItem
          icon="🔒"
          title="Encrypted in Transit"
          description="All network traffic uses TLS encryption to protect your data."
        />
        <FeatureItem
          icon="👤"
          title="Private by Default"
          description="Your sessions are only accessible to you unless you explicitly share them with others."
        />
      </div>
    </Modal>
  );
}

export default PrivacyModal;
