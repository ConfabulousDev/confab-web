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

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!loading && !isAuthenticated) {
      const intendedPath = location.pathname + location.search;
      const loginUrl = `/auth/login?redirect=${encodeURIComponent(intendedPath)}`;
      window.location.href = loginUrl;
    }
  }, [loading, isAuthenticated, location.pathname, location.search]);

  // Show nothing while loading or redirecting
  if (loading || !isAuthenticated) {
    return null;
  }

  return <>{children}</>;
}

export default ProtectedRoute;
