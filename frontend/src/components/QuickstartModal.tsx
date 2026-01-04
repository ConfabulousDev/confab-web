import Modal from './Modal';
import Quickstart from './Quickstart';
import styles from './QuickstartModal.module.css';

interface QuickstartModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function QuickstartModal({ isOpen, onClose }: QuickstartModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="Quickstart">
      <Quickstart />
    </Modal>
  );
}

export default QuickstartModal;
