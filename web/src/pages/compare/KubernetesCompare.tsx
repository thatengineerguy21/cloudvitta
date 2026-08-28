// web/src/pages/compare/KubernetesCompare.tsx
import React, { useMemo } from 'react';
import { useUrlParams } from '../../hooks/useUrlParams';
import { useKubernetesComparison } from '../../api/queries/useComparisonQueries';
import { CompareTemplate, ComparisonResultRow } from '../../components/compare/CompareTemplate';
import {
  KubernetesTierSelect,
  ClusterTopologySelect,
} from '../../components/forms/taxonomy/TaxonomySelects';
import { MissingAttributesIndicator } from '../../components/honesty/MissingAttributesIndicator';
import type { KubernetesQueryParams } from '../../types/api';

const DEFAULT_KUBERNETES_PARAMS: KubernetesQueryParams = {
  region: 'us-east',
  currency: 'USD',
  tier: 'standard',
  cluster_topology: 'zonal',
};

export const KubernetesCompare: React.FC = () => {
  const [params, setParams] = useUrlParams<KubernetesQueryParams>(DEFAULT_KUBERNETES_PARAMS);

  const { data, isLoading, isError, error, refetch } = useKubernetesComparison(params);

  const querySummary = useMemo(() => {
    return `${params.tier} Tier • ${params.cluster_topology} • ${params.region} • ${params.currency}`;
  }, [params]);

  const handleReset = () => {
    setParams(DEFAULT_KUBERNETES_PARAMS, { replace: false });
  };

  const sidebarControls = (
    <>
      <KubernetesTierSelect
        value={params.tier || 'standard'}
        onChange={(e) => setParams({ tier: e.target.value })}
        data-testid="kubernetes-tier-select"
      />

      <ClusterTopologySelect
        value={params.cluster_topology || 'zonal'}
        onChange={(e) => setParams({ cluster_topology: e.target.value })}
        data-testid="kubernetes-topology-select"
      />
    </>
  );

  return (
    <CompareTemplate<ComparisonResultRow>
      categoryTitle="Kubernetes Control-Plane Pricing Comparison"
      categoryDescription="Compare managed Kubernetes control-plane hourly fees (EKS, AKS, GKE) with conditional zone/credit adjustments."
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
        const tier = (spec?.tier as string | undefined) ?? (row.tier as string | undefined) ?? params.tier;
        const topology = (spec?.cluster_topology as string | undefined) ?? (row.cluster_topology as string | undefined) ?? params.cluster_topology;
        const credit = row.credit_applied_hourly_usd ?? (spec?.credit_applied_hourly_usd as string | number | undefined);
        return (
          <div className="space-y-0.5">
            <div className="font-mono text-text-primary font-medium">
              {tier ? `${String(tier).replace('_', ' ').toUpperCase()} Tier` : 'Managed Control Plane'}
            </div>
            <div className="text-[11px] text-text-secondary">
              {topology ? `${String(topology).toUpperCase()} topology` : 'Zonal Master'}
              {credit && Number(credit) > 0 && (
                <span className="ml-1.5 text-status-matchExact font-semibold">
                  (GKE Credit: ${String(credit)}/hr)
                </span>
              )}
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
