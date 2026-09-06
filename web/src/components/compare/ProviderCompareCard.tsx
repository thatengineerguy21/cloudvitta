import React, { useState } from 'react';
import { cn } from '../../lib/utils';
import { formatProviderName, formatRelativeTime } from '../../lib/format';
import { MatchQualityBadge, UIMatchQuality } from '../honesty/MatchQualityBadge';
import { MissingAttributesIndicator } from '../honesty/MissingAttributesIndicator';
import { StaleDataBadge } from '../honesty/StaleDataBadge';
import { AnomalyFlag } from '../honesty/AnomalyFlag';
import { ProviderIcon } from '../common/ProviderIcon';
import {
  ArrowRight,
  Check,
  Code,
  X,
} from 'lucide-react';
import type { ComparisonResultRow } from './CompareTemplate';

export interface ProviderCompareCardProps {
  row: ComparisonResultRow;
  currency?: string;
  timeframe?: 'hourly' | 'monthly';
  isLowestTCO?: boolean;
  hasAnomaly?: boolean;
  renderCustomSpec?: (row: ComparisonResultRow) => React.ReactNode;
}

const getProviderIconBg = (provider?: string) => {
  const p = provider?.toLowerCase() || '';
  if (p === 'aws') return 'bg-amber-50 dark:bg-amber-950/40 text-[#232F3E] dark:text-amber-400 border-amber-200 dark:border-amber-800/40';
  if (p === 'azure') return 'bg-sky-50 dark:bg-sky-950/40 text-sky-700 dark:text-sky-400 border-sky-200 dark:border-sky-800/40';
  if (p === 'gcp') return 'bg-white dark:bg-neutral-900 border-border-default/80 shadow-2xs';
  if (p === 'oracle') return 'bg-red-50 dark:bg-red-950/40 text-red-600 dark:text-red-400 border-red-200 dark:border-red-900/40';
  if (p === 'digitalocean') return 'bg-blue-50 dark:bg-blue-950/40 text-blue-600 border-blue-200 dark:border-blue-900/40';
  if (p === 'ibm') return 'bg-indigo-50 dark:bg-indigo-950/40 text-indigo-700 border-indigo-200 dark:border-indigo-900/40';
  if (p === 'alibaba') return 'bg-orange-50 dark:bg-orange-950/40 text-orange-600 border-orange-200 dark:border-orange-900/40';
  return 'bg-surface-raised text-text-primary border-border-default';
};

export const ProviderCompareCard: React.FC<ProviderCompareCardProps> = ({
  row,
  currency = 'USD',
  timeframe = 'hourly',
  isLowestTCO = false,
  hasAnomaly = false,
  renderCustomSpec,
}) => {
  const [isRawJsonOpen, setIsRawJsonOpen] = useState(false);
  const [isTierSelected, setIsTierSelected] = useState(false);

  const parseNum = (val: unknown): number => {
    if (val === undefined || val === null || val === '') return 0;
    const n = typeof val === 'number' ? val : Number(val);
    return isNaN(n) ? 0 : n;
  };

  const hourlyPrice = parseNum(row.normalized_hourly_usd ?? row.price?.amount);
  const monthlyPrice =
    row.normalized_monthly_usd !== undefined || row.monthly_cost_usd !== undefined
      ? parseNum(row.normalized_monthly_usd ?? row.monthly_cost_usd)
      : hourlyPrice * 730;

  const displayPrice = timeframe === 'monthly' ? monthlyPrice : hourlyPrice;
  const unitLabel = timeframe === 'monthly' ? '/ mo' : '/ hr';
  const currSymbol = currency === 'USD' ? '$' : `${currency} `;

  const formatAmount = (val: unknown): string => {
    const num = parseNum(val);
    if (num >= 100) {
      return num.toLocaleString('en-US', {
        minimumFractionDigits: Number.isInteger(num) ? 0 : 2,
        maximumFractionDigits: 2,
      });
    }
    return num.toLocaleString('en-US', { minimumFractionDigits: 4, maximumFractionDigits: 4 });
  };

  const iconStyle = getProviderIconBg(row.provider);
  const matchQuality = (row.match_quality as UIMatchQuality) || 'approximate';

  const spec = row.matched_spec as Record<string, unknown> | undefined;
  const vcpu = (spec?.vcpu as number | undefined) ?? row.vcpu;
  const ram = (spec?.ram_gb as number | undefined) ?? (spec?.memory_gib as number | undefined) ?? row.ram_gb;
  const network = (spec?.network_performance as string | undefined) ?? (spec?.network as string | undefined);
  const rawSilicon = (spec?.cpu_architecture as string | undefined) || row.architecture;
  const silicon = rawSilicon || (row.family ? `${row.family} Gen` : undefined);

  const hasComputeSpecs = vcpu !== undefined || ram !== undefined || Boolean(row.family) || Boolean(spec?.family);

  // Derive human-readable title without colliding with custom spec renderers
  const instanceType = (spec?.instance_type as string | undefined) || row.instance_type;
  const displayTitle = instanceType || row.spec_summary || 'Standard SKU';

  return (
    <div
      data-testid={`row-${row.provider || 'unknown'}`}
      className={cn(
        'bg-surface-card rounded-2xl border p-5 flex flex-col justify-between transition-all duration-200 relative',
        isLowestTCO
          ? 'border-2 border-brand-500 shadow-md ring-4 ring-brand-500/10'
          : 'border-border-default/80 shadow-sm hover:border-brand-500/30 hover:shadow-md'
      )}
    >
      {/* Floating Lowest TCO Pill */}
      {isLowestTCO && (
        <div className="absolute -top-3 right-6 bg-brand-500 text-white text-[10px] font-extrabold uppercase px-2.5 py-0.5 rounded-full tracking-wider shadow-sm">
          Lowest TCO
        </div>
      )}

      <div>
        {/* Top Header Row */}
        <div className="flex items-start gap-3">
          <div className={cn('w-9 h-9 rounded-xl border flex items-center justify-center shrink-0 shadow-2xs mt-0.5 p-1.5', iconStyle)}>
            <ProviderIcon provider={row.provider} className="w-full h-full object-contain" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center justify-between gap-1.5">
              <span className="text-[11px] font-bold uppercase text-text-secondary tracking-wider truncate">
                {formatProviderName(row.provider)}
              </span>
              <MatchQualityBadge
                quality={matchQuality}
                score={row.match_score}
                deltaPct={row.match_delta_pct}
                className="shrink-0"
              />
            </div>
            <h3 className="font-bold text-text-primary text-base font-mono leading-tight mt-1 truncate" title={displayTitle}>
              {displayTitle}
            </h3>
          </div>
        </div>

        {/* Missing attributes alert if applicable */}
        {row.missing_attributes && row.missing_attributes.length > 0 && (
          <div className="mt-2.5">
            <MissingAttributesIndicator missingAttributes={row.missing_attributes} />
          </div>
        )}

        {/* Specification Section */}
        {hasComputeSpecs ? (
          <>
            {/* Silicon / Sub-architecture Tag */}
            <div className="mt-3.5 pt-2.5 border-t border-border-default/50 flex items-center justify-between text-xs">
              <span className="text-text-secondary">Silicon / Profile:</span>
              <span className="font-semibold text-text-primary text-right truncate max-w-[180px]">
                {row.family ? `${row.family} ` : ''}
                <span className="text-text-secondary font-normal font-mono text-[11px]">({silicon || 'x86_64'})</span>
              </span>
            </div>

            {/* 2x2 Spec Matrix Pills */}
            <div className="grid grid-cols-2 gap-2 mt-3 bg-surface-raised p-3 rounded-xl border border-border-default/60">
              <div>
                <span className="text-[10px] font-bold uppercase text-text-secondary block">vCPU</span>
                <span className="font-bold text-text-primary text-sm font-mono">
                  {vcpu ? `${vcpu} Cores` : 'Included'}
                </span>
              </div>
              <div>
                <span className="text-[10px] font-bold uppercase text-text-secondary block">Memory</span>
                <span className="font-bold text-text-primary text-sm font-mono">
                  {ram ? `${ram} GiB` : 'Dynamic'}
                </span>
              </div>
              <div className="mt-1">
                <span className="text-[10px] font-bold uppercase text-text-secondary block">Network</span>
                <span className="font-bold text-text-primary text-xs font-mono truncate block" title={network || 'Standard'}>
                  {network || 'Up to 12.5G'}
                </span>
              </div>
              <div className="mt-1">
                <span className="text-[10px] font-bold uppercase text-text-secondary block">SKU Code</span>
                <span className="font-bold text-text-primary text-xs font-mono truncate block" title={row.sku_id}>
                  {row.sku_id || 'Base'}
                </span>
              </div>
            </div>

            {/* Custom Specification Renderer (if provided on compute-like card, e.g. RDBMS) */}
            {renderCustomSpec && (
              <div className="mt-3 pt-2 border-t border-border-default/40">
                {renderCustomSpec(row)}
              </div>
            )}
          </>
        ) : renderCustomSpec ? (
          <div className="mt-3.5 p-3 rounded-xl bg-surface-raised border border-border-default/60 space-y-2">
            {renderCustomSpec(row)}
            {row.sku_id && (
              <div className="pt-2 border-t border-border-default/40 flex items-center justify-between text-xs">
                <span className="text-[10px] font-bold uppercase text-text-secondary">SKU Code</span>
                <span className="font-bold text-text-primary font-mono text-xs truncate max-w-[160px]" title={row.sku_id}>
                  {row.sku_id}
                </span>
              </div>
            )}
          </div>
        ) : (
          <div className="mt-3.5 p-3 rounded-xl bg-surface-raised border border-border-default/60 flex items-center justify-between text-xs">
            <span className="text-[10px] font-bold uppercase text-text-secondary">SKU Code</span>
            <span className="font-bold text-text-primary font-mono text-xs truncate max-w-[160px]" title={row.sku_id}>
              {row.sku_id || 'Base'}
            </span>
          </div>
        )}

        {/* Costing Section */}
        <div className="mt-4">
          <div className="flex items-baseline justify-between gap-2">
            <div className="flex items-baseline gap-1.5 min-w-0">
              <span className="text-2xl font-extrabold text-text-primary tracking-tight font-mono tabular-nums truncate">
                {currSymbol}{formatAmount(displayPrice)}
              </span>
              <span className="text-xs font-medium text-text-secondary whitespace-nowrap shrink-0">{unitLabel}</span>
              {hasAnomaly && <AnomalyFlag className="ml-1 shrink-0" />}
            </div>

            {isLowestTCO ? (
              <span className="text-xs font-bold text-status-matchExact bg-emerald-50 dark:bg-emerald-950/30 px-2 py-0.5 rounded-full border border-emerald-200 dark:border-emerald-800/40 shrink-0">
                Lowest TCO
              </span>
            ) : row.match_delta_pct !== undefined && row.match_delta_pct > 0 ? (
              <span className="text-xs font-bold text-amber-700 dark:text-amber-300 bg-amber-50 dark:bg-amber-950/30 px-2 py-0.5 rounded-full border border-amber-200 dark:border-amber-800/40 shrink-0">
                +{row.match_delta_pct}%
              </span>
            ) : (
              <span className="text-xs font-semibold text-text-secondary bg-surface-raised px-2 py-0.5 rounded-full shrink-0">
                Standard Rate
              </span>
            )}
          </div>

          <div className="text-xs text-text-secondary mt-1 font-medium whitespace-nowrap">
            {timeframe === 'hourly' ? (
              <>
                Approx. <span className="font-bold text-text-primary font-mono">{currSymbol}{formatAmount(monthlyPrice)}</span> / mo
              </>
            ) : (
              <>
                Approx. <span className="font-bold text-text-primary font-mono">{currSymbol}{formatAmount(hourlyPrice)}</span> / hr
              </>
            )}
          </div>
        </div>
      </div>

      {/* Card Actions & Freshness Footer */}
      <div className="mt-5 pt-3.5 border-t border-border-default/60 space-y-2">
        <button
          type="button"
          onClick={() => setIsTierSelected((prev) => !prev)}
          className={cn(
            'w-full py-2 px-3 text-xs font-semibold rounded-xl flex items-center justify-center gap-1.5 transition-all shadow-sm focus-visible:ring-1 focus-visible:ring-brand-500 focus-visible:outline-none',
            isTierSelected
              ? 'bg-emerald-600 text-white'
              : 'bg-brand-500 hover:bg-brand-600 text-white shadow-brand-500/20'
          )}
        >
          <span>{isTierSelected ? `Selected ${formatProviderName(row.provider)} Tier` : `Select ${formatProviderName(row.provider)} Tier`}</span>
          {isTierSelected ? <Check className="w-3.5 h-3.5" /> : <ArrowRight className="w-3.5 h-3.5" />}
        </button>

        <button
          type="button"
          onClick={() => setIsRawJsonOpen(true)}
          className="w-full py-1.5 text-xs text-text-secondary hover:text-text-primary font-medium text-center transition-colors flex items-center justify-center gap-1 cursor-pointer"
        >
          <Code className="w-3.5 h-3.5" />
          <span>Inspect Raw Catalog JSON</span>
        </button>

        {/* Freshness Tag */}
        <div className="text-[11px] text-text-secondary flex items-center justify-center gap-1 pt-1 font-mono">
          {row.stale ? (
            <StaleDataBadge fetchedAt={row.fetched_at} />
          ) : (
            <>
              <Check className="w-3.5 h-3.5 text-status-matchExact" />
              <span>Verified • </span>
              <span>{row.fetched_at ? formatRelativeTime(row.fetched_at) : 'Active'}</span>
            </>
          )}
        </div>
      </div>

      {/* Raw Catalog JSON Dialog Modal */}
      {isRawJsonOpen && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4"
        >
          <div className="bg-surface-card rounded-2xl border border-border-default max-w-xl w-full p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between border-b border-border-default pb-3">
              <h4 className="font-bold text-sm text-text-primary flex items-center gap-2">
                <Code className="w-4 h-4 text-brand-500" />
                <span>Raw SKU Catalog Entry ({formatProviderName(row.provider)})</span>
              </h4>
              <button
                type="button"
                onClick={() => setIsRawJsonOpen(false)}
                className="p-1 text-text-secondary hover:text-text-primary rounded-lg hover:bg-surface-raised"
                aria-label="Close dialog"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <pre className="bg-surface-raised p-4 rounded-xl text-xs font-mono overflow-auto max-h-80 text-text-primary">
              {JSON.stringify(row, null, 2)}
            </pre>
            <div className="flex justify-end">
              <button
                type="button"
                onClick={() => setIsRawJsonOpen(false)}
                className="px-4 py-1.5 text-xs font-semibold rounded-xl bg-surface-raised border border-border-default hover:bg-surface-card text-text-primary transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
