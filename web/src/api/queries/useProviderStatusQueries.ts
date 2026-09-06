// web/src/api/queries/useProviderStatusQueries.ts
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { providerApi } from '../client';
import type { ProviderStatusResponse } from '../../types/api';
import type { Provider } from '../../types';

/** All 7 curated providers in canonical order. */
export const ALL_PROVIDERS: readonly Provider[] = [
  'aws',
  'azure',
  'gcp',
  'oracle',
  'ibm',
  'alibaba',
  'digitalocean',
] as const;

/**
 * Queries the status of a single provider with 60s staleTime.
 * Retains previous data during background revalidation to prevent layout shift.
 */
export function useProviderStatus(
  provider: Provider
): UseQueryResult<ProviderStatusResponse, Error> {
  return useQuery<ProviderStatusResponse, Error>({
    queryKey: ['provider-status', provider],
    queryFn: ({ signal }) => providerApi.getStatus(provider, { signal }),
    staleTime: 60_000,
    placeholderData: (previousData) => previousData,
  });
}

export interface AllProviderStatuses {
  statuses: UseQueryResult<ProviderStatusResponse, Error>[];
  isAnyLoading: boolean;
}

/**
 * Executes 7 independent parallel provider status queries.
 * Each query operates independently — one failure does not block others.
 */
export function useAllProviderStatuses(): AllProviderStatuses {
  const aws = useProviderStatus('aws');
  const azure = useProviderStatus('azure');
  const gcp = useProviderStatus('gcp');
  const oracle = useProviderStatus('oracle');
  const ibm = useProviderStatus('ibm');
  const alibaba = useProviderStatus('alibaba');
  const digitalocean = useProviderStatus('digitalocean');

  const statuses = [aws, azure, gcp, oracle, ibm, alibaba, digitalocean];
  const isAnyLoading = statuses.some((s) => s.isLoading);

  return { statuses, isAnyLoading };
}

export type HealthSummaryState = 'loading' | 'healthy' | 'degraded' | 'error';

/** Maps health summary states to text color tokens. */
export const HEALTH_BADGE_COLORS: Record<HealthSummaryState, string> = {
  healthy: 'text-status-matchExact',
  degraded: 'text-status-stale',
  error: 'text-status-anomaly',
  loading: 'text-text-secondary',
};

export interface ProviderHealthSummary {
  total: number;
  healthyCount: number;
  degradedCount: number;
  errorCount: number;
  notIngestedCount: number;
  isLoading: boolean;
  summaryState: HealthSummaryState;
  summaryLabel: string;
}

/**
 * Aggregates health telemetry across all 7 providers into a single summary.
 * Used by the Header live badge and Landing page health pill.
 */
export function useProviderHealthSummary(): ProviderHealthSummary {
  const { statuses } = useAllProviderStatuses();

  const total = ALL_PROVIDERS.length;

  // Check if any query is loading with no cached data
  const isLoading = statuses.some((s) => s.isPending && !s.data);

  // Check if ALL queries failed
  const allErrored = statuses.every((s) => s.isError);

  let healthyCount = 0;
  let degradedCount = 0;
  let errorCount = 0;
  let notIngestedCount = 0;

  for (const s of statuses) {
    if (s.isError && !s.data) {
      errorCount++;
      continue;
    }
    if (!s.data) continue;

    const providerStatus = s.data.status || 'degraded';

    switch (providerStatus) {
      case 'healthy':
      case 'partially_healthy':
        healthyCount++;
        break;
      case 'degraded':
      case 'stale':
        degradedCount++;
        break;
      case 'blocked':
        errorCount++;
        break;
      case 'not_yet_ingested':
        notIngestedCount++;
        break;
      default:
        degradedCount++;
        break;
    }
  }

  let summaryState: HealthSummaryState;
  let summaryLabel: string;

  if (isLoading) {
    summaryState = 'loading';
    summaryLabel = 'Checking...';
  } else if (allErrored || errorCount === total) {
    summaryState = 'error';
    summaryLabel = 'Telemetry Offline';
  } else if (healthyCount === total) {
    summaryState = 'healthy';
    summaryLabel = `${total}/${total} Providers Active`;
  } else {
    summaryState = 'degraded';
    const activeCount = healthyCount + degradedCount;
    summaryLabel = `${activeCount}/${total} Active`;
  }

  return {
    total,
    healthyCount,
    degradedCount,
    errorCount,
    notIngestedCount,
    isLoading,
    summaryState,
    summaryLabel,
  };
}
