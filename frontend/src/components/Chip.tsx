import { useState, type ReactNode, type MouseEvent } from 'react';
import styles from './Chip.module.css';
import { ClipboardCheckIcon } from './icons';

export type ChipVariant = 'neutral' | 'blue' | 'green' | 'purple';

interface ChipProps {
  children: ReactNode;
  icon?: ReactNode;
  variant?: ChipVariant;
  copyValue?: string;
}

function Chip({ children, icon, variant = 'neutral', copyValue }: ChipProps) {
  const [copied, setCopied] = useState(false);

  const handleClick = copyValue
    ? (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();
        navigator.clipboard.writeText(copyValue);
        setCopied(true);
        setTimeout(() => setCopied(false), 800);
      }
    : undefined;

  return (
    <span
      className={`${styles.chip} ${styles[variant]} ${copyValue ? styles.clickable : ''}`}
      onClick={handleClick}
    >
      {icon && <span className={styles.icon}>{icon}</span>}
      <span className={styles.textWrapper}>
        <span className={`${styles.text} ${copied ? styles.hidden : ''}`}>{children}</span>
        {copyValue && (
          <span className={`${styles.copiedText} ${copied ? '' : styles.hidden}`}>
            {ClipboardCheckIcon}
          </span>
        )}
      </span>
    </span>
  );
}

export default Chip;
