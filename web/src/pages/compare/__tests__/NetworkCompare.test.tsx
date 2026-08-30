// web/src/pages/compare/__tests__/NetworkCompare.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Router } from '../../../router';
import { NetworkCompare } from '../NetworkCompare';
import { pricesApi } from '../../../api/client';
import type { NetworkComparisonResponse } from '../../../types/api';

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

describe('NetworkCompare Page', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/network');
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('queries network prices and renders transfer type and egress volumes', async () => {
    const mockResponse: NetworkComparisonResponse = {
      results: [
        {
          provider: 'gcp',
          sku_id: 'gcp-network-egress',
          matched_spec: {
            transfer_type: 'internet_egress',
            egress_gb: 1000,
          },
          match_quality: 'exact',
          monthly_cost_usd: 85.0,
          price: { amount: 85.0, currency: 'USD', unit: '/mo' },
          stale: false,
        },
      ],
      warnings: [],
    };

    const getNetworkSpy = vi
      .spyOn(pricesApi, 'getNetwork')
      .mockResolvedValue(mockResponse);

    renderWithProviders(<NetworkCompare />);

    expect(screen.getByText('Network & Egress Pricing Comparison')).toBeInTheDocument();
    expect(screen.getByTestId('network-egress-input')).toHaveValue(1000);

    await waitFor(() => {
      expect(screen.getByText('GCP')).toBeInTheDocument();
      expect(screen.getByText(/INTERNET EGRESS/i)).toBeInTheDocument();
      expect(screen.getByText(/1000 GB outbound transfer/i)).toBeInTheDocument();
    });

    expect(getNetworkSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        egress_gb: '1000',
        transfer_type: 'internet_egress',
        region: 'us-east',
        currency: 'USD',
      }),
      expect.anything()
    );
  });

  it('updates transfer type and URL query parameters', async () => {
    vi.spyOn(pricesApi, 'getNetwork').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<NetworkCompare />);

    const transferSelect = screen.getByTestId('network-transfer-type-select');
    fireEvent.change(transferSelect, { target: { value: 'inter_region' } });

    expect(window.location.search).toContain('transfer_type=inter_region');
  });
});
