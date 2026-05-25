import FeatureItem from './FeatureItem';
import Modal from './Modal';
import styles from './OrgCostMetricsModal.module.css';

interface OrgCostMetricsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function OrgCostMetricsModal({ isOpen, onClose }: OrgCostMetricsModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Org cost metrics">
      <h2 className={styles.title}>Org cost metrics</h2>
      <p className={styles.subtitle}>
        Per-user visibility into team usage and spend
      </p>
      <div className={styles.features}>
        <FeatureItem
          icon="💰"
          title="Estimated cost per user"
          description="Total and average estimated cost across all sessions, computed from token usage and current model pricing."
        />
        <FeatureItem
          icon="⏱️"
          title="Time breakdown"
          description="Total and average session duration, plus the split between assistant response time and user active time."
        />
        <FeatureItem
          icon="📊"
          title="Sortable team view"
          description="Sort any column to find your heaviest users, longest sessions, or most efficient sessions at a glance."
        />
      </div>
    </Modal>
  );
}

export default OrgCostMetricsModal;
