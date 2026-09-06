// web/src/api/queries/__tests__/useProviderStatusQueries.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { providerApi } from '../../client';
import {
  useProviderStatus,
  useAllProviderStatuses,
  useProviderHealthSummary,
} from '../useProviderStatusQueries';
import type { ProviderStatusResponse } from '../../../types/api';

const createWrapper = () => {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
};

const mockStatus = (
  provider: string,
  status: string
): ProviderStatusResponse => ({
  provider,
  status,
  stale: status === 'stale',
  categories: {},
  warnings:
    status === 'not_yet_ingested'
      ? [{ provider, code: 'not_yet_ingested', message: `${provider} ingestion pending` }]
      : [],
});

describe('useProviderStatusQueries', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('useProviderStatus', () => {
    it('queries single provider and returns data', async () => {
      vi.spyOn(providerApi, 'getStatus').mockResolvedValue(mockStatus('aws', 'healthy'));

      const { result } = renderHook(() => useProviderStatus('aws'), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data?.status).toBe('healthy');
      expect(result.current.data?.provider).toBe('aws');
      expect(providerApi.getStatus).toHaveBeenCalledWith('aws', expect.objectContaining({}));
    });
  });

  describe('useAllProviderStatuses', () => {
    it('runs all 7 providers in parallel', async () => {
      vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) =>
        Promise.resolve(mockStatus(provider, 'healthy'))
      );

      const { result } = renderHook(() => useAllProviderStatuses(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isAnyLoading).toBe(false);
      });

      expect(result.current.statuses).toHaveLength(7);
      expect(result.current.statuses.every((s) => s.isSuccess)).toBe(true);
      expect(providerApi.getStatus).toHaveBeenCalledTimes(7);
    });
  });

  describe('useProviderHealthSummary', () => {
    it('calculates all healthy state', async () => {
      vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) =>
        Promise.resolve(mockStatus(provider, 'healthy'))
      );

      const { result } = renderHook(() => useProviderHealthSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.summaryState).toBe('healthy');
      expect(result.current.summaryLabel).toBe('7/7 Providers Active');
      expect(result.current.healthyCount).toBe(7);
      expect(result.current.total).toBe(7);
    });

    it('calculates degraded with stale providers', async () => {
      vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) => {
        if (provider === 'aws' || provider === 'azure') {
          return Promise.resolve(mockStatus(provider, 'stale'));
        }
        return Promise.resolve(mockStatus(provider, 'healthy'));
      });

      const { result } = renderHook(() => useProviderHealthSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.summaryState).toBe('degraded');
      expect(result.current.degradedCount).toBe(2);
      expect(result.current.healthyCount).toBe(5);
      expect(result.current.summaryLabel).toBe('7/7 Active');
    });

    it('counts partially_healthy providers as healthy', async () => {
      vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) => {
        if (provider === 'aws' || provider === 'azure' || provider === 'gcp') {
          return Promise.resolve(mockStatus(provider, 'partially_healthy'));
        }
        return Promise.resolve(mockStatus(provider, 'not_yet_ingested'));
      });

      const { result } = renderHook(() => useProviderHealthSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.summaryState).toBe('degraded');
      expect(result.current.healthyCount).toBe(3);
      expect(result.current.notIngestedCount).toBe(4);
      expect(result.current.summaryLabel).toBe('3/7 Active');
    });

    it('calculates error when all queries fail', async () => {
      vi.spyOn(providerApi, 'getStatus').mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useProviderHealthSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.summaryState).toBe('error');
      expect(result.current.summaryLabel).toBe('Telemetry Offline');
      expect(result.current.errorCount).toBe(7);
    });

    it('reports not_yet_ingested providers as degraded and reports activeCount accurately', async () => {
      vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) => {
        if (provider === 'oracle' || provider === 'ibm') {
          return Promise.resolve(mockStatus(provider, 'not_yet_ingested'));
        }
        return Promise.resolve(mockStatus(provider, 'healthy'));
      });

      const { result } = renderHook(() => useProviderHealthSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.summaryState).toBe('degraded');
      expect(result.current.notIngestedCount).toBe(2);
      expect(result.current.healthyCount).toBe(5);
      expect(result.current.summaryLabel).toBe('5/7 Active');
    });

    it('escalates to degraded when a provider is blocked due to DLQ failure coexisting with healthy providers', async () => {
      vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) => {
        if (provider === 'digitalocean') {
          return Promise.resolve(mockStatus(provider, 'blocked'));
        }
        return Promise.resolve(mockStatus(provider, 'healthy'));
      });

      const { result } = renderHook(() => useProviderHealthSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.summaryState).toBe('degraded');
      expect(result.current.errorCount).toBe(1);
      expect(result.current.healthyCount).toBe(6);
      expect(result.current.summaryLabel).toBe('6/7 Active');
    });

    it('escalates to degraded on partial query network failures', async () => {
      vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) => {
        if (provider === 'azure' || provider === 'gcp') {
          return Promise.reject(new Error('Connection timeout'));
        }
        return Promise.resolve(mockStatus(provider, 'healthy'));
      });

      const { result } = renderHook(() => useProviderHealthSummary(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.summaryState).toBe('degraded');
      expect(result.current.errorCount).toBe(2);
      expect(result.current.healthyCount).toBe(5);
      expect(result.current.summaryLabel).toBe('5/7 Active');
    });
  });
});
