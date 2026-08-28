// web/src/pages/compare/ServerlessCompare.tsx
import React, { useMemo } from 'react';
import { useUrlParams } from '../../hooks/useUrlParams';
import { useServerlessComparison } from '../../api/queries/useComparisonQueries';
import { CompareTemplate, ComparisonResultRow } from '../../components/compare/CompareTemplate';
import { DebouncedInput } from '../../components/forms/DebouncedInput';
import { ServerlessArchSelect, ServerlessTierSelect } from '../../components/forms/taxonomy/TaxonomySelects';
import { MissingAttributesIndicator } from '../../components/honesty/MissingAttributesIndicator';
import type { ServerlessQueryParams } from '../../types/api';

const DEFAULT_SERVERLESS_PARAMS: ServerlessQueryParams = {
  region: 'us-east',
  currency: 'USD',
  architecture: 'x86_64',
  tier: 'consumption',
  requests_per_month: 1000000,
  memory_mb: 512,
  execution_duration_ms: 200,
};

export const ServerlessCompare: React.FC = () => {
  const [params, setParams] = useUrlParams<ServerlessQueryParams>(DEFAULT_SERVERLESS_PARAMS);

  const { data, isLoading, isError, error, refetch } = useServerlessComparison(params);

  const querySummary = useMemo(() => {
    return `${params.architecture} • ${params.requests_per_month} req/mo • ${params.memory_mb} MB • ${params.execution_duration_ms} ms`;
  }, [params]);

  const handleReset = () => {
    setParams(DEFAULT_SERVERLESS_PARAMS, { replace: false });
  };

  const sidebarControls = (
    <>
      <ServerlessArchSelect
        value={params.architecture || 'x86_64'}
        onChange={(e) => setParams({ architecture: e.target.value })}
        data-testid="serverless-arch-select"
      />

      <ServerlessTierSelect
        value={params.tier || 'consumption'}
        onChange={(e) => setParams({ tier: e.target.value })}
        data-testid="serverless-tier-select"
      />

      <DebouncedInput
        label="Monthly Requests"
        type="number"
        min={1}
        max={1000000000}
        value={params.requests_per_month}
        onChange={(val) => setParams({ requests_per_month: val ? Number(val) : 1000000 })}
        placeholder="e.g. 1000000"
        data-testid="serverless-requests-input"
      />

      <DebouncedInput
        label="Allocated Memory (MB)"
        type="number"
        min={128}
        max={10240}
        value={params.memory_mb}
        onChange={(val) => setParams({ memory_mb: val ? Number(val) : 512 })}
        placeholder="e.g. 512"
        data-testid="serverless-memory-input"
      />

      <DebouncedInput
        label="Avg Execution Duration (ms)"
        type="number"
        min={1}
        max={900000}
        value={params.execution_duration_ms}
        onChange={(val) => setParams({ execution_duration_ms: val ? Number(val) : 200 })}
        placeholder="e.g. 200"
        data-testid="serverless-duration-input"
      />
    </>
  );

  return (
    <CompareTemplate<ComparisonResultRow>
      categoryTitle="Serverless Compute (FaaS) Pricing Comparison"
      categoryDescription="Compare event-driven serverless function execution and invocation costs across AWS Lambda, Azure Functions, and GCP Cloud Functions."
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
        const arch = (spec?.architecture as string | undefined) ?? (row.architecture as string | undefined) ?? params.architecture;
        const tier = (spec?.tier as string | undefined) ?? (row.tier as string | undefined) ?? params.tier;
        const reqs = (spec?.requests_per_month as number | undefined) ?? (row.requests_per_month as number | undefined) ?? params.requests_per_month;
        const mem = (spec?.memory_mb as number | undefined) ?? (row.memory_mb as number | undefined) ?? params.memory_mb;
        const duration = (spec?.execution_duration_ms as number | undefined) ?? (row.execution_duration_ms as number | undefined) ?? params.execution_duration_ms;
        return (
          <div className="space-y-0.5">
            <div className="font-mono text-text-primary font-medium flex items-center space-x-2">
              <span>{arch ? String(arch) : 'x86_64'}</span>
              {tier && (
                <span className="text-xs text-text-secondary">({String(tier)})</span>
              )}
            </div>
            <div className="text-[11px] text-text-secondary">
              {reqs} req/mo • {mem} MB • {duration} ms
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
