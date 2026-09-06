// web/src/components/layout/Header.tsx
import React, { useState, useEffect } from 'react';
import { ThemeToggle } from '../common/ThemeToggle';
import { AuthModal } from '../auth/AuthModal';
import { useAuth } from '../../auth/AuthContext';
import { Link, useLocation } from '../../router';
import {
  useProviderHealthSummary,
} from '../../api/queries/useProviderStatusQueries';
import { ExternalLink, User, LogOut, Menu, X } from 'lucide-react';

export const Header: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuth();
  const { pathname } = useLocation();
  const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { summaryLabel, isLoading } = useProviderHealthSummary();

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
      <header className="border-b border-border-default/80 bg-surface-card/85 backdrop-blur-md sticky top-0 z-40 transition-colors">
        <div className="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16 gap-4">
            {/* Brand Logo & Live Telemetry Badge */}
            <div className="flex items-center space-x-4 lg:space-x-6">
              <Link to="/" className="flex items-center space-x-2.5" exact>
                <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-brand-500 to-amber-600 flex items-center justify-center text-white shadow-sm shadow-brand-500/20 shrink-0">
                  <svg className="w-5 h-5 fill-current" viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z" />
                  </svg>
                </div>
                <div className="flex flex-col">
                  <div className="flex items-center gap-1.5 leading-none">
                    <span className="font-brand text-2xl font-bold tracking-wider text-text-primary">
                      CloudVitta
                    </span>
                    <span className="text-[10px] uppercase tracking-widest px-1.5 py-0.5 border border-border-accent text-border-accent font-bold rounded-full">
                      Pricing Tool
                    </span>
                  </div>
                  <span className="text-[11px] text-text-secondary hidden sm:inline font-medium">
                    Multi-Cloud Pricing
                  </span>
                </div>
              </Link>

              {/* Provider Health Status Affordance — Live Telemetry Pill */}
              <Link
                to="/status"
                className="hidden lg:flex items-center space-x-2 text-xs font-semibold px-2.5 py-1 rounded-full border border-emerald-200 dark:border-border-default bg-emerald-50/80 dark:bg-surface-raised text-emerald-800 dark:text-text-primary transition-all hover:border-border-accent"
                activeClassName="border-border-accent"
              >
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
                <span className={isLoading ? 'animate-pulse' : ''}>{summaryLabel}</span>
              </Link>
            </div>

            {/* Mobile Navigation Trigger */}
            <button
              type="button"
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
              className="md:hidden p-2 rounded-xl border border-border-default hover:border-border-accent bg-surface-card text-text-primary transition-colors flex items-center justify-center cursor-pointer focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
              aria-label={isMobileMenuOpen ? 'Close menu' : 'Open menu'}
              aria-expanded={isMobileMenuOpen}
            >
              {isMobileMenuOpen ? (
                <X className="w-5 h-5 text-border-accent" aria-hidden="true" />
              ) : (
                <Menu className="w-5 h-5 text-text-secondary" aria-hidden="true" />
              )}
            </button>

            {/* Navigation Pill Track */}
            <nav className="hidden md:flex items-center gap-1 bg-stone-100/80 dark:bg-surface-raised p-1 rounded-xl border border-border-default/60">
              {navCategories.map((item) => {
                const isActive = pathname === item.href;
                return (
                  <Link
                    key={item.label}
                    to={item.href}
                    className={`px-3 py-1.5 text-xs font-semibold rounded-lg transition-all focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none ${
                      isActive
                        ? 'bg-brand-500 text-white shadow-sm shadow-brand-500/20 font-bold'
                        : 'text-text-secondary hover:text-text-primary hover:bg-white/60 dark:hover:bg-surface-card'
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
                  <div className="hidden sm:flex items-center space-x-1.5 px-2.5 py-1.5 rounded-xl border border-border-accent bg-surface-raised text-xs font-bold text-text-primary">
                    <User className="w-3.5 h-3.5 text-border-accent" />
                    <span className="truncate max-w-[120px]">{user.email}</span>
                  </div>
                  <button
                    onClick={() => logout()}
                    className="flex items-center space-x-1 text-xs font-semibold text-text-secondary hover:text-status-anomaly px-2.5 py-1.5 rounded-xl border border-border-default bg-surface-card transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
                    title="Log Out"
                  >
                    <LogOut className="w-3.5 h-3.5" />
                    <span className="hidden sm:inline">Logout</span>
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => setIsAuthModalOpen(true)}
                  className="flex items-center space-x-1.5 text-xs font-semibold text-text-primary hover:border-border-accent px-3 py-1.5 rounded-xl border border-border-default bg-surface-raised transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none shadow-sm"
                >
                  <User className="w-3.5 h-3.5 text-border-accent" />
                  <span>Sign In</span>
                </button>
              )}

              {/* Swagger Docs Link */}
              <a
                href="/docs/"
                target="_blank"
                rel="noreferrer"
                className="hidden sm:flex items-center space-x-1.5 text-xs font-semibold text-text-secondary hover:text-border-accent px-2.5 py-1.5 rounded-xl border border-border-default bg-surface-card transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
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
                  className={`block px-3 py-2 text-xs font-semibold rounded-xl transition-all border focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none ${
                    isActive
                      ? 'border-border-accent text-white bg-brand-500 font-bold'
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
