// web/src/components/status/ProviderStatusCard.tsx
import React from 'react';
import { cn } from '../../lib/utils';
import { useProviderStatus } from '../../api/queries/useProviderStatusQueries';
import { ProviderStatusSkeleton } from './ProviderStatusSkeleton';
import { WarningsBanner } from '../honesty/WarningsBanner';
import { formatRelativeTime } from '../../lib/format';
import { RefreshCw, AlertTriangle } from 'lucide-react';
import type { Provider } from '../../types';
import type { CategoryStatusResponse, DLQStatusResponse } from '../../types/api';

interface ProviderStatusCardProps {
  provider: Provider;
}

/** Canonical display names for all 7 curated providers. */
const PROVIDER_DISPLAY_NAMES: Record<Provider, string> = {
  aws: 'Amazon Web Services',
  azure: 'Microsoft Azure',
  gcp: 'Google Cloud Platform',
  oracle: 'Oracle Cloud Infrastructure',
  ibm: 'IBM Cloud',
  alibaba: 'Alibaba Cloud',
  digitalocean: 'DigitalOcean',
};

/** Maps provider aggregate status to badge styling tokens. */
const STATUS_BADGE_STYLES: Record<string, string> = {
  healthy: 'bg-status-matchExact/10 text-status-matchExact border-status-matchExact',
  partially_healthy: 'bg-status-matchClose/10 text-status-matchClose border-status-matchClose',
  degraded: 'bg-status-stale/10 text-status-stale border-status-stale',
  stale: 'bg-status-stale/10 text-status-stale border-status-stale',
  blocked: 'bg-status-anomaly/10 text-status-anomaly border-status-anomaly',
  not_yet_ingested: 'bg-surface-raised text-text-secondary border-border-default',
};

/**
 * Renders the status of a single cloud provider.
 * Queries its status independently so that a failure in one provider
 * does not block other provider cards from rendering.
 */
export const ProviderStatusCard: React.FC<ProviderStatusCardProps> = ({ provider }) => {
  const { data, error, isLoading, refetch } = useProviderStatus(provider);

  // Loading state: show skeleton
  if (isLoading && !data) {
    return <ProviderStatusSkeleton />;
  }

  // Error state with no cached data: show error card with retry
  if (error && !data) {
    return (
      <div
        className="border border-status-anomaly bg-surface-card p-6"
        data-testid={`provider-error-${provider}`}
      >
        <div className="flex items-center space-x-2 mb-3">
          <AlertTriangle className="w-4 h-4 text-status-anomaly" />
          <span className="text-xs uppercase font-bold tracking-wider text-text-primary">
            {provider.toUpperCase()}
          </span>
        </div>
        <p className="text-xs text-text-secondary mb-3">
          {error.message || 'Failed to fetch provider status.'}
        </p>
        <button
          onClick={() => refetch()}
          className="inline-flex items-center space-x-1.5 border border-border-default bg-surface-card px-3 py-1.5 text-xs uppercase font-bold tracking-wider text-text-primary hover:border-border-accent transition-colors"
        >
          <RefreshCw className="w-3 h-3" />
          <span>Retry</span>
        </button>
      </div>
    );
  }

  // No data at all (shouldn't happen but guard)
  if (!data) return null;

  const status = data.status || 'degraded';
  const badgeStyle = STATUS_BADGE_STYLES[status] || STATUS_BADGE_STYLES.degraded;
  const categories = Object.entries(data.categories || {}) as [string, CategoryStatusResponse][];

  return (
    <div
      className="bg-surface-card border border-border-default p-6"
      data-testid={`provider-card-${provider}`}
    >
      {/* Header: Provider name + status badge */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <span className="text-xs uppercase font-bold tracking-wider text-text-primary">
            {provider.toUpperCase()}
          </span>
          <span className="block text-[11px] text-text-secondary mt-0.5">
            {PROVIDER_DISPLAY_NAMES[provider]}
          </span>
        </div>
        <span
          className={cn(
            'px-2 py-0.5 text-[11px] uppercase font-bold tracking-wider border',
            badgeStyle
          )}
          data-testid={`status-badge-${provider}`}
        >
          {status.replace(/_/g, ' ')}
        </span>
      </div>

      {/* Last fetch timestamp */}
      <div className="text-[11px] text-text-secondary mb-4">
        <span className="font-bold uppercase tracking-wider">Last Fetch: </span>
        {data.last_successful_fetch
          ? formatRelativeTime(data.last_successful_fetch as string)
          : 'Never'}
      </div>

      {/* Categories table */}
      {categories.length > 0 && (
        <div className="border-t border-border-default pt-3 space-y-2">
          {categories.map(([name, cat]) => (
            <CategoryRow key={name} name={name} category={cat} />
          ))}
        </div>
      )}

      {/* Warnings banner */}
      {data.warnings && data.warnings.length > 0 && (
        <div className="mt-4">
          <WarningsBanner warnings={data.warnings} />
        </div>
      )}
    </div>
  );
};

/** Renders a single category row within a provider status card. */
const CategoryRow: React.FC<{ name: string; category: CategoryStatusResponse }> = ({
  name,
  category,
}) => {
  const categoryLabel = name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

  return (
    <div>
      <div className="flex items-center justify-between">
        <span className="text-xs font-bold text-text-primary">{categoryLabel}</span>
        <div className="flex items-center space-x-3">
          {category.supported ? (
            <>
              <span className="text-[11px] text-text-secondary font-mono">
                {category.observation_count} obs
              </span>
              <span
                className={cn(
                  'text-[11px] font-bold uppercase tracking-wider',
                  category.stale ? 'text-status-stale' : 'text-status-matchExact'
                )}
              >
                {category.stale ? 'Stale' : 'Fresh'}
              </span>
            </>
          ) : (
            <span className="text-[11px] text-text-secondary italic">Not Supported</span>
          )}
        </div>
      </div>

      {/* DLQ failure details */}
      {category.dlq && <DLQDetail dlq={category.dlq} />}
    </div>
  );
};

/** Renders DLQ failure details in a compact red box. */
const DLQDetail: React.FC<{ dlq: DLQStatusResponse }> = ({ dlq }) => (
  <div className="mt-1 border border-status-anomaly bg-status-anomaly/5 p-2 text-[11px]">
    <div className="flex items-center space-x-2">
      <span className="font-bold uppercase text-status-anomaly">DLQ {dlq.status}</span>
      <span className="text-text-secondary">
        {dlq.consecutive_failures} consecutive failure{dlq.consecutive_failures === 1 ? '' : 's'}
      </span>
    </div>
    <p className="text-text-secondary font-mono mt-0.5 truncate">{dlq.last_error}</p>
  </div>
);
