import FeatureItem from './FeatureItem';
import Modal from './Modal';
import styles from './TILModal.module.css';

interface TILModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function TILModal({ isOpen, onClose }: TILModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Today I Learned">
      <h2 className={styles.title}>Today I Learned</h2>
      <p className={styles.subtitle}>
        Capture and share insights from your coding sessions
      </p>
      <div className={styles.features}>
        <FeatureItem
          icon="💡"
          title="Capture Learnings"
          description="Save key insights, gotchas, and discoveries by typing /til in Claude Code."
        />
        <FeatureItem
          icon="🔍"
          title="Search & Filter"
          description="Full-text search across all your TILs. Filter by repo, branch, or team member."
        />
        <FeatureItem
          icon="🔗"
          title="Linked to Context"
          description="Each TIL links back to the exact transcript message where the insight occurred."
        />
        <FeatureItem
          icon="📤"
          title="Export Anywhere"
          description="Export your learnings to Notion, Confluence, or any external system via the REST API."
        />
      </div>
      <div className={styles.cliHint}>
        <code className={styles.command}>/til</code>
        <span className={styles.commandLabel}>Save a TIL from within Claude Code</span>
      </div>
    </Modal>
  );
}

export default TILModal;
