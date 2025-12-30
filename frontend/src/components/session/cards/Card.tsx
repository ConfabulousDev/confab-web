import styles from '../SessionSummaryPanel.module.css';

interface CardWrapperProps {
  title: string;
  subtitle?: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
}

/**
 * Base wrapper for summary cards. Provides consistent header and styling.
 */
export function CardWrapper({ title, subtitle, icon, children }: CardWrapperProps) {
  return (
    <div className={styles.card}>
      <div className={styles.cardHeader}>
        <span className={styles.cardTitle}>
          {icon && <span className={styles.cardTitleIcon}>{icon}</span>}
          {title}
        </span>
        {subtitle && <span className={styles.cardSubtitle}>{subtitle}</span>}
      </div>
      <div className={styles.cardContent}>{children}</div>
    </div>
  );
}

interface StatRowProps {
  label: string;
  value: React.ReactNode;
  icon?: React.ReactNode;
  tooltip?: string;
  valueClassName?: string;
}

/**
 * A single stat row within a card.
 */
export function StatRow({ label, value, icon, tooltip, valueClassName }: StatRowProps) {
  return (
    <div className={styles.statRow} title={tooltip}>
      <span className={styles.statLabel}>
        {icon && <span className={styles.statIcon}>{icon}</span>}
        {label}
      </span>
      <span className={`${styles.statValue} ${valueClassName ?? ''}`}>{value}</span>
    </div>
  );
}

/**
 * Loading placeholder for a card.
 */
export function CardLoading() {
  return <div className={styles.loading}>Loading...</div>;
}

interface SectionHeaderProps {
  label: string;
}

/**
 * A section header within a card to group related stats.
 */
export function SectionHeader({ label }: SectionHeaderProps) {
  return <div className={styles.sectionHeader}>{label}</div>;
}
