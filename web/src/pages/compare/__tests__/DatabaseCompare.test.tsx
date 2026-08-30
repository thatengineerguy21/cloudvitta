// web/src/pages/compare/__tests__/DatabaseCompare.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Router } from '../../../router';
import { DatabaseCompare } from '../DatabaseCompare';
import { pricesApi } from '../../../api/client';
import type { DatabaseComparisonResponse } from '../../../types/api';

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

describe('DatabaseCompare Page', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/database');
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('queries database prices and renders engine and joined instance/storage specs', async () => {
    const mockResponse: DatabaseComparisonResponse = {
      results: [
        {
          provider: 'aws',
          sku_id: 'aws-rds-postgres-r5',
          matched_spec: {
            engine: 'postgresql',
            vcpu: 4,
            ram_gb: 32,
            storage_gb: 100,
            iops: 3000,
            multi_az: true,
          },
          match_quality: 'exact',
          normalized_hourly_usd: 0.52,
          price: { amount: 0.52, currency: 'USD', unit: '/hr' },
          stale: false,
        },
      ],
      warnings: [],
    };

    const getDatabaseSpy = vi
      .spyOn(pricesApi, 'getDatabase')
      .mockResolvedValue(mockResponse);

    renderWithProviders(<DatabaseCompare />);

    expect(
      screen.getByText('Relational Database (RDBMS) Pricing Comparison')
    ).toBeInTheDocument();
    expect(screen.getByTestId('database-engine-select')).toHaveValue('postgresql');
    expect(screen.getByTestId('database-storage-family-select')).toHaveValue('');
    expect(screen.getByTestId('database-vcpu-input')).toHaveValue(4);

    await waitFor(() => {
      expect(screen.getByText('AWS')).toBeInTheDocument();
      expect(screen.getByText('POSTGRESQL')).toBeInTheDocument();
      expect(screen.getByText('Multi-AZ')).toBeInTheDocument();
    });

    expect(getDatabaseSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        engine: 'postgresql',
        vcpu: '4',
        ram_gb: '16',
        storage_gb: '100',
        iops: '3000',
        multi_az: 'false',
        region: 'us-east',
        currency: 'USD',
      }),
      expect.anything()
    );
  });

  it('updates engine dropdown, storage_family dropdown, and multi_az toggle in URL', async () => {
    vi.spyOn(pricesApi, 'getDatabase').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<DatabaseCompare />);

    const engineSelect = screen.getByTestId('database-engine-select');
    fireEvent.change(engineSelect, { target: { value: 'mysql' } });

    const storageFamilySelect = screen.getByTestId('database-storage-family-select');
    fireEvent.change(storageFamilySelect, { target: { value: 'gp3' } });

    const multiAzCheckbox = screen.getByTestId('database-multiaz-checkbox');
    fireEvent.click(multiAzCheckbox);

    expect(window.location.search).toContain('engine=mysql');
    expect(window.location.search).toContain('storage_family=gp3');
    expect(window.location.search).toContain('multi_az=true');
  });
});
