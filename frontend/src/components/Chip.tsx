import type { ReactNode } from 'react';
import styles from './Chip.module.css';

export type ChipVariant = 'neutral' | 'blue' | 'green' | 'purple';

interface ChipProps {
  children: ReactNode;
  icon?: ReactNode;
  variant?: ChipVariant;
}

function Chip({ children, icon, variant = 'neutral' }: ChipProps) {
  return (
    <span className={`${styles.chip} ${styles[variant]}`}>
      {icon && <span className={styles.icon}>{icon}</span>}
      <span className={styles.text}>{children}</span>
    </span>
  );
}

export default Chip;
