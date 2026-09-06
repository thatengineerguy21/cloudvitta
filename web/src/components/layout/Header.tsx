// web/src/components/layout/Header.tsx
import React, { useState, useEffect } from 'react';
import { ThemeToggle } from '../common/ThemeToggle';
import { AuthModal } from '../auth/AuthModal';
import { useAuth } from '../../auth/AuthContext';
import { Link, useLocation } from '../../router';
import { cn } from '../../lib/utils';
import {
  useProviderHealthSummary,
  HEALTH_BADGE_COLORS,
} from '../../api/queries/useProviderStatusQueries';
import { ExternalLink, Activity, User, LogOut, Menu, X } from 'lucide-react';

export const Header: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuth();
  const { pathname } = useLocation();
  const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { summaryState, summaryLabel, isLoading } = useProviderHealthSummary();

  const healthBadgeColor = HEALTH_BADGE_COLORS[summaryState];

  // Close mobile menu on route change
  useEffect(() => {
    setIsMobileMenuOpen(false);
  }, [pathname]);

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
        <div className="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            {/* Brand Logo & Name */}
            <div className="flex items-center space-x-6">
              <Link to="/" className="flex items-center space-x-3" exact>
                <span className="font-brand text-2xl font-bold tracking-tight text-text-primary">
                  CloudVitta
                </span>
                <span className="text-xs uppercase tracking-widest px-1.5 py-0.5 border border-border-accent text-border-accent font-bold">
                  Pricing Tool
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

            {/* Mobile Navigation Trigger */}
            <button
              type="button"
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
              className="md:hidden p-2 border border-border-default hover:border-border-accent bg-surface-card text-text-primary transition-colors flex items-center justify-center cursor-pointer focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
              aria-label={isMobileMenuOpen ? 'Close menu' : 'Open menu'}
              aria-expanded={isMobileMenuOpen}
            >
              {isMobileMenuOpen ? (
                <X className="w-5 h-5 text-border-accent" aria-hidden="true" />
              ) : (
                <Menu className="w-5 h-5 text-text-secondary" aria-hidden="true" />
              )}
            </button>

            {/* Navigation Links */}
            <nav className="hidden md:flex items-center space-x-1 lg:space-x-4">
              {navCategories.map((item) => {
                const isActive = pathname === item.href;
                return (
                  <Link
                    key={item.label}
                    to={item.href}
                    className={`px-2.5 py-1 text-xs uppercase font-bold tracking-wider transition-colors border focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none ${
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
            <div className="flex items-center space-x-4">
              {/* Authenticated State vs Sign In Trigger */}
              {isAuthenticated && user ? (
                <div className="flex items-center space-x-2">
                  <div className="hidden sm:flex items-center space-x-1.5 px-2 py-1 border border-border-accent bg-surface-raised text-xs font-bold text-text-primary">
                    <User className="w-3 h-3 text-border-accent" />
                    <span className="truncate max-w-[120px]">{user.email}</span>
                  </div>
                  <button
                    onClick={() => logout()}
                    className="flex items-center space-x-1 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-status-anomaly px-2 py-1.5 border border-border-default bg-surface-card transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
                    title="Log Out"
                  >
                    <LogOut className="w-3 h-3" />
                    <span className="hidden sm:inline">Logout</span>
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => setIsAuthModalOpen(true)}
                  className="flex items-center space-x-1.5 text-xs uppercase font-bold tracking-wider text-text-primary hover:border-border-accent px-2.5 py-1.5 border border-border-default bg-surface-raised transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
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
                className="flex items-center space-x-1.5 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-border-accent px-2 py-1.5 border border-border-default bg-surface-card transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
                title="Open Swagger API Documentation"
              >
                <span>Swagger</span>
                <ExternalLink className="w-3 h-3" />
              </a>

              <ThemeToggle />
            </div>
          </div>
        </div>

        {/* Mobile Navigation Dropdown Panel */}
        {isMobileMenuOpen && (
          <div className="md:hidden border-t border-border-default bg-surface-card px-4 py-3 space-y-1">
            {navCategories.map((item) => {
              const isActive = pathname === item.href;
              return (
                <Link
                  key={item.label}
                  to={item.href}
                  onClick={() => setIsMobileMenuOpen(false)}
                  className={`block px-3 py-2 text-xs uppercase font-bold tracking-wider transition-colors border focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none ${
                    isActive
                      ? 'border-border-accent text-text-primary bg-surface-raised'
                      : 'border-transparent text-text-secondary hover:text-text-primary hover:bg-surface-raised'
                  }`}
                >
                  {item.label}
                </Link>
              );
            })}
          </div>
        )}
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
