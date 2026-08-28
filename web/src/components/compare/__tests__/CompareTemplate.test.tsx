// web/src/components/compare/__tests__/CompareTemplate.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { CompareTemplate, ComparisonResultRow } from '../CompareTemplate';
import { ApiError } from '../../../api/errors';

const mockResults: ComparisonResultRow[] = [
  {
    provider: 'aws',
    sku_id: 'aws-c5-xlarge',
    instance_type: 'c5.xlarge',
    match_quality: 'exact',
    match_score: 1.0,
    match_delta_pct: 0,
    price: { amount: 0.17, currency: 'USD', unit: '/hr' },
    normalized_hourly_usd: 0.17,
    stale: false,
    fetched_at: '2026-08-28T00:00:00Z',
  },
  {
    provider: 'azure',
    sku_id: 'azure-d4s-v5',
    instance_type: 'Standard_D4s_v5',
    match_quality: 'close',
    match_score: 0.95,
    match_delta_pct: 5,
    missing_attributes: ['network_performance'],
    price: { amount: 0.19, currency: 'USD', unit: '/hr' },
    normalized_hourly_usd: 0.19,
    stale: true,
    fetched_at: '2026-08-10T00:00:00Z',
  },
  {
    provider: 'gcp',
    sku_id: 'gcp-c2-standard-4',
    instance_type: 'c2-standard-4',
    match_quality: 'approximate',
    match_score: 0.85,
    price: { amount: 0.21, currency: 'USD', unit: '/hr' },
    normalized_hourly_usd: 0.21,
    stale: false,
  },
];

describe('CompareTemplate Component', () => {
  it('renders title, query summary, and results rows in table', () => {
    render(
      <CompareTemplate
        categoryTitle="Compute Pricing Comparison"
        region="us-east"
        currency="USD"
        onRegionChange={vi.fn()}
        onCurrencyChange={vi.fn()}
        querySummary="4 vCPU • 16 GB RAM • us-east • USD"
        onReset={vi.fn()}
        sidebarControls={<div>Sidebar Fields</div>}
        isLoading={false}
        isError={false}
        results={mockResults}
      />
    );

    expect(screen.getByText('Compute Pricing Comparison')).toBeInTheDocument();
    expect(screen.getByText('4 vCPU • 16 GB RAM • us-east • USD')).toBeInTheDocument();
    expect(screen.getByText('AWS')).toBeInTheDocument();
    expect(screen.getByText('Azure')).toBeInTheDocument();
    expect(screen.getByText('GCP')).toBeInTheDocument();
    expect(screen.getByText('Exact Match')).toBeInTheDocument();
    expect(screen.getByText('Close Match')).toBeInTheDocument();
    expect(screen.getByText('Approximate')).toBeInTheDocument();
    expect(screen.getByText(/1 unverified/i)).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument(); // GCP (no fetched_at)
  });

  it('renders loading skeleton when isLoading is true', () => {
    render(
      <CompareTemplate
        categoryTitle="Compute Pricing Comparison"
        region="us-east"
        currency="USD"
        onRegionChange={vi.fn()}
        onCurrencyChange={vi.fn()}
        querySummary="4 vCPU • 16 GB RAM • us-east • USD"
        onReset={vi.fn()}
        sidebarControls={<div>Sidebar Fields</div>}
        isLoading={true}
        isError={false}
        results={[]}
      />
    );

    expect(screen.getByTestId('compare-skeleton')).toBeInTheDocument();
  });

  it('renders RFC 7807 error container and retry affordance when isError is true', () => {
    const refetchSpy = vi.fn();
    const apiError = new ApiError({
      type: 'https://cloudvitta.dev/errors/rate-limit-exceeded',
      title: 'Too Many Requests',
      status: 429,
      detail: 'Rate limit quota exceeded.',
      instance: '/api/v1/prices/compute',
    });

    render(
      <CompareTemplate
        categoryTitle="Compute Pricing Comparison"
        region="us-east"
        currency="USD"
        onRegionChange={vi.fn()}
        onCurrencyChange={vi.fn()}
        querySummary="4 vCPU • 16 GB RAM • us-east • USD"
        onReset={vi.fn()}
        sidebarControls={<div>Sidebar Fields</div>}
        isLoading={false}
        isError={true}
        error={apiError}
        refetch={refetchSpy}
        results={[]}
      />
    );

    expect(screen.getByTestId('compare-error-container')).toBeInTheDocument();
    expect(screen.getByText('Too Many Requests')).toBeInTheDocument();
    expect(screen.getByText('Rate limit quota exceeded.')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Retry Query'));
    expect(refetchSpy).toHaveBeenCalledTimes(1);
  });

  it('renders uncollapsed warnings banner when warnings array is populated', () => {
    render(
      <CompareTemplate
        categoryTitle="Compute Pricing Comparison"
        region="us-east"
        currency="USD"
        onRegionChange={vi.fn()}
        onCurrencyChange={vi.fn()}
        querySummary="4 vCPU • 16 GB RAM • us-east • USD"
        onReset={vi.fn()}
        sidebarControls={<div>Sidebar Fields</div>}
        isLoading={false}
        isError={false}
        results={mockResults}
        warnings={[
          {
            provider: 'digitalocean',
            code: 'pricing_anomaly_flagged',
            message: 'Significant price deviation detected.',
          },
        ]}
      />
    );

    expect(screen.getByTestId('compare-warnings-banner')).toBeInTheDocument();
    expect(screen.getByText(/Significant price deviation detected/i)).toBeInTheDocument();
  });

  it('sorts results by price ascending and descending when sort selector changes', () => {
    render(
      <CompareTemplate
        categoryTitle="Compute Pricing Comparison"
        region="us-east"
        currency="USD"
        onRegionChange={vi.fn()}
        onCurrencyChange={vi.fn()}
        querySummary="4 vCPU • 16 GB RAM • us-east • USD"
        onReset={vi.fn()}
        sidebarControls={<div>Sidebar Fields</div>}
        isLoading={false}
        isError={false}
        results={mockResults}
      />
    );

    const sortSelect = screen.getByTestId('sort-order-select');
    expect(sortSelect).toHaveValue('asc');

    const rowsBefore = screen.getAllByTestId(/^row-/);
    expect(rowsBefore[0]).toHaveAttribute('data-testid', 'row-aws'); // 0.17
    expect(rowsBefore[2]).toHaveAttribute('data-testid', 'row-gcp'); // 0.21

    fireEvent.change(sortSelect, { target: { value: 'desc' } });

    const rowsAfter = screen.getAllByTestId(/^row-/);
    expect(rowsAfter[0]).toHaveAttribute('data-testid', 'row-gcp'); // 0.21
    expect(rowsAfter[2]).toHaveAttribute('data-testid', 'row-aws'); // 0.17
  });

  it('renders empty state message when no results match', () => {
    render(
      <CompareTemplate
        categoryTitle="Compute Pricing Comparison"
        region="us-east"
        currency="USD"
        onRegionChange={vi.fn()}
        onCurrencyChange={vi.fn()}
        querySummary="4 vCPU • 16 GB RAM • us-east • USD"
        onReset={vi.fn()}
        sidebarControls={<div>Sidebar Fields</div>}
        isLoading={false}
        isError={false}
        results={[]}
      />
    );

    expect(screen.getByTestId('compare-empty-state')).toBeInTheDocument();
    expect(screen.getByText('No Matching SKUs Found')).toBeInTheDocument();
  });
});
