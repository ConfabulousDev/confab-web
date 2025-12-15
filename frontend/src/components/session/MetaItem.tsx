import type { ReactNode } from 'react';
import styles from './SessionHeader.module.css';

interface MetaItemProps {
  icon: ReactNode;
  value: string;
  href?: string;
}

function MetaItem({ icon, value, href }: MetaItemProps) {
  return (
    <span className={styles.metaItem}>
      <span className={styles.metaIcon}>{icon}</span>
      {href ? (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className={styles.metaLink}
        >
          {value}
        </a>
      ) : (
        <span className={styles.metaValue}>{value}</span>
      )}
    </span>
  );
}

export default MetaItem;
