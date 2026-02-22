import Modal from './Modal';
import styles from './SelfHostedModal.module.css';

interface SelfHostedModalProps {
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

function SelfHostedModal({ isOpen, onClose }: SelfHostedModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Self-hosted">
      <h2 className={styles.title}>Self-Hosted</h2>
      <p className={styles.subtitle}>
        Your infrastructure, your data, your rules
      </p>
      <div className={styles.features}>
        <FeatureItem
          icon="🏠"
          title="Your Data, Your Servers"
          description="Session data never leaves your infrastructure. No third-party analytics, no external telemetry."
        />
        <FeatureItem
          icon="📖"
          title="Open Source"
          description="MIT licensed. Inspect, modify, and extend the entire codebase to fit your needs."
        />
        <FeatureItem
          icon="🔧"
          title="Configurable"
          description="Bring your own database, object storage, OAuth provider, and domain."
        />
      </div>
      <a
        href="https://github.com/ConfabulousDev/confab-web"
        target="_blank"
        rel="noopener noreferrer"
        className={styles.githubLink}
      >
        View on GitHub &rarr;
      </a>
    </Modal>
  );
}

export default SelfHostedModal;
