// web/src/components/status/__tests__/ComputeCatalogSummaryCard.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { catalogApi } from '../../../api/client';
import { ComputeCatalogSummaryCard } from '../ComputeCatalogSummaryCard';
import type { CatalogSummaryResponse } from '../../../types/api';

const createWrapper = () => {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
};

const mockSummaryData: CatalogSummaryResponse = {
  total_instances: 2950,
  provider_totals: {
    aws: 1200,
    azure: 950,
    gcp: 800,
  },
  category_breakdown: {
    aws: {
      general_purpose: 400,
      compute_optimized: 300,
      memory_optimized: 250,
      gpu_accelerated: 150,
      storage_optimized: 100,
    },
    azure: {
      general_purpose: 350,
      compute_optimized: 250,
      memory_optimized: 200,
      gpu_accelerated: 100,
      storage_optimized: 50,
    },
  },
};

describe('ComputeCatalogSummaryCard', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders skeleton while loading', () => {
    vi.spyOn(catalogApi, 'getComputeSummary').mockReturnValue(new Promise(() => {}));

    render(<ComputeCatalogSummaryCard />, { wrapper: createWrapper() });

    expect(screen.getByTestId('catalog-summary-skeleton')).toBeInTheDocument();
  });

  it('renders nothing if query fails or has no data', async () => {
    vi.spyOn(catalogApi, 'getComputeSummary').mockRejectedValue(new Error('Network error'));

    const { container } = render(<ComputeCatalogSummaryCard />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(container.firstChild).toBeNull();
    });
  });

  it('renders total instances, provider counts, and category breakdown', async () => {
    vi.spyOn(catalogApi, 'getComputeSummary').mockResolvedValue(mockSummaryData);

    render(<ComputeCatalogSummaryCard />, { wrapper: createWrapper() });

    await screen.findByTestId('compute-catalog-summary-card');

    expect(screen.getByTestId('catalog-total-instances')).toHaveTextContent('2,950');
    expect(screen.getByTestId('catalog-provider-total-aws')).toHaveTextContent('1,200');
    expect(screen.getByTestId('catalog-provider-total-azure')).toHaveTextContent('950');
    expect(screen.getByTestId('catalog-provider-total-gcp')).toHaveTextContent('800');

    expect(screen.getByTestId('catalog-category-table')).toBeInTheDocument();
    expect(screen.getByText('Registered Virtual Machine Specifications')).toBeInTheDocument();
  });
});
