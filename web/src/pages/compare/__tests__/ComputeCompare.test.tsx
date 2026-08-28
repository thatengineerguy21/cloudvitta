// web/src/pages/compare/__tests__/ComputeCompare.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Router } from '../../../router';
import { ComputeCompare } from '../ComputeCompare';
import { pricesApi } from '../../../api/client';
import type { ComputeComparisonResponse } from '../../../types/api';

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

describe('ComputeCompare Page', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/compute');
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('queries compute prices with default parameters and renders results', async () => {
    const mockResponse: ComputeComparisonResponse = {
      results: [
        {
          provider: 'aws',
          sku_id: 'aws-c5-xlarge',
          matched_spec: {
            vcpu: 4,
            ram_gb: 16,
            family: 'compute_optimized',
          },
          match_quality: 'exact',
          normalized_hourly_usd: 0.17,
          price: { amount: 0.17, currency: 'USD', unit: '/hr' },
          stale: false,
        },
      ],
      warnings: [],
    };

    const getComputeSpy = vi
      .spyOn(pricesApi, 'getCompute')
      .mockResolvedValue(mockResponse);

    renderWithProviders(<ComputeCompare />);

    expect(screen.getByText('Compute Pricing Comparison')).toBeInTheDocument();
    expect(screen.getByTestId('compute-vcpu-input')).toHaveValue(4);
    expect(screen.getByTestId('compute-ram-input')).toHaveValue(16);

    await waitFor(() => {
      expect(screen.getByText('AWS')).toBeInTheDocument();
      expect(screen.getByText(/4 vCPU • 16 GB RAM • compute_optimized/i)).toBeInTheDocument();
      expect(screen.getByText('Exact Match')).toBeInTheDocument();
    });

    expect(getComputeSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        vcpu: '4',
        ram_gb: '16',
        region: 'us-east',
        currency: 'USD',
        strict_family: 'true',
      }),
      expect.anything()
    );
  });

  it('updates parameters and URL query parameters when inputs change', async () => {
    vi.spyOn(pricesApi, 'getCompute').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<ComputeCompare />);

    const strictCheckbox = screen.getByTestId('compute-strict-family-checkbox');
    expect(strictCheckbox).toBeChecked();

    fireEvent.click(strictCheckbox);
    expect(strictCheckbox).not.toBeChecked();

    const familySelect = screen.getByTestId('compute-family-select');
    fireEvent.change(familySelect, { target: { value: 'compute_optimized' } });

    expect(window.location.search).toContain('strict_family=false');
    expect(window.location.search).toContain('family=compute_optimized');
  });

  it('resets query parameters back to default on reset button click', async () => {
    window.history.pushState(
      null,
      '',
      '/compare/compute?vcpu=32&ram_gb=128&family=memory_optimized&region=eu-central'
    );

    vi.spyOn(pricesApi, 'getCompute').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<ComputeCompare />);

    expect(screen.getByTestId('compute-vcpu-input')).toHaveValue(32);
    expect(screen.getByTestId('compute-ram-input')).toHaveValue(128);

    fireEvent.click(screen.getByTestId('reset-query-btn'));

    expect(screen.getByTestId('compute-vcpu-input')).toHaveValue(4);
    expect(screen.getByTestId('compute-ram-input')).toHaveValue(16);
  });
});
