import { useLocation, Navigate } from 'react-router-dom';
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

  // Show nothing while loading
  if (loading) {
    return null;
  }

  // Redirect to login if not authenticated (SPA navigation)
  if (!isAuthenticated) {
    const intendedPath = location.pathname + location.search;
    return <Navigate to={`/login?redirect=${encodeURIComponent(intendedPath)}`} replace />;
  }

  return <>{children}</>;
}

export default ProtectedRoute;
