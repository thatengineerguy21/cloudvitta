// web/src/pages/compare/NetworkCompare.tsx
import React, { useMemo } from 'react';
import { useUrlParams } from '../../hooks/useUrlParams';
import { useNetworkComparison } from '../../api/queries/useComparisonQueries';
import { CompareTemplate, ComparisonResultRow } from '../../components/compare/CompareTemplate';
import { DebouncedInput } from '../../components/forms/DebouncedInput';
import { TransferTypeSelect } from '../../components/forms/taxonomy/TaxonomySelects';
import { MissingAttributesIndicator } from '../../components/honesty/MissingAttributesIndicator';
import type { NetworkQueryParams } from '../../types/api';

const DEFAULT_NETWORK_PARAMS: NetworkQueryParams = {
  region: 'us-east',
  currency: 'USD',
  egress_gb: 1000,
  transfer_type: 'internet_egress',
};

export const NetworkCompare: React.FC = () => {
  const [params, setParams] = useUrlParams<NetworkQueryParams>(DEFAULT_NETWORK_PARAMS);

  const { data, isLoading, isError, error, refetch } = useNetworkComparison(params);

  const querySummary = useMemo(() => {
    return `${params.egress_gb} GB • ${params.transfer_type} • ${params.region} • ${params.currency}`;
  }, [params]);

  const handleReset = () => {
    setParams(DEFAULT_NETWORK_PARAMS, { replace: false });
  };

  const sidebarControls = (
    <>
      <DebouncedInput
        label="Egress Volume (GB)"
        type="number"
        min={1}
        max={10000000}
        value={params.egress_gb}
        onChange={(val) => setParams({ egress_gb: val ? Number(val) : 1000 })}
        placeholder="e.g. 1000"
        data-testid="network-egress-input"
      />

      <TransferTypeSelect
        value={params.transfer_type || 'internet_egress'}
        onChange={(e) => setParams({ transfer_type: e.target.value })}
        data-testid="network-transfer-type-select"
      />
    </>
  );

  return (
    <CompareTemplate<ComparisonResultRow>
      categoryTitle="Network & Egress Pricing Comparison"
      categoryDescription="Compare data transfer egress and inter-region bandwidth pricing across 7 cloud providers."
      region={params.region || 'us-east'}
      currency={params.currency || 'USD'}
      onRegionChange={(region) => setParams({ region })}
      onCurrencyChange={(currency) => setParams({ currency })}
      querySummary={querySummary}
      onReset={handleReset}
      sidebarControls={sidebarControls}
      isLoading={isLoading}
      isError={isError}
      error={error}
      refetch={refetch}
      results={data?.results}
      warnings={data?.warnings}
      renderCustomSpec={(row) => {
        const spec = row.matched_spec as Record<string, unknown> | undefined;
        const transferType = (spec?.transfer_type as string | undefined) ?? (row.transfer_type as string | undefined) ?? params.transfer_type;
        const egressGb = (spec?.egress_gb as number | undefined) ?? (row.egress_gb as number | undefined) ?? params.egress_gb;
        return (
          <div className="space-y-0.5">
            <div className="font-mono text-text-primary font-medium">
              {transferType ? String(transferType).replace('_', ' ').toUpperCase() : 'Network Egress'}
            </div>
            <div className="text-[11px] text-text-secondary">
              {egressGb} GB outbound transfer
            </div>
            {row.missing_attributes && row.missing_attributes.length > 0 && (
              <MissingAttributesIndicator missingAttributes={row.missing_attributes} />
            )}
          </div>
        );
      }}
    />
  );
};
