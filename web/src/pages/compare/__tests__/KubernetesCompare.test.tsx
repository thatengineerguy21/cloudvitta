// web/src/pages/compare/__tests__/KubernetesCompare.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Router } from '../../../router';
import { KubernetesCompare } from '../KubernetesCompare';
import { pricesApi } from '../../../api/client';
import type { KubernetesComparisonResponse } from '../../../types/api';

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

describe('KubernetesCompare Page', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/kubernetes');
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('queries Kubernetes control-plane prices and renders cluster credit adjustments', async () => {
    const mockResponse: KubernetesComparisonResponse = {
      results: [
        {
          provider: 'gcp',
          sku_id: 'gcp-gke-standard',
          matched_spec: {
            tier: 'standard',
            cluster_topology: 'zonal',
          },
          match_quality: 'exact',
          normalized_hourly_usd: 0.0,
          price: { amount: 0.0, currency: 'USD', unit: '/hr' },
          stale: false,
        },
      ],
      warnings: [],
    };

    const getKubernetesSpy = vi
      .spyOn(pricesApi, 'getKubernetes')
      .mockResolvedValue(mockResponse);

    renderWithProviders(<KubernetesCompare />);

    expect(
      screen.getByText('Kubernetes Control-Plane Pricing Comparison')
    ).toBeInTheDocument();
    expect(screen.getByTestId('kubernetes-tier-select')).toHaveValue('standard');
    expect(screen.getByTestId('kubernetes-topology-select')).toHaveValue('zonal');

    await waitFor(() => {
      expect(screen.getByText('GCP')).toBeInTheDocument();
      expect(screen.getByText('STANDARD Tier')).toBeInTheDocument();
    });

    expect(getKubernetesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        tier: 'standard',
        cluster_topology: 'zonal',
        region: 'us-east',
        currency: 'USD',
      }),
      expect.anything()
    );
  });

  it('updates management tier and cluster topology', async () => {
    vi.spyOn(pricesApi, 'getKubernetes').mockResolvedValue({
      results: [],
      warnings: [],
    });

    renderWithProviders(<KubernetesCompare />);

    const tierSelect = screen.getByTestId('kubernetes-tier-select');
    fireEvent.change(tierSelect, { target: { value: 'extended_support' } });

    const topologySelect = screen.getByTestId('kubernetes-topology-select');
    fireEvent.change(topologySelect, { target: { value: 'regional' } });

    expect(window.location.search).toContain('tier=extended_support');
    expect(window.location.search).toContain('cluster_topology=regional');
  });
});
