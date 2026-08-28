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

    it('includes not_yet_ingested in active count for healthy summary', async () => {
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

      // All reachable, no stale/degraded/blocked, so should be healthy
      expect(result.current.summaryState).toBe('healthy');
      expect(result.current.notIngestedCount).toBe(2);
      expect(result.current.healthyCount).toBe(5);
    });
  });
});
