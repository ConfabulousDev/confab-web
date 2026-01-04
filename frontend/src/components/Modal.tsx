import type { ReactNode } from 'react';
import styles from './Modal.module.css';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  children: ReactNode;
  /** Optional CSS class for the modal container (for size/spacing customization) */
  className?: string;
  /** Accessible label for the modal (defaults to "Modal") */
  ariaLabel?: string;
  /** Whether to show the default close button (defaults to true) */
  showCloseButton?: boolean;
}

/**
 * Reusable modal wrapper component.
 *
 * Provides consistent overlay, close-on-click-outside, close button, and styling.
 * Use className prop to customize modal size/spacing per use case.
 *
 * @example
 * <Modal isOpen={isOpen} onClose={onClose} className={styles.myModal}>
 *   <h2>Title</h2>
 *   <p>Content</p>
 * </Modal>
 */
function Modal({ isOpen, onClose, children, className, ariaLabel = 'Modal', showCloseButton = true }: ModalProps) {
  if (!isOpen) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div
        className={`${styles.modal} ${className ?? ''}`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel}
      >
        {showCloseButton && (
          <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
            ×
          </button>
        )}
        {children}
      </div>
    </div>
  );
}

export default Modal;
