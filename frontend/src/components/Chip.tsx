import { useState, type ReactNode, type MouseEvent } from 'react';
import styles from './Chip.module.css';

type ChipVariant = 'neutral' | 'blue' | 'green' | 'purple';

interface ChipProps {
  children: ReactNode;
  icon?: ReactNode;
  variant?: ChipVariant;
  copyValue?: string;
  linkUrl?: string;
}

function Chip({ children, icon, variant = 'neutral', copyValue, linkUrl }: ChipProps) {
  const [copied, setCopied] = useState(false);

  const isClickable = !!(copyValue || linkUrl);

  const handleClick = isClickable
    ? (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();
        if (linkUrl) {
          window.open(linkUrl, '_blank', 'noopener,noreferrer');
        } else if (copyValue) {
          navigator.clipboard.writeText(copyValue);
          setCopied(true);
          setTimeout(() => setCopied(false), 800);
        }
      }
    : undefined;

  return (
    <span
      className={[styles.chip, styles[variant], isClickable && styles.clickable, linkUrl && styles.linkable].filter(Boolean).join(' ')}
      onClick={handleClick}
    >
      {icon && <span className={styles.icon}>{icon}</span>}
      <span className={styles.textWrapper}>
        <span className={`${styles.text} ${copied ? styles.hidden : ''}`}>{children}</span>
        {copyValue && !linkUrl && (
          <span className={`${styles.copiedText} ${copied ? '' : styles.hidden}`}>
            copied!
          </span>
        )}
      </span>
    </span>
  );
}

export default Chip;
