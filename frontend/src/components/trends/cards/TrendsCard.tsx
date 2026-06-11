import styles from './TrendsCard.module.css';

interface TrendsCardProps {
  title: string;
  subtitle?: string;
  icon?: React.ReactNode;
  /** Optional control rendered on the right of the header (e.g. a selector). */
  headerAction?: React.ReactNode;
  /**
   * Subtle ⓘ affordance next to the title with this text as its tooltip. Used
   * to flag that an active model filter is session-level so a card's numbers
   * still reflect full-session cost (2hh1).
   */
  caveat?: string;
  children: React.ReactNode;
}

export function TrendsCard({ title, subtitle, icon, headerAction, caveat, children }: TrendsCardProps) {
  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <span className={styles.title}>
          {icon && <span className={styles.icon}>{icon}</span>}
          {title}
          {caveat && (
            <span className={styles.caveat} role="note" aria-label={caveat} title={caveat}>
              ⓘ
            </span>
          )}
        </span>
        {(subtitle || headerAction) && (
          <span className={styles.headerRight}>
            {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
            {headerAction}
          </span>
        )}
      </div>
      <div className={styles.content}>{children}</div>
    </div>
  );
}

interface StatRowProps {
  label: string;
  value: React.ReactNode;
  icon?: React.ReactNode;
}

export function StatRow({ label, value, icon }: StatRowProps) {
  return (
    <div className={styles.statRow}>
      <span className={styles.statLabel}>
        {icon && <span className={styles.statIcon}>{icon}</span>}
        {label}
      </span>
      <span className={styles.statValue}>{value}</span>
    </div>
  );
}
