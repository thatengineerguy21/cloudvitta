// web/src/components/compare/__tests__/ProviderCompareCard.test.tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ProviderCompareCard } from '../ProviderCompareCard';
import type { ComparisonResultRow } from '../CompareTemplate';

describe('ProviderCompareCard Component', () => {
  it('formats high-precision string price without decimal overflow and formats monthly integer with commas', () => {
    const row: ComparisonResultRow = {
      provider: 'gcp',
      sku_id: '05F5-1E12-C17F',
      match_quality: 'approximate',
      match_delta_pct: 50,
      // String with 16 decimal places as returned by Go decimal.Decimal JSON
      normalized_hourly_usd: '1095.8904109589041096' as unknown as number,
      monthly_cost_usd: '800000' as unknown as number,
      matched_spec: {
        transfer_type: 'inter_region',
        egress_gb: 1,
      },
      stale: false,
    };

    render(<ProviderCompareCard row={row} timeframe="hourly" currency="USD" />);

    // Primary display price should be formatted to 2 decimals with commas: $1,095.89
    expect(screen.getByText('$1,095.89')).toBeInTheDocument();
    // Not raw 16-decimal-place string
    expect(screen.queryByText(/1095\.8904109589041096/)).not.toBeInTheDocument();

    // Secondary monthly estimate should be formatted with commas: $800,000
    expect(screen.getByText('$800,000')).toBeInTheDocument();
    expect(screen.queryByText('$800000')).not.toBeInTheDocument();

    // Unit label should have whitespace-nowrap and not break
    const unitLabel = screen.getByText('/ hr');
    expect(unitLabel).toHaveClass('whitespace-nowrap');

    // Title defaults cleanly to Standard SKU without truncation
    expect(screen.getByText('Standard SKU')).toBeInTheDocument();
  });

  it('renders custom spec without dummy compute specs when row has no compute attributes', () => {
    const row: ComparisonResultRow = {
      provider: 'gcp',
      sku_id: '05F5-1E12-C17F',
      match_quality: 'approximate',
      match_delta_pct: 50,
      normalized_hourly_usd: 1.5,
      monthly_cost_usd: 1095,
      matched_spec: {
        transfer_type: 'inter_region',
        egress_gb: 1,
      },
    };

    render(
      <ProviderCompareCard
        row={row}
        renderCustomSpec={() => (
          <div data-testid="custom-network-spec">
            <span>INTER REGION</span>
            <span>1 GB outbound transfer</span>
          </div>
        )}
      />
    );

    // Should render custom spec
    expect(screen.getByTestId('custom-network-spec')).toBeInTheDocument();

    // Should NOT render dummy compute pills
    expect(screen.queryByText('Silicon / Profile:')).not.toBeInTheDocument();
    expect(screen.queryByText('vCPU')).not.toBeInTheDocument();
    expect(screen.queryByText('Memory')).not.toBeInTheDocument();
    expect(screen.queryByText('Included')).not.toBeInTheDocument();
    expect(screen.queryByText('Dynamic')).not.toBeInTheDocument();
  });

  it('renders 2x2 grid and silicon profile when row has compute specs', () => {
    const row: ComparisonResultRow = {
      provider: 'aws',
      sku_id: 'aws-c5-xlarge',
      instance_type: 'c5.xlarge',
      family: 'c5',
      vcpu: 4,
      ram_gb: 8,
      match_quality: 'exact',
      normalized_hourly_usd: 0.17,
      matched_spec: {
        instance_type: 'c5.xlarge',
        family: 'c5',
        vcpu: 4,
        ram_gb: 8,
        network_performance: 'Up to 10 Gbps',
        cpu_architecture: 'x86_64',
      },
    };

    render(<ProviderCompareCard row={row} isLowestTCO />);

    // Should display instance type as title
    expect(screen.getByText('c5.xlarge')).toBeInTheDocument();

    // Should render Lowest TCO pills (floating pill + badge)
    expect(screen.getAllByText('Lowest TCO')).toHaveLength(2);

    // Should render compute specs
    expect(screen.getByText('Silicon / Profile:')).toBeInTheDocument();
    expect(screen.getByText('4 Cores')).toBeInTheDocument();
    expect(screen.getByText('8 GiB')).toBeInTheDocument();
    expect(screen.getByText('Up to 10 Gbps')).toBeInTheDocument();
    expect(screen.getByText('aws-c5-xlarge')).toBeInTheDocument();
  });

  it('resolves instance_type from matched_spec when top-level row.instance_type is undefined', () => {
    const row: ComparisonResultRow = {
      provider: 'azure',
      sku_id: 'azure-d4s-v5',
      match_quality: 'close',
      normalized_hourly_usd: 0.19,
      matched_spec: {
        instance_type: 'Standard_D4s_v5',
        family: 'Dsv5',
        vcpu: 4,
        ram_gb: 16,
      },
    };

    render(<ProviderCompareCard row={row} />);

    // Should resolve "Standard_D4s_v5" instead of falling back to "Standard SKU"
    expect(screen.getByText('Standard_D4s_v5')).toBeInTheDocument();
  });

  it('formats fractional prices below 100 with 4 decimals', () => {
    const row: ComparisonResultRow = {
      provider: 'aws',
      sku_id: 'aws-t3-micro',
      instance_type: 't3.micro',
      match_quality: 'exact',
      normalized_hourly_usd: 0.0104,
      monthly_cost_usd: 7.592,
    };

    render(<ProviderCompareCard row={row} timeframe="hourly" />);

    expect(screen.getByText('$0.0104')).toBeInTheDocument();
    expect(screen.getByText('$7.5920')).toBeInTheDocument();
  });
});
