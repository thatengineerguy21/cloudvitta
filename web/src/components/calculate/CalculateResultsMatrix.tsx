// web/src/components/calculate/CalculateResultsMatrix.tsx
import React, { useState, useMemo } from 'react';
import { cn } from '../../lib/utils';
import { formatProviderName } from '../../lib/format';
import { PriceDisplay } from '../honesty/PriceDisplay';
import { WarningsBanner } from '../honesty/WarningsBanner';
import { CalculateCategoryBreakdown } from './CalculateCategoryBreakdown';
import { CompareSkeleton } from '../compare/CompareSkeleton';
import { ApiError } from '../../api/errors';
import type {
  CalculateProviderResult,
  ProviderWarning,
} from '../../types/api';
import {
  ArrowUpDown,
  ChevronDown,
  ChevronUp,
  AlertTriangle,
  RefreshCw,
  Layers,
} from 'lucide-react';

export interface CalculateResultsMatrixProps {
  results?: CalculateProviderResult[];
  warnings?: ProviderWarning[];
  requestedCategories: string[];
  currency?: string;
  isLoading?: boolean;
  isError?: boolean;
  error?: Error | ApiError | null;
  refetch?: () => void;
  onReset?: () => void;
}

function getEffectivePrice(result: CalculateProviderResult): number {
  if (result.total_normalized_hourly_usd != null) {
    return Number(result.total_normalized_hourly_usd);
  }
  if (result.partial_total_normalized_hourly_usd != null) {
    return Number(result.partial_total_normalized_hourly_usd);
  }
  return Infinity;
}

export const CalculateResultsMatrix: React.FC<CalculateResultsMatrixProps> = ({
  results = [],
  warnings = [],
  requestedCategories,
  isLoading = false,
  isError = false,
  error,
  refetch,
}) => {
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');
  const [expandedProviders, setExpandedProviders] = useState<Record<string, boolean>>({});
  const toggleProviderExpanded = (provider: string) => {
    setExpandedProviders((prev) => ({
      ...prev,
      [provider]: !prev[provider],
    }));
  };

  const safeResults = useMemo(() => results ?? [], [results]);

  const setAllExpanded = (expanded: boolean) => {
    const next: Record<string, boolean> = {};
    safeResults.forEach((r) => {
      if (r.provider) next[r.provider] = expanded;
    });
    setExpandedProviders(next);
  };

  // ADR 0022 Honesty Sorting: Complete workload results always sort ahead of partial estimates
  const sortedResults = useMemo(() => {
    return [...safeResults].sort((a, b) => {
      const aIsPartial = Boolean(a.partial);
      const bIsPartial = Boolean(b.partial);

      if (aIsPartial !== bIsPartial) {
        return aIsPartial ? 1 : -1;
      }

      const priceA = getEffectivePrice(a);
      const priceB = getEffectivePrice(b);
      return sortOrder === 'asc' ? priceA - priceB : priceB - priceA;
    });
  }, [safeResults, sortOrder]);

  if (isError) {
    const isApiErr = error instanceof ApiError;
    return (
      <div className="border border-status-anomaly bg-surface-card p-6 space-y-4">
        <div className="flex items-center space-x-2 text-status-anomaly">
          <AlertTriangle className="w-5 h-5" />
          <h3 className="font-display text-lg font-semibold">
            {isApiErr ? (error as ApiError).title : 'Calculation Error'}
          </h3>
        </div>
        <p className="text-sm text-text-secondary">
          {error?.message || 'An unexpected error occurred during workload calculation.'}
        </p>
        {isApiErr && ((error as ApiError).invalidParams || (error as ApiError).rawProblem?.invalid_params) && (
          <ul className="text-xs font-mono space-y-1 bg-surface-raised p-3 border border-border-default">
            {((error as ApiError).invalidParams || (error as ApiError).rawProblem?.invalid_params)?.map((p, idx) => (
              <li key={idx} className="text-status-anomaly">
                {p.name}: {p.reason}
              </li>
            ))}
          </ul>
        )}
        {refetch && (
          <button
            type="button"
            onClick={refetch}
            className="flex items-center space-x-1.5 text-xs font-bold uppercase tracking-wider px-3 py-2 border border-border-default bg-surface-raised hover:border-border-accent text-text-primary transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            <span>Retry Calculation</span>
          </button>
        )}
      </div>
    );
  }

  if (isLoading) {
    return <CompareSkeleton rows={4} />;
  }

  if (requestedCategories.length === 0) {
    return (
      <div className="border border-dashed border-border-default bg-surface-card p-12 text-center space-y-3">
        <Layers className="w-8 h-8 text-text-secondary mx-auto" />
        <h3 className="font-display text-xl font-medium text-text-primary">
          No Workload Components Selected
        </h3>
        <p className="text-sm text-text-secondary max-w-md mx-auto">
          Enable at least one category in the workload builder on the left to calculate normalized infrastructure costs across cloud providers.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Top Header & Sort Toolbar */}
      <div className="flex items-center justify-between p-3.5 border border-border-default bg-surface-card">
        <div className="flex items-center space-x-3">
          <span className="text-xs uppercase tracking-widest font-bold text-text-primary">
            Provider Architecture Matrix
          </span>
          <span className="text-xs font-mono text-text-secondary">
            ({sortedResults.length} Providers Evaluated)
          </span>
        </div>

        <div className="flex items-center space-x-3">
          <button
            type="button"
            onClick={() => setSortOrder((prev) => (prev === 'asc' ? 'desc' : 'asc'))}
            className="flex items-center space-x-1.5 text-xs font-semibold text-text-secondary hover:text-text-primary px-2.5 py-1 border border-border-default bg-surface-raised hover:border-border-accent transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
            title={`Sort by total price (${sortOrder === 'asc' ? 'Lowest First' : 'Highest First'})`}
          >
            <ArrowUpDown className="w-3 h-3" />
            <span className="uppercase tracking-wider">
              {sortOrder === 'asc' ? 'Price: Low → High' : 'Price: High → Low'}
            </span>
          </button>

          <div className="hidden sm:flex items-center space-x-1.5 text-xs">
            <button
              type="button"
              onClick={() => setAllExpanded(true)}
              className="text-border-accent hover:underline font-medium focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
            >
              Expand All
            </button>
            <span className="text-border-default">•</span>
            <button
              type="button"
              onClick={() => setAllExpanded(false)}
              className="text-text-secondary hover:text-text-primary font-medium focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
            >
              Collapse All
            </button>
          </div>
        </div>
      </div>

      {/* Top Warnings Banner */}
      {warnings.length > 0 && <WarningsBanner warnings={warnings} />}

      {/* Provider Matrix Cards */}
      <div className="space-y-3">
        {sortedResults.map((result) => {
          const provider = result.provider || 'unknown';
          const isExpanded = expandedProviders[provider] ?? true;
          const isPartial = Boolean(result.partial);
          const matchedCategoryCount = Object.keys(result.categories || {}).length;
          const totalRequestedCount = requestedCategories.length;

          return (
            <div
              key={provider}
              className={cn(
                'border bg-surface-card transition-colors',
                isPartial ? 'border-border-default' : 'border-border-default hover:border-border-accent/60'
              )}
            >
              {/* Provider Row Summary Header */}
              <div
                role="button"
                tabIndex={0}
                onClick={() => toggleProviderExpanded(provider)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    toggleProviderExpanded(provider);
                  }
                }}
                className="flex flex-col sm:flex-row sm:items-center justify-between p-4 cursor-pointer hover:bg-surface-raised/40 transition-colors border-b border-border-default/40 gap-3 focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
              >
                {/* Left: Provider Identity & Badge */}
                <div className="flex items-center space-x-3 min-w-0">
                  <div
                    data-testid={`provider-name-${provider}`}
                    className="px-2.5 py-1 border border-border-default bg-surface-raised text-xs font-mono font-bold uppercase tracking-wider text-text-primary"
                  >
                    {formatProviderName(provider)}
                  </div>
                  <span className="text-xs text-text-secondary font-mono">
                    {matchedCategoryCount} of {totalRequestedCount} categories matched
                  </span>
                </div>

                {/* Right: ADR 0022 Honesty Contract Price Display */}
                <div className="flex items-center space-x-4">
                  {isPartial ? (
                    // ADR 0022 PARTIAL TOTAL CONTAINER: Harsh 0px dashed border, complete total cell is blank
                    <div className="flex flex-col items-end">
                      <div
                        className="border border-dashed border-status-partial text-status-partial bg-status-partial/5 px-3 py-1 text-xs font-mono font-bold flex items-center space-x-1.5"
                        data-testid={`partial-total-${provider}`}
                      >
                        <span>Partial Estimate:</span>
                        <PriceDisplay
                          amount={result.partial_total_normalized_hourly_usd}
                          currency="USD"
                          unit="/hr"
                          isPartial
                          className="inline-flex space-y-0"
                        />
                      </div>
                      <span className="text-[10px] text-status-partial/80 mt-0.5 font-mono">
                        ({matchedCategoryCount}/{totalRequestedCount} categories included)
                      </span>
                    </div>
                  ) : (
                    // ADR 0022 COMPLETE TOTAL CONTAINER: Solid typography
                    <div className="flex flex-col items-end" data-testid={`complete-total-${provider}`}>
                      <PriceDisplay
                        amount={result.total_normalized_hourly_usd}
                        currency="USD"
                        unit="/hr"
                      />
                      <span className="text-[10px] text-text-secondary uppercase tracking-widest font-mono">
                        Complete Workload Total
                      </span>
                    </div>
                  )}

                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      toggleProviderExpanded(provider);
                    }}
                    aria-label={isExpanded ? `Collapse ${provider}` : `Expand ${provider}`}
                    className="p-1 hover:bg-surface-raised text-text-secondary hover:text-text-primary transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
                  >
                    {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              {/* Expanded Category Breakdown */}
              {isExpanded && (
                <div className="p-4 bg-surface-raised/20 border-t border-border-default/30">
                  <div className="text-[11px] uppercase tracking-wider font-bold text-text-secondary mb-2">
                    Component Breakdown ({formatProviderName(provider)})
                  </div>
                  <CalculateCategoryBreakdown
                    categories={result.categories}
                    requestedCategories={requestedCategories}
                    provider={provider}
                    warnings={warnings}
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
