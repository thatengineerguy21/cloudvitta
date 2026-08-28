// web/src/components/status/__tests__/ProviderStatusCard.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { Router } from '../../../router';
import { providerApi } from '../../../api/client';
import { ProviderStatusCard } from '../ProviderStatusCard';
import type { ProviderStatusResponse } from '../../../types/api';

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

const makeStatus = (
  provider: string,
  status: string,
  overrides: Partial<ProviderStatusResponse> = {}
): ProviderStatusResponse => ({
  provider,
  status,
  stale: status === 'stale',
  categories: {},
  warnings: [],
  ...overrides,
});

describe('ProviderStatusCard', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders skeleton while loading', () => {
    // Never resolve to keep loading forever
    vi.spyOn(providerApi, 'getStatus').mockReturnValue(new Promise(() => {}));

    render(<ProviderStatusCard provider="aws" />, { wrapper: createWrapper() });

    expect(screen.getByTestId('provider-status-skeleton')).toBeInTheDocument();
  });

  it('renders healthy provider card with categories', async () => {
    vi.spyOn(providerApi, 'getStatus').mockResolvedValue(
      makeStatus('aws', 'healthy', {
        last_successful_fetch: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
        categories: {
          compute: {
            category: 'compute',
            supported: true,
            observation_count: 450,
            stale: false,
            staleness_threshold_hours: 168,
          },
          storage: {
            category: 'storage',
            supported: true,
            observation_count: 32,
            stale: false,
            staleness_threshold_hours: 168,
          },
        },
      })
    );

    render(<ProviderStatusCard provider="aws" />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-aws')).toBeInTheDocument();
    });

    expect(screen.getByTestId('status-badge-aws')).toHaveTextContent('healthy');
    expect(screen.getByText('450 obs')).toBeInTheDocument();
    expect(screen.getByText('32 obs')).toBeInTheDocument();
    // Fresh indicators
    const freshLabels = screen.getAllByText('Fresh');
    expect(freshLabels.length).toBe(2);
  });

  it('renders not_yet_ingested provider with warnings banner', async () => {
    vi.spyOn(providerApi, 'getStatus').mockResolvedValue(
      makeStatus('oracle', 'not_yet_ingested', {
        warnings: [
          {
            provider: 'oracle',
            code: 'not_yet_ingested',
            message: 'Oracle OCI ingestion lands in stage 4.',
          },
        ],
      })
    );

    render(<ProviderStatusCard provider="oracle" />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-oracle')).toBeInTheDocument();
    });

    expect(screen.getByTestId('status-badge-oracle')).toHaveTextContent('not yet ingested');
    expect(screen.getByText('Oracle OCI ingestion lands in stage 4.')).toBeInTheDocument();
  });

  it('renders error state with retry button', async () => {
    const getStatusSpy = vi
      .spyOn(providerApi, 'getStatus')
      .mockRejectedValueOnce(new Error('Connection timeout'))
      .mockResolvedValueOnce(makeStatus('azure', 'healthy'));

    const user = userEvent.setup();
    render(<ProviderStatusCard provider="azure" />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByTestId('provider-error-azure')).toBeInTheDocument();
    });

    expect(screen.getByText('Connection timeout')).toBeInTheDocument();

    // Click retry — should re-call getStatus
    await user.click(screen.getByRole('button', { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-azure')).toBeInTheDocument();
    });

    expect(getStatusSpy).toHaveBeenCalledTimes(2);
  });

  it('renders stale category indicators', async () => {
    vi.spyOn(providerApi, 'getStatus').mockResolvedValue(
      makeStatus('gcp', 'degraded', {
        categories: {
          compute: {
            category: 'compute',
            supported: true,
            observation_count: 100,
            stale: true,
            staleness_threshold_hours: 168,
          },
          storage: {
            category: 'storage',
            supported: true,
            observation_count: 20,
            stale: false,
            staleness_threshold_hours: 168,
          },
        },
      })
    );

    render(<ProviderStatusCard provider="gcp" />, { wrapper: createWrapper() });

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-gcp')).toBeInTheDocument();
    });

    expect(screen.getByText('Stale')).toBeInTheDocument();
    expect(screen.getByText('Fresh')).toBeInTheDocument();
  });
});
