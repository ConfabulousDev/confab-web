import styles from './UpdateBadge.module.css';

interface UpdateBadgeViewProps {
  show: boolean;
  current: string;
  latest?: string;
  latestUrl?: string;
  severity?: 'available' | 'recommended';
}

function UpdateBadgeView({ show, current, latest, latestUrl, severity }: UpdateBadgeViewProps) {
  if (!show || !latestUrl) return null;

  const title = current === '' ? `(dev) → ${latest}` : `${current} → ${latest}`;
  const recommended = severity === 'recommended';

  return (
    <a
      href={latestUrl}
      target="_blank"
      rel="noopener noreferrer"
      title={title}
      className={`${styles.badge} ${recommended ? styles.recommended : ''}`}
    >
      <span className={styles.dot} aria-hidden="true" />
      {recommended ? 'Update recommended' : 'Update available'}
    </a>
  );
}

export default UpdateBadgeView;
