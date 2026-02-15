import { useState, useRef, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
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

  const navigate = useNavigate();
  const handleLogin = () => {
    navigate('/login');
  };

  if (!isAuthenticated) {
    return (
      <header className={styles.header}>
        <Link to="/" className={styles.logo}>Confabulous</Link>
        <div className={styles.actions}>
          <ThemeToggle />
        </div>
        <div className={styles.right}>
          <button className={styles.loginBtn} onClick={handleLogin}>
            Log in
          </button>
        </div>
      </header>
    );
  }

  return (
    <header className={styles.header}>
      <Link to="/" className={styles.logo}>Confabulous</Link>

      <nav className={styles.nav}>
        <Link to={user?.email ? `/sessions?owner=${encodeURIComponent(user.email)}` : '/sessions'} className={styles.navLink}>Sessions</Link>
        <Link to="/trends" className={styles.navLink}>Personal Trends</Link>
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
