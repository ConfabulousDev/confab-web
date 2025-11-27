import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

/**
 * Wrapper component that protects routes requiring authentication.
 * - Shows nothing while auth is loading (prevents flash of content)
 * - Redirects to login page if not authenticated, preserving intended destination
 * - Renders children if authenticated
 */
function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { loading, isAuthenticated } = useAuth();
  const location = useLocation();

  // Debug logging
  console.log('[ProtectedRoute]', { loading, isAuthenticated, pathname: location.pathname });

  // Redirect to login if not authenticated
  useEffect(() => {
    console.log('[ProtectedRoute useEffect]', { loading, isAuthenticated, pathname: location.pathname });
    if (!loading && !isAuthenticated) {
      console.log('[ProtectedRoute] REDIRECTING TO LOGIN');
      const intendedPath = location.pathname + location.search;
      const loginUrl = `/auth/login?redirect=${encodeURIComponent(intendedPath)}`;
      window.location.href = loginUrl;
    }
  }, [loading, isAuthenticated, location.pathname, location.search]);

  // Show nothing while loading or redirecting
  if (loading || !isAuthenticated) {
    console.log('[ProtectedRoute] returning null', { loading, isAuthenticated });
    return null;
  }

  return <>{children}</>;
}

export default ProtectedRoute;
