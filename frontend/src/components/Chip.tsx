import type { ReactNode } from 'react';
import styles from './Chip.module.css';

export type ChipVariant = 'neutral' | 'blue' | 'green' | 'purple';

interface ChipProps {
  children: ReactNode;
  icon?: ReactNode;
  variant?: ChipVariant;
  title?: string;
  ellipsis?: 'start' | 'end'; // Where to show ellipsis on overflow (default: 'end')
}

function Chip({ children, icon, variant = 'neutral', title, ellipsis = 'end' }: ChipProps) {
  const textClass = ellipsis === 'start' ? styles.textEllipsisStart : styles.text;
  return (
    <span className={`${styles.chip} ${styles[variant]}`} title={title}>
      {icon && <span className={styles.icon}>{icon}</span>}
      <span className={textClass}>{children}</span>
    </span>
  );
}

export default Chip;
