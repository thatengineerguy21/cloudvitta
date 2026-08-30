// web/src/components/compare/CompareTemplate.tsx
import React, { useState, useMemo } from 'react';
import { BentoGrid } from '../bento/BentoGrid';
import { BentoCard } from '../bento/BentoCard';
import { RegionSelect, CurrencySelect } from '../forms/taxonomy/TaxonomySelects';
import { WarningsBanner } from '../honesty/WarningsBanner';
import { MatchQualityBadge } from '../honesty/MatchQualityBadge';
import { MissingAttributesIndicator } from '../honesty/MissingAttributesIndicator';
import { StaleDataBadge } from '../honesty/StaleDataBadge';
import { PriceDisplay, PriceDisplayProps } from '../honesty/PriceDisplay';
import { CompareSkeleton } from './CompareSkeleton';
import { formatProviderName, formatRelativeTime } from '../../lib/format';
import { ArrowUpDown, RotateCcw, AlertTriangle, RefreshCw } from 'lucide-react';
import type { ProviderWarning, PriceDetail } from '../../types/api';
import { ApiError } from '../../api/errors';

export interface ComparisonResultRow {
  provider?: string;
  sku_id?: string;
  match_quality?: string;
  match_score?: number;
  match_delta_pct?: number;
  missing_attributes?: string[];
  price?: PriceDetail;
  normalized_hourly_usd?: number;
  normalized_monthly_usd?: number;
  monthly_cost_usd?: number;
  stale?: boolean;
  fetched_at?: string;
  instance_type?: string;
  family?: string;
  spec_summary?: string;
  storage_class?: string;
  transfer_type?: string;
  engine?: string;
  multi_az?: boolean;
  data_model?: string;
  pricing_mode?: string;
  multi_region?: boolean;
  tier?: string;
  cluster_topology?: string;
  credit_applied_hourly_usd?: string | number;
  architecture?: string;
  vcpu?: number;
  ram_gb?: number;
  storage_gb?: number;
  iops?: number;
  size_gb?: number;
  egress_gb?: number;
  read_units?: number;
  write_units?: number;
  requests_per_month?: number;
  memory_mb?: number;
  execution_duration_ms?: number;
  matched_spec?: Record<string, unknown>;
}

export interface CompareTemplateProps<TResult extends ComparisonResultRow> {
  categoryTitle: string;
  categoryDescription?: string;
  region: string;
  currency: string;
  onRegionChange: (region: string) => void;
  onCurrencyChange: (currency: string) => void;
  querySummary: string;
  onReset: () => void;
  sidebarControls: React.ReactNode;
  isLoading: boolean;
  isError: boolean;
  error?: Error | ApiError | null;
  refetch?: () => void;
  results?: TResult[];
  warnings?: ProviderWarning[];
  renderCustomSpec?: (row: TResult) => React.ReactNode;
}

export function CompareTemplate<TResult extends ComparisonResultRow>({
  categoryTitle,
  categoryDescription,
  region,
  currency,
  onRegionChange,
  onCurrencyChange,
  querySummary,
  onReset,
  sidebarControls,
  isLoading,
  isError,
  error,
  refetch,
  results = [],
  warnings = [],
  renderCustomSpec,
}: CompareTemplateProps<TResult>) {
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');

  const sortedResults = useMemo(() => {
    return [...results].sort((a, b) => {
      const priceA = a.normalized_hourly_usd != null ? Number(a.normalized_hourly_usd) : Infinity;
      const priceB = b.normalized_hourly_usd != null ? Number(b.normalized_hourly_usd) : Infinity;
      if (priceA === priceB) return 0;
      return sortOrder === 'asc' ? priceA - priceB : priceB - priceA;
    });
  }, [results, sortOrder]);

  const anomalyProviders = useMemo(() => {
    const set = new Set<string>();
    warnings.forEach((w) => {
      if (w.code === 'pricing_anomaly_flagged') {
        set.add(w.provider?.toLowerCase() || '');
      }
    });
    return set;
  }, [warnings]);

  return (
    <div className="w-full space-y-6">
      {/* Page Title & Context Header */}
      <div className="border-b border-border-default pb-4">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
          <div>
            <span className="text-xs uppercase tracking-widest text-border-accent font-bold">
              Comparison View
            </span>
            <h1 className="font-display text-3xl font-medium text-text-primary mt-1">
              {categoryTitle}
            </h1>
            {categoryDescription && (
              <p className="text-xs text-text-secondary mt-1">{categoryDescription}</p>
            )}
          </div>
          <div className="flex items-center space-x-2">
            <span className="px-2.5 py-1 text-xs font-mono border border-border-default bg-surface-raised text-text-secondary">
              {querySummary}
            </span>
          </div>
        </div>
      </div>

      {/* Main 12-Column Grid */}
      <BentoGrid columns={12} gap="md">
        {/* Left Sticky Query Sidebar (3 Columns) */}
        <BentoCard
          colSpan={3}
          header={
            <div className="flex items-center justify-between">
              <h2 className="font-display text-lg font-medium text-text-primary">Query Parameters</h2>
              <span className="text-xs font-mono uppercase text-border-accent font-bold">Curated</span>
            </div>
          }
          className="lg:sticky lg:top-20 h-fit"
        >
          <div className="space-y-4 pt-2">
            {/* Global Region & Currency Selectors */}
            <RegionSelect
              value={region}
              onChange={(e) => onRegionChange(e.target.value)}
              data-testid="global-region-select"
            />
            <CurrencySelect
              value={currency}
              onChange={(e) => onCurrencyChange(e.target.value)}
              data-testid="global-currency-select"
            />

            <div className="border-t border-border-default pt-4 space-y-4">
              {sidebarControls}
            </div>

            <div className="border-t border-border-default pt-4">
              <button
                type="button"
                onClick={onReset}
                className="w-full flex items-center justify-center space-x-1.5 px-3 py-2 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-text-primary hover:bg-surface-raised border border-border-default bg-surface-card transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
                data-testid="reset-query-btn"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Reset to Defaults</span>
              </button>
            </div>
          </div>
        </BentoCard>

        {/* Right Results Container (9 Columns) */}
        <BentoCard
          colSpan={9}
          header={
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
              <div className="flex items-center space-x-2">
                <h2 className="font-display text-lg font-medium text-text-primary">Normalized Comparison</h2>
                <span className="text-xs font-mono text-text-secondary font-normal">
                  ({results.length} matched)
                </span>
              </div>
              <div className="flex items-center space-x-2">
                <label htmlFor="sort-order" className="sr-only">Sort by Price</label>
                <div className="flex items-center space-x-1 text-xs text-text-secondary">
                  <ArrowUpDown className="w-3 h-3" />
                  <span className="font-mono">Sort:</span>
                </div>
                <select
                  id="sort-order"
                  value={sortOrder}
                  onChange={(e) => setSortOrder(e.target.value as 'asc' | 'desc')}
                  className="px-2 py-1 text-xs bg-surface-raised border border-border-default text-text-primary focus:outline-none focus:border-border-accent focus-visible:ring-1 focus-visible:ring-border-accent"
                  aria-label="Sort by Price"
                  data-testid="sort-order-select"
                >
                  <option value="asc">Price: Low to High</option>
                  <option value="desc">Price: High to Low</option>
                </select>
              </div>
            </div>
          }
        >
          <div className="space-y-4 pt-2">
            {/* Warnings Banner */}
            {warnings.length > 0 && (
              <WarningsBanner warnings={warnings} data-testid="compare-warnings-banner" />
            )}

            {/* Loading State */}
            {isLoading && <CompareSkeleton rows={4} />}

            {/* Error State */}
            {!isLoading && isError && (
              <div
                className="p-6 border border-status-anomaly bg-surface-raised space-y-3"
                data-testid="compare-error-container"
              >
                <div className="flex items-center space-x-2 text-status-anomaly font-bold text-sm">
                  <AlertTriangle className="w-4 h-4" />
                  <span>
                    {error instanceof ApiError ? error.title : 'Query Error'}
                  </span>
                </div>
                <p className="text-xs text-text-secondary">
                  {error instanceof ApiError
                    ? error.detail
                    : error?.message || 'Failed to fetch comparison prices.'}
                </p>
                {refetch && (
                  <button
                    type="button"
                    onClick={() => refetch()}
                    className="inline-flex items-center space-x-1.5 px-3 py-1.5 text-xs uppercase font-bold tracking-wider text-text-primary hover:border-border-accent border border-border-default bg-surface-card transition-colors focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
                  >
                    <RefreshCw className="w-3 h-3" />
                    <span>Retry Query</span>
                  </button>
                )}
              </div>
            )}

            {/* Empty State */}
            {!isLoading && !isError && sortedResults.length === 0 && (
              <div
                className="p-8 border border-dashed border-border-default bg-surface-raised text-center space-y-2"
                data-testid="compare-empty-state"
              >
                <div className="font-display text-lg text-text-primary">No Matching SKUs Found</div>
                <p className="text-xs text-text-secondary max-w-md mx-auto">
                  No cloud provider matched the specified parameters in {region}. Review the warnings
                  above or adjust your requirements.
                </p>
              </div>
            )}

            {/* Results Table */}
            {!isLoading && !isError && sortedResults.length > 0 && (
              <div className="overflow-x-auto border border-border-default">
                <table className="w-full min-w-[640px] text-left border-collapse text-xs" data-testid="results-table" aria-label="Provider Comparison Results">
                  <thead>
                    <tr className="border-b border-border-default bg-surface-raised font-mono text-text-secondary uppercase">
                      <th scope="col" className="py-2.5 px-4 font-bold">Provider</th>
                      <th scope="col" className="py-2.5 px-4 font-bold">Matched Specification</th>
                      <th scope="col" className="py-2.5 px-4 font-bold">Match Quality</th>
                      <th scope="col" className="py-2.5 px-4 font-bold">Price</th>
                      <th scope="col" className="py-2.5 px-4 font-bold">Freshness</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-default bg-surface-card">
                    {sortedResults.map((row, idx) => {
                      const isAnomaly = anomalyProviders.has(row.provider?.toLowerCase() || '');
                      const matchQuality = (row.match_quality as 'exact' | 'close' | 'approximate') || 'approximate';
                      return (
                        <tr
                          key={`${row.provider || 'unknown'}-${row.sku_id || idx}`}
                          className="hover:bg-surface-raised transition-colors"
                          data-testid={`row-${row.provider}`}
                        >
                          {/* Provider Column */}
                          <td className="py-3 px-4 align-top">
                            <div className="flex flex-col">
                              <span className="font-bold text-text-primary uppercase tracking-wide">
                                {formatProviderName(row.provider)}
                              </span>
                              {row.sku_id && (
                                <span className="font-mono text-[10px] text-text-secondary truncate max-w-[120px]" title={row.sku_id}>
                                  {row.sku_id}
                                </span>
                              )}
                            </div>
                          </td>

                          {/* Specification Column */}
                          <td className="py-3 px-4 align-top">
                            {renderCustomSpec ? (
                              renderCustomSpec(row)
                            ) : (
                              <div className="space-y-0.5">
                                <div className="font-mono text-text-primary font-medium">
                                  {row.instance_type || row.spec_summary || 'Standard SKU'}
                                </div>
                                {row.missing_attributes && row.missing_attributes.length > 0 && (
                                  <MissingAttributesIndicator
                                    missingAttributes={row.missing_attributes}
                                  />
                                )}
                              </div>
                            )}
                          </td>

                          {/* Match Quality Column */}
                          <td className="py-3 px-4 align-top">
                            <MatchQualityBadge
                              quality={matchQuality}
                              score={row.match_score}
                              deltaPct={row.match_delta_pct}
                            />
                          </td>

                          {/* Price Column */}
                          <td className="py-3 px-4 align-top">
                            <PriceDisplay
                              amount={row.price?.amount}
                              currency={row.price?.currency || currency}
                              unit={(row.price?.unit as PriceDisplayProps['unit']) || '/hr'}
                              normalizedHourlyUSD={row.normalized_hourly_usd}
                              normalizedMonthlyUSD={row.normalized_monthly_usd || row.monthly_cost_usd}
                              hasAnomaly={isAnomaly}
                            />
                          </td>

                          {/* Freshness Column */}
                          <td className="py-3 px-4 align-top">
                            {row.stale ? (
                              <StaleDataBadge fetchedAt={row.fetched_at} />
                            ) : (
                              <span className="text-text-secondary text-[11px] font-mono">
                                {row.fetched_at ? formatRelativeTime(row.fetched_at) : 'Active'}
                              </span>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </BentoCard>
      </BentoGrid>
    </div>
  );
}
