// web/src/api/queries/__tests__/useComparisonQueries.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import {
  cleanParams,
  useComputeComparison,
  useStorageComparison,
  useNetworkComparison,
  useDatabaseComparison,
  useDatabaseNoSQLComparison,
  useKubernetesComparison,
  useServerlessComparison,
} from '../useComparisonQueries';
import { pricesApi } from '../../client';
import type {
  ComputeComparisonResponse,
  StorageComparisonResponse,
  NetworkComparisonResponse,
  DatabaseComparisonResponse,
  DatabaseNoSQLComparisonResponse,
  KubernetesComparisonResponse,
  ServerlessComparisonResponse,
} from '../../../types/api';

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe('useComparisonQueries hooks & cleanParams', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('cleanParams removes undefined, null, and empty string properties', () => {
    const raw = {
      vcpu: 4,
      family: '',
      region: 'us-east',
      emptyVal: null,
      undefVal: undefined,
      strict: true,
    };

    const cleaned = cleanParams(raw);

    expect(cleaned).toEqual({
      vcpu: '4',
      region: 'us-east',
      strict: 'true',
    });
  });

  it('useComputeComparison invokes pricesApi.getCompute and returns data', async () => {
    const mockData: ComputeComparisonResponse = { results: [], warnings: [] };
    const spy = vi.spyOn(pricesApi, 'getCompute').mockResolvedValue(mockData);

    const { result } = renderHook(
      () => useComputeComparison({ vcpu: 4, ram_gb: 16, region: 'us-east' }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockData);
    expect(spy).toHaveBeenCalledWith(
      { vcpu: '4', ram_gb: '16', region: 'us-east' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it('useStorageComparison invokes pricesApi.getStorage and returns data', async () => {
    const mockData: StorageComparisonResponse = { results: [], warnings: [] };
    const spy = vi.spyOn(pricesApi, 'getStorage').mockResolvedValue(mockData);

    const { result } = renderHook(
      () => useStorageComparison({ size_gb: 500, storage_class: 'standard' }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockData);
    expect(spy).toHaveBeenCalledWith(
      { size_gb: '500', storage_class: 'standard' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it('useNetworkComparison invokes pricesApi.getNetwork and returns data', async () => {
    const mockData: NetworkComparisonResponse = { results: [], warnings: [] };
    const spy = vi.spyOn(pricesApi, 'getNetwork').mockResolvedValue(mockData);

    const { result } = renderHook(
      () => useNetworkComparison({ egress_gb: 1000, transfer_type: 'internet_egress' }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockData);
    expect(spy).toHaveBeenCalledWith(
      { egress_gb: '1000', transfer_type: 'internet_egress' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it('useDatabaseComparison invokes pricesApi.getDatabase and returns data', async () => {
    const mockData: DatabaseComparisonResponse = { results: [], warnings: [] };
    const spy = vi.spyOn(pricesApi, 'getDatabase').mockResolvedValue(mockData);

    const { result } = renderHook(
      () => useDatabaseComparison({ engine: 'postgresql', vcpu: 4, ram_gb: 16 }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockData);
    expect(spy).toHaveBeenCalledWith(
      { engine: 'postgresql', vcpu: '4', ram_gb: '16' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it('useDatabaseNoSQLComparison invokes pricesApi.getDatabaseNoSQL and returns data', async () => {
    const mockData: DatabaseNoSQLComparisonResponse = { results: [], warnings: [] };
    const spy = vi.spyOn(pricesApi, 'getDatabaseNoSQL').mockResolvedValue(mockData);

    const { result } = renderHook(
      () => useDatabaseNoSQLComparison({ data_model: 'document', read_units: 100 }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockData);
    expect(spy).toHaveBeenCalledWith(
      { data_model: 'document', read_units: '100' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it('useKubernetesComparison invokes pricesApi.getKubernetes and returns data', async () => {
    const mockData: KubernetesComparisonResponse = { results: [], warnings: [] };
    const spy = vi.spyOn(pricesApi, 'getKubernetes').mockResolvedValue(mockData);

    const { result } = renderHook(
      () => useKubernetesComparison({ tier: 'standard', cluster_topology: 'zonal' }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockData);
    expect(spy).toHaveBeenCalledWith(
      { tier: 'standard', cluster_topology: 'zonal' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });

  it('useServerlessComparison invokes pricesApi.getServerless and returns data', async () => {
    const mockData: ServerlessComparisonResponse = { results: [], warnings: [] };
    const spy = vi.spyOn(pricesApi, 'getServerless').mockResolvedValue(mockData);

    const { result } = renderHook(
      () => useServerlessComparison({ architecture: 'x86_64', requests_per_month: 1000000 }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(mockData);
    expect(spy).toHaveBeenCalledWith(
      { architecture: 'x86_64', requests_per_month: '1000000' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
  });
});
