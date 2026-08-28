// web/src/components/calculate/__tests__/CalculateResultsMatrix.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { CalculateResultsMatrix } from '../CalculateResultsMatrix';
import type { CalculateProviderResult, ProviderWarning } from '../../../types/api';
import { ApiError } from '../../../api/errors';

describe('CalculateResultsMatrix & ADR 0022 Honesty Contract', () => {
  const mockResults: CalculateProviderResult[] = [
    {
      provider: 'aws',
      partial: false,
      total_normalized_hourly_usd: 0.254,
      categories: {
        compute: {
          sku_id: 'aws-ec2-t4g-xlarge',
          match_quality: 'exact',
          normalized_hourly_usd: 0.1344,
          match_delta_pct: 0,
        },
        storage: {
          sku_id: 'aws-s3-standard',
          match_quality: 'exact',
          normalized_hourly_usd: 0.023,
          match_delta_pct: 0,
        },
      },
    },
    {
      provider: 'gcp',
      partial: true,
      partial_total_normalized_hourly_usd: 0.128,
      categories: {
        compute: {
          sku_id: 'gcp-compute-t2a-standard-4',
          match_quality: 'close',
          normalized_hourly_usd: 0.128,
          match_delta_pct: 3.2,
          missing_attributes: ['gpu_count'],
          stale: true,
        },
      },
    },
  ];

  it('renders complete total in solid typography and omits partial badge when partial === false (ADR 0022)', () => {
    render(
      <CalculateResultsMatrix
        results={mockResults}
        requestedCategories={['compute', 'storage']}
        currency="USD"
      />
    );

    // AWS has partial === false
    expect(screen.getByTestId('complete-total-aws')).toBeInTheDocument();
    expect(screen.queryByTestId('partial-total-aws')).not.toBeInTheDocument();
    expect(screen.getByText('Complete Workload Total')).toBeInTheDocument();
  });

  it('renders partial total with dashed border and omits complete total when partial === true (ADR 0022)', () => {
    render(
      <CalculateResultsMatrix
        results={mockResults}
        requestedCategories={['compute', 'storage']}
        currency="USD"
      />
    );

    // GCP has partial === true
    expect(screen.getByTestId('partial-total-gcp')).toBeInTheDocument();
    expect(screen.queryByTestId('complete-total-gcp')).not.toBeInTheDocument();
    expect(screen.getByText(/1\/2 categories included/i)).toBeInTheDocument();
  });

  it('renders empty state when no categories are requested', () => {
    render(
      <CalculateResultsMatrix
        results={[]}
        requestedCategories={[]}
        currency="USD"
      />
    );

    expect(screen.getByText('No Workload Components Selected')).toBeInTheDocument();
    expect(screen.getByText(/Enable at least one category/i)).toBeInTheDocument();
  });

  it('renders RFC 7807 error details and retry affordance on error', () => {
    const refetch = vi.fn();
    const apiErr = new ApiError({
      type: 'https://cloudvitta.dev/errors/missing-categories',
      title: 'Missing Workload Categories',
      status: 400,
      detail: 'At least one workload category must be specified.',
      instance: '/api/v1/calculate',
      invalid_params: [{ name: 'compute', reason: 'At least one category required' }],
    });

    render(
      <CalculateResultsMatrix
        isError
        error={apiErr}
        refetch={refetch}
        requestedCategories={['compute']}
      />
    );

    expect(screen.getByText('Missing Workload Categories')).toBeInTheDocument();
    expect(screen.getByText(/At least one category required/i)).toBeInTheDocument();

    const retryBtn = screen.getByRole('button', { name: /retry calculation/i });
    fireEvent.click(retryBtn);
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('renders category honesty elements: match quality, missing attributes, and stale badges', () => {
    render(
      <CalculateResultsMatrix
        results={mockResults}
        requestedCategories={['compute', 'storage']}
        currency="USD"
      />
    );

    // GCP missing attributes and stale
    expect(screen.getByText(/1 unverified/i)).toBeInTheDocument();
    expect(screen.getByText(/Stale/i)).toBeInTheDocument();
    expect(screen.getByText('gcp-compute-t2a-standard-4')).toBeInTheDocument();
  });

  it('toggles sort order when price sort button is clicked', () => {
    render(
      <CalculateResultsMatrix
        results={mockResults}
        requestedCategories={['compute', 'storage']}
        currency="USD"
      />
    );

    const sortBtn = screen.getByTitle(/sort by total price/i);
    expect(sortBtn).toHaveTextContent('Price: Low → High');

    fireEvent.click(sortBtn);
    expect(sortBtn).toHaveTextContent('Price: High → Low');
  });

  it('renders top-level calculation warnings banner', () => {
    const warnings: ProviderWarning[] = [
      {
        provider: 'alibaba',
        code: 'category_not_supported',
        message: 'Alibaba does not support Kubernetes control-plane comparisons.',
      },
    ];

    render(
      <CalculateResultsMatrix
        results={mockResults}
        warnings={warnings}
        requestedCategories={['compute', 'storage']}
        currency="USD"
      />
    );

    expect(screen.getByText(/Alibaba does not support Kubernetes/i)).toBeInTheDocument();
  });

  it('sorts complete providers ahead of partial estimates in ascending price sort (ADR 0022)', () => {
    const mixedResults: CalculateProviderResult[] = [
      {
        provider: 'gcp',
        partial: true,
        partial_total_normalized_hourly_usd: 0.038, // artificially low because 1 category is missing
      },
      {
        provider: 'aws',
        partial: false,
        total_normalized_hourly_usd: 0.192, // full 2-category total
      },
    ];

    render(
      <CalculateResultsMatrix
        results={mixedResults}
        requestedCategories={['compute', 'storage']}
        currency="USD"
      />
    );

    const awsBadge = screen.getByTestId('provider-name-aws');
    const gcpBadge = screen.getByTestId('provider-name-gcp');

    // AWS (complete) must appear before GCP (partial) in document order
    expect(awsBadge.compareDocumentPosition(gcpBadge)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });
});
