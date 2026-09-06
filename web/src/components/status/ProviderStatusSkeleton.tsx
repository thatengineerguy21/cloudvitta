// web/src/components/status/ProviderStatusSkeleton.tsx
import React from 'react';

/**
 * Rectilinear 0px pulsing skeleton for a provider status card.
 * Renders placeholder shapes for: provider name, status badge,
 * last fetch timestamp, and 4 category rows.
 */
export const ProviderStatusSkeleton: React.FC = () => {
  return (
    <div
      className="bg-surface-card border border-border-default/80 rounded-2xl shadow-sm p-6 animate-pulse"
      data-testid="provider-status-skeleton"
      aria-label="Loading provider status"
    >
      {/* Provider name + status badge */}
      <div className="flex items-center justify-between mb-4">
        <div className="h-6 w-32 bg-surface-raised border border-border-default rounded-md" />
        <div className="h-5 w-20 bg-surface-raised border border-border-default rounded-full" />
      </div>

      {/* Last fetch timestamp */}
      <div className="h-4 w-48 bg-surface-raised mb-4 rounded" />

      {/* Category rows */}
      <div className="space-y-3">
        {Array.from({ length: 4 }).map((_, idx) => (
          <div key={idx} className="flex items-center justify-between">
            <div className="h-4 w-24 bg-surface-raised rounded" />
            <div className="flex items-center space-x-3">
              <div className="h-4 w-16 bg-surface-raised rounded" />
              <div className="h-4 w-12 bg-surface-raised rounded" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
