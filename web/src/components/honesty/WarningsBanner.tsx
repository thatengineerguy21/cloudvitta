import React from 'react';
import { cn } from '../../lib/utils';
import { ProviderWarning, WarningCode } from '../../types';
import { AlertCircle, AlertTriangle, Info, LucideIcon } from 'lucide-react';
import { FRIENDLY_WARNING_TITLES } from './friendlyWarnings';

export interface WarningsBannerProps extends React.HTMLAttributes<HTMLDivElement> {
  warnings: ProviderWarning[];
}

type SeverityTier = 'anomaly' | 'warning' | 'info';

interface WarningStyleConfig {
  icon: LucideIcon;
  iconClass: string;
  containerClass: string;
}

const SEVERITY_CONFIGS: Record<SeverityTier, WarningStyleConfig> = {
  // 1. Anomaly / Hard Failure Tier (Red / Alert)
  anomaly: {
    icon: AlertTriangle,
    iconClass: 'text-status-anomaly',
    containerClass: 'border-status-anomaly bg-status-anomaly/5 text-status-anomaly',
  },
  // 2. Data Missing / Staleness / Unavailability Tier (Amber / Warning)
  warning: {
    icon: AlertCircle,
    iconClass: 'text-status-stale',
    containerClass: 'border-status-stale bg-status-stale/5 text-status-stale',
  },
  // 3. Informational / Routine Filter Exclusion Tier (Neutral / Informational)
  info: {
    icon: Info,
    iconClass: 'text-text-secondary',
    containerClass: 'border-border-default bg-surface-raised text-text-primary',
  },
};

const WARNING_TIER_MAP: Partial<Record<WarningCode, SeverityTier>> = {
  pricing_anomaly_flagged: 'anomaly',
  fetch_failed: 'anomaly',
  stale_pricing_data: 'warning',
  category_not_supported: 'warning',
  not_yet_ingested: 'warning',
  no_match: 'warning',
  no_data_available: 'warning',
  non_usd_currency_unsupported: 'warning',
  engine_mismatch_excluded: 'info',
  architecture_unsupported_excluded: 'info',
  cluster_topology_unspecified: 'info',
};

const getWarningConfig = (code?: string): WarningStyleConfig => {
  const tier = (code && WARNING_TIER_MAP[code as WarningCode]) || 'info';
  return SEVERITY_CONFIGS[tier];
};

export const WarningsBanner: React.FC<WarningsBannerProps> = ({
  warnings,
  className,
  ...props
}) => {
  if (!warnings || warnings.length === 0) {
    return null;
  }

  return (
    <div className={cn('flex flex-col space-y-2 w-full', className)} {...props}>
      {warnings.map((w, idx) => {
        const config = getWarningConfig(w.code);
        const Icon = config.icon;
        const friendlyTitle = w.code
          ? FRIENDLY_WARNING_TITLES[w.code as WarningCode] || w.code
          : undefined;

        return (
          <div
            key={`${w.provider || 'all'}-${w.code || idx}-${idx}`}
            className={cn('flex items-start space-x-3 p-3 border rounded-xl text-xs', config.containerClass)}
          >
            <Icon className={cn('w-4 h-4 shrink-0', config.iconClass)} aria-hidden="true" />
            <div className="flex-1">
              <div className="flex items-center space-x-2">
                {w.provider && (
                  <span className="font-bold uppercase tracking-wider text-text-primary">
                    [{w.provider}]
                  </span>
                )}
                {friendlyTitle && (
                  <span
                    className="font-semibold text-xs text-text-primary"
                    title={w.code}
                    data-code={w.code}
                  >
                    {friendlyTitle}
                  </span>
                )}
              </div>
              <p className="text-text-primary mt-0.5">{w.message}</p>
            </div>
          </div>
        );
      })}
    </div>
  );
};
