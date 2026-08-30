// web/src/pages/compare/StorageCompare.tsx
import React, { useMemo } from 'react';
import { useUrlParams } from '../../hooks/useUrlParams';
import { useStorageComparison } from '../../api/queries/useComparisonQueries';
import { CompareTemplate, ComparisonResultRow } from '../../components/compare/CompareTemplate';
import { DebouncedInput } from '../../components/forms/DebouncedInput';
import { StorageClassSelect } from '../../components/forms/taxonomy/TaxonomySelects';
import { MissingAttributesIndicator } from '../../components/honesty/MissingAttributesIndicator';
import type { StorageQueryParams } from '../../types/api';

const DEFAULT_STORAGE_PARAMS: StorageQueryParams = {
  region: 'us-east',
  currency: 'USD',
  size_gb: 500,
  storage_class: 'standard',
};

export const StorageCompare: React.FC = () => {
  const [params, setParams] = useUrlParams<StorageQueryParams>(DEFAULT_STORAGE_PARAMS);

  const { data, isLoading, isError, error, refetch } = useStorageComparison(params);

  const querySummary = useMemo(() => {
    return `${params.size_gb} GB • ${params.storage_class} • ${params.region} • ${params.currency}`;
  }, [params]);

  const handleReset = () => {
    setParams(DEFAULT_STORAGE_PARAMS, { replace: false });
  };

  const sidebarControls = (
    <>
      <DebouncedInput
        label="Storage Capacity (GB)"
        type="number"
        min={1}
        max={1000000}
        value={params.size_gb}
        onChange={(val) => setParams({ size_gb: val ? Number(val) : 500 })}
        placeholder="e.g. 500"
        data-testid="storage-size-input"
      />

      <StorageClassSelect
        value={params.storage_class || 'standard'}
        onChange={(e) => setParams({ storage_class: e.target.value })}
        data-testid="storage-class-select"
      />
    </>
  );

  return (
    <CompareTemplate<ComparisonResultRow>
      categoryTitle="Storage Pricing Comparison"
      categoryDescription="Compare normalized object and block storage pricing per hour/month across 7 cloud providers."
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
        const storageClass = (spec?.storage_class as string | undefined) ?? (row.storage_class as string | undefined) ?? params.storage_class;
        const sizeGb = (spec?.size_gb as number | undefined) ?? (row.size_gb as number | undefined) ?? params.size_gb;
        return (
          <div className="space-y-0.5">
            <div className="font-mono text-text-primary font-medium">
              {storageClass ? `${String(storageClass).replace('_', ' ').toUpperCase()} Tier` : 'Standard Storage'}
            </div>
            <div className="text-[11px] text-text-secondary">
              {sizeGb} GB provisioned capacity
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
