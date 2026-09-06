// web/src/components/calculate/CalculateCategoryBreakdown.tsx
import React from 'react';
import { MatchQualityBadge, UIMatchQuality } from '../honesty/MatchQualityBadge';
import { MissingAttributesIndicator } from '../honesty/MissingAttributesIndicator';
import { StaleDataBadge } from '../honesty/StaleDataBadge';
import { AnomalyFlag } from '../honesty/AnomalyFlag';
import { PriceDisplay } from '../honesty/PriceDisplay';
import type { CalculateCategoryResult, ProviderWarning } from '../../types/api';
import {
  Cpu,
  HardDrive,
  Network,
  Database,
  Server,
  Box,
  Zap,
  LucideIcon,
  AlertCircle,
} from 'lucide-react';

interface CategoryConfig {
  label: string;
  icon: LucideIcon;
}

const CATEGORY_CONFIGS: Record<string, CategoryConfig> = {
  compute: { label: 'Compute', icon: Cpu },
  storage: { label: 'Storage', icon: HardDrive },
  network: { label: 'Network', icon: Network },
  database_rdbms: { label: 'Relational DB (RDBMS)', icon: Database },
  database: { label: 'Relational DB (RDBMS)', icon: Database },
  database_nosql: { label: 'NoSQL DB', icon: Server },
  kubernetes: { label: 'Kubernetes', icon: Box },
  serverless: { label: 'Serverless (FaaS)', icon: Zap },
};

export interface CalculateCategoryBreakdownProps {
  categories?: Record<string, CalculateCategoryResult>;
  requestedCategories: string[];
  provider?: string;
  warnings?: ProviderWarning[];
}

export const CalculateCategoryBreakdown: React.FC<CalculateCategoryBreakdownProps> = ({
  categories = {},
  requestedCategories,
  provider,
  warnings,
}) => {
  const safeCats = categories ?? {};

  const hasProviderAnomaly = warnings?.some(
    (w) =>
      w.code === 'pricing_anomaly_flagged' &&
      (!w.provider || (provider && w.provider.toLowerCase() === provider.toLowerCase()))
  );

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2.5 pt-2">
      {requestedCategories.map((catKey) => {
        const catConfig = CATEGORY_CONFIGS[catKey] || {
          label: catKey.toUpperCase(),
          icon: Box,
        };
        const Icon = catConfig.icon;
        const result = safeCats[catKey] || (catKey === 'database_rdbms' ? safeCats['database'] : undefined);

        if (!result) {
          return (
            <div
              key={catKey}
              className="border border-dashed border-border-default bg-surface-raised/40 p-3 space-y-1.5 rounded-xl"
            >
              <div className="flex items-center space-x-1.5 text-xs font-semibold text-text-secondary">
                <Icon className="w-3.5 h-3.5" />
                <span>{catConfig.label}</span>
              </div>
              <div className="flex items-center space-x-1.5 text-xs text-status-anomaly font-mono">
                <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                <span>Not matched / unfulfilled</span>
              </div>
            </div>
          );
        }

        return (
          <div
            key={catKey}
            className="border border-border-default/80 bg-surface-card p-3 space-y-2 rounded-xl shadow-xs"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-1.5 text-xs font-bold text-text-primary">
                <Icon className="w-3.5 h-3.5 text-border-accent" />
                <span>{catConfig.label}</span>
              </div>
              {result.match_quality && result.match_quality !== 'none' && (
                <MatchQualityBadge quality={result.match_quality as UIMatchQuality} />
              )}
            </div>

            <div className="flex items-baseline justify-between border-t border-b border-border-default/40 py-1.5">
              <span className="text-[11px] text-text-secondary uppercase tracking-wider">Normalized Rate</span>
              <PriceDisplay
                amount={result.normalized_hourly_usd}
                currency="USD"
                unit="/hr"
              />
            </div>

            <div className="space-y-1 text-xs">
              <div className="flex items-center justify-between font-mono text-[11px] text-text-secondary">
                <span className="truncate max-w-[150px]" title={result.sku_id}>
                  {result.sku_id || 'SKU unassigned'}
                </span>
                {result.match_delta_pct !== undefined && result.match_delta_pct !== null && (
                  <span className="font-mono">
                    {result.match_delta_pct > 0 ? `+${result.match_delta_pct.toFixed(1)}%` : `${result.match_delta_pct.toFixed(1)}%`}
                  </span>
                )}
              </div>

              <div className="flex items-center space-x-2 pt-1 flex-wrap gap-y-1">
                {result.missing_attributes && result.missing_attributes.length > 0 && (
                  <MissingAttributesIndicator missingAttributes={result.missing_attributes} />
                )}
                {result.stale && <StaleDataBadge />}
                {hasProviderAnomaly && <AnomalyFlag />}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
};
