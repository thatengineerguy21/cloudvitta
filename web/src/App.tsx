// web/src/App.tsx
import React from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { queryClient } from './lib/queryClient';
import { Router, useLocation } from './router';
import { AuthProvider } from './auth/AuthContext';
import { Header } from './components/layout/Header';
import { Footer } from './components/layout/Footer';
import {
  ComputeCompare,
  StorageCompare,
  NetworkCompare,
  DatabaseCompare,
  DatabaseNoSQLCompare,
  KubernetesCompare,
  ServerlessCompare,
} from './pages/compare';
import { CalculatePage } from './pages/CalculatePage';
import { LandingPage } from './pages/LandingPage';
import { ProviderStatusPage } from './pages/ProviderStatusPage';
import { NotFoundPage } from './pages/NotFoundPage';
import { CheckEmailPage } from './pages/CheckEmailPage';
import { VerifyEmailPage } from './pages/VerifyEmailPage';
import { MCPPlaygroundPage } from './pages/MCPPlaygroundPage';

export const AppRoutes: React.FC = () => {
  const { pathname } = useLocation();

  switch (pathname) {
    case '/compare/compute':
      return <ComputeCompare />;
    case '/compare/storage':
      return <StorageCompare />;
    case '/compare/network':
      return <NetworkCompare />;
    case '/compare/database':
      return <DatabaseCompare />;
    case '/compare/database-nosql':
      return <DatabaseNoSQLCompare />;
    case '/compare/kubernetes':
      return <KubernetesCompare />;
    case '/compare/serverless':
      return <ServerlessCompare />;
    case '/calculate':
      return <CalculatePage />;
    case '/status':
      return <ProviderStatusPage />;
    case '/check-email':
      return <CheckEmailPage />;
    case '/verify-email':
      return <VerifyEmailPage />;
    case '/playground':
      return <MCPPlaygroundPage />;
    case '/':
      return <LandingPage />;
    default:
      return <NotFoundPage />;
  }
};

export const App: React.FC = () => {
  return (
    <QueryClientProvider client={queryClient}>
      <Router>
        <AuthProvider>
          <div className="min-h-screen flex flex-col bg-surface-page text-text-primary">
            <Header />
            <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
              <AppRoutes />
            </main>
            <Footer />
          </div>
        </AuthProvider>
      </Router>
    </QueryClientProvider>
  );
};
