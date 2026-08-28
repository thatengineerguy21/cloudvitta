// web/src/pages/__tests__/CalculatePage.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { CalculatePage } from '../CalculatePage';
import { calculateApi } from '../../api/client';
import { Router } from '../../router';
import type { CalculateResponse } from '../../types/api';

function createWrapper(initialPath = '/calculate') {
  window.history.replaceState(null, '', initialPath);
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <Router>{children}</Router>
    </QueryClientProvider>
  );
}

describe('CalculatePage integration', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders page header, builder form, and fetches calculation results', async () => {
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
        {
          provider: 'gcp',
          partial: true,
          partial_total_normalized_hourly_usd: 0.038,
          categories: {
            compute: {
              sku_id: 'gcp-compute-e2-medium',
              match_quality: 'exact',
              normalized_hourly_usd: 0.038,
            },
          },
        },
      ],
      warnings: [
        {
          provider: 'digitalocean',
          code: 'category_not_supported',
          message: 'DigitalOcean does not support relational database workloads.',
        },
      ],
    };

    const calculateSpy = vi.spyOn(calculateApi, 'calculate').mockResolvedValueOnce(mockResponse);

    render(<CalculatePage />, {
      wrapper: createWrapper('/calculate?c_on=1&c_vcpu=2&c_ram=4'),
    });

    expect(screen.getByRole('heading', { level: 1, name: 'Composite Workload Calculator' })).toBeInTheDocument();
    expect(screen.getByText('Compute Instances')).toBeInTheDocument();

    await waitFor(() => {
      expect(calculateSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          compute: expect.objectContaining({ vcpu: 2, ram_gb: 4 }),
        }),
        expect.anything()
      );
    });

    await waitFor(() => {
      expect(screen.getByTestId('complete-total-aws')).toBeInTheDocument();
      expect(screen.getByTestId('partial-total-gcp')).toBeInTheDocument();
      expect(screen.getByText(/DigitalOcean does not support relational database/i)).toBeInTheDocument();
    });
  });
});
