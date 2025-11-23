import { Link } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import Button from '@/components/Button';
import styles from './HomePage.module.css';

function HomePage() {
  const { user, loading } = useAuth();

  const handleLogin = () => {
    window.location.href = '/auth/github/login';
  };

  const handleLogout = () => {
    window.location.href = '/auth/logout';
  };

  return (
    <div className={styles.container}>
      <div className={styles.hero}>
        <h1>Confab</h1>
        <p>Distributed quantum mesh for temporal data harmonization</p>

        {loading ? (
          <p>Loading...</p>
        ) : user ? (
          <div className={styles.userInfo}>
            <h2>You're authenticated!</h2>
            {user.avatar_url && (
              <img
                src={user.avatar_url}
                alt={user.name}
                className={styles.avatar}
              />
            )}
            <p>
              <strong>Name:</strong> {user.name || 'N/A'}
            </p>
            <p>
              <strong>Email:</strong> {user.email}
            </p>
            <div className={styles.actions}>
              <Link to="/sessions" className={`${styles.btn} ${styles.btnPrimary}`}>
                View Sessions
              </Link>
              <Link to="/keys" className={`${styles.btn} ${styles.btnPrimary}`}>
                Manage API Keys
              </Link>
              <Button variant="secondary" onClick={handleLogout}>
                Logout
              </Button>
            </div>
          </div>
        ) : (
          <Button variant="github" onClick={handleLogin}>
            Login with GitHub
          </Button>
        )}
      </div>
    </div>
  );
}

export default HomePage;
