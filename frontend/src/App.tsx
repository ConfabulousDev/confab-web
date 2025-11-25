import { useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { initCSRF } from '@/services/csrf';
import ErrorBoundary from '@/components/ErrorBoundary';
import Header from '@/components/Header';
import Footer from '@/components/Footer';
import './index.css';

// Create a client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes
    },
  },
});

function App() {
  useEffect(() => {
    // Initialize CSRF token on app load
    initCSRF();
  }, []);

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <div className="app-container">
          <Header />
          <main>
            <Outlet />
          </main>
          <Footer />
        </div>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
