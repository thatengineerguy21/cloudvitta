// web/src/pages/compare/ComputeCompare.tsx
import React, { useMemo } from 'react';
import { useUrlParams } from '../../hooks/useUrlParams';
import { useComputeComparison } from '../../api/queries/useComparisonQueries';
import { CompareTemplate, ComparisonResultRow } from '../../components/compare/CompareTemplate';
import { DebouncedInput } from '../../components/forms/DebouncedInput';
import { InstanceFamilySelect } from '../../components/forms/taxonomy/TaxonomySelects';
import { MissingAttributesIndicator } from '../../components/honesty/MissingAttributesIndicator';
import type { ComputeQueryParams } from '../../types/api';

const DEFAULT_COMPUTE_PARAMS: ComputeQueryParams = {
  region: 'us-east',
  currency: 'USD',
  vcpu: 4,
  ram_gb: 16,
  family: '',
  strict_family: true,
};

export const ComputeCompare: React.FC = () => {
  const [params, setParams] = useUrlParams<ComputeQueryParams>(DEFAULT_COMPUTE_PARAMS);

  const { data, isLoading, isError, error, refetch } = useComputeComparison(params);

  const querySummary = useMemo(() => {
    return `${params.vcpu} vCPU • ${params.ram_gb} GB RAM • ${params.region} • ${params.currency}`;
  }, [params]);

  const handleReset = () => {
    setParams(DEFAULT_COMPUTE_PARAMS, { replace: false });
  };

  const sidebarControls = (
    <>
      <DebouncedInput
        label="vCPU Cores"
        type="number"
        min={1}
        max={256}
        value={params.vcpu}
        onChange={(val) => setParams({ vcpu: val ? Number(val) : 4 })}
        placeholder="e.g. 4"
        data-testid="compute-vcpu-input"
      />

      <DebouncedInput
        label="Memory (RAM GB)"
        type="number"
        min={1}
        max={1024}
        value={params.ram_gb}
        onChange={(val) => setParams({ ram_gb: val ? Number(val) : 16 })}
        placeholder="e.g. 16"
        data-testid="compute-ram-input"
      />

      <InstanceFamilySelect
        value={params.family || ''}
        onChange={(e) => setParams({ family: e.target.value })}
        data-testid="compute-family-select"
      />

      <div className="flex items-center justify-between pt-1">
        <label htmlFor="strict-family-toggle" className="text-xs font-mono text-text-secondary cursor-pointer">
          Strict Family Match
        </label>
        <input
          id="strict-family-toggle"
          type="checkbox"
          checked={params.strict_family === true || params.strict_family === 'true'}
          onChange={(e) => setParams({ strict_family: e.target.checked })}
          className="h-4 w-4 rounded-none border-border-default bg-surface-raised accent-border-accent cursor-pointer"
          data-testid="compute-strict-family-checkbox"
        />
      </div>
    </>
  );

  return (
    <CompareTemplate<ComparisonResultRow>
      categoryTitle="Compute Pricing Comparison"
      categoryDescription="Compare virtual machine instance pricing normalized per hour across 7 cloud providers."
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
        const vcpu = (spec?.vcpu as number | undefined) ?? (row.vcpu as number | undefined) ?? params.vcpu;
        const ram = (spec?.ram_gb as number | undefined) ?? (row.ram_gb as number | undefined) ?? params.ram_gb;
        const fam = (spec?.family as string | undefined) ?? (row.family as string | undefined);
        return (
          <div className="space-y-0.5">
            <div className="font-mono text-text-primary font-medium">
              {row.instance_type || fam || 'Standard VM'}
            </div>
            <div className="text-[11px] text-text-secondary">
              {vcpu} vCPU • {ram} GB RAM
              {fam ? ` • ${fam}` : ''}
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
