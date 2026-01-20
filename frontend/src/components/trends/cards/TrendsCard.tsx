import styles from './TrendsCard.module.css';

interface TrendsCardProps {
  title: string;
  subtitle?: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
}

export function TrendsCard({ title, subtitle, icon, children }: TrendsCardProps) {
  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <span className={styles.title}>
          {icon && <span className={styles.icon}>{icon}</span>}
          {title}
        </span>
        {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
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
