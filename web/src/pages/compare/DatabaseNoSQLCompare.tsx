// web/src/pages/compare/DatabaseNoSQLCompare.tsx
import React, { useMemo } from 'react';
import { useUrlParams } from '../../hooks/useUrlParams';
import { useDatabaseNoSQLComparison } from '../../api/queries/useComparisonQueries';
import { CompareTemplate, ComparisonResultRow } from '../../components/compare/CompareTemplate';
import { DebouncedInput } from '../../components/forms/DebouncedInput';
import {
  NoSQLDataModelSelect,
  NoSQLPricingModeSelect,
} from '../../components/forms/taxonomy/TaxonomySelects';
import { MissingAttributesIndicator } from '../../components/honesty/MissingAttributesIndicator';
import type { DatabaseNoSQLQueryParams } from '../../types/api';

const DEFAULT_NOSQL_PARAMS: DatabaseNoSQLQueryParams = {
  region: 'us-east',
  currency: 'USD',
  data_model: 'document',
  pricing_mode: 'provisioned',
  read_units: 100,
  write_units: 20,
  storage_gb: 50,
  multi_region: false,
};

export const DatabaseNoSQLCompare: React.FC = () => {
  const [params, setParams] = useUrlParams<DatabaseNoSQLQueryParams>(DEFAULT_NOSQL_PARAMS);

  const { data, isLoading, isError, error, refetch } = useDatabaseNoSQLComparison(params);

  const querySummary = useMemo(() => {
    return `${params.data_model} • ${params.pricing_mode} • ${params.read_units} R / ${params.write_units} W • ${params.storage_gb} GB`;
  }, [params]);

  const handleReset = () => {
    setParams(DEFAULT_NOSQL_PARAMS, { replace: false });
  };

  const sidebarControls = (
    <>
      <NoSQLDataModelSelect
        value={params.data_model || 'document'}
        onChange={(e) => setParams({ data_model: e.target.value })}
        data-testid="nosql-data-model-select"
      />

      <NoSQLPricingModeSelect
        value={params.pricing_mode || 'provisioned'}
        onChange={(e) => setParams({ pricing_mode: e.target.value })}
        data-testid="nosql-pricing-mode-select"
      />

      <DebouncedInput
        label="Read Throughput (Units/sec)"
        type="number"
        min={0}
        max={10000000}
        value={params.read_units}
        onChange={(val) => setParams({ read_units: val !== '' ? Number(val) : 100 })}
        placeholder="e.g. 100"
        data-testid="nosql-read-units-input"
      />

      <DebouncedInput
        label="Write Throughput (Units/sec)"
        type="number"
        min={0}
        max={10000000}
        value={params.write_units}
        onChange={(val) => setParams({ write_units: val !== '' ? Number(val) : 20 })}
        placeholder="e.g. 20"
        data-testid="nosql-write-units-input"
      />

      <DebouncedInput
        label="Storage Capacity (GB)"
        type="number"
        min={0}
        max={1000000}
        value={params.storage_gb}
        onChange={(val) => setParams({ storage_gb: val !== '' ? Number(val) : 50 })}
        placeholder="e.g. 50"
        data-testid="nosql-storage-input"
      />

      <div className="flex items-center justify-between pt-1">
        <label htmlFor="multi-region-toggle" className="text-xs font-mono text-text-secondary cursor-pointer">
          Multi-Region Replication
        </label>
        <input
          id="multi-region-toggle"
          type="checkbox"
          checked={params.multi_region === true || params.multi_region === 'true'}
          onChange={(e) => setParams({ multi_region: e.target.checked })}
          className="h-4 w-4 rounded-none border-border-default bg-surface-raised accent-border-accent cursor-pointer"
          data-testid="nosql-multiregion-checkbox"
        />
      </div>
    </>
  );

  return (
    <CompareTemplate<ComparisonResultRow>
      categoryTitle="NoSQL Database Pricing Comparison"
      categoryDescription="Compare managed NoSQL databases (DynamoDB, Cosmos DB, Firestore) across throughput and storage capacity."
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
        const dataModel = (spec?.data_model as string | undefined) ?? (row.data_model as string | undefined) ?? params.data_model;
        const pricingMode = (spec?.pricing_mode as string | undefined) ?? (row.pricing_mode as string | undefined) ?? params.pricing_mode;
        const readUnits = (spec?.read_units as number | undefined) ?? (row.read_units as number | undefined) ?? params.read_units;
        const writeUnits = (spec?.write_units as number | undefined) ?? (row.write_units as number | undefined) ?? params.write_units;
        const storageGb = (spec?.storage_gb as number | undefined) ?? (row.storage_gb as number | undefined) ?? params.storage_gb;
        const multiRegion = (spec?.multi_region as boolean | undefined) ?? (row.multi_region as boolean | undefined);
        return (
          <div className="space-y-0.5">
            <div className="font-mono text-text-primary font-medium flex items-center space-x-2">
              <span>{dataModel ? String(dataModel).replace('_', ' ').toUpperCase() : 'NoSQL Database'}</span>
              {pricingMode && (
                <span className="text-xs text-text-secondary">({String(pricingMode)})</span>
              )}
              {multiRegion && (
                <span className="text-[10px] uppercase px-1 py-0.5 border border-border-accent text-border-accent font-bold">
                  Multi-Region
                </span>
              )}
            </div>
            <div className="text-[11px] text-text-secondary">
              {readUnits} RCU/s • {writeUnits} WCU/s • {storageGb} GB storage
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
