import { useState, useRef, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useAppConfig } from '@/hooks/useAppConfig';
import { getDemoIdentity, isDemoViewer } from '@/utils/demoIdentity';
import ThemeToggle from './ThemeToggle';
import UpdateBadge from './UpdateBadge';
import styles from './Header.module.css';

// Logo with optional "demo" badge. Beta badge removed; the badge only
// shows when DEMO_IDENTITY_EMAIL is configured (CF-483).
function Logo() {
  const demoEmail = getDemoIdentity();
  return (
    <Link to="/" className={styles.logo}>
      Confabulous
      {demoEmail !== null && <span className={styles.badge}>demo</span>}
    </Link>
  );
}

function Header() {
  const { user, isAuthenticated } = useAuth();
  const { sharesEnabled, orgAnalyticsEnabled } = useAppConfig();
  // CF-483 follow-up: the demo identity owns nothing, so pre-filling
  // ?owner=<demo email> on these nav links collapses the page to zero
  // rows. Skip the auto-injection for demo viewers; they still get the
  // owner dropdown to filter manually.
  const demoViewer = isDemoViewer(user?.email);
  const ownerQS = user?.email && !demoViewer ? `?owner=${encodeURIComponent(user.email)}` : '';
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
        <Logo />
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
      <Logo />

      <nav className={styles.nav}>
        <Link to={`/sessions${ownerQS}`} className={styles.navLink}>Sessions</Link>
        <Link to="/trends" className={styles.navLink}>Personal Trends</Link>
        <Link to={`/tils${ownerQS}`} className={styles.navLink}>TILs</Link>
        {orgAnalyticsEnabled && <Link to="/org" className={styles.navLink}>Organization</Link>}
      </nav>

      <div className={styles.actions}>
        <UpdateBadge />
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
            {sharesEnabled && (
              <Link to="/shares" className={styles.dropdownItem} onClick={() => setMenuOpen(false)}>
                Shares
              </Link>
            )}
            {user?.is_admin && (
              <Link to="/admin" className={styles.dropdownItem} onClick={() => setMenuOpen(false)}>
                Admin
              </Link>
            )}
            <div className={styles.dropdownDivider} />
            {/* CF-483: the demo identity has no real session to log out
                of — clicking Logout just re-impersonates on the next
                request. Swap in a "Log in as yourself" link instead so
                real users can claim a real session. */}
            {demoViewer ? (
              <Link to="/login" className={styles.dropdownItem} onClick={() => setMenuOpen(false)}>
                Log in as yourself
              </Link>
            ) : (
              <button className={styles.dropdownItem} onClick={handleLogout}>
                Logout
              </button>
            )}
          </div>
        )}
      </div>
    </header>
  );
}

export default Header;
