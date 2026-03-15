import { Link } from 'react-router-dom';
import { useDropdown } from '@/hooks';
import type { TIL } from '@/schemas/api';
import styles from './TILBadge.module.css';

const LightbulbIcon = (
  <svg viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
    <path d="M8 1a4.5 4.5 0 0 0-1.8 8.63v.87a1 1 0 0 0 1 1h1.6a1 1 0 0 0 1-1v-.87A4.5 4.5 0 0 0 8 1zm0 1.5a3 3 0 0 1 1.2 5.75.75.75 0 0 0-.45.69v.56H7.25v-.56a.75.75 0 0 0-.45-.69A3 3 0 0 1 8 2.5zM6.75 12.5a.75.75 0 0 0 0 1.5h2.5a.75.75 0 0 0 0-1.5h-2.5z"/>
  </svg>
);

interface TILBadgeProps {
  tils: TIL[];
}

export default function TILBadge({ tils }: TILBadgeProps) {
  const { isOpen, toggle, containerRef } = useDropdown<HTMLDivElement>();

  if (tils.length === 0) return null;

  const label = tils.length === 1 ? 'TIL' : `TIL (${tils.length})`;

  return (
    <div className={styles.container} ref={containerRef}>
      <button
        className={styles.badge}
        onClick={(e) => {
          e.stopPropagation();
          toggle();
        }}
        title={`${tils.length} TIL${tils.length > 1 ? 's' : ''} on this message`}
      >
        {LightbulbIcon}
        {label}
      </button>

      {isOpen && (
        <div className={styles.popover} onClick={(e) => e.stopPropagation()}>
          {tils.map((til) => (
            <div key={til.id} className={styles.tilItem}>
              <div className={styles.tilItemTitle}>{til.title}</div>
              <div className={styles.tilItemSummary}>{til.summary}</div>
            </div>
          ))}
          <Link to="/tils" className={styles.viewLink}>
            View in TILs
          </Link>
        </div>
      )}
    </div>
  );
}
