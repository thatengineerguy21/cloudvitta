// web/src/components/status/ComputeCatalogSummaryCard.tsx
import React from 'react';
import { BentoCard } from '../bento/BentoCard';
import { useComputeCatalogSummary } from '../../api/queries/useCatalogQueries';
import { Server, Layers } from 'lucide-react';

export const ComputeCatalogSummaryCard: React.FC = () => {
  const { data, isLoading, isError } = useComputeCatalogSummary();

  if (isLoading) {
    return (
      <BentoCard colSpan={12}>
        <div className="animate-pulse space-y-4" data-testid="catalog-summary-skeleton">
          <div className="h-4 bg-border-default/40 w-1/4" />
          <div className="h-8 bg-border-default/20 w-1/2" />
        </div>
      </BentoCard>
    );
  }

  if (isError || !data) {
    return null;
  }

  const providerTotals = data.provider_totals || {};
  const categoryBreakdown = data.category_breakdown || {};

  return (
    <BentoCard colSpan={12} data-testid="compute-catalog-summary-card">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-border-default pb-4 mb-4">
        <div>
          <span className="text-xs uppercase tracking-widest text-border-accent font-bold flex items-center gap-1.5">
            <Server className="w-3.5 h-3.5 text-border-accent" />
            Compute Hardware Catalog Inventory
          </span>
          <h2 className="font-display text-2xl font-medium text-text-primary mt-1">
            Registered Virtual Machine Specifications
          </h2>
          <p className="text-xs text-text-secondary mt-0.5">
            Slowly changing hardware dimensions normalized across cloud providers for instant specification search.
          </p>
        </div>

        <div className="flex items-baseline gap-2 bg-bg-surface px-4 py-2 border border-border-default rounded-xl">
          <span className="text-xs uppercase text-text-secondary font-mono">Total Instances</span>
          <span className="font-mono text-2xl font-bold text-text-primary" data-testid="catalog-total-instances">
            {data.total_instances?.toLocaleString() ?? 0}
          </span>
        </div>
      </div>

      {/* Provider Totals Badges */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-7 gap-3 mb-6">
        {Object.entries(providerTotals).map(([provider, count]) => (
          <div
            key={provider}
            className="p-3 border border-border-default bg-bg-surface rounded-xl flex flex-col justify-between"
            data-testid={`catalog-provider-total-${provider}`}
          >
            <span className="text-xs font-mono uppercase text-text-secondary">{provider}</span>
            <span className="font-mono text-lg font-bold text-text-primary mt-1">
              {count.toLocaleString()}
            </span>
          </div>
        ))}
      </div>

      {/* Category Breakdown Table */}
      {Object.keys(categoryBreakdown).length > 0 && (
        <div className="space-y-2">
          <span className="text-xs uppercase tracking-wider text-text-secondary font-bold flex items-center gap-1.5">
            <Layers className="w-3.5 h-3.5 text-text-secondary" />
            Category Distribution by Provider
          </span>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs font-mono border-collapse" data-testid="catalog-category-table">
              <thead>
                <tr className="border-b border-border-default text-text-secondary">
                  <th className="py-2 pr-4 font-semibold uppercase">Provider</th>
                  <th className="py-2 px-3 font-semibold uppercase">General Purpose</th>
                  <th className="py-2 px-3 font-semibold uppercase">Compute Optimized</th>
                  <th className="py-2 px-3 font-semibold uppercase">Memory Optimized</th>
                  <th className="py-2 px-3 font-semibold uppercase">GPU Accelerated</th>
                  <th className="py-2 pl-3 font-semibold uppercase">Storage Optimized</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-default/40">
                {Object.entries(categoryBreakdown).map(([provider, categories]) => (
                  <tr key={provider} className="hover:bg-bg-surface/50">
                    <td className="py-2.5 pr-4 font-bold text-text-primary uppercase">{provider}</td>
                    <td className="py-2.5 px-3 text-text-secondary">
                      {categories['general_purpose'] ?? 0}
                    </td>
                    <td className="py-2.5 px-3 text-text-secondary">
                      {categories['compute_optimized'] ?? 0}
                    </td>
                    <td className="py-2.5 px-3 text-text-secondary">
                      {categories['memory_optimized'] ?? 0}
                    </td>
                    <td className="py-2.5 px-3 text-text-secondary">
                      {categories['gpu_accelerated'] ?? 0}
                    </td>
                    <td className="py-2.5 pl-3 text-text-secondary">
                      {categories['storage_optimized'] ?? 0}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </BentoCard>
  );
};
