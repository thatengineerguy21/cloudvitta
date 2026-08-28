// web/src/components/layout/Header.tsx
import React, { useState } from 'react';
import { ThemeToggle } from '../common/ThemeToggle';
import { AuthModal } from '../auth/AuthModal';
import { useAuth } from '../../auth/AuthContext';
import { Link, useLocation } from '../../router';
import { cn } from '../../lib/utils';
import { useProviderHealthSummary, type HealthSummaryState } from '../../api/queries/useProviderStatusQueries';
import { ExternalLink, Activity, User, LogOut } from 'lucide-react';

const HEALTH_BADGE_COLORS: Record<HealthSummaryState, string> = {
  healthy: 'text-status-matchExact',
  degraded: 'text-status-stale',
  error: 'text-status-anomaly',
  loading: 'text-text-secondary',
};

export const Header: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuth();
  const { pathname } = useLocation();
  const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);
  const { summaryState, summaryLabel, isLoading } = useProviderHealthSummary();

  const healthBadgeColor = HEALTH_BADGE_COLORS[summaryState];

  const navCategories = [
    { label: 'Compute', href: '/compare/compute' },
    { label: 'Storage', href: '/compare/storage' },
    { label: 'Network', href: '/compare/network' },
    { label: 'RDBMS', href: '/compare/database' },
    { label: 'NoSQL', href: '/compare/database-nosql' },
    { label: 'Kubernetes', href: '/compare/kubernetes' },
    { label: 'Serverless', href: '/compare/serverless' },
    { label: 'Calculate', href: '/calculate' },
  ];

  return (
    <>
      <header className="border-b border-border-default bg-surface-card sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            {/* Brand Logo & Name */}
            <div className="flex items-center space-x-6">
              <Link to="/" className="flex items-center space-x-3" exact>
                <span className="font-display text-2xl font-bold tracking-tight text-text-primary">
                  CloudVitta
                </span>
                <span className="text-xs uppercase tracking-widest px-1.5 py-0.5 border border-border-accent text-border-accent font-bold">
                  API Engine
                </span>
              </Link>

              {/* Provider Health Status Affordance — Live Telemetry */}
              <Link
                to="/status"
                className="hidden lg:flex items-center space-x-2 text-xs font-semibold text-text-secondary hover:text-text-primary px-2.5 py-1 border border-border-default bg-surface-raised transition-colors"
                activeClassName="border-border-accent text-text-primary bg-surface-raised"
              >
                <Activity className={cn('w-3.5 h-3.5', healthBadgeColor)} />
                <span className={isLoading ? 'animate-pulse' : ''}>{summaryLabel}</span>
              </Link>
            </div>

            {/* Navigation Links */}
            <nav className="hidden md:flex items-center space-x-1 lg:space-x-2">
              {navCategories.map((item) => {
                const isActive = pathname === item.href;
                return (
                  <Link
                    key={item.label}
                    to={item.href}
                    className={`px-2.5 py-1 text-xs uppercase font-bold tracking-wider transition-colors border ${
                      isActive
                        ? 'border-border-accent text-text-primary bg-surface-raised'
                        : 'border-transparent text-text-secondary hover:text-text-primary hover:bg-surface-raised'
                    }`}
                  >
                    {item.label}
                  </Link>
                );
              })}
            </nav>

            {/* Action Bar (Auth, Swagger, Theme Toggle) */}
            <div className="flex items-center space-x-3">
              {/* Authenticated State vs Sign In Trigger */}
              {isAuthenticated && user ? (
                <div className="flex items-center space-x-2">
                  <div className="hidden sm:flex items-center space-x-1.5 px-2 py-1 border border-border-accent bg-surface-raised text-xs font-bold text-text-primary">
                    <User className="w-3 h-3 text-border-accent" />
                    <span className="truncate max-w-[120px]">{user.email}</span>
                  </div>
                  <button
                    onClick={() => logout()}
                    className="flex items-center space-x-1 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-status-anomaly px-2 py-1.5 border border-border-default bg-surface-card transition-colors"
                    title="Log Out"
                  >
                    <LogOut className="w-3 h-3" />
                    <span className="hidden sm:inline">Logout</span>
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => setIsAuthModalOpen(true)}
                  className="flex items-center space-x-1.5 text-xs uppercase font-bold tracking-wider text-text-primary hover:border-border-accent px-2.5 py-1.5 border border-border-default bg-surface-raised transition-colors"
                >
                  <User className="w-3 h-3 text-border-accent" />
                  <span>Sign In</span>
                </button>
              )}

              {/* Swagger Docs Link */}
              <a
                href="/docs/"
                target="_blank"
                rel="noreferrer"
                className="flex items-center space-x-1.5 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-border-accent px-2 py-1.5 border border-border-default bg-surface-card transition-colors"
                title="Open Swagger API Documentation"
              >
                <span>Swagger</span>
                <ExternalLink className="w-3 h-3" />
              </a>

              <ThemeToggle />
            </div>
          </div>
        </div>
      </header>

      {/* Auth Modal Portal */}
      <AuthModal
        isOpen={isAuthModalOpen}
        onClose={() => setIsAuthModalOpen(false)}
        initialMode="login"
      />
    </>
  );
};
