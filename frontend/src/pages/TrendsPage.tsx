import { useDocumentTitle } from '@/hooks';
import PageHeader from '@/components/PageHeader';
import styles from './TrendsPage.module.css';

function TrendsPage() {
  useDocumentTitle('Trends');

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <PageHeader leftContent={<h1 className={styles.title}>Trends</h1>} />
        <div className={styles.container}>
          <div className={styles.placeholder}>
            Coming soon
          </div>
        </div>
      </div>
    </div>
  );
}

export default TrendsPage;
