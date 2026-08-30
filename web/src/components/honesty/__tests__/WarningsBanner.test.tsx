import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { WarningsBanner } from '../WarningsBanner';
import { ProviderWarning, WarningCode } from '../../../types';

describe('WarningsBanner Component', () => {
  it('returns null when warnings array is empty or undefined', () => {
    const { container } = render(<WarningsBanner warnings={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('maps all 11 canonical warning codes to explicit, intentional severity tiers (Task 3 acceptance criterion)', () => {
    const all11Codes: WarningCode[] = [
      'pricing_anomaly_flagged',
      'fetch_failed',
      'stale_pricing_data',
      'category_not_supported',
      'not_yet_ingested',
      'no_match',
      'no_data_available',
      'non_usd_currency_unsupported',
      'engine_mismatch_excluded',
      'architecture_unsupported_excluded',
      'cluster_topology_unspecified',
    ];

    const testWarnings: ProviderWarning[] = all11Codes.map((code) => ({
      provider: 'aws',
      code,
      message: `Test warning for ${code}`,
    }));

    const { container } = render(<WarningsBanner warnings={testWarnings} />);

    // Assert all 11 warnings rendered with provider tag and message
    expect(screen.getAllByText('[aws]').length).toBe(11);

    // Verify 1. Anomaly / Failure Tier (2 codes)
    const anomalyWarnings = ['pricing_anomaly_flagged', 'fetch_failed'];
    anomalyWarnings.forEach((code) => {
      const codeSpan = screen.getByText(code);
      const row = codeSpan.closest('div.border')!;
      expect(row).toHaveClass('border-status-anomaly', 'bg-status-anomaly/5');
    });

    // Verify 2. Stale / Data Missing Tier (6 codes)
    const staleWarnings = [
      'stale_pricing_data',
      'category_not_supported',
      'not_yet_ingested',
      'no_match',
      'no_data_available',
      'non_usd_currency_unsupported',
    ];
    staleWarnings.forEach((code) => {
      const codeSpan = screen.getByText(code);
      const row = codeSpan.closest('div.border')!;
      expect(row).toHaveClass('border-status-stale', 'bg-status-stale/5');
    });

    // Verify 3. Informational Filter Explanation Tier (3 codes)
    const informationalWarnings = [
      'engine_mismatch_excluded',
      'architecture_unsupported_excluded',
      'cluster_topology_unspecified',
    ];
    informationalWarnings.forEach((code) => {
      const codeSpan = screen.getByText(code);
      const row = codeSpan.closest('div.border')!;
      expect(row).toHaveClass('border-border-default', 'bg-surface-raised');
    });

    // Assert decorative icons carry aria-hidden="true" (Task 4)
    const svgs = container.querySelectorAll('svg');
    expect(svgs.length).toBe(11);
    svgs.forEach((svg) => {
      expect(svg).toHaveAttribute('aria-hidden', 'true');
    });
  });
});
