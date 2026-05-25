import FeatureItem from './FeatureItem';
import Modal from './Modal';
import styles from './MultiProviderModal.module.css';

interface MultiProviderModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function MultiProviderModal({ isOpen, onClose }: MultiProviderModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Multi-provider support">
      <h2 className={styles.title}>Multi-provider support</h2>
      <p className={styles.subtitle}>
        One dashboard for every AI coding session
      </p>
      <div className={styles.features}>
        <FeatureItem
          icon="🤖"
          title="Claude Code"
          description="Where we started. Full transcript, tool calls, hooks, subagents, skills — every detail of every session."
        />
        <FeatureItem
          icon="🧠"
          title="OpenAI Codex"
          description="First-class support: reasoning, tool calls, subagent threads, and the same analytics as Claude Code."
        />
        <FeatureItem
          icon="🚀"
          title="More coming"
          description="OpenCode is next. New providers slot into the same sync, storage, and analytics pipeline."
        />
      </div>
    </Modal>
  );
}

export default MultiProviderModal;
