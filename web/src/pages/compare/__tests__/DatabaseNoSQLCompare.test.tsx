// web/src/pages/compare/__tests__/DatabaseNoSQLCompare.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Router } from '../../../router';
import { DatabaseNoSQLCompare } from '../DatabaseNoSQLCompare';
import { pricesApi } from '../../../api/client';
import type { DatabaseNoSQLComparisonResponse } from '../../../types/api';

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

describe('DatabaseNoSQLCompare Page', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/database-nosql');
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('queries NoSQL database prices and renders data models and throughput specs', async () => {
    const mockResponse: DatabaseNoSQLComparisonResponse = {
      results: [
        {
          provider: 'aws',
          sku_id: 'aws-dynamodb-doc',
          matched_spec: {
            data_model: 'document',
            pricing_mode: 'provisioned',
            read_units: 100,
            write_units: 20,
            storage_gb: 50,
            multi_region: false,
          },
          match_quality: 'exact',
          normalized_hourly_usd: 0.038,
          price: { amount: 0.038, currency: 'USD', unit: '/hr' },
          stale: false,
        },
      ],
      warnings: [],
    };

    const getNoSQLSpy = vi
      .spyOn(pricesApi, 'getDatabaseNoSQL')
      .mockResolvedValue(mockResponse);

    renderWithProviders(<DatabaseNoSQLCompare />);

    expect(
      screen.getByText('NoSQL Database Pricing Comparison')
    ).toBeInTheDocument();
    expect(screen.getByTestId('nosql-data-model-select')).toHaveValue('document');
    expect(screen.getByTestId('nosql-read-units-input')).toHaveValue(100);

    await waitFor(() => {
      expect(screen.getByText('AWS')).toBeInTheDocument();
      expect(screen.getByText('DOCUMENT')).toBeInTheDocument();
      expect(screen.getByText('(provisioned)')).toBeInTheDocument();
    });

    expect(getNoSQLSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        data_model: 'document',
        pricing_mode: 'provisioned',
        read_units: '100',
        write_units: '20',
        storage_gb: '50',
        multi_region: 'false',
        region: 'us-east',
        currency: 'USD',
      }),
      expect.anything()
    );
  });

  it('updates pricing mode and multi-region checkbox', async () => {
    vi.spyOn(pricesApi, 'getDatabaseNoSQL').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<DatabaseNoSQLCompare />);

    const modeSelect = screen.getByTestId('nosql-pricing-mode-select');
    fireEvent.change(modeSelect, { target: { value: 'on_demand' } });

    const multiRegionCheckbox = screen.getByTestId('nosql-multiregion-checkbox');
    fireEvent.click(multiRegionCheckbox);

    expect(window.location.search).toContain('pricing_mode=on_demand');
    expect(window.location.search).toContain('multi_region=true');
  });
});
