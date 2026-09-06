// web/src/components/compare/CompareSkeleton.tsx
import React from 'react';

export interface CompareSkeletonProps {
  rows?: number;
  className?: string;
}

export const CompareSkeleton: React.FC<CompareSkeletonProps> = ({ rows = 4, className = '' }) => {
  return (
    <div className={`w-full space-y-3 animate-pulse ${className}`} data-testid="compare-skeleton" role="status" aria-label="Loading comparison data">
      <div className="h-10 bg-surface-raised border border-border-default/80 rounded-xl w-full" />
      {Array.from({ length: rows }).map((_, idx) => (
        <div
          key={idx}
          className="h-20 bg-surface-card border border-border-default/80 rounded-2xl p-4 flex items-center justify-between space-x-4 shadow-sm"
        >
          <div className="flex items-center space-x-4 flex-1">
            <div className="h-8 w-20 bg-surface-raised border border-border-default/60 rounded-xl" />
            <div className="h-4 w-32 bg-surface-raised rounded-lg" />
            <div className="h-5 w-24 bg-surface-raised rounded-full" />
          </div>
          <div className="flex items-center space-x-6">
            <div className="h-6 w-28 bg-surface-raised rounded-lg" />
            <div className="h-4 w-16 bg-surface-raised rounded-full" />
          </div>
        </div>
      ))}
    </div>
  );
};
