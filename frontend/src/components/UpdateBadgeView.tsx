import styles from './UpdateBadge.module.css';

interface UpdateBadgeViewProps {
  show: boolean;
  current: string;
  latest?: string;
  latestUrl?: string;
}

function UpdateBadgeView({ show, current, latest, latestUrl }: UpdateBadgeViewProps) {
  if (!show || !latestUrl) return null;

  const title = current === '' ? `(dev) → ${latest}` : `${current} → ${latest}`;

  return (
    <a
      href={latestUrl}
      target="_blank"
      rel="noopener noreferrer"
      title={title}
      className={styles.badge}
    >
      <span className={styles.dot} aria-hidden="true" />
      Update available
    </a>
  );
}

export default UpdateBadgeView;
