// web/src/components/compare/CompareTemplate.tsx
import React, { useState, useMemo } from 'react';
import { RegionSelect, CurrencySelect } from '../forms/taxonomy/TaxonomySelects';
import { HonestyContractBanner } from '../honesty/HonestyContractBanner';
import { WarningsBanner } from '../honesty/WarningsBanner';
import { BentoMetricHighlights } from './BentoMetricHighlights';
import { ProviderCompareCard } from './ProviderCompareCard';
import { MatchQualityBadge } from '../honesty/MatchQualityBadge';
import { MissingAttributesIndicator } from '../honesty/MissingAttributesIndicator';
import { StaleDataBadge } from '../honesty/StaleDataBadge';
import { PriceDisplay, PriceDisplayProps } from '../honesty/PriceDisplay';
import { CompareSkeleton } from './CompareSkeleton';
import { formatProviderName, formatRelativeTime } from '../../lib/format';
import {
  ArrowUpDown,
  RotateCcw,
  AlertTriangle,
  RefreshCw,
  LayoutGrid,
  Table as TableIcon,
} from 'lucide-react';
import type { ProviderWarning, PriceDetail } from '../../types/api';
import { ApiError } from '../../api/errors';
import { cn } from '../../lib/utils';

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
  const [timeframe, setTimeframe] = useState<'hourly' | 'monthly'>('hourly');
  const [viewMode, setViewMode] = useState<'cards' | 'table'>('cards');
  const [isUpdating, setIsUpdating] = useState(false);

  const safeResults = useMemo(() => results ?? [], [results]);

  const sortedResults = useMemo(() => {
    return [...safeResults].sort((a, b) => {
      const priceA = a.normalized_hourly_usd ?? a.monthly_cost_usd ?? a.price?.amount ?? Infinity;
      const priceB = b.normalized_hourly_usd ?? b.monthly_cost_usd ?? b.price?.amount ?? Infinity;
      if (priceA === priceB) return 0;
      return sortOrder === 'asc' ? Number(priceA) - Number(priceB) : Number(priceB) - Number(priceA);
    });
  }, [safeResults, sortOrder]);

  const anomalyProviders = useMemo(() => {
    const set = new Set<string>();
    warnings.forEach((w) => {
      if (w.code === 'pricing_anomaly_flagged') {
        set.add(w.provider?.toLowerCase() || '');
      }
    });
    return set;
  }, [warnings]);

  const handleUpdateMatrix = () => {
    setIsUpdating(true);
    refetch?.();
    setTimeout(() => setIsUpdating(false), 500);
  };

  return (
    <div className="w-full space-y-7">
      {/* 1. Page Title & Subheader Area */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-md bg-stone-200/70 dark:bg-surface-raised border border-border-default/80 text-[11px] font-semibold text-text-secondary uppercase tracking-wider mb-2">
            <span>TOPOLOGY V4.2</span>
            <span className="text-text-secondary/50">•</span>
            <span>Live Multi-Cloud Pricing Engine</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold text-text-primary tracking-tight font-display">
            {categoryTitle}
          </h1>
          {categoryDescription && (
            <p className="text-text-secondary text-sm sm:text-base mt-1.5 max-w-2xl leading-relaxed">
              {categoryDescription}
            </p>
          )}
        </div>

        {/* Hourly vs Monthly Toggle */}
        <div className="flex items-center self-start md:self-auto bg-stone-200/80 dark:bg-surface-raised p-1 rounded-xl border border-border-default/80">
          <button
            type="button"
            onClick={() => setTimeframe('hourly')}
            className={cn(
              'px-3.5 py-1.5 text-xs font-semibold rounded-lg transition-all',
              timeframe === 'hourly'
                ? 'bg-brand-500 text-white shadow-sm'
                : 'text-text-secondary hover:text-text-primary'
            )}
          >
            Hourly ($/hr)
          </button>
          <button
            type="button"
            onClick={() => setTimeframe('monthly')}
            className={cn(
              'px-3.5 py-1.5 text-xs font-semibold rounded-lg transition-all',
              timeframe === 'monthly'
                ? 'bg-brand-500 text-white shadow-sm'
                : 'text-text-secondary hover:text-text-primary'
            )}
          >
            Monthly (730h)
          </button>
        </div>
      </div>

      {/* 2. Floating Query Console (Header / Filter Matrix Panel) */}
      <section className="w-full bg-surface-card rounded-2xl border border-border-default/80 shadow-sm p-5 space-y-4">
        <div className="flex flex-col lg:flex-row items-stretch lg:items-end justify-between gap-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3.5 flex-1">
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
            {sidebarControls}
          </div>

          <div className="flex items-center gap-2 pt-1 lg:pt-0 shrink-0">
            <button
              type="button"
              onClick={handleUpdateMatrix}
              className="flex-1 lg:flex-none h-[38px] px-4 rounded-xl bg-brand-500 hover:bg-brand-600 text-white font-semibold text-xs flex items-center justify-center gap-2 shadow-sm shadow-brand-500/20 transition-all cursor-pointer"
            >
              <RefreshCw className={cn('w-4 h-4', isUpdating && 'animate-spin')} />
              <span>Update Matrix</span>
            </button>

            <button
              type="button"
              onClick={onReset}
              className="h-[38px] px-3 rounded-xl bg-surface-raised hover:bg-surface-card text-text-secondary hover:text-text-primary border border-border-default/80 text-xs font-semibold flex items-center justify-center gap-1.5 transition-colors cursor-pointer"
              title="Reset query parameters to defaults"
              data-testid="reset-query-btn"
            >
              <RotateCcw className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Reset</span>
            </button>
          </div>
        </div>

        {/* Secondary Parity Tuning Bar */}
        <div className="pt-3 border-t border-border-default/60 flex flex-wrap items-center justify-between gap-3 text-xs text-text-secondary">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-text-primary font-mono text-[11px] uppercase tracking-wider">Active Query:</span>
            <span className="font-mono text-text-secondary text-xs">{querySummary}</span>
          </div>

          <div className="flex items-center gap-1.5 text-text-secondary font-medium">
            <span className="w-2 h-2 rounded-full bg-status-matchExact"></span>
            <span>7 Providers Synced ({results.length} matched options)</span>
          </div>
        </div>
      </section>

      {/* 3. Honesty Contract Banner */}
      <HonestyContractBanner />

      {/* Warnings Banner (if any) */}
      {warnings.length > 0 && (
        <WarningsBanner warnings={warnings} data-testid="compare-warnings-banner" />
      )}

      {/* 4. Bento Metric Highlights Row */}
      {!isLoading && !isError && sortedResults.length > 0 && (
        <BentoMetricHighlights
          results={sortedResults}
          currency={currency}
          timeframe={timeframe}
        />
      )}

      {/* 5. Main Results Container */}
      <div className="space-y-4">
        {/* Results Matrix Header Bar with View Switcher */}
        <div className="bg-surface-card rounded-2xl border border-border-default/80 p-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 shadow-2xs">
          <div className="flex items-center gap-3">
            <h2 className="font-bold text-base text-text-primary tracking-tight">
              Provider Comparison Matrix
            </h2>
            <span className="px-2 py-0.5 rounded-full text-xs font-mono bg-surface-raised text-text-secondary border border-border-default/60">
              {results.length} SKUs
            </span>
          </div>

          <div className="flex items-center gap-3">
            {/* View Mode Toggle: [ Cards Grid | Table View ] */}
            <div className="flex items-center bg-surface-raised p-1 rounded-xl border border-border-default/80">
              <button
                type="button"
                onClick={() => setViewMode('cards')}
                className={cn(
                  'flex items-center gap-1.5 px-2.5 py-1 text-xs font-semibold rounded-lg transition-all',
                  viewMode === 'cards'
                    ? 'bg-brand-500 text-white shadow-sm'
                    : 'text-text-secondary hover:text-text-primary'
                )}
                aria-label="Cards Grid View"
              >
                <LayoutGrid className="w-3.5 h-3.5" />
                <span>Cards</span>
              </button>
              <button
                type="button"
                onClick={() => setViewMode('table')}
                className={cn(
                  'flex items-center gap-1.5 px-2.5 py-1 text-xs font-semibold rounded-lg transition-all',
                  viewMode === 'table'
                    ? 'bg-brand-500 text-white shadow-sm'
                    : 'text-text-secondary hover:text-text-primary'
                )}
                aria-label="Detailed Table View"
              >
                <TableIcon className="w-3.5 h-3.5" />
                <span>Table</span>
              </button>
            </div>

            {/* Price Sort Order Selector */}
            <div className="flex items-center space-x-1 text-xs text-text-secondary">
              <ArrowUpDown className="w-3.5 h-3.5" />
              <label htmlFor="sort-order" className="sr-only">Sort by Price</label>
              <select
                id="sort-order"
                value={sortOrder}
                onChange={(e) => setSortOrder(e.target.value as 'asc' | 'desc')}
                className="px-2.5 py-1 text-xs bg-surface-raised border border-border-default/80 rounded-xl text-text-primary focus:outline-none focus:border-brand-500 cursor-pointer"
                aria-label="Sort by Price"
                data-testid="sort-order-select"
              >
                <option value="asc">Price: Low to High</option>
                <option value="desc">Price: High to Low</option>
              </select>
            </div>
          </div>
        </div>

        {/* Loading State */}
        {isLoading && <CompareSkeleton rows={4} />}

        {/* Error State */}
        {!isLoading && isError && (
          <div
            className="p-6 border border-status-anomaly/80 bg-status-anomaly/5 rounded-2xl space-y-3"
            data-testid="compare-error-container"
          >
            <div className="flex items-center space-x-2 text-status-anomaly font-bold text-sm">
              <AlertTriangle className="w-4 h-4" />
              <span>{error instanceof ApiError ? error.title : 'Query Error'}</span>
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
                className="inline-flex items-center space-x-1.5 px-3.5 py-2 text-xs font-semibold rounded-xl text-white bg-brand-500 hover:bg-brand-600 transition-colors shadow-sm"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>Retry Query</span>
              </button>
            )}
          </div>
        )}

        {/* Empty State */}
        {!isLoading && !isError && sortedResults.length === 0 && (
          <div
            className="p-10 border border-dashed border-border-default rounded-2xl bg-surface-card text-center space-y-2 shadow-2xs"
            data-testid="compare-empty-state"
          >
            <div className="font-bold text-lg text-text-primary">No Matching SKUs Found</div>
            <p className="text-xs text-text-secondary max-w-md mx-auto">
              No cloud provider matched the specified parameters in {region}. Review the warnings
              above or adjust your requirements.
            </p>
          </div>
        )}

        {/* Results Presentation Container with data-testid="results-table" */}
        {!isLoading && !isError && sortedResults.length > 0 && (
          <div data-testid="results-table">
            {viewMode === 'cards' ? (
              /* Bento Cards Grid View (Matching Stitch mockups) */
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-5">
                {sortedResults.map((row, idx) => {
                  const isAnomaly = anomalyProviders.has(row.provider?.toLowerCase() || '');
                  const isLowestTCO = idx === 0 && sortOrder === 'asc';

                  return (
                    <ProviderCompareCard
                      key={`${row.provider || 'unknown'}-${row.sku_id || idx}`}
                      row={row}
                      currency={currency}
                      timeframe={timeframe}
                      isLowestTCO={isLowestTCO}
                      hasAnomaly={isAnomaly}
                      renderCustomSpec={renderCustomSpec as ((row: ComparisonResultRow) => React.ReactNode) | undefined}
                    />
                  );
                })}
              </div>
            ) : (
              /* Detailed Table View */
              <div className="overflow-x-auto border border-border-default/80 rounded-2xl bg-surface-card shadow-sm">
                <table
                  className="w-full min-w-[640px] text-left border-collapse text-xs"
                  aria-label="Provider Comparison Results"
                >
                  <thead>
                    <tr className="border-b border-border-default bg-surface-raised font-mono text-text-secondary uppercase">
                      <th scope="col" className="py-3 px-4 font-bold">Provider</th>
                      <th scope="col" className="py-3 px-4 font-bold">Matched Specification</th>
                      <th scope="col" className="py-3 px-4 font-bold">Match Quality</th>
                      <th scope="col" className="py-3 px-4 font-bold">Price</th>
                      <th scope="col" className="py-3 px-4 font-bold">Freshness</th>
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
                          <td className="py-3.5 px-4 align-top">
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

                          <td className="py-3.5 px-4 align-top">
                            {renderCustomSpec ? (
                              renderCustomSpec(row)
                            ) : (
                              <div className="space-y-0.5">
                                <div className="font-mono text-text-primary font-semibold">
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

                          <td className="py-3.5 px-4 align-top">
                            <MatchQualityBadge
                              quality={matchQuality}
                              score={row.match_score}
                              deltaPct={row.match_delta_pct}
                            />
                          </td>

                          <td className="py-3.5 px-4 align-top">
                            <PriceDisplay
                              amount={row.price?.amount}
                              currency={row.price?.currency || currency}
                              unit={(row.price?.unit as PriceDisplayProps['unit']) || '/hr'}
                              normalizedHourlyUSD={row.normalized_hourly_usd}
                              normalizedMonthlyUSD={row.normalized_monthly_usd || row.monthly_cost_usd}
                              hasAnomaly={isAnomaly}
                            />
                          </td>

                          <td className="py-3.5 px-4 align-top">
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
        )}
      </div>
    </div>
  );
}
