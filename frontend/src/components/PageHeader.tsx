import styles from './PageHeader.module.css';

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
  metadata?: React.ReactNode;
}

function PageHeader({
  title,
  subtitle,
  actions,
  metadata,
}: PageHeaderProps) {
  return (
    <header className={styles.header}>
      <div className={styles.titleSection}>
        <div className={styles.titleRow}>
          <h1 className={styles.title}>{title}</h1>
          {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
        </div>
        {metadata && (
          <div className={styles.metadata}>
            {metadata}
          </div>
        )}
      </div>

      {actions && (
        <div className={styles.actions}>
          {actions}
        </div>
      )}
    </header>
  );
}

export default PageHeader;
