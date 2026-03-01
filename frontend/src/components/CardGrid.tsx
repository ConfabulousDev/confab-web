import type { ReactNode } from 'react';
import styles from './CardGrid.module.css';

interface CardGridProps {
  children: ReactNode;
  className?: string;
}

/** Responsive card grid: 5 cols (ultrawide) → 4 → 2 (tablet) → 1 (mobile). */
export default function CardGrid({ children, className }: CardGridProps) {
  return (
    <div className={className ? `${styles.grid} ${className}` : styles.grid}>
      {children}
    </div>
  );
}
