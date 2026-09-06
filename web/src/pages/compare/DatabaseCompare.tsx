// web/src/pages/compare/DatabaseCompare.tsx
import React, { useMemo } from 'react';
import { useUrlParams } from '../../hooks/useUrlParams';
import { useDatabaseComparison } from '../../api/queries/useComparisonQueries';
import { CompareTemplate, ComparisonResultRow } from '../../components/compare/CompareTemplate';
import { DebouncedInput } from '../../components/forms/DebouncedInput';
import { DatabaseEngineSelect, StorageFamilySelect } from '../../components/forms/taxonomy/TaxonomySelects';
import { MissingAttributesIndicator } from '../../components/honesty/MissingAttributesIndicator';
import type { DatabaseQueryParams } from '../../types/api';

const DEFAULT_DATABASE_PARAMS: DatabaseQueryParams = {
  region: 'us-east',
  currency: 'USD',
  engine: 'postgresql',
  vcpu: 4,
  ram_gb: 16,
  storage_gb: 100,
  iops: 3000,
  multi_az: false,
  storage_family: '',
};

export const DatabaseCompare: React.FC = () => {
  const [params, setParams] = useUrlParams<DatabaseQueryParams>(DEFAULT_DATABASE_PARAMS);

  const { data, isLoading, isError, error, refetch } = useDatabaseComparison(params);

  const querySummary = useMemo(() => {
    return `${params.engine} • ${params.vcpu} vCPU • ${params.ram_gb} GB RAM • ${params.storage_gb} GB • ${params.region}`;
  }, [params]);

  const handleReset = () => {
    setParams(DEFAULT_DATABASE_PARAMS, { replace: false });
  };

  const sidebarControls = (
    <>
      <DatabaseEngineSelect
        value={params.engine || 'postgresql'}
        onChange={(e) => setParams({ engine: e.target.value })}
        data-testid="database-engine-select"
      />

      <StorageFamilySelect
        value={params.storage_family || ''}
        onChange={(e) => setParams({ storage_family: e.target.value })}
        data-testid="database-storage-family-select"
      />

      <DebouncedInput
        label="Database vCPU Cores"
        type="number"
        min={1}
        max={128}
        value={params.vcpu}
        onChange={(val) => setParams({ vcpu: val ? Number(val) : 4 })}
        placeholder="e.g. 4"
        data-testid="database-vcpu-input"
      />

      <DebouncedInput
        label="Database Memory (RAM GB)"
        type="number"
        min={1}
        max={512}
        value={params.ram_gb}
        onChange={(val) => setParams({ ram_gb: val ? Number(val) : 16 })}
        placeholder="e.g. 16"
        data-testid="database-ram-input"
      />

      <DebouncedInput
        label="Storage Capacity (GB)"
        type="number"
        min={10}
        max={1000000}
        value={params.storage_gb}
        onChange={(val) => setParams({ storage_gb: val ? Number(val) : 100 })}
        placeholder="e.g. 100"
        data-testid="database-storage-input"
      />

      <DebouncedInput
        label="Provisioned IOPS"
        type="number"
        min={100}
        max={256000}
        value={params.iops}
        onChange={(val) => setParams({ iops: val ? Number(val) : 3000 })}
        placeholder="e.g. 3000"
        data-testid="database-iops-input"
      />

      <div className="flex items-center justify-between pt-1">
        <label htmlFor="multi-az-toggle" className="text-xs font-mono text-text-secondary cursor-pointer">
          Multi-AZ / High Availability
        </label>
        <input
          id="multi-az-toggle"
          type="checkbox"
          checked={params.multi_az === true || params.multi_az === 'true'}
          onChange={(e) => setParams({ multi_az: e.target.checked })}
          className="h-4 w-4 rounded border-border-default bg-surface-raised accent-brand-500 cursor-pointer"
          data-testid="database-multiaz-checkbox"
        />
      </div>
    </>
  );

  return (
    <CompareTemplate<ComparisonResultRow>
      categoryTitle="Relational Database (RDBMS) Pricing Comparison"
      categoryDescription="Compare managed relational database instances (compute + storage join) across PostgreSQL, MySQL, and SQL Server."
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
        const engine = (spec?.engine as string | undefined) ?? (row.engine as string | undefined) ?? params.engine;
        const vcpu = (spec?.vcpu as number | undefined) ?? (row.vcpu as number | undefined) ?? params.vcpu;
        const ram = (spec?.ram_gb as number | undefined) ?? (row.ram_gb as number | undefined) ?? params.ram_gb;
        const storageGb = (spec?.storage_gb as number | undefined) ?? (row.storage_gb as number | undefined) ?? params.storage_gb;
        const iops = (spec?.iops as number | undefined) ?? (row.iops as number | undefined) ?? params.iops;
        const multiAz = (spec?.multi_az as boolean | undefined) ?? (row.multi_az as boolean | undefined);
        return (
          <div className="space-y-0.5">
            <div className="font-mono text-text-primary font-medium flex items-center space-x-2">
              <span>{engine ? String(engine).toUpperCase() : 'RDBMS'}</span>
              {row.instance_type && (
                <span className="text-xs text-text-secondary">({row.instance_type})</span>
              )}
              {multiAz && (
                <span className="text-[10px] uppercase px-1.5 py-0.5 rounded border border-border-accent text-border-accent font-bold">
                  Multi-AZ
                </span>
              )}
            </div>
            <div className="text-[11px] text-text-secondary">
              {vcpu} vCPU • {ram} GB RAM • {storageGb} GB ({iops} IOPS)
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
