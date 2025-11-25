import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import styles from './Header.module.css';

function Header() {
  const { user, isAuthenticated } = useAuth();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleLogout = () => {
    window.location.href = '/auth/logout';
  };

  const handleGitHubLogin = () => {
    window.location.href = '/auth/github/login';
  };

  const handleGoogleLogin = () => {
    window.location.href = '/auth/google/login';
  };

  if (!isAuthenticated) {
    return (
      <header className={styles.header}>
        <Link to="/" className={styles.logo}>Confab</Link>
        <div className={styles.right} ref={menuRef}>
          <button
            className={styles.loginBtn}
            onClick={() => setMenuOpen(!menuOpen)}
          >
            Login
          </button>
          {menuOpen && (
            <div className={styles.dropdown}>
              <button className={styles.dropdownItem} onClick={handleGitHubLogin}>
                Continue with GitHub
              </button>
              <button className={styles.dropdownItem} onClick={handleGoogleLogin}>
                Continue with Google
              </button>
            </div>
          )}
        </div>
      </header>
    );
  }

  return (
    <header className={styles.header}>
      <Link to="/" className={styles.logo}>Confab</Link>

      <nav className={styles.nav}>
        <Link to="/sessions" className={styles.navLink}>Sessions</Link>
      </nav>

      <div className={styles.right} ref={menuRef}>
        <button
          className={styles.avatarBtn}
          onClick={() => setMenuOpen(!menuOpen)}
          aria-label="User menu"
        >
          {user?.avatar_url ? (
            <img
              src={user.avatar_url}
              alt={user.name || 'User'}
              className={styles.avatar}
            />
          ) : (
            <div className={styles.avatarPlaceholder}>
              {user?.name?.charAt(0) || user?.email?.charAt(0) || '?'}
            </div>
          )}
        </button>

        {menuOpen && (
          <div className={styles.dropdown}>
            <div className={styles.userInfo}>
              {user?.name && <div className={styles.userName}>{user.name}</div>}
              {user?.email && <div className={styles.userEmail}>{user.email}</div>}
            </div>
            <div className={styles.dropdownDivider} />
            <Link to="/keys" className={styles.dropdownItem} onClick={() => setMenuOpen(false)}>
              API Keys
            </Link>
            <Link to="/shares" className={styles.dropdownItem} onClick={() => setMenuOpen(false)}>
              Share Links
            </Link>
            <div className={styles.dropdownDivider} />
            <button className={styles.dropdownItem} onClick={handleLogout}>
              Logout
            </button>
          </div>
        )}
      </div>
    </header>
  );
}

export default Header;
