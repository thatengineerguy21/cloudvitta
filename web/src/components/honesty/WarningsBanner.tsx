import React from 'react';
import { cn } from '../../lib/utils';
import { ProviderWarning, WarningCode } from '../../types';
import { AlertCircle, AlertTriangle, Info } from 'lucide-react';

export interface WarningsBannerProps extends React.HTMLAttributes<HTMLDivElement> {
  warnings: ProviderWarning[];
}

export const WarningsBanner: React.FC<WarningsBannerProps> = ({
  warnings,
  className,
  ...props
}) => {
  if (!warnings || warnings.length === 0) {
    return null;
  }

  // Explicit severity tier mapping across all 11 canonical warning codes (08-CONSISTENCY-RULES.md §2, 12-API-CONTRACT.md)
  const getWarningIcon = (code?: string) => {
    switch (code as WarningCode) {
      // 1. Anomaly / Hard Failure Tier (Red / Alert)
      case 'pricing_anomaly_flagged':
      case 'fetch_failed':
        return <AlertTriangle className="w-4 h-4 text-status-anomaly shrink-0" aria-hidden="true" />;

      // 2. Data Missing / Staleness / Unavailability Tier (Amber / Warning)
      case 'stale_pricing_data':
      case 'category_not_supported':
      case 'not_yet_ingested':
      case 'no_match':
      case 'no_data_available':
      case 'non_usd_currency_unsupported':
        return <AlertCircle className="w-4 h-4 text-status-stale shrink-0" aria-hidden="true" />;

      // 3. Informational / Routine Filter Exclusion Tier (Neutral / Informational)
      // Routine explanations for candidate omission based on user filters (engine, arch, topology)
      case 'engine_mismatch_excluded':
      case 'architecture_unsupported_excluded':
      case 'cluster_topology_unspecified':
      default:
        return <Info className="w-4 h-4 text-text-secondary shrink-0" aria-hidden="true" />;
    }
  };

  const getWarningBorder = (code?: string) => {
    switch (code as WarningCode) {
      // 1. Anomaly / Hard Failure Tier
      case 'pricing_anomaly_flagged':
      case 'fetch_failed':
        return 'border-status-anomaly bg-status-anomaly/5 text-status-anomaly';

      // 2. Data Missing / Staleness / Unavailability Tier
      case 'stale_pricing_data':
      case 'category_not_supported':
      case 'not_yet_ingested':
      case 'no_match':
      case 'no_data_available':
      case 'non_usd_currency_unsupported':
        return 'border-status-stale bg-status-stale/5 text-status-stale';

      // 3. Informational / Routine Filter Exclusion Tier
      case 'engine_mismatch_excluded':
      case 'architecture_unsupported_excluded':
      case 'cluster_topology_unspecified':
      default:
        return 'border-border-default bg-surface-raised text-text-primary';
    }
  };

  return (
    <div className={cn('flex flex-col space-y-2 w-full', className)} {...props}>
      {warnings.map((w, idx) => (
        <div
          key={`${w.provider || 'all'}-${w.code || idx}-${idx}`}
          className={cn(
            'flex items-start space-x-3 p-3 border text-xs',
            getWarningBorder(w.code)
          )}
        >
          {getWarningIcon(w.code)}
          <div className="flex-1">
            <div className="flex items-center space-x-2">
              {w.provider && (
                <span className="font-bold uppercase tracking-wider text-text-primary">
                  [{w.provider}]
                </span>
              )}
              {w.code && (
                <span className="font-mono text-[11px] text-text-secondary">
                  {w.code}
                </span>
              )}
            </div>
            <p className="text-text-primary mt-0.5">{w.message}</p>
          </div>
        </div>
      ))}
    </div>
  );
};
