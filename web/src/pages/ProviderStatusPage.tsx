// web/src/pages/ProviderStatusPage.tsx
import React from 'react';
import { BentoCard } from '../components/bento/BentoCard';
import { BentoGrid } from '../components/bento/BentoGrid';
import { ProviderStatusCard } from '../components/status/ProviderStatusCard';
import { CardErrorBoundary } from '../components/common/CardErrorBoundary';
import {
  ALL_PROVIDERS,
  useProviderHealthSummary,
  type HealthSummaryState,
} from '../api/queries/useProviderStatusQueries';
import { cn } from '../lib/utils';
import { Activity } from 'lucide-react';

const SUMMARY_STATE_COLORS: Record<HealthSummaryState, string> = {
  healthy: 'text-status-matchExact border-status-matchExact',
  degraded: 'text-status-stale border-status-stale',
  error: 'text-status-anomaly border-status-anomaly',
  loading: 'text-text-secondary border-border-default',
};

/**
 * Provider Status Page (`/status`).
 * Renders 7 independent provider status cards in a 12-column Bento Grid.
 * Each card executes its own query and is wrapped in a CardErrorBoundary
 * so that a single provider failure never blanks the page.
 */
export const ProviderStatusPage: React.FC = () => {
  const { summaryState, summaryLabel, isLoading } = useProviderHealthSummary();
  const stateColor = SUMMARY_STATE_COLORS[summaryState];

  return (
    <div className="space-y-6" data-testid="provider-status-page">
      {/* Page Header */}
      <BentoCard colSpan={12}>
        <span className="text-xs uppercase tracking-widest text-border-accent font-bold">
          Provider Health Telemetry
        </span>
        <h1 className="font-display text-3xl font-medium text-text-primary mt-2">
          Provider Operational Status &amp; Data Freshness
        </h1>
        <p className="text-sm text-text-secondary mt-2 max-w-2xl">
          Live health telemetry across all 7 supported cloud providers. Each card queries
          independently — a single provider failure does not block others.
        </p>

        {/* Summary telemetry pill */}
        <div className="mt-4">
          <span
            className={cn(
              'inline-flex items-center space-x-2 px-3 py-1 border text-xs font-bold uppercase tracking-wider',
              stateColor
            )}
          >
            <Activity className={cn('w-3.5 h-3.5', isLoading && 'animate-pulse')} />
            <span>{summaryLabel}</span>
          </span>
        </div>
      </BentoCard>

      {/* Provider Status Cards Grid */}
      <BentoGrid columns={12} gap="md">
        {ALL_PROVIDERS.map((provider) => (
          <div key={provider} className="col-span-1 md:col-span-6">
            <CardErrorBoundary>
              <ProviderStatusCard provider={provider} />
            </CardErrorBoundary>
          </div>
        ))}
      </BentoGrid>
    </div>
  );
};
