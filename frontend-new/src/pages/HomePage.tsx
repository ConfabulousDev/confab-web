import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import styles from './HomePage.module.css';

interface User {
  name: string;
  email: string;
  avatar_url: string;
}

function HomePage() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Check if user is authenticated
    const checkAuth = async () => {
      try {
        const response = await fetch('/api/v1/me', {
          credentials: 'include',
        });

        if (response.ok) {
          const userData = await response.json();
          setUser(userData);
        }
      } catch (error) {
        console.error('Failed to check auth:', error);
      } finally {
        setLoading(false);
      }
    };

    checkAuth();
  }, []);

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
              <button className={`${styles.btn} ${styles.logout}`} onClick={handleLogout}>
                Logout
              </button>
            </div>
          </div>
        ) : (
          <button className={`${styles.btn} ${styles.btnGithub}`} onClick={handleLogin}>
            Login with GitHub
          </button>
        )}
      </div>
    </div>
  );
}

export default HomePage;
