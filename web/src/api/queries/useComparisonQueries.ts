// web/src/api/queries/useComparisonQueries.ts
import { useQuery } from '@tanstack/react-query';
import { pricesApi } from '../client';
import type {
  ComputeQueryParams,
  StorageQueryParams,
  NetworkQueryParams,
  DatabaseQueryParams,
  DatabaseNoSQLQueryParams,
  KubernetesQueryParams,
  ServerlessQueryParams,
  ComputeComparisonResponse,
  StorageComparisonResponse,
  NetworkComparisonResponse,
  DatabaseComparisonResponse,
  DatabaseNoSQLComparisonResponse,
  KubernetesComparisonResponse,
  ServerlessComparisonResponse,
  QueryParamValue,
} from '../../types/api';

export function cleanParams<T extends Record<string, QueryParamValue>>(params: T): Record<string, string> {
  const result: Record<string, string> = {};
  Object.entries(params).forEach(([key, val]) => {
    if (val !== undefined && val !== null && val !== '') {
      result[key] = String(val);
    }
  });
  return result;
}

export function useComputeComparison(params: ComputeQueryParams) {
  const cleaned = cleanParams(params);
  return useQuery<ComputeComparisonResponse, Error>({
    queryKey: ['prices', 'compute', cleaned],
    queryFn: ({ signal }) => pricesApi.getCompute(cleaned, { signal }),
    placeholderData: (previousData) => previousData,
  });
}

export function useStorageComparison(params: StorageQueryParams) {
  const cleaned = cleanParams(params);
  return useQuery<StorageComparisonResponse, Error>({
    queryKey: ['prices', 'storage', cleaned],
    queryFn: ({ signal }) => pricesApi.getStorage(cleaned, { signal }),
    placeholderData: (previousData) => previousData,
  });
}

export function useNetworkComparison(params: NetworkQueryParams) {
  const cleaned = cleanParams(params);
  return useQuery<NetworkComparisonResponse, Error>({
    queryKey: ['prices', 'network', cleaned],
    queryFn: ({ signal }) => pricesApi.getNetwork(cleaned, { signal }),
    placeholderData: (previousData) => previousData,
  });
}

export function useDatabaseComparison(params: DatabaseQueryParams) {
  const cleaned = cleanParams(params);
  return useQuery<DatabaseComparisonResponse, Error>({
    queryKey: ['prices', 'database', cleaned],
    queryFn: ({ signal }) => pricesApi.getDatabase(cleaned, { signal }),
    placeholderData: (previousData) => previousData,
  });
}

export function useDatabaseNoSQLComparison(params: DatabaseNoSQLQueryParams) {
  const cleaned = cleanParams(params);
  return useQuery<DatabaseNoSQLComparisonResponse, Error>({
    queryKey: ['prices', 'database-nosql', cleaned],
    queryFn: ({ signal }) => pricesApi.getDatabaseNoSQL(cleaned, { signal }),
    placeholderData: (previousData) => previousData,
  });
}

export function useKubernetesComparison(params: KubernetesQueryParams) {
  const cleaned = cleanParams(params);
  return useQuery<KubernetesComparisonResponse, Error>({
    queryKey: ['prices', 'kubernetes', cleaned],
    queryFn: ({ signal }) => pricesApi.getKubernetes(cleaned, { signal }),
    placeholderData: (previousData) => previousData,
  });
}

export function useServerlessComparison(params: ServerlessQueryParams) {
  const cleaned = cleanParams(params);
  return useQuery<ServerlessComparisonResponse, Error>({
    queryKey: ['prices', 'serverless', cleaned],
    queryFn: ({ signal }) => pricesApi.getServerless(cleaned, { signal }),
    placeholderData: (previousData) => previousData,
  });
}
