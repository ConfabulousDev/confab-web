import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import { SUPPORT_EMAIL } from '@/config';
import Alert from '@/components/Alert';
import styles from './LoginPage.module.css';

interface ProviderInfo {
  name: string;
  display_name: string;
  login_url: string;
}

interface AuthConfig {
  providers: ProviderInfo[];
}

function GitHubIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
    </svg>
  );
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24">
      <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
      <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
      <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
      <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
    </svg>
  );
}

function LockIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 2C9.243 2 7 4.243 7 7v2H6c-1.103 0-2 .897-2 2v9c0 1.103.897 2 2 2h12c1.103 0 2-.897 2-2v-9c0-1.103-.897-2-2-2h-1V7c0-2.757-2.243-5-5-5zM9 7c0-1.654 1.346-3 3-3s3 1.346 3 3v2H9V7zm9 4v9H6v-9h12zm-5 2h-2v5h2v-5z" />
    </svg>
  );
}

function LoginPage() {
  useDocumentTitle('Log in');
  const { isAuthenticated, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [config, setConfig] = useState<AuthConfig | null>(null);

  const redirectParam = searchParams.get('redirect') || '';
  const emailParam = searchParams.get('email') || '';

  // Extract auth error from URL params into state on initial render
  const [authError] = useState(() => {
    const error = searchParams.get('error');
    if (error) {
      return {
        type: error,
        message: searchParams.get('error_description') || 'Authentication failed. Please try again.',
      };
    }
    return null;
  });

  // Clear error params from URL after initial render
  useEffect(() => {
    if (searchParams.has('error')) {
      const next = new URLSearchParams(searchParams);
      next.delete('error');
      next.delete('error_description');
      setSearchParams(next, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  // Redirect authenticated users to /sessions
  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      navigate('/sessions', { replace: true });
    }
  }, [authLoading, isAuthenticated, navigate]);

  // Fetch auth config
  useEffect(() => {
    let cancelled = false;
    fetch('/api/v1/auth/config')
      .then((res) => res.json())
      .then((data: AuthConfig) => {
        if (!cancelled) setConfig(data);
      })
      .catch(() => {
        // If config fetch fails, show empty providers
        if (!cancelled) setConfig({ providers: [] });
      });
    return () => { cancelled = true; };
  }, []);

  // Auto-redirect: single OAuth provider (not password)
  // Skip redirect when there's an auth error to avoid infinite redirect loops
  useEffect(() => {
    if (!config || authError) return;
    const sole = config.providers.length === 1 ? config.providers[0] : undefined;
    if (sole && sole.name !== 'password') {
      let loginURL = sole.login_url;
      const params = new URLSearchParams();
      if (redirectParam) params.set('redirect', redirectParam);
      if (emailParam) params.set('email', emailParam);
      const qs = params.toString();
      if (qs) loginURL += '?' + qs;
      window.location.href = loginURL;
    }
  }, [config, authError, redirectParam, emailParam]);

  // Show nothing while loading
  if (authLoading || !config) return null;
  if (isAuthenticated) return null;

  // If single non-password provider and no error, we're about to redirect — show nothing
  const soleProvider = config.providers.length === 1 ? config.providers[0] : undefined;
  if (soleProvider && soleProvider.name !== 'password' && !authError) {
    return null;
  }

  const hasPassword = config.providers.some((p) => p.name === 'password');
  const oauthProviders = config.providers.filter((p) => p.name !== 'password');

  // Build query string for OAuth links
  function buildOAuthURL(loginURL: string): string {
    const params = new URLSearchParams();
    if (redirectParam) params.set('redirect', redirectParam);
    if (emailParam) params.set('email', emailParam);
    const qs = params.toString();
    return qs ? `${loginURL}?${qs}` : loginURL;
  }

  function getProviderIcon(name: string) {
    switch (name) {
      case 'github': return <GitHubIcon />;
      case 'google': return <GoogleIcon />;
      default: return <LockIcon />;
    }
  }

  function getButtonClass(name: string): string {
    switch (name) {
      case 'github': return `${styles.oauthBtn} ${styles.githubBtn}`;
      case 'google': return `${styles.oauthBtn} ${styles.googleBtn}`;
      default: return `${styles.oauthBtn} ${styles.oidcBtn}`;
    }
  }

  return (
    <div className={styles.wrapper}>
      <div className={styles.card}>
        <h1 className={styles.title}>Log in</h1>
        {emailParam ? (
          <p className={styles.subtitle}>
            Sign in with <strong>{emailParam}</strong> to view this shared session
          </p>
        ) : (
          <p className={styles.subtitle}>Choose your authentication method</p>
        )}

        {authError && (
          <Alert variant="error" className={styles.errorAlert}>
            {authError.type === 'access_denied' ? (
              <>Please request access <a href={`mailto:${SUPPORT_EMAIL}?subject=${encodeURIComponent('Requesting access to Confabulous')}`}>here</a>.</>
            ) : (
              authError.message
            )}
          </Alert>
        )}

        {hasPassword && (
          <form className={styles.passwordForm} action="/auth/password/login" method="POST">
            <input type="hidden" name="redirect" value={redirectParam} />
            <input
              type="email"
              name="email"
              placeholder="Email"
              defaultValue={emailParam}
              required
              className={styles.input}
            />
            <input
              type="password"
              name="password"
              placeholder="Password"
              required
              className={styles.input}
            />
            <button type="submit" className={styles.submitBtn}>
              Sign in
            </button>
          </form>
        )}

        {hasPassword && oauthProviders.length > 0 && (
          <div className={styles.divider}>or continue with</div>
        )}

        {oauthProviders.length > 0 && (
          <div className={styles.oauthButtons}>
            {oauthProviders.map((provider) => (
              <a
                key={provider.name}
                href={buildOAuthURL(provider.login_url)}
                className={getButtonClass(provider.name)}
              >
                {getProviderIcon(provider.name)}
                Continue with {provider.display_name}
              </a>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export default LoginPage;
