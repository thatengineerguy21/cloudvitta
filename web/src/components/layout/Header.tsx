import React from 'react';
import { ThemeToggle } from '../common/ThemeToggle';
import { ExternalLink, Activity } from 'lucide-react';

export const Header: React.FC = () => {
  const navCategories = [
    { label: 'Compute', href: '#/compare/compute' },
    { label: 'Storage', href: '#/compare/storage' },
    { label: 'Network', href: '#/compare/network' },
    { label: 'RDBMS', href: '#/compare/database' },
    { label: 'NoSQL', href: '#/compare/database-nosql' },
    { label: 'Kubernetes', href: '#/compare/kubernetes' },
    { label: 'Serverless', href: '#/compare/serverless' },
    { label: 'Calculate', href: '#/calculate' },
  ];

  return (
    <header className="border-b border-border-default bg-surface-card sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Brand Logo & Name */}
          <div className="flex items-center space-x-6">
            <a href="#/" className="flex items-center space-x-3">
              <span className="font-display text-2xl font-bold tracking-tight text-text-primary">
                CloudVitta
              </span>
              <span className="text-xs uppercase tracking-widest px-1.5 py-0.5 border border-border-accent text-border-accent font-bold">
                API Engine
              </span>
            </a>

            {/* Provider Health Status Affordance */}
            <a
              href="#/status"
              className="hidden lg:flex items-center space-x-2 text-xs font-semibold text-text-secondary hover:text-text-primary px-2.5 py-1 border border-border-default bg-surface-raised transition-colors"
            >
              <Activity className="w-3.5 h-3.5 text-status-matchExact" />
              <span>7/7 Providers Active</span>
            </a>
          </div>

          {/* Navigation Links */}
          <nav className="hidden md:flex items-center space-x-1 lg:space-x-2">
            {navCategories.map((item) => (
              <a
                key={item.label}
                href={item.href}
                className="px-2.5 py-1 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-text-primary hover:bg-surface-raised transition-colors"
              >
                {item.label}
              </a>
            ))}
          </nav>

          {/* Action Bar (Swagger UI, Auth Modal Trigger, Theme Toggle) */}
          <div className="flex items-center space-x-3">
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
  );
};
