import styles from './FeatureItem.module.css';

interface FeatureItemProps {
  icon: string;
  title: string;
  description: string;
}

function FeatureItem({ icon, title, description }: FeatureItemProps) {
  return (
    <div className={styles.featureItem}>
      <span className={styles.featureIcon}>{icon}</span>
      <div className={styles.featureContent}>
        <span className={styles.featureTitle}>{title}</span>
        <span className={styles.featureDescription}>{description}</span>
      </div>
    </div>
  );
}

export default FeatureItem;
