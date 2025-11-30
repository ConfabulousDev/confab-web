import styles from './PageHeader.module.css';

interface PageHeaderProps {
  title?: string;
  subtitle?: string;
  actions?: React.ReactNode;
  metadata?: React.ReactNode;
  leftContent?: React.ReactNode;
}

function PageHeader({
  title,
  subtitle,
  actions,
  metadata,
  leftContent,
}: PageHeaderProps) {
  return (
    <header className={styles.header}>
      <div className={styles.titleSection}>
        {leftContent ? (
          <div className={styles.leftContent}>{leftContent}</div>
        ) : (
          <div className={styles.titleRow}>
            {title && <h1 className={styles.title}>{title}</h1>}
            {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
          </div>
        )}
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
