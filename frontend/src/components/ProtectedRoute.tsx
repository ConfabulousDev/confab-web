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

  // Show nothing while loading - prevents flash of protected content
  if (loading) {
    return null;
  }

  // Redirect to login if not authenticated, with redirect back to intended page
  if (!isAuthenticated) {
    const intendedPath = location.pathname + location.search;
    const loginUrl = `/auth/login?redirect=${encodeURIComponent(intendedPath)}`;
    window.location.href = loginUrl;
    return null;
  }

  return <>{children}</>;
}

export default ProtectedRoute;
