// web/src/api/queries/__tests__/useCalculateQuery.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { useCalculateWorkload, hasActiveCategories } from '../useCalculateQuery';
import { calculateApi } from '../../client';
import type { CalculateRequestBody, CalculateResponse } from '../../../types/api';

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

describe('useCalculateQuery & hasActiveCategories', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('hasActiveCategories detects presence of any category', () => {
    expect(hasActiveCategories(null)).toBe(false);
    expect(hasActiveCategories({})).toBe(false);
    expect(hasActiveCategories({ region: 'us-east', currency: 'USD' })).toBe(false);

    expect(hasActiveCategories({ compute: { vcpu: 2, ram_gb: 4 } })).toBe(true);
    expect(hasActiveCategories({ storage: { size_gb: 100 } })).toBe(true);
    expect(hasActiveCategories({ network: { egress_gb: 50 } })).toBe(true);
    expect(hasActiveCategories({ database_rdbms: { engine: 'postgresql', vcpu: 2 } })).toBe(true);
    expect(hasActiveCategories({ database: { engine: 'mysql', vcpu: 2 } })).toBe(true);
    expect(hasActiveCategories({ database_nosql: { data_model: 'document' } })).toBe(true);
    expect(hasActiveCategories({ kubernetes: { tier: 'standard' } })).toBe(true);
    expect(hasActiveCategories({ serverless: { requests_per_month: 1000 } })).toBe(true);
  });

  it('useCalculateWorkload fetches composite calculation when categories are present', async () => {
    const mockResponse: CalculateResponse = {
      meta: {
        api_version: 'v1',
        generated_at: '2026-08-28T00:00:00Z',
      },
      results: [
        {
          provider: 'aws',
          partial: false,
          total_normalized_hourly_usd: 0.192,
          categories: {
            compute: {
              sku_id: 'aws-ec2-t4g-medium',
              match_quality: 'exact',
              normalized_hourly_usd: 0.0336,
            },
          },
        },
      ],
      warnings: [],
    };

    const calculateSpy = vi.spyOn(calculateApi, 'calculate').mockResolvedValueOnce(mockResponse);

    const body: CalculateRequestBody = {
      region: 'us-east',
      currency: 'USD',
      compute: { vcpu: 2, ram_gb: 4 },
    };

    const { result } = renderHook(() => useCalculateWorkload(body), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(calculateSpy).toHaveBeenCalledWith(
      body,
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    );
    expect(result.current.data).toEqual(mockResponse);
  });

  it('useCalculateWorkload is disabled when no category is specified', () => {
    const calculateSpy = vi.spyOn(calculateApi, 'calculate');
    const body: CalculateRequestBody = {
      region: 'us-east',
      currency: 'USD',
    };

    const { result } = renderHook(() => useCalculateWorkload(body), {
      wrapper: createWrapper(),
    });

    expect(result.current.fetchStatus).toBe('idle');
    expect(calculateSpy).not.toHaveBeenCalled();
  });

  it('useCalculateWorkload respects explicit enabled = false prop', () => {
    const calculateSpy = vi.spyOn(calculateApi, 'calculate');
    const body: CalculateRequestBody = {
      region: 'us-east',
      currency: 'USD',
      compute: { vcpu: 2, ram_gb: 4 },
    };

    const { result } = renderHook(() => useCalculateWorkload(body, false), {
      wrapper: createWrapper(),
    });

    expect(result.current.fetchStatus).toBe('idle');
    expect(calculateSpy).not.toHaveBeenCalled();
  });
});
