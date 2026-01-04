import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import ThemeToggle from './ThemeToggle';
import styles from './Header.module.css';

function Header() {
  const { user, isAuthenticated } = useAuth();
  const [menuOpen, setMenuOpen] = useState(false);
  const [avatarError, setAvatarError] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu when clicking outside
  // NOTE: This is a manual implementation of click-outside detection. If this pattern
  // is needed in other components, consider extracting to a reusable useClickOutside hook:
  //   function useClickOutside(ref: RefObject<HTMLElement>, handler: () => void) { ... }
  // For now, this is the only usage, so inline implementation is acceptable.
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      const target = event.target;
      if (menuRef.current && target instanceof Node && !menuRef.current.contains(target)) {
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
        <Link to="/" className={styles.logo}>Confabulous</Link>
        <div className={styles.actions}>
          <ThemeToggle />
        </div>
        <div className={styles.right} ref={menuRef}>
          <button
            className={styles.loginBtn}
            onClick={() => setMenuOpen(!menuOpen)}
          >
            Login
          </button>
          {menuOpen && (
            <div className={styles.loginDropdown}>
              <button
                className={styles.loginIconBtn}
                onClick={handleGitHubLogin}
                aria-label="Login with GitHub"
              >
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
                </svg>
              </button>
              <button
                className={styles.loginIconBtn}
                onClick={handleGoogleLogin}
                aria-label="Login with Google"
              >
                <svg viewBox="0 0 24 24">
                  <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                  <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                  <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                  <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                </svg>
              </button>
            </div>
          )}
        </div>
      </header>
    );
  }

  return (
    <header className={styles.header}>
      <Link to="/" className={styles.logo}>Confabulous</Link>

      <nav className={styles.nav}>
        <Link to="/sessions" className={styles.navLink}>Sessions</Link>
        <Link to="/trends" className={styles.navLink}>Trends</Link>
      </nav>

      <div className={styles.actions}>
        <ThemeToggle />
      </div>

      <div className={styles.right} ref={menuRef}>
        <button
          className={styles.avatarBtn}
          onClick={() => setMenuOpen(!menuOpen)}
          aria-label="User menu"
        >
          {user?.avatar_url && !avatarError ? (
            <img
              src={user.avatar_url}
              alt={user.name || 'User'}
              className={styles.avatar}
              onError={() => setAvatarError(true)}
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
              Shares
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
