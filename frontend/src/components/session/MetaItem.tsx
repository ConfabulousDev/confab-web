import type { ReactNode } from 'react';
import styles from './SessionHeader.module.css';

interface MetaItemProps {
  icon: ReactNode;
  value: string;
}

function MetaItem({ icon, value }: MetaItemProps) {
  return (
    <span className={styles.metaItem}>
      <span className={styles.metaIcon}>{icon}</span>
      <span className={styles.metaValue}>{value}</span>
    </span>
  );
}

export default MetaItem;
