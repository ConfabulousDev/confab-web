import FeatureItem from './FeatureItem';
import Modal from './Modal';
import styles from './RetroModal.module.css';

interface RetroModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function RetroModal({ isOpen, onClose }: RetroModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Retro">
      <h2 className={styles.title}>Retro</h2>
      <p className={styles.subtitle}>
        Load any past session into a new one with <code className={styles.code}>/retro</code>
      </p>
      <div className={styles.features}>
        <FeatureItem
          icon="🔁"
          title="Replay your own work"
          description="Pull a previous session's condensed transcript into a fresh agent so you can pick up where you left off, even days later."
        />
        <FeatureItem
          icon="👥"
          title="Learn from teammates"
          description="Load a teammate's session — even one run in a different provider — and reference how they solved a tricky problem."
        />
        <FeatureItem
          icon="🛠️"
          title="Synthesize a reusable skill"
          description="When a session captures a workflow worth keeping, ask the agent to distill it into a Claude Code or Codex skill on the spot."
        />
      </div>
    </Modal>
  );
}

export default RetroModal;
