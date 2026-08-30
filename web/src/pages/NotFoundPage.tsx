// web/src/pages/NotFoundPage.tsx
import React from 'react';
import { Link, useLocation } from '../router';

/**
 * 404 Not Found Page.
 * Displays the invalid path and a return home action.
 * Editorial Bento styled with 0px borders.
 */
export const NotFoundPage: React.FC = () => {
  const { pathname } = useLocation();

  return (
    <div className="max-w-lg mx-auto" data-testid="not-found-page">
      <div className="border border-border-default bg-surface-card p-6 sm:p-12 text-center">
        <span className="text-xs font-mono uppercase text-status-anomaly">
          404 &bull; Not Found
        </span>
        <h1 className="font-display text-4xl font-medium text-text-primary mt-3">
          Page Not Found
        </h1>
        <p className="text-sm text-text-secondary mt-3">
          The requested path{' '}
          <code className="text-border-accent font-mono break-all">{pathname}</code>{' '}
          does not exist in the CloudVitta routing registry.
        </p>
        <div className="mt-6">
          <Link
            to="/"
            className="inline-flex items-center space-x-2 px-4 py-2.5 text-xs uppercase font-bold tracking-wider text-text-primary border border-border-default bg-surface-raised hover:border-border-accent transition-colors"
          >
            <span>Return to Landing Page</span>
          </Link>
        </div>
      </div>
    </div>
  );
};
