import type { ReactNode } from 'react';
import styles from './Alert.module.css';

interface AlertProps {
  variant?: 'success' | 'error' | 'warning' | 'info';
  children: ReactNode;
  onClose?: () => void;
}

function Alert({ variant = 'info', children, onClose }: AlertProps) {
  return (
    <div className={`${styles.alert} ${styles[variant]}`}>
      <div className={styles.content}>{children}</div>
      {onClose && (
        <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
          ×
        </button>
      )}
    </div>
  );
}

export default Alert;
