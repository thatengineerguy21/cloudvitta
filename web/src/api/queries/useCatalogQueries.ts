// web/src/api/queries/useCatalogQueries.ts
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { catalogApi } from '../client';
import type {
  CatalogSummaryResponse,
  CatalogInstancesResponse,
  ComputeCatalogQueryParams,
} from '../../types/api';

/**
 * Queries the global compute instance catalog summary rollup.
 * 5-minute staleTime with placeholderData caching.
 */
export function useComputeCatalogSummary(): UseQueryResult<CatalogSummaryResponse, Error> {
  return useQuery<CatalogSummaryResponse, Error>({
    queryKey: ['compute-catalog-summary'],
    queryFn: ({ signal }) => catalogApi.getComputeSummary({ signal }),
    staleTime: 300_000,
    placeholderData: (previousData: CatalogSummaryResponse | undefined) => previousData,
  });
}

/**
 * Queries the filtered compute instance catalog with pagination and hardware specifications.
 * 60-second staleTime.
 */
export function useComputeCatalogInstances(
  params?: ComputeCatalogQueryParams,
  enabled: boolean = true
): UseQueryResult<CatalogInstancesResponse, Error> {
  return useQuery<CatalogInstancesResponse, Error>({
    queryKey: ['compute-catalog-instances', params],
    queryFn: ({ signal }) => catalogApi.getComputeInstances(params, { signal }),
    staleTime: 60_000,
    enabled,
    placeholderData: (previousData: CatalogInstancesResponse | undefined) => previousData,
  });
}
