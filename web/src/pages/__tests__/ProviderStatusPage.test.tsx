// web/src/pages/__tests__/ProviderStatusPage.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { Router } from '../../router';
import { providerApi } from '../../api/client';
import { ProviderStatusPage } from '../ProviderStatusPage';
import type { ProviderStatusResponse } from '../../types/api';

const createWrapper = () => {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>
      <Router>{children}</Router>
    </QueryClientProvider>
  );
};

const makeHealthy = (provider: string): ProviderStatusResponse => ({
  provider,
  status: 'healthy',
  stale: false,
  categories: {
    compute: { category: 'compute', supported: true, observation_count: 100, stale: false, staleness_threshold_hours: 168 },
  },
  warnings: [],
});

describe('ProviderStatusPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders all 7 provider cards in grid', async () => {
    vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) =>
      Promise.resolve(makeHealthy(provider))
    );

    render(<ProviderStatusPage />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByTestId('provider-status-page')).toBeInTheDocument();
    });

    // Wait for all cards to load
    await waitFor(() => {
      expect(screen.getByTestId('provider-card-aws')).toBeInTheDocument();
      expect(screen.getByTestId('provider-card-azure')).toBeInTheDocument();
      expect(screen.getByTestId('provider-card-gcp')).toBeInTheDocument();
      expect(screen.getByTestId('provider-card-oracle')).toBeInTheDocument();
      expect(screen.getByTestId('provider-card-ibm')).toBeInTheDocument();
      expect(screen.getByTestId('provider-card-alibaba')).toBeInTheDocument();
      expect(screen.getByTestId('provider-card-digitalocean')).toBeInTheDocument();
    });

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Cloud Provider Operational Status & Data Freshness'
    );
  });

  it('isolates provider failure — other 6 cards still render', async () => {
    vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) => {
      if (provider === 'azure') {
        return Promise.reject(new Error('Azure API timeout'));
      }
      return Promise.resolve(makeHealthy(provider));
    });

    render(<ProviderStatusPage />, { wrapper: createWrapper() });

    // Wait for the healthy cards to load
    await waitFor(() => {
      expect(screen.getByTestId('provider-card-aws')).toBeInTheDocument();
      expect(screen.getByTestId('provider-card-gcp')).toBeInTheDocument();
    });

    // Azure shows error, not blank
    await waitFor(() => {
      expect(screen.getByTestId('provider-error-azure')).toBeInTheDocument();
    });

    // Other providers still rendered
    expect(screen.getByTestId('provider-card-oracle')).toBeInTheDocument();
    expect(screen.getByTestId('provider-card-ibm')).toBeInTheDocument();
    expect(screen.getByTestId('provider-card-alibaba')).toBeInTheDocument();
    expect(screen.getByTestId('provider-card-digitalocean')).toBeInTheDocument();
  });
});
