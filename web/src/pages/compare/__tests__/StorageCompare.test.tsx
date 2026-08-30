// web/src/pages/compare/__tests__/StorageCompare.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Router } from '../../../router';
import { StorageCompare } from '../StorageCompare';
import { pricesApi } from '../../../api/client';
import type { StorageComparisonResponse } from '../../../types/api';

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

describe('StorageCompare Page', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/storage');
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('queries storage prices and renders provider cards and storage classes', async () => {
    const mockResponse: StorageComparisonResponse = {
      results: [
        {
          provider: 'aws',
          sku_id: 'aws-s3-standard',
          matched_spec: {
            storage_class: 'standard',
            size_gb: 500,
          },
          match_quality: 'exact',
          monthly_cost_usd: 11.5,
          price: { amount: 11.5, currency: 'USD', unit: '/mo' },
          stale: false,
        },
      ],
      warnings: [],
    };

    const getStorageSpy = vi
      .spyOn(pricesApi, 'getStorage')
      .mockResolvedValue(mockResponse);

    renderWithProviders(<StorageCompare />);

    expect(screen.getByText('Storage Pricing Comparison')).toBeInTheDocument();
    expect(screen.getByTestId('storage-size-input')).toHaveValue(500);

    await waitFor(() => {
      const table = screen.getByTestId('results-table');
      expect(within(table).getByText('AWS')).toBeInTheDocument();
      expect(within(table).getByText(/STANDARD Tier/i)).toBeInTheDocument();
    });

    expect(getStorageSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        size_gb: '500',
        storage_class: 'standard',
        region: 'us-east',
        currency: 'USD',
      }),
      expect.anything()
    );
  });

  it('updates storage tier and URL query parameters', async () => {
    vi.spyOn(pricesApi, 'getStorage').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<StorageCompare />);

    const classSelect = screen.getByTestId('storage-class-select');
    fireEvent.change(classSelect, { target: { value: 'archive' } });

    expect(window.location.search).toContain('storage_class=archive');
  });
});
