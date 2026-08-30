// web/src/pages/compare/__tests__/ServerlessCompare.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Router } from '../../../router';
import { ServerlessCompare } from '../ServerlessCompare';
import { pricesApi } from '../../../api/client';
import type { ServerlessComparisonResponse } from '../../../types/api';

function renderWithProviders(ui: React.ReactElement) {
  const testQueryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return render(
    <QueryClientProvider client={testQueryClient}>
      <Router>{ui}</Router>
    </QueryClientProvider>
  );
}

describe('ServerlessCompare Page', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/serverless');
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('queries Serverless function prices and renders architecture and invocation specs', async () => {
    const mockResponse: ServerlessComparisonResponse = {
      results: [
        {
          provider: 'aws',
          sku_id: 'aws-lambda-x86',
          matched_spec: {
            architecture: 'x86_64',
            tier: 'consumption',
          },
          match_quality: 'exact',
          normalized_hourly_usd: 0.002,
          price: { amount: 0.002, currency: 'USD', unit: '/hr' },
          stale: false,
        },
      ],
      warnings: [],
    };

    const getServerlessSpy = vi
      .spyOn(pricesApi, 'getServerless')
      .mockResolvedValue(mockResponse);

    renderWithProviders(<ServerlessCompare />);

    expect(
      screen.getByText('Serverless Compute (FaaS) Pricing Comparison')
    ).toBeInTheDocument();
    expect(screen.getByTestId('serverless-arch-select')).toHaveValue('x86_64');
    expect(screen.getByTestId('serverless-tier-select')).toHaveValue('consumption');
    expect(screen.getByTestId('serverless-requests-input')).toHaveValue(1000000);

    await waitFor(() => {
      expect(screen.getByText('AWS')).toBeInTheDocument();
      expect(screen.getByText('x86_64')).toBeInTheDocument();
      expect(screen.getByText('(consumption)')).toBeInTheDocument();
    });

    expect(getServerlessSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        architecture: 'x86_64',
        tier: 'consumption',
        requests_per_month: '1000000',
        memory_mb: '512',
        execution_duration_ms: '200',
        region: 'us-east',
        currency: 'USD',
      }),
      expect.anything()
    );
  });

  it('updates architecture and memory inputs', async () => {
    vi.spyOn(pricesApi, 'getServerless').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<ServerlessCompare />);

    const archSelect = screen.getByTestId('serverless-arch-select');
    fireEvent.change(archSelect, { target: { value: 'arm64' } });

    expect(window.location.search).toContain('architecture=arm64');
  });
});
